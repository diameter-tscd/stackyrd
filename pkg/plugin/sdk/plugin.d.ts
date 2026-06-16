/** Arguments passed to every plugin execution */
declare const $args: Record<string, any>;

/** Access runtime infrastructure components */
declare const $infra: {
    /** Look up an infrastructure component by registered name (e.g. "redis", "postgres", "mongo") */
    get(name: string): InfrastructureComponent | null;
};

/** Scoped logger — tags output with the plugin ID */
declare const $logger: {
    info(msg: string): void;
    warn(msg: string): void;
    error(msg: string): void;
    debug(msg: string): void;
};

/** Resource limits enforced on this execution */
declare const $limits: {
    max_timeout_ms: number;
    max_memory_bytes: number;
};

/** Signal completion and return a result. Must be called exactly once per execution. */
declare function $done(result: {
    success: boolean;
    data?: any;
    error?: string;
}): void;

/** Persistent state bag — values survive across executions */
declare const $state: {
    get(key: string): any;
    set(key: string, value: any): void;
    delete(key: string): void;
    clear(): void;
    keys(): string[];
};

/** WebSocket session globals — only available in WS route handlers */
declare const $ws: {
    send(data: Record<string, any>): void;
    close(): void;
};

/** Background execution globals — only available in background: true plugins */
declare const $background: {
    setInterval(ms: number, fn: () => void): string;
    setTimeout(ms: number, fn: () => void): string;
    clearInterval(id: string): void;
    clearTimeout(id: string): void;
    sleep(ms: number): void;
};

/** Request context — injected as $args.request for route handler plugins */
interface RequestContext {
    method: string;
    path: string;
    query: Record<string, string[]>;
    headers: Record<string, string[]>;
    body: string;
    params: Record<string, string>;
}

// ── Infrastructure component shape ──────────────────────────────────────
interface InfrastructureComponent {
    /** Human-readable display name */
    Name(): string;
    /** Current health/status snapshot */
    GetStatus(): Record<string, any>;
}

// ── Inspector plugin types ──────────────────────────────────────────────
interface ComponentStatus {
    name: string;
    available: boolean;
    status: Record<string, any> | null;
    error: string | null;
}

interface InspectorResult {
    mode: string;
    summary: {
        total: number;
        available: number;
        unavailable: number;
    };
    components: ComponentStatus[];
    limits: Record<string, number>;
}

// ── Aggregator plugin types ─────────────────────────────────────────────
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

interface TransformResult {
    mode: "transform";
    runtime: { elapsed_ms: number; limits: Record<string, number> };
    original: Record<string, any>;
    transformed: Record<string, any>;
    applied_rules: number;
}
