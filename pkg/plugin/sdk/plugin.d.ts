declare const $args: Record<string, any>;

declare const $infra: {
    get(name: string): any;
};

declare const $logger: {
    info(msg: string): void;
    warn(msg: string): void;
    error(msg: string): void;
    debug(msg: string): void;
};

declare const $limits: {
    max_timeout_ms: number;
    max_memory_bytes: number;
};

declare function $done(result: {
    success: boolean;
    data?: any;
    error?: string;
}): void;
