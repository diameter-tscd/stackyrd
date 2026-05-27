function handler(): void {
    const input = $args.input || "world";
    $logger.info("Processing input: " + input);

    const result = {
        message: "Hello, " + input + "!",
        timestamp: new Date().toISOString(),
        limits: $limits,
    };

    $done({ success: true, data: result });
}

handler();
