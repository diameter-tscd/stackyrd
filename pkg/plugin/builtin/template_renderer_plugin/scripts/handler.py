"""Template Renderer Plugin.

Renders Python string templates with variable substitution, validation,
and introspection.

Modes:
  render (default) — Substitute variables into a template string.
  validate — Check a template for undefined variables and syntax errors.
  list_vars — Extract all variable placeholders from a template.
"""

import re
import string

from sdk import Plugin


class TemplateRendererPlugin(Plugin):
    MODES = ("render", "validate", "list_vars")

    # Pattern to find ${var} or $var placeholders
    _SIMPLE_VAR_PATTERN = re.compile(r"\$\{([^}]+)\}|\$([a-zA-Z_][a-zA-Z0-9_.]*)")

    def execute(self, args):
        mode = args.get("mode", "render")
        if mode not in self.MODES:
            return {
                "success": False,
                "error": f"Unknown mode '{mode}'. Choose from: {', '.join(self.MODES)}",
            }
        handler = getattr(self, f"_{mode}")
        result = handler(args)
        return {"success": True, "data": result}

    # ── render mode ─────────────────────────────────────────────────

    def _render(self, args):
        template_str = args.get("template", "")
        variables = args.get("variables", {})
        engine = args.get("engine", "string_template")

        if not template_str:
            return {"error": "No template provided"}

        if engine == "string_template":
            result = self._render_string_template(template_str, variables)
        elif engine == "format":
            result = self._render_format(template_str, variables)
        elif engine == "percent":
            result = self._render_percent(template_str, variables)
        else:
            return {"error": f"Unknown engine: {engine}"}

        return {
            "rendered": result,
            "engine": engine,
            "variable_count": len(variables),
            "template_length": len(template_str),
            "result_length": len(result),
        }

    def _render_string_template(self, template_str, variables):
        class SafeTemplate(string.Template):
            delimiter = "$"
            idpattern = r"[a-zA-Z_][a-zA-Z0-9_.]*"

        # Pre-process: replace ${foo.bar} with the resolved nested value
        def resolve_nested(match):
            var_name = match.group(1) or match.group(2)
            parts = var_name.split(".")
            val = variables
            for part in parts:
                if isinstance(val, dict):
                    val = val.get(part, "")
                else:
                    return ""
            if val is None:
                return ""
            if isinstance(val, (dict, list)):
                import json
                return json.dumps(val)
            return str(val)

        # First pass: resolve dotted variables
        interpolated = self._SIMPLE_VAR_PATTERN.sub(resolve_nested, template_str)

        # Second pass: resolve simple ${var} with string.Template for safety
        try:
            tmpl = SafeTemplate(interpolated)
            # Only pass top-level string variables to string.Template
            safe_vars = {
                k: v for k, v in variables.items()
                if isinstance(v, (str, int, float, bool)) or v is None
            }
            result = tmpl.safe_substitute(safe_vars)
            return result
        except (ValueError, KeyError) as e:
            # If safe_substitute leaves placeholders, return as-is
            return interpolated

    def _render_format(self, template_str, variables):
        try:
            return template_str.format(**variables)
        except KeyError as e:
            return template_str
        except ValueError as e:
            return f"<format error: {e}>"

    def _render_percent(self, template_str, variables):
        try:
            return template_str % variables
        except (KeyError, TypeError, ValueError) as e:
            return f"<percent formatting error: {e}>"

    # ── validate mode ───────────────────────────────────────────────

    def _validate(self, args):
        template_str = args.get("template", "")
        variables = args.get("variables", {})
        engine = args.get("engine", "string_template")

        if not template_str:
            return {"error": "No template provided"}

        issues = []

        # Check for Python string.Template syntax
        tmpl = string.Template(template_str)
        try:
            tmpl.substitute({})
        except KeyError as e:
            pass  # Expected — undefined vars are validation info, not errors
        except ValueError as e:
            issues.append({"severity": "error", "message": f"Syntax error: {e}"})

        # Find all variable placeholders
        var_names = self._extract_vars(template_str)
        undefined = [v for v in var_names if v not in variables]

        if undefined:
            issues.append({
                "severity": "warning",
                "message": f"Undefined variables: {', '.join(undefined)}",
                "undefined_vars": undefined,
            })

        unused = [v for v in variables if v not in var_names]
        if unused:
            issues.append({
                "severity": "info",
                "message": f"Unused variables: {', '.join(unused)}",
                "unused_vars": unused,
            })

        # Dry-run render to detect runtime errors
        try:
            if engine == "string_template":
                self._render_string_template(template_str, variables)
            elif engine == "format":
                self._render_format(template_str, variables)
            elif engine == "percent":
                self._render_percent(template_str, variables)
        except Exception as e:
            issues.append({
                "severity": "error",
                "message": f"Render dry-run failed: {e}",
            })

        return {
            "valid": len([i for i in issues if i["severity"] == "error"]) == 0,
            "issues": issues,
            "variable_count": len(var_names),
            "provided_variable_count": len(variables),
            "undefined_count": len(undefined),
            "unused_count": len(unused),
        }

    # ── list_vars mode ──────────────────────────────────────────────

    def _list_vars(self, args):
        template_str = args.get("template", "")
        if not template_str:
            return {"error": "No template provided"}

        var_names = self._extract_vars(template_str)
        var_info = []
        for var_name in var_names:
            parts = var_name.split(".")
            is_nested = len(parts) > 1
            top_level = parts[0]
            var_info.append({
                "name": var_name,
                "top_level": top_level,
                "is_nested": is_nested,
                "depth": len(parts),
            })

        top_level_vars = sorted(set(v["top_level"] for v in var_info))
        return {
            "variables": var_info,
            "total_count": len(var_info),
            "unique_top_level": top_level_vars,
            "unique_top_level_count": len(top_level_vars),
        }

    # ── helpers ─────────────────────────────────────────────────────

    def _extract_vars(self, template_str):
        names = []
        seen = set()
        for match in self._SIMPLE_VAR_PATTERN.finditer(template_str):
            name = match.group(1) or match.group(2)
            if name not in seen:
                seen.add(name)
                names.append(name)
        return names
