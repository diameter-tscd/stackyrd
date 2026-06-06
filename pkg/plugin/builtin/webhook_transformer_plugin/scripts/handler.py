"""Webhook Transformer Plugin.

Transforms, filters, enriches, and inspects webhook payloads.

Modes:
  transform (default) — Apply field mappings and transformations to a payload.
  filter — Filter a payload by field value conditions.
  enrich — Add computed/enriched fields to a payload.
  inspect — Introspect a payload structure (keys, types, sizes).
"""

import copy
import datetime
import hashlib
import json
import re
import uuid

from sdk import Plugin


class WebhookTransformerPlugin(Plugin):
    MODES = ("transform", "filter", "enrich", "inspect")

    def execute(self, args):
        mode = args.get("mode", "transform")
        if mode not in self.MODES:
            return {
                "success": False,
                "error": f"Unknown mode '{mode}'. Choose from: {', '.join(self.MODES)}",
            }
        handler = getattr(self, f"_{mode}")
        result = handler(args)
        return {"success": True, "data": result}

    # ── transform mode ──────────────────────────────────────────────

    def _transform(self, args):
        payload = args.get("payload", {})
        mappings = args.get("mappings", [])

        if not payload:
            return {"error": "No payload provided", "original": payload}
        if not mappings:
            return {"error": "No mappings provided", "original": payload}

        result = {}
        warnings = []

        for mapping in mappings:
            source_field = mapping.get("from", "")
            target_field = mapping.get("to", source_field)
            default = mapping.get("default")
            transform_type = mapping.get("transform", "copy")

            value = self._resolve_path(payload, source_field)

            if value is None and default is not None:
                value = default
                warnings.append(f"Field '{source_field}' missing, using default")

            if value is None:
                continue

            try:
                transformed = self._apply_transform(value, transform_type, mapping)
            except (ValueError, TypeError) as e:
                warnings.append(f"Transform '{transform_type}' failed on '{source_field}': {e}")
                transformed = value

            self._set_path(result, target_field, transformed)

        return {
            "transformed": result,
            "warnings": warnings if warnings else None,
            "field_count": len(result),
        }

    def _apply_transform(self, value, transform_type, mapping):
        if transform_type == "copy":
            return value
        elif transform_type == "uppercase":
            return value.upper() if isinstance(value, str) else value
        elif transform_type == "lowercase":
            return value.lower() if isinstance(value, str) else value
        elif transform_type == "trim":
            return value.strip() if isinstance(value, str) else value
        elif transform_type == "split":
            delim = mapping.get("delimiter", ",")
            return value.split(delim) if isinstance(value, str) else [value]
        elif transform_type == "join":
            delim = mapping.get("delimiter", ",")
            if isinstance(value, list):
                return delim.join(str(v) for v in value)
            return str(value)
        elif transform_type == "prefix":
            prefix = mapping.get("prefix", "")
            return prefix + str(value)
        elif transform_type == "suffix":
            suffix = mapping.get("suffix", "")
            return str(value) + suffix
        elif transform_type == "regex_replace":
            pattern = mapping.get("pattern", "")
            replacement = mapping.get("replacement", "")
            if isinstance(value, str) and pattern:
                return re.sub(pattern, replacement, value)
            return value
        elif transform_type == "template":
            template = mapping.get("template", "{value}")
            if isinstance(value, str):
                return template.replace("{value}", value)
            return template.replace("{value}", str(value))
        elif transform_type == "to_int":
            return int(value)
        elif transform_type == "to_float":
            return float(value)
        elif transform_type == "to_string":
            return str(value)
        elif transform_type == "to_bool":
            if isinstance(value, bool):
                return value
            if isinstance(value, str):
                return value.lower() in ("true", "1", "yes", "y")
            return bool(value)
        elif transform_type == "sha256":
            return hashlib.sha256(str(value).encode()).hexdigest()
        elif transform_type == "md5":
            return hashlib.md5(str(value).encode()).hexdigest()
        elif transform_type == "uuid":
            return str(uuid.uuid5(uuid.NAMESPACE_DNS, str(value)))
        else:
            raise ValueError(f"Unknown transform type: {transform_type}")

    # ── filter mode ─────────────────────────────────────────────────

    def _filter(self, args):
        payload = args.get("payload", {})
        conditions = args.get("conditions", [])

        if not payload:
            return {"error": "No payload provided"}
        if not conditions:
            return {"matched": True, "reason": "No conditions — pass-through"}

        logic = args.get("logic", "and").lower()

        results = []
        for condition in conditions:
            field = condition.get("field", "")
            op = condition.get("op", "exists")
            value = condition.get("value")

            actual = self._resolve_path(payload, field)
            matched = self._evaluate_condition(actual, op, value)

            results.append({
                "field": field,
                "op": op,
                "expected": value,
                "actual": actual,
                "matched": matched,
            })

        if logic == "and":
            overall = all(r["matched"] for r in results)
        elif logic == "or":
            overall = any(r["matched"] for r in results)
        elif logic == "nor":
            overall = not any(r["matched"] for r in results)
        elif logic == "nand":
            overall = not all(r["matched"] for r in results)
        elif logic == "xor":
            overall = sum(1 for r in results if r["matched"]) == 1
        else:
            return {"error": f"Unknown logic: {logic}"}

        return {
            "matched": overall,
            "logic": logic,
            "conditions": results,
            "condition_count": len(conditions),
        }

    def _evaluate_condition(self, actual, op, expected):
        if op == "exists":
            return actual is not None
        elif op == "not_exists":
            return actual is None
        elif op == "eq":
            return actual == expected
        elif op == "neq":
            return actual != expected
        elif op == "gt":
            return actual is not None and expected is not None and actual > expected
        elif op == "gte":
            return actual is not None and expected is not None and actual >= expected
        elif op == "lt":
            return actual is not None and expected is not None and actual < expected
        elif op == "lte":
            return actual is not None and expected is not None and actual <= expected
        elif op == "in":
            return isinstance(expected, list) and actual in expected
        elif op == "not_in":
            return isinstance(expected, list) and actual not in expected
        elif op == "contains":
            return isinstance(actual, (str, list)) and expected in actual
        elif op == "starts_with":
            return isinstance(actual, str) and actual.startswith(expected)
        elif op == "ends_with":
            return isinstance(actual, str) and actual.endswith(expected)
        elif op == "matches":
            return isinstance(actual, str) and bool(re.match(expected, actual))
        elif op == "is_type":
            return isinstance(actual, self._parse_type_name(expected))
        elif op == "len_gt":
            return actual is not None and len(actual) > expected
        elif op == "len_lt":
            return actual is not None and len(actual) < expected
        return False

    def _parse_type_name(self, name):
        mapping = {
            "str": str, "string": str,
            "int": int, "integer": int,
            "float": float, "double": float,
            "bool": bool, "boolean": bool,
            "list": list, "array": list,
            "dict": dict, "object": dict,
        }
        return mapping.get(name, object)

    # ── enrich mode ─────────────────────────────────────────────────

    def _enrich(self, args):
        payload = args.get("payload", {})
        enrichments = args.get("enrichments", [])

        result = copy.deepcopy(payload) if isinstance(payload, dict) else dict(payload)
        added = []

        for enrichment in enrichments:
            field = enrichment.get("field", "")
            enrichment_type = enrichment.get("type", "timestamp")
            params = enrichment.get("params", {})

            try:
                value = self._compute_enrichment(enrichment_type, result, params)
                self._set_path(result, field, value)
                added.append({"field": field, "type": enrichment_type})
            except Exception as e:
                self.logger.warn("Enrichment failed", field=field, type=enrichment_type, error=str(e))

        return {
            "enriched": result,
            "added_fields": added,
            "enrichment_count": len(added),
        }

    def _compute_enrichment(self, enrichment_type, payload, params):
        if enrichment_type == "timestamp":
            fmt = params.get("format", "iso")
            now = datetime.datetime.utcnow()
            if fmt == "iso":
                return now.isoformat() + "Z"
            elif fmt == "unix":
                return now.timestamp()
            elif fmt == "unix_ms":
                return int(now.timestamp() * 1000)
            elif fmt == "date":
                return now.strftime(params.get("date_format", "%Y-%m-%d"))
            else:
                return now.strftime(fmt)

        elif enrichment_type == "uuid":
            version = params.get("version", 4)
            if version == 4:
                return str(uuid.uuid4())
            elif version == 1:
                return str(uuid.uuid1())
            else:
                return str(uuid.uuid4())

        elif enrichment_type == "hash":
            field = params.get("field", "")
            algorithm = params.get("algorithm", "sha256")
            value = self._resolve_path(payload, field)
            if value is None:
                raise ValueError(f"Cannot hash: field '{field}' not found")
            data = str(value).encode()
            if algorithm == "sha256":
                return hashlib.sha256(data).hexdigest()
            elif algorithm == "md5":
                return hashlib.md5(data).hexdigest()
            elif algorithm == "sha1":
                return hashlib.sha1(data).hexdigest()
            else:
                raise ValueError(f"Unknown hash algorithm: {algorithm}")

        elif enrichment_type == "count":
            field = params.get("field", "")
            value = self._resolve_path(payload, field)
            if value is None:
                return 0
            return len(value)

        elif enrichment_type == "length":
            field = params.get("field", "")
            value = self._resolve_path(payload, field)
            if value is None:
                return 0
            return len(str(value))

        elif enrichment_type == "extract":
            field = params.get("field", "")
            expression = params.get("expression", "")
            value = self._resolve_path(payload, field)
            if isinstance(value, str) and expression:
                match = re.search(expression, value)
                return match.group(0) if match else None
            return None

        elif enrichment_type == "default":
            field = params.get("field", "")
            default_val = params.get("default", "")
            value = self._resolve_path(payload, field)
            return value if value is not None else default_val

        elif enrichment_type == "merge":
            source = params.get("source", {})
            if isinstance(result_dict := payload, dict):
                merged = copy.deepcopy(result_dict)
                merged.update(source)
                return merged
            return payload

        else:
            raise ValueError(f"Unknown enrichment type: {enrichment_type}")

    # ── inspect mode ────────────────────────────────────────────────

    def _inspect(self, args):
        payload = args.get("payload", {})
        if not payload:
            return {"error": "No payload provided"}

        flat = self._flatten(payload)
        return {
            "structure": self._describe_structure(payload),
            "keys": list(flat.keys()),
            "flat_count": len(flat),
            "keys_by_type": self._keys_by_type(flat),
            "size_bytes": len(json.dumps(payload)),
            "depth": self._max_depth(payload),
        }

    def _describe_structure(self, obj, prefix=""):
        if isinstance(obj, dict):
            return {
                k: self._describe_structure(v, f"{prefix}.{k}" if prefix else k)
                for k, v in obj.items()
            }
        elif isinstance(obj, list):
            if obj:
                return f"list[{len(obj)}] of {type(obj[0]).__name__}"
            return "list[0]"
        else:
            return type(obj).__name__

    def _flatten(self, obj, prefix=""):
        items = {}
        if isinstance(obj, dict):
            for k, v in obj.items():
                path = f"{prefix}.{k}" if prefix else k
                if isinstance(v, (dict, list)):
                    items.update(self._flatten(v, path))
                else:
                    items[path] = v
        elif isinstance(obj, list):
            for i, v in enumerate(obj):
                path = f"{prefix}[{i}]"
                if isinstance(v, (dict, list)):
                    items.update(self._flatten(v, path))
                else:
                    items[path] = v
        return items

    def _keys_by_type(self, flat):
        by_type = {}
        for key, value in flat.items():
            t = type(value).__name__
            if t not in by_type:
                by_type[t] = []
            by_type[t].append(key)
        return by_type

    def _max_depth(self, obj):
        if isinstance(obj, dict):
            if not obj:
                return 1
            return 1 + max(self._max_depth(v) for v in obj.values())
        elif isinstance(obj, list):
            if not obj:
                return 1
            return 1 + max(self._max_depth(v) for v in obj)
        return 1

    # ── helpers ─────────────────────────────────────────────────────

    def _resolve_path(self, obj, path):
        parts = path.replace("[", ".").replace("]", "").split(".")
        current = obj
        for part in parts:
            if not part:
                continue
            if isinstance(current, dict):
                current = current.get(part)
            elif isinstance(current, list) and part.isdigit():
                idx = int(part)
                if 0 <= idx < len(current):
                    current = current[idx]
                else:
                    return None
            else:
                return None
            if current is None:
                return None
        return current

    def _set_path(self, obj, path, value):
        parts = path.replace("[", ".").replace("]", "").split(".")
        current = obj
        for i, part in enumerate(parts[:-1]):
            if not part:
                continue
            if part.isdigit() and isinstance(current, list):
                idx = int(part)
                while len(current) <= idx:
                    current.append({})
                current = current[idx]
            else:
                if part not in current:
                    current[part] = {}
                current = current[part]
        last = parts[-1]
        if last:
            current[last] = value
