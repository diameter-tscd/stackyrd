"""Metric Computer Plugin.

Computes aggregation metrics, percentiles, sliding windows, and
derived metrics from time-series or numeric data arrays.

Modes:
  compute (default) — Standard aggregation metrics (sum, avg, min, max, rate).
  percentile — Compute percentile values (P50, P90, P95, P99, etc.).
  window — Sliding window computations (SMA, EMA).
  derive — Compute derived metrics (ratios, deltas, rates of change).
"""

import math
from sdk import Plugin


class MetricComputerPlugin(Plugin):
    MODES = ("compute", "percentile", "window", "derive")

    def setup(self, args):
        self.values = args.get("values", [])
        if not isinstance(self.values, list):
            raise ValueError("'values' must be a list of numbers")

    def execute(self, args):
        mode = args.get("mode", "compute")
        if mode not in self.MODES:
            return {
                "success": False,
                "error": f"Unknown mode '{mode}'. Choose from: {', '.join(self.MODES)}",
            }
        handler = getattr(self, f"_{mode}")
        return {"success": True, "data": handler(args)}

    # ── compute mode ────────────────────────────────────────────────

    def _compute(self, args):
        if not self.values:
            return {"error": "No values provided", "count": 0}

        numeric = [v for v in self.values if isinstance(v, (int, float))]
        if not numeric:
            return {"error": "No numeric values found", "count": 0}

        n = len(numeric)
        total = sum(numeric)
        avg = total / n
        sorted_vals = sorted(numeric)
        mn, mx = sorted_vals[0], sorted_vals[-1]

        result = {
            "count": n,
            "sum": total,
            "avg": avg,
            "min": mn,
            "max": mx,
            "range": mx - mn,
            "median": self._percentile(sorted_vals, 50),
            "p90": self._percentile(sorted_vals, 90),
            "p95": self._percentile(sorted_vals, 95),
            "p99": self._percentile(sorted_vals, 99),
        }

        if n > 1:
            variance = sum((x - avg) ** 2 for x in numeric) / (n - 1)
            result["variance"] = variance
            result["stddev"] = math.sqrt(variance)
            result["cv"] = variance / avg if avg != 0 else 0

        # Rate: values per second if timestamps provided
        timestamps = args.get("timestamps", [])
        if timestamps and len(timestamps) == n:
            ts_numeric = [t for t in timestamps if isinstance(t, (int, float))]
            if len(ts_numeric) >= 2:
                duration = ts_numeric[-1] - ts_numeric[0]
                if duration > 0:
                    result["rate_per_second"] = n / duration
                    result["duration_seconds"] = duration

        # Peak-to-peak metrics
        if n > 2:
            deltas = [abs(numeric[i] - numeric[i - 1]) for i in range(1, n)]
            result["avg_delta"] = sum(deltas) / len(deltas)
            result["max_delta"] = max(deltas)
            result["min_delta"] = min(deltas)

        # Trend direction
        if n >= 2:
            first_half = numeric[: n // 2]
            second_half = numeric[n // 2 :]
            first_avg = sum(first_half) / len(first_half)
            second_avg = sum(second_half) / len(second_half)
            diff = second_avg - first_avg
            result["trend"] = "up" if diff > 0.01 * abs(first_avg) else (
                "down" if diff < -0.01 * abs(first_avg) else "stable"
            )
            result["trend_magnitude"] = diff

        return result

    # ── percentile mode ─────────────────────────────────────────────

    def _percentile(self, args):
        if isinstance(args, list):
            sorted_vals = sorted(args)
        else:
            values = args.get("values", self.values)
            sorted_vals = sorted(values)

        if not sorted_vals:
            return {"error": "No values provided"}

        percentiles = args.get("percentiles", [50, 90, 95, 99]) if isinstance(args, dict) else [50, 90, 95, 99]
        if isinstance(args, dict) and "percentiles" in args:
            percentiles = args["percentiles"]

        result = {}
        for p in percentiles:
            key = f"p{p}"
            result[key] = self._percentile_value(sorted_vals, p)

        result["count"] = len(sorted_vals)
        result["min"] = sorted_vals[0]
        result["max"] = sorted_vals[-1]
        result["requested_percentiles"] = sorted(percentiles)

        return result

    def _percentile_value(self, sorted_vals, p):
        if not sorted_vals:
            return None
        if p <= 0:
            return sorted_vals[0]
        if p >= 100:
            return sorted_vals[-1]
        rank = (p / 100.0) * (len(sorted_vals) - 1)
        lower = int(math.floor(rank))
        upper = int(math.ceil(rank))
        if lower == upper:
            return sorted_vals[lower]
        frac = rank - lower
        return sorted_vals[lower] * (1 - frac) + sorted_vals[upper] * frac

    # ── window mode ─────────────────────────────────────────────────

    def _window(self, args):
        if not self.values:
            return {"error": "No values provided"}

        numeric = [v for v in self.values if isinstance(v, (int, float))]
        if not numeric:
            return {"error": "No numeric values found"}

        window_type = args.get("window_type", "sma")
        window_size = args.get("window_size", 5)

        if window_size < 1:
            return {"error": "window_size must be >= 1"}

        n = len(numeric)
        if window_size > n:
            return {
                "error": f"window_size ({window_size}) exceeds data length ({n})",
                "count": n,
            }

        if window_type == "sma":
            result = self._sma(numeric, window_size)
        elif window_type == "ema":
            smoothing = args.get("smoothing", 2.0)
            result = self._ema(numeric, window_size, smoothing)
        elif window_type == "cumulative":
            result = self._cumulative_avg(numeric)
        else:
            return {"error": f"Unknown window_type '{window_type}'. Choose: sma, ema, cumulative"}

        return {
            "window_type": window_type,
            "window_size": window_size,
            "count": len(numeric),
            "window_count": len(result),
            "windows": result,
        }

    def _sma(self, values, window_size):
        result = []
        for i in range(len(values) - window_size + 1):
            window = values[i : i + window_size]
            result.append({
                "index": i,
                "start": i,
                "end": i + window_size - 1,
                "value": sum(window) / window_size,
                "min": min(window),
                "max": max(window),
            })
        return result

    def _ema(self, values, window_size, smoothing=2.0):
        k = smoothing / (1 + window_size)
        result = []
        ema = sum(values[:window_size]) / window_size
        result.append({
            "index": window_size - 1,
            "value": ema,
        })
        for i in range(window_size, len(values)):
            ema = values[i] * k + ema * (1 - k)
            result.append({
                "index": i,
                "value": ema,
            })
        return result

    def _cumulative_avg(self, values):
        result = []
        running_sum = 0.0
        for i, v in enumerate(values):
            running_sum += v
            result.append({
                "index": i,
                "value": running_sum / (i + 1),
                "sum": running_sum,
                "count": i + 1,
            })
        return result

    # ── derive mode ─────────────────────────────────────────────────

    def _derive(self, args):
        if not self.values:
            return {"error": "No values provided"}

        numeric = [v for v in self.values if isinstance(v, (int, float))]
        if not numeric:
            return {"error": "No numeric values found"}

        n = len(numeric)
        derivations = args.get("derivations", ["delta"])
        result = {"count": n}
        timestamps = args.get("timestamps", [])

        if n < 2:
            return {"error": "Need at least 2 values for derived metrics", "count": n}

        for derivation in derivations:
            if derivation == "delta":
                deltas = [numeric[i] - numeric[i - 1] for i in range(1, n)]
                result["delta"] = {
                    "values": deltas,
                    "min": min(deltas),
                    "max": max(deltas),
                    "avg": sum(deltas) / len(deltas),
                    "sum": sum(deltas),
                }

            elif derivation == "pct_change":
                pct = [
                    ((numeric[i] - numeric[i - 1]) / numeric[i - 1]) * 100
                    if numeric[i - 1] != 0 else 0
                    for i in range(1, n)
                ]
                result["pct_change"] = {
                    "values": pct,
                    "min": min(pct),
                    "max": max(pct),
                    "avg": sum(pct) / len(pct),
                }

            elif derivation == "ratio":
                divisor_field = args.get("divisor_field", "")
                divisors = args.get("divisors", [])
                if divisors and len(divisors) == n:
                    ratios = [
                        numeric[i] / divisors[i] if divisors[i] != 0 else 0
                        for i in range(n)
                    ]
                    result["ratio"] = {
                        "values": ratios,
                        "min": min(ratios),
                        "max": max(ratios),
                        "avg": sum(ratios) / len(ratios),
                    }

            elif derivation == "rate_of_change":
                if len(timestamps) >= n:
                    ts = [
                        t for t in timestamps if isinstance(t, (int, float))
                    ]
                    if len(ts) >= 2:
                        roc = [
                            (numeric[i] - numeric[i - 1]) / (ts[i] - ts[i - 1])
                            if (ts[i] - ts[i - 1]) > 0 else 0
                            for i in range(1, min(n, len(ts)))
                        ]
                        result["rate_of_change"] = {
                            "values": roc,
                            "min": min(roc) if roc else 0,
                            "max": max(roc) if roc else 0,
                            "avg": sum(roc) / len(roc) if roc else 0,
                        }
                else:
                    result["rate_of_change"] = {
                        "error": "timestamps required and must match values length",
                    }

            elif derivation == "zscore":
                mean = sum(numeric) / n
                variance = sum((x - mean) ** 2 for x in numeric) / (n - 1) if n > 1 else 1
                stddev = math.sqrt(variance) if variance > 0 else 1
                zscores = [(x - mean) / stddev for x in numeric]
                result["zscore"] = {
                    "values": zscores,
                    "mean": mean,
                    "stddev": stddev,
                    "outliers": [
                        {"index": i, "value": numeric[i], "zscore": z}
                        for i, z in enumerate(zscores)
                        if abs(z) > 2
                    ],
                    "outlier_count": sum(1 for z in zscores if abs(z) > 2),
                }

            elif derivation == "cumulative_sum":
                cumsum = []
                running = 0
                for v in numeric:
                    running += v
                    cumsum.append(running)
                result["cumulative_sum"] = {
                    "values": cumsum,
                    "total": running,
                }

            elif derivation == "log":
                result["log"] = {
                    "values": [
                        math.log(v) if v > 0 else None for v in numeric
                    ],
                }

            elif derivation == "normalize":
                mn, mx = min(numeric), max(numeric)
                rng = mx - mn if mx != mn else 1
                normalized = [(v - mn) / rng for v in numeric]
                result["normalize"] = {
                    "values": normalized,
                    "min": mn,
                    "max": mx,
                    "range": rng,
                }

        return result
