// ─────────────────────────────────────────────────────────────────────────
// aggregator — full-featured plugin demonstrating all plugin system
// capabilities: $args, $logger, $infra.get(), $limits, $done, error
// handling, multiple execution modes, data transformation, and more.
// ─────────────────────────────────────────────────────────────────────────

// ── Types ────────────────────────────────────────────────────────────────

interface ComponentHealth {
    name: string;
    available: boolean;
    status: Record<string, any> | null;
    latency_ms: number | null;
    error: string | null;
}

interface DashboardResult {
    mode: "dashboard";
    runtime: { elapsed_ms: number; limits: Record<string, number> };
    summary: { total: number; healthy: number; degraded: number; down: number };
    components: ComponentHealth[];
}

interface QueryArgs {
    component: string;
    command?: string;
    payload?: Record<string, any>;
}

interface QueryResult {
    mode: "query";
    runtime: { elapsed_ms: number; limits: Record<string, number> };
    target: string;
    command: string;
    data: any;
}

interface TransformRule {
    field: string;
    operation: "uppercase" | "lowercase" | "reverse" | "trim" | "prefix" | "suffix";
    value?: string;
}

interface TransformArgs {
    input: Record<string, any>;
    rules: TransformRule[];
}

interface TransformResult {
    mode: "transform";
    runtime: { elapsed_ms: number; limits: Record<string, number> };
    original: Record<string, any>;
    transformed: Record<string, any>;
    applied_rules: number;
}

type ResultData = DashboardResult | QueryResult | TransformResult | Record<string, any>;

// ── Constants ────────────────────────────────────────────────────────────

const ALL_COMPONENTS: string[] = [
    "redis", "postgres", "mongo", "kafka", "grafana", "minio", "cron",
];

// ── Helpers ──────────────────────────────────────────────────────────────

function timing(): { start: number; elapsed(): number } {
    const start = Date.now();
    return { start, elapsed: () => Date.now() - start };
}

function runtimeInfo(t: { elapsed(): number }): { elapsed_ms: number; limits: Record<string, number> } {
    return { elapsed_ms: t.elapsed(), limits: { max_timeout_ms: $limits.max_timeout_ms, max_memory_bytes: $limits.max_memory_bytes } };
}

function nowISO(): string {
    return new Date().toISOString();
}

// ── Core: inspect a single component ─────────────────────────────────────

function inspectComponent(name: string): ComponentHealth {
    $logger.debug("Inspecting: " + name);

    const t = timing();

    try {
        const comp: any = $infra.get(name);

        if (comp == null) {
            $logger.info("Component not available: " + name);
            return { name, available: false, status: null, latency_ms: null, error: null };
        }

        let status: Record<string, any> | null = null;
        let error: string | null = null;

        try {
            status = comp.GetStatus();
        } catch (e: any) {
            error = "GetStatus failed: " + (e.message || String(e));
            $logger.warn(name + " GetStatus error: " + error);
        }

        return { name, available: true, status, latency_ms: t.elapsed(), error };
    } catch (e: any) {
        const msg = "Inspect error: " + (e.message || String(e));
        $logger.error(name + " " + msg);
        return { name, available: false, status: null, latency_ms: null, error: msg };
    }
}

// ── Mode: dashboard — collect health from all or selected components ─────

function handleDashboard(components: string[]): DashboardResult {
    $logger.info("Dashboard mode, inspecting " + components.length + " component(s)");

    const t = timing();

    const results: ComponentHealth[] = components.map(inspectComponent);

    const healthy = results.filter((c) => c.available && !c.error).length;
    const degraded = results.filter((c) => c.available && c.error).length;
    const down = results.filter((c) => !c.available).length;

    $logger.info("Dashboard complete: " + healthy + " healthy, " + degraded + " degraded, " + down + " down");

    return {
        mode: "dashboard",
        runtime: runtimeInfo(t),
        summary: { total: results.length, healthy, degraded, down },
        components: results,
    };
}

// ── Mode: query — run a command against a single component ───────────────

function handleQuery(args: QueryArgs): QueryResult {
    const target = args.component || "unknown";
    const command = args.command || "status";
    $logger.info("Query mode, target=" + target + " command=" + command);

    const t = timing();
    let data: any = null;

    try {
        const comp: any = $infra.get(target);

        if (comp == null) {
            $logger.warn("Query target not available: " + target);
            data = { error: "component not available" };
        } else if (command === "status") {
            data = comp.GetStatus();
        } else if (command === "name") {
            data = { name: comp.Name() };
        } else if (command === "ping") {
            data = { ping: true, timestamp: nowISO() };
        } else {
            if (typeof comp[command] === "function") {
                const payload = args.payload || {};
                data = comp[command](payload);
            } else {
                data = { error: "unknown command: " + command };
            }
        }
    } catch (e: any) {
        data = { error: e.message || String(e) };
        $logger.error("Query failed: " + data.error);
    }

    return {
        mode: "query",
        runtime: runtimeInfo(t),
        target,
        command,
        data,
    };
}

// ── Mode: transform — apply rules to an input object ─────────────────────

function applyRule(value: any, rule: TransformRule): any {
    if (typeof value !== "string") return value;

    switch (rule.operation) {
        case "uppercase":  return value.toUpperCase();
        case "lowercase":  return value.toLowerCase();
        case "reverse":    return value.split("").reverse().join("");
        case "trim":       return value.trim();
        case "prefix":     return (rule.value || "") + value;
        case "suffix":     return value + (rule.value || "");
        default:           return value;
    }
}

function handleTransform(args: TransformArgs): TransformResult {
    $logger.info("Transform mode, " + (args.rules || []).length + " rule(s)");

    const t = timing();
    const input: Record<string, any> = args.input || {};
    const rules: TransformRule[] = args.rules || [];
    const transformed: Record<string, any> = { ...input };
    let applied = 0;

    for (const rule of rules) {
        if (rule.field && transformed[rule.field] !== undefined) {
            transformed[rule.field] = applyRule(transformed[rule.field], rule);
            applied++;
            $logger.debug("Applied " + rule.operation + " on field=" + rule.field);
        } else {
            $logger.warn("Rule skipped: field=" + rule.field + " not found or undefined");
        }
    }

    return {
        mode: "transform",
        runtime: runtimeInfo(t),
        original: input,
        transformed,
        applied_rules: applied,
    };
}

// ── Mode: echo — simple connectivity test ────────────────────────────────

function handleEcho(): Record<string, any> {
    $logger.info("Echo mode");

    return {
        mode: "echo",
        message: "Plugin system is operational",
        timestamp: nowISO(),
        args_received: $args,
        limits: $limits,
    };
}

// ── Entry point — route by $args.mode ────────────────────────────────────

function main(): void {
    $logger.info("Aggregator plugin started");

    const mode: string = ($args.mode as string) || "dashboard";

    $logger.info("Mode selected: " + mode);

    let result: ResultData;

    try {
        switch (mode) {
            case "dashboard": {
                const components: string[] = ($args.components as string[]) || ALL_COMPONENTS;
                result = handleDashboard(components);
                break;
            }

            case "query": {
                result = handleQuery($args as any);
                break;
            }

            case "transform": {
                result = handleTransform($args as any);
                break;
            }

            case "echo": {
                result = handleEcho();
                break;
            }

            default: {
                result = {
                    mode: "error",
                    error: "Unknown mode: " + mode,
                    valid_modes: ["dashboard", "query", "transform", "echo"],
                    args_received: $args,
                };
                $logger.warn("Unknown mode: " + mode);
                break;
            }
        }

        $logger.info("Aggregator plugin finished (" + mode + ")");
        $done({ success: true, data: result });
    } catch (e: any) {
        const msg = "Unhandled error: " + (e.message || String(e));
        $logger.error(msg);
        $done({ success: false, error: msg, data: { mode, args: $args } });
    }
}

main();
