"""Data Processor Plugin.

Aggregates, filters, sorts, and computes statistics on data arrays.

Modes:
  aggregate (default) — Group records by key and compute aggregations.
  filter — Filter records by compound conditions.
  sort — Sort records by one or more fields.
  stats — Statistical summary of a numeric field.
  batch — Split a large dataset into smaller batches.
"""

import copy
import statistics
import math

from sdk import Plugin


class DataProcessorPlugin(Plugin):
    MODES = ("aggregate", "filter", "sort", "stats", "batch")

    def setup(self, args):
        self.data = args.get("data", [])
        if not isinstance(self.data, list):
            raise ValueError("'data' must be a list of records")

    def execute(self, args):
        mode = args.get("mode", "aggregate")
        if mode not in self.MODES:
            return {
                "success": False,
                "error": f"Unknown mode '{mode}'. Choose from: {', '.join(self.MODES)}",
            }
        handler = getattr(self, f"_{mode}")
        return {"success": True, "data": handler(args)}

    # ── aggregate mode ──────────────────────────────────────────────

    def _aggregate(self, args):
        group_by = args.get("group_by", [])
        aggregations = args.get("aggregations", [])
        if not group_by:
            return {"error": "'group_by' is required for aggregate mode"}
        if not aggregations:
            return {"error": "'aggregations' is required for aggregate mode"}

        groups = {}
        for record in self.data:
            key = tuple(
                self._resolve(record, field) for field in group_by
            )
            if key not in groups:
                groups[key] = []
            groups[key].append(record)

        results = []
        for key, records in groups.items():
            group_key = {
                field: val for field, val in zip(group_by, key)
            }
            aggs = {}
            for aggr in aggregations:
                field = aggr.get("field", "")
                op = aggr.get("op", "count")
                alias = aggr.get("alias", f"{op}_{field}")
                values = [
                    self._resolve(r, field)
                    for r in records
                    if self._resolve(r, field) is not None
                ]
                aggs[alias] = self._compute_aggregation(values, op)
            results.append({**group_key, "count": len(records), **aggs})

        return {
            "groups": results,
            "group_count": len(results),
            "total_records": len(self.data),
            "grouped_by": group_by,
        }

    def _compute_aggregation(self, values, op):
        if not values:
            return None
        try:
            numeric = [v for v in values if isinstance(v, (int, float))]
            if op == "count":
                return len(values)
            elif op == "sum":
                return sum(numeric) if numeric else None
            elif op == "avg":
                return sum(numeric) / len(numeric) if numeric else None
            elif op == "min":
                return min(numeric) if numeric else None
            elif op == "max":
                return max(numeric) if numeric else None
            elif op == "first":
                return values[0]
            elif op == "last":
                return values[-1]
            elif op == "unique":
                return list(dict.fromkeys(values))
            elif op == "concat":
                delimiter = ", "
                return delimiter.join(str(v) for v in values)
            elif op == "stddev":
                return statistics.stdev(numeric) if len(numeric) > 1 else 0.0
            elif op == "variance":
                return statistics.variance(numeric) if len(numeric) > 1 else 0.0
            elif op == "range":
                return max(numeric) - min(numeric) if len(numeric) > 1 else 0.0
            elif op == "median":
                return statistics.median(numeric) if numeric else None
            else:
                return None
        except (statistics.StatisticsError, TypeError):
            return None

    # ── filter mode ─────────────────────────────────────────────────

    def _filter(self, args):
        conditions = args.get("conditions", [])
        if not conditions:
            return {
                "records": self.data,
                "total": len(self.data),
                "filtered": 0,
            }

        logic = args.get("logic", "and").lower()
        filtered = []
        for record in self.data:
            results = []
            for condition in conditions:
                field = condition.get("field", "")
                op = condition.get("op", "exists")
                value = condition.get("value")
                actual = self._resolve(record, field)
                results.append(self._evaluate(actual, op, value))

            if logic == "and" and all(results):
                filtered.append(record)
            elif logic == "or" and any(results):
                filtered.append(record)
            elif logic == "not" and not all(results):
                filtered.append(record)

        return {
            "records": filtered,
            "total": len(self.data),
            "filtered": len(self.data) - len(filtered),
            "logic": logic,
            "condition_count": len(conditions),
        }

    def _evaluate(self, actual, op, expected):
        if op == "exists":
            return actual is not None
        if op == "not_exists":
            return actual is None
        if op == "eq":
            return actual == expected
        if op == "neq":
            return actual != expected
        if op == "gt":
            return actual is not None and expected is not None and actual > expected
        if op == "gte":
            return actual is not None and expected is not None and actual >= expected
        if op == "lt":
            return actual is not None and expected is not None and actual < expected
        if op == "lte":
            return actual is not None and expected is not None and actual <= expected
        if op == "in":
            return isinstance(expected, list) and actual in expected
        if op == "contains":
            return isinstance(actual, (str, list)) and expected in actual
        if op == "between":
            if not isinstance(expected, (list, tuple)) or len(expected) != 2:
                return False
            return actual is not None and expected[0] <= actual <= expected[1]
        if op == "is_null":
            return actual is None or (isinstance(actual, str) and actual == "")
        if op == "not_null":
            return actual is not None and not (isinstance(actual, str) and actual == "")
        if op == "matches":
            import re
            return isinstance(actual, str) and bool(re.match(expected, actual))
        return False

    # ── sort mode ───────────────────────────────────────────────────

    def _sort(self, args):
        sort_by = args.get("sort_by", [])
        if not sort_by:
            return {"records": self.data, "total": len(self.data)}

        def sort_key(record):
            keys = []
            for spec in sort_by:
                field = spec if isinstance(spec, str) else spec.get("field", "")
                val = self._resolve(record, field)
                if val is None:
                    val = ""
                keys.append(val)
            return tuple(keys)

        reverse = args.get("order", "asc").lower() == "desc"
        sorted_data = sorted(self.data, key=sort_key, reverse=reverse)

        return {
            "records": sorted_data,
            "total": len(sorted_data),
            "sort_by": sort_by,
            "order": "desc" if reverse else "asc",
        }

    # ── stats mode ──────────────────────────────────────────────────

    def _stats(self, args):
        field = args.get("field", "")
        if not field:
            return {"error": "'field' is required for stats mode"}

        values = [
            self._resolve(r, field)
            for r in self.data
            if self._resolve(r, field) is not None
        ]

        if not values:
            return {
                "field": field,
                "count": 0,
                "error": "No non-null values found",
            }

        numeric = [v for v in values if isinstance(v, (int, float))]
        string = [v for v in values if isinstance(v, str)]

        result = {
            "field": field,
            "count": len(values),
            "null_count": len(self.data) - len(values),
            "non_null_count": len(values),
            "unique_count": len(set(values)),
        }

        if numeric:
            result["type"] = "numeric"
            result["min"] = min(numeric)
            result["max"] = max(numeric)
            result["sum"] = sum(numeric)
            result["avg"] = sum(numeric) / len(numeric)
            result["median"] = statistics.median(numeric)
            if len(numeric) > 1:
                result["stddev"] = statistics.stdev(numeric)
                result["variance"] = statistics.variance(numeric)
                result["range"] = max(numeric) - min(numeric)
                result["p50"] = statistics.median(numeric)
                sorted_n = sorted(numeric)
                result["p90"] = sorted_n[int(len(sorted_n) * 0.9)]
                result["p95"] = sorted_n[int(len(sorted_n) * 0.95)]
                result["p99"] = sorted_n[int(len(sorted_n) * 0.99)]
            result["distribution"] = self._distribution(numeric, 10)
        elif string:
            result["type"] = "string"
            result["min_length"] = min(len(s) for s in string)
            result["max_length"] = max(len(s) for s in string)
            result["avg_length"] = sum(len(s) for s in string) / len(string)
        else:
            result["type"] = "mixed"

        return result

    def _distribution(self, values, buckets=10):
        if not values or min(values) == max(values):
            return []
        mn, mx = min(values), max(values)
        width = (mx - mn) / buckets
        if width == 0:
            return [{"bucket": 0, "range": f"{mn}", "count": len(values)}]
        dist = [0] * buckets
        for v in values:
            idx = min(int((v - mn) / width), buckets - 1)
            dist[idx] += 1
        return [
            {
                "bucket": i,
                "range": f"{mn + i * width:.2f} - {mn + (i + 1) * width:.2f}",
                "count": c,
            }
            for i, c in enumerate(dist)
        ]

    # ── batch mode ──────────────────────────────────────────────────

    def _batch(self, args):
        batch_size = args.get("batch_size", 100)
        if batch_size < 1:
            return {"error": "batch_size must be >= 1"}

        batches = []
        for i in range(0, len(self.data), batch_size):
            batches.append(self.data[i : i + batch_size])

        return {
            "batches": batches,
            "batch_count": len(batches),
            "batch_size": batch_size,
            "total_records": len(self.data),
        }

    # ── helpers ─────────────────────────────────────────────────────

    def _resolve(self, record, path):
        if not isinstance(path, str):
            return path
        parts = path.split(".")
        current = record
        for part in parts:
            if isinstance(current, dict):
                current = current.get(part)
            elif isinstance(current, list) and part.lstrip("-").isdigit():
                idx = int(part)
                current = current[idx] if 0 <= idx < len(current) else None
            else:
                return None
            if current is None:
                return None
        return current
