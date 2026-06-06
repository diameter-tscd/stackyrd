"""Schema Validator Plugin.

Validates and coerces JSON/dict structures against a type rules schema.
Supports nested validation, type coercion, custom constraints, and
schema description/introspection.

Modes:
  validate (default) — Validate data against a schema, return errors.
  coerce — Validate AND coerce values to expected types.
  describe — Describe a schema in human-readable form.
"""

import datetime
import re

from sdk import Plugin


class SchemaValidatorPlugin(Plugin):
    MODES = ("validate", "coerce", "describe")

    def execute(self, args):
        mode = args.get("mode", "validate")
        if mode not in self.MODES:
            return {
                "success": False,
                "error": f"Unknown mode '{mode}'. Choose from: {', '.join(self.MODES)}",
            }
        handler = getattr(self, f"_{mode}")
        result = handler(args)
        return {"success": True, "data": result}

    # ── validate mode ───────────────────────────────────────────────

    def _validate(self, args):
        data = args.get("data", {})
        schema = args.get("schema", {})
        strict = args.get("strict", False)

        if not schema:
            return {"error": "No schema provided"}
        if data is None:
            return {"error": "No data provided"}

        errors = []
        coerced = {}

        if strict:
            schema_keys = set(self._flatten_schema_keys(schema))
            data_keys = set(self._flatten_data_keys(data))
            extra = data_keys - schema_keys
            for key in sorted(extra):
                errors.append({
                    "path": key,
                    "rule": "strict",
                    "message": f"Unexpected field '{key}' (strict mode)",
                })

        self._validate_node(data, schema, errors, coerced, path="")

        return {
            "valid": len(errors) == 0,
            "errors": errors if errors else None,
            "error_count": len(errors),
            "total_fields_validated": len(self._flatten_data_keys(data)),
        }

    def _validate_node(self, data, schema, errors, coerced, path):
        for field, rules in schema.items():
            current_path = f"{path}.{field}" if path else field
            value = self._resolve(data, field)

            required = rules.get("required", False)
            type_name = rules.get("type", "any")

            if value is None and not required:
                continue

            if value is None and required:
                errors.append({
                    "path": current_path,
                    "rule": "required",
                    "message": f"Required field '{current_path}' is missing",
                })
                continue

            if type_name != "any":
                type_ok, type_error = self._check_type(value, type_name, rules)
                if not type_ok:
                    errors.append({
                        "path": current_path,
                        "rule": "type",
                        "expected": type_name,
                        "actual": type(value).__name__,
                        "message": type_error,
                    })
                    continue

            for constraint_name, constraint_value in rules.items():
                if constraint_name in ("type", "required", "description", "default", "properties"):
                    continue
                if constraint_name == "items" and type_name == "array":
                    self._validate_array_items(
                        value, constraint_value, errors, current_path
                    )
                    continue
                if constraint_name == "properties" and type_name == "object":
                    self._validate_node(
                        value, constraint_value, errors, coerced, current_path
                    )
                    continue
                constraint_ok, constraint_msg = self._check_constraint(
                    value, constraint_name, constraint_value, rules
                )
                if not constraint_ok:
                    errors.append({
                        "path": current_path,
                        "rule": constraint_name,
                        "value": constraint_value,
                        "message": constraint_msg,
                    })

    def _check_type(self, value, type_name, rules):
        if type_name == "string":
            if not isinstance(value, str):
                return False, f"Expected string, got {type(value).__name__}"
        elif type_name == "integer":
            if isinstance(value, bool):
                return False, "Expected integer, got boolean"
            if not isinstance(value, int):
                return False, f"Expected integer, got {type(value).__name__}"
        elif type_name == "number":
            if isinstance(value, bool):
                return False, "Expected number, got boolean"
            if not isinstance(value, (int, float)):
                return False, f"Expected number, got {type(value).__name__}"
        elif type_name == "boolean":
            if not isinstance(value, bool):
                return False, f"Expected boolean, got {type(value).__name__}"
        elif type_name == "array":
            if not isinstance(value, list):
                return False, f"Expected array, got {type(value).__name__}"
        elif type_name == "object":
            if not isinstance(value, dict):
                return False, f"Expected object, got {type(value).__name__}"
        elif type_name == "null":
            if value is not None:
                return False, f"Expected null, got {type(value).__name__}"
        elif type_name == "any":
            pass
        elif type_name == "email":
            if not isinstance(value, str) or not re.match(
                r"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$", value
            ):
                return False, f"Invalid email: {value}"
        elif type_name == "url":
            if not isinstance(value, str) or not re.match(
                r"^https?://\S+", value
            ):
                return False, f"Invalid URL: {value}"
        elif type_name == "date":
            if isinstance(value, str):
                try:
                    datetime.datetime.fromisoformat(value)
                except ValueError:
                    return False, f"Invalid date string: {value}"
            elif not isinstance(value, datetime.datetime):
                return False, f"Expected date, got {type(value).__name__}"
        elif type_name == "ipv4":
            if not isinstance(value, str) or not re.match(
                r"^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$", value
            ):
                return False, f"Invalid IPv4: {value}"
        else:
            return False, f"Unknown type: {type_name}"
        return True, None

    def _check_constraint(self, value, constraint_name, constraint_value, rules):
        if constraint_name == "min_length":
            if isinstance(value, (str, list)):
                if len(value) < constraint_value:
                    return False, f"Minimum length is {constraint_value}, got {len(value)}"
        elif constraint_name == "max_length":
            if isinstance(value, (str, list)):
                if len(value) > constraint_value:
                    return False, f"Maximum length is {constraint_value}, got {len(value)}"
        elif constraint_name == "minimum":
            if isinstance(value, (int, float)) and value < constraint_value:
                return False, f"Minimum value is {constraint_value}, got {value}"
        elif constraint_name == "maximum":
            if isinstance(value, (int, float)) and value > constraint_value:
                return False, f"Maximum value is {constraint_value}, got {value}"
        elif constraint_name == "exclusive_minimum":
            if isinstance(value, (int, float)) and value <= constraint_value:
                return False, f"Value must be > {constraint_value}, got {value}"
        elif constraint_name == "exclusive_maximum":
            if isinstance(value, (int, float)) and value >= constraint_value:
                return False, f"Value must be < {constraint_value}, got {value}"
        elif constraint_name == "pattern":
            if isinstance(value, str) and not re.match(constraint_value, value):
                return False, f"Does not match pattern {constraint_value}: {value}"
        elif constraint_name == "enum":
            if value not in constraint_value:
                return False, f"Value '{value}' not in enum {constraint_value}"
        elif constraint_name == "min_items":
            if isinstance(value, list) and len(value) < constraint_value:
                return False, f"Minimum items is {constraint_value}, got {len(value)}"
        elif constraint_name == "max_items":
            if isinstance(value, list) and len(value) > constraint_value:
                return False, f"Maximum items is {constraint_value}, got {len(value)}"
        return True, None

    def _validate_array_items(self, items, item_schema, errors, path):
        for i, item in enumerate(items):
            item_path = f"{path}[{i}]"
            if isinstance(item_schema, dict):
                self._validate_node(
                    item, item_schema, errors, {}, item_path
                )

    # ── coerce mode ─────────────────────────────────────────────────

    def _coerce(self, args):
        data = args.get("data", {})
        schema = args.get("schema", {})

        if not schema:
            return {"error": "No schema provided"}

        errors = []
        coerced = {}

        self._coerce_node(data, schema, errors, coerced, path="")

        return {
            "valid": len(errors) == 0,
            "coerced": coerced,
            "errors": errors if errors else None,
            "error_count": len(errors),
        }

    def _coerce_node(self, data, schema, errors, coerced, path):
        for field, rules in schema.items():
            current_path = f"{path}.{field}" if path else field
            value = self._resolve(data, field)

            required = rules.get("required", False)
            default = rules.get("default")
            type_name = rules.get("type", "any")

            if value is None and not required:
                if default is not None:
                    self._set_path(coerced, current_path, default)
                continue

            if value is None and required:
                if default is not None:
                    self._set_path(coerced, current_path, default)
                    continue
                errors.append({
                    "path": current_path,
                    "rule": "required",
                    "message": f"Required field '{current_path}' is missing",
                })
                continue

            if type_name != "any":
                coercions = {
                    "string": str,
                    "integer": lambda v: int(v) if not isinstance(v, bool) and v is not None else v,
                    "number": float,
                    "boolean": lambda v: (
                        v if isinstance(v, bool)
                        else str(v).lower() in ("true", "1", "yes") if v is not None
                        else v
                    ),
                }
                coerce_fn = coercions.get(type_name)
                if coerce_fn:
                    try:
                        value = coerce_fn(value)
                    except (ValueError, TypeError) as e:
                        errors.append({
                            "path": current_path,
                            "rule": "coerce",
                            "expected": type_name,
                            "message": f"Cannot coerce {type(value).__name__} to {type_name}: {e}",
                        })
                        continue

            type_ok, type_error = self._check_type(value, type_name, rules)
            if not type_ok:
                errors.append({
                    "path": current_path,
                    "rule": "type",
                    "expected": type_name,
                    "message": type_error,
                })
                continue

            for constraint_name, constraint_value in rules.items():
                if constraint_name in ("type", "required", "description", "default", "properties"):
                    continue
                if constraint_name == "properties" and type_name == "object":
                    nested = {}
                    self._coerce_node(
                        value, constraint_value, errors, nested, current_path
                    )
                    self._set_path(coerced, current_path, nested)
                    continue
                constraint_ok, constraint_msg = self._check_constraint(
                    value, constraint_name, constraint_value, rules
                )
                if not constraint_ok:
                    errors.append({
                        "path": current_path,
                        "rule": constraint_name,
                        "value": constraint_value,
                        "message": constraint_msg,
                    })

            self._set_path(coerced, current_path, value)

    # ── describe mode ───────────────────────────────────────────────

    def _describe(self, args):
        schema = args.get("schema", {})
        if not schema:
            return {"error": "No schema provided"}

        description = self._describe_node(schema, indent=0)
        field_count = len(self._flatten_schema_fields(schema))

        return {
            "description": description,
            "field_count": field_count,
            "schema": schema,
        }

    def _describe_node(self, schema, indent=0):
        lines = []
        prefix = "  " * indent
        for field, rules in sorted(schema.items()):
            type_name = rules.get("type", "any")
            required = rules.get("required", False)
            desc = rules.get("description", "")
            req_mark = "required" if required else "optional"

            constraints = []
            for key in ("minimum", "maximum", "min_length", "max_length",
                        "pattern", "enum", "min_items", "max_items",
                        "exclusive_minimum", "exclusive_maximum"):
                if key in rules:
                    constraints.append(f"{key}={rules[key]}")

            constraint_str = f" [{', '.join(constraints)}]" if constraints else ""
            desc_str = f" — {desc}" if desc else ""

            line = f"{prefix}{field}: {type_name} ({req_mark}){constraint_str}{desc_str}"
            lines.append(line)

            if "properties" in rules:
                lines.append(self._describe_node(rules["properties"], indent + 1))

        return "\n".join(lines)

    def _flatten_schema_fields(self, schema, prefix=""):
        fields = {}
        for field, rules in schema.items():
            path = f"{prefix}.{field}" if prefix else field
            fields[path] = rules
            if "properties" in rules:
                fields.update(self._flatten_schema_fields(rules["properties"], path))
        return fields

    # ── helpers ─────────────────────────────────────────────────────

    def _flatten_schema_keys(self, schema, prefix=""):
        keys = set()
        for field in schema:
            path = f"{prefix}.{field}" if prefix else field
            keys.add(path)
            if "properties" in schema[field]:
                keys.update(self._flatten_schema_keys(
                    schema[field]["properties"], path
                ))
        return keys

    def _flatten_data_keys(self, data, prefix=""):
        keys = set()
        if isinstance(data, dict):
            for field, value in data.items():
                path = f"{prefix}.{field}" if prefix else field
                keys.add(path)
                if isinstance(value, dict):
                    keys.update(self._flatten_data_keys(value, path))
        return keys

    def _resolve(self, obj, path):
        parts = path.split(".")
        current = obj
        for part in parts:
            if isinstance(current, dict):
                current = current.get(part)
            else:
                return None
            if current is None:
                return None
        return current

    def _set_path(self, obj, path, value):
        parts = path.split(".")
        current = obj
        for part in parts[:-1]:
            if part not in current:
                current[part] = {}
            current = current[part]
        current[parts[-1]] = value
