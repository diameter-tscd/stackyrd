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

const COMPONENT_NAMES: string[] = [
    "redis",
    "postgres",
    "mongo",
    "kafka",
    "grafana",
    "minio",
    "cron",
];

function getComponentStatus(name: string): ComponentStatus {
    $logger.debug("Inspecting component: " + name);

    try {
        const comp: any = $infra.get(name);

        if (comp == null) {
            $logger.info("Component not available: " + name);
            return {
                name,
                available: false,
                status: null,
                error: null,
            };
        }

        let status: Record<string, any> | null = null;
        let error: string | null = null;

        try {
            status = comp.GetStatus();
            $logger.info("Component " + name + " status: " + JSON.stringify(status));
        } catch (e: any) {
            error = "GetStatus failed: " + (e.message || String(e));
            $logger.warn("Component " + name + " status error: " + error);
        }

        return {
            name,
            available: true,
            status,
            error,
        };
    } catch (e: any) {
        const msg = "Unexpected error inspecting " + name + ": " + (e.message || String(e));
        $logger.error(msg);
        return {
            name,
            available: false,
            status: null,
            error: msg,
        };
    }
}

function inspect(): InspectorResult {
    const mode: string = ($args.mode as string) || "status";

    $logger.info("Inspector plugin started, mode=" + mode);

    if (mode === "ping") {
        const results: ComponentStatus[] = COMPONENT_NAMES.map(getComponentStatus);

        return {
            mode,
            summary: {
                total: results.length,
                available: results.filter((c) => c.available).length,
                unavailable: results.filter((c) => !c.available).length,
            },
            components: results,
            limits: {
                max_timeout_ms: $limits.max_timeout_ms,
                max_memory_bytes: $limits.max_memory_bytes,
            },
        };
    }

    const specific: string[] = ($args.components as string[]) || COMPONENT_NAMES;
    const results: ComponentStatus[] = specific.map(getComponentStatus);

    return {
        mode,
        summary: {
            total: results.length,
            available: results.filter((c) => c.available).length,
            unavailable: results.filter((c) => !c.available).length,
        },
        components: results,
        limits: {
            max_timeout_ms: $limits.max_timeout_ms,
            max_memory_bytes: $limits.max_memory_bytes,
        },
    };
}

const startTime: number = Date.now();
const result: InspectorResult = inspect();
const elapsed: number = Date.now() - startTime;

$logger.info("Inspector plugin finished in " + elapsed + "ms");

$done({ success: true, data: { elapsed_ms: elapsed, ...result } });
