# stackyrd — Design & Architecture

## High-level architecture

```mermaid
flowchart TB
    subgraph cmd["cmd/app/"]
        main["main.go<br/>Parse flags"]
        app["Application<br/>Run()"]
    end

    subgraph config["config/"]
        cfg["Config (viper)<br/>YAML + env overrides"]
    end

    subgraph infra["pkg/infrastructure/"]
        compReg["ComponentRegistry<br/>singleton"]
        infraInit["InfraInitManager<br/>async init"]
    end

    subgraph deps["pkg/registry/"]
        depBag["Dependencies<br/>sealed bag, typed getters<br/>Redis() / Postgres() / ..."]
    end

    subgraph plugin["pkg/plugin/"]
        plugInit["Init()<br/>scan builtins → instantiate"]
        plugBridge["PluginBridge<br/>infra component"]
    end

    subgraph mw["internal/middleware/"]
        mwReg["MiddlewareRegistry"]
    end

    subgraph svc["internal/services/modules/"]
        svcReg["ServiceRegistry"]
    end

    subgraph server["internal/server/"]
        srv["Echo server<br/>routes / health / metrics"]
    end

    main --> app
    app --> cfg
    cfg --> infraInit
    infraInit --> compReg
    compReg --> depBag
    depBag --> plugInit
    plugInit --> plugBridge
    plugBridge --> compReg
    compReg --> mwReg
    mwReg --> srv
    mwReg --> svcReg
    svcReg --> srv
```

## Boot sequence

```mermaid
sequenceDiagram
    participant main as main.go
    participant cfg as ConfigManager
    participant app as Application
    participant srv as Server.Start()
    participant infraInit as InfraInitManager
    participant compReg as ComponentRegistry
    participant deps as Dependencies
    participant plugin as plugin.Init()
    participant mw as MiddlewareRegistry
    participant svc as ServiceRegistry

    main->>cfg: LoadConfig() / ValidateConfig()
    main->>app: Run()
    app->>srv: New(cfg, logger)

    srv->>infraInit: StartAsyncInitialization()
    infraInit->>compReg: Initialize(cfg, logger)
    Note over compReg: Call every registered factory<br/>(redis, postgres, mongo, kafka, ...)
    compReg-->>infraInit: registry

    srv->>deps: NewDependencies()
    srv->>deps: Set(name, component) for each infra
    srv->>deps: Seal() — read-only after boot
    srv->>srv: setConnectionDefaults()

    srv->>plugin: Init(cfg, logger, pluginGroup)
    plugin->>compReg: scanBuiltinPlugins()
    plugin->>plugin: instantiatePlugin() per meta
    plugin->>compReg: SetComponent("plugins", bridge)

    srv->>mw: ApplyConfig(cfg)
    srv->>mw: AutoDiscoverMiddlewares()
    mw-->>srv: []echo.MiddlewareFunc
    srv->>srv: e.Use(mw...)

    srv->>compReg: RouteRegistrar routes

    srv->>svc: AutoDiscoverServices(cfg, logger, deps)
    svc-->>srv: []interfaces.Service
    srv->>svc: Boot(echo)
    svc->>srv: RegisterRoutes on /api/v1

    srv->>srv: e.Start(":" + port)
```

## Extension points

```mermaid
flowchart LR
    subgraph Service["Service (internal/services/modules/)"]
        S1["interface<br/>Name / WireName / Enabled<br/>Endpoints / RegisterRoutes / Get"]
        S2["init() → RegisterService(name, factory)<br/>or RegisterServiceWithDeps(name, factory)"]
        S3["plain factory: cfg, logger → Service<br/>dep factory: cfg, logger, deps → Service"]
        S4["config.yaml: services.{wire_name}: true/false"]
    end

    subgraph Middleware["Middleware (internal/middleware/)"]
        M1["echo.MiddlewareFunc"]
        M2["init() → RegisterMiddleware(name, factory)"]
        M3["factory: cfg, logger → echo.MiddlewareFunc"]
        M4["config.yaml: middleware.{name}: true/false"]
    end

    subgraph Infra["Infrastructure (pkg/infrastructure/)"]
        I1["interface<br/>Name / Close / GetStatus"]
        I2["init() → RegisterComponent(name, factory)"]
        I3["factory: cfg, logger → InfrastructureComponent"]
        I4["Optional: RouteRegistrar<br/>RouteHandlers() []RouteHandler"]
        I5["config.yaml: {redis|postgres|mongo|...}.enabled"]
    end

    subgraph Plugin["Plugin (pkg/plugin/builtin/{name}/)"]
        P1["Plugin interface<br/>Execute(ctx, args) → Result"]
        P2["PluginMeta (plugin.yaml)<br/>name / version / entrypoint / limits"]
        P3["Runtime registry<br/>ts: / goja: / lua: / grpc: / wasm:"]
        P4["PluginBridge (infra component)"]
        P5["config.yaml: plugins.enabled / overrides / allowlist"]
    end

    S1 --- S2 --- S3 --- S4
    M1 --- M2 --- M3 --- M4
    I1 --- I2 --- I3 --- I4 --- I5
    P1 --- P2 --- P3 --- P4 --- P5
```

## Request lifecycle

```mermaid
flowchart TD
    req["HTTP Request"]
    recover["echo.Recover()"]
    mws["Auto-discovered Middleware<br/>request_id → logger → permission_check → jwt → security"]
    notfound["404 / 405 handler"]
    infraRoutes["Infrastructure Routes<br/>(RouteRegistrar)"]
    apiGroup["/api/v1 group"]
    services["Service Routes<br/>users / products / tasks / ..."]
    handler["Handler func"]
    bind["request.Bind(c, &target)<br/>Validate via go-playground/validator"]
    deps["deps.Redis() / deps.Postgres() / ...<br/>typed getters (nil if absent)"]
    response["response.Success / Created / Error"]

    req --> recover
    recover --> mws
    mws --> notfound
    notfound --> infraRoutes
    infraRoutes --> apiGroup
    apiGroup --> services
    services --> handler
    handler --> bind
    handler --> deps
    handler --> response
```

## Plugin system architecture

```mermaid
flowchart TB
    subgraph BuiltinFS["embed.FS (builtin/)"]
        p1["plugin.yaml"]
        ts["ts:scripts/main.ts"]
        goja["goja:scripts/main.js"]
        lua["lua:main.lua"]
        grpc["grpc:./bin/worker"]
        wasm["wasm:main.wasm"]
    end

    subgraph PluginInit["plugin.Init()"]
        scan["scanBuiltinPlugins()<br/>read plugin.yaml → meta"]
        instantiate["instantiatePlugin()<br/>match entrypoint prefix → Runtime"]
        store["reg.Store(name, plugin)"]
        bridge["PluginBridge<br/>SetComponent('plugins', bridge)"]
    end

    subgraph RuntimeRegistry["runtime_registry.go"]
        rr["RegisterRuntime(rt)<br/>prefix-based dispatch"]
    end

    subgraph Runtimes["Runtimes"]
        tsrt["TSRuntime<br/>transpile → goja VM"]
        gojart["GojaRuntime<br/>goja VM + program cache"]
        luart["LuaRuntime<br/>gopher-lua VM"]
        grpcrt["gRPCRuntime<br/>subprocess stdin/stdout"]
        wasmrt["WasmRuntime<br/>goja/VM sandbox"]
    end

    subgraph Execution["PluginBridge.Execute()"]
        ctx["Context{Logger, Registry, Limits}"]
        exec["p.Execute(ctx, args)"]
        metrics["CollectMetrics()<br/>active_execs / goroutines / memory"]
    end

    BuiltinFS --> scan
    scan --> instantiate
    instantiate --> rr
    rr --> tsrt
    rr --> gojart
    rr --> luart
    rr --> grpcrt
    rr --> wasmrt
    instantiate --> store
    store --> bridge
    bridge --> Execution
    ctx --> exec
    exec --> metrics
```

## Infrastructure component lifecycle

```mermaid
flowchart LR
    subgraph Registration["init() phase"]
        reg["RegisterComponent(name, factory)"]
        factories["factories map[string]ComponentFactory"]
    end

    subgraph Initialization["Server.Start()"]
        init["ComponentRegistry.Initialize(cfg, logger)"]
        create["factory(cfg, logger) → component"]
        store["components[name] = component"]
    end

    subgraph AsyncHealth["InfraInitManager"]
        health["GetStatus() goroutine per component<br/>with 10s timeout"]
        status["status map[string]*InfraInitStatus<br/>progress 0.0 → 1.0"]
    end

    subgraph Runtime["running server"]
        get["registry.Get(name)"]
        getAll["registry.GetAll()<br/>TTL-cached snapshot (2s)"]
        routes["RouteRegistrar routes mounted"]
        depsSet["dependencies.Set(name, component)"]
    end

    subgraph Shutdown["graceful shutdown"]
        close["component.Close()<br/>10s timeout per component"]
        drain["WorkerPool.Stop()<br/>drain queue → close stopChan"]
    end

    reg --> factories
    factories --> init
    init --> create
    create --> store
    store --> asyncHealth
    asyncHealth --> runtime
    runtime --> shutdown
```

## Dependency injection flow

```mermaid
flowchart LR
    subgraph InfraComponents["Infrastructure Components"]
        redis["redis.RedisManager"]
        pg["postgres.PostgresConnectionManager"]
        mongo["mongo.MongoConnectionManager"]
        kafka["kafka.KafkaManager"]
        grafana["grafana.GrafanaManager"]
        cron["cron.CronManager"]
        minio["minio.MinIOManager"]
        plugins["plugin.PluginBridge"]
    end

    subgraph DepsBag["Dependencies (sealed after boot)"]
        getters["typed getters:<br/>Redis() / Postgres() / Mongo()<br/>Kafka() / Grafana() / Cron() / MinIO()<br/>each returns *T or nil"]
        cache["GetAll() — TTL-cached snapshot (2s)"]
    end

    subgraph ServiceFactories["Service Factory (init)"]
        f1["RegisterService('users', factory)<br/>plain: cfg, logger → Service"]
        f2["RegisterServiceWithDeps('tasks', factory)<br/>deps: cfg, logger, deps → Service"]
    end

    subgraph AutoDiscover["AutoDiscoverServices()"]
        ad["for each plain factory:<br/>  svc = factory(cfg, logger)<br/>for each dep factory:<br/>  svc = factory(cfg, logger, deps)<br/>serviceDiscovered.Store(name, svc.Get())"]
    end

    InfraComponents --> DepsBag
    DepsBag --> ServiceFactories
    ServiceFactories --> AutoDiscover
```

## Config loading & validation

```mermaid
flowchart TD
    flags["parseFlags()<br/>-c / -port / -verbose / -env"]
    cm["ConfigManager"]
    url{{"configURL != ''?"}}
    file["LoadConfigFromFile()<br/>viper.ReadInConfig()<br/>./config.yaml or ./config/config.yaml"]
    remote["LoadConfigFromURL()<br/>utils.LoadConfigFromURL()"]
    unmarshal["viper.Unmarshal(&cfg)<br/>Config struct"]
    validate["ValidateConfig(cfg)<br/>port availability"]
    defaults["viper defaults:<br/>app.name / server.port /<br/>redis.enabled=false / ..."]
    env["viper.AutomaticEnv()<br/>APP_NAME / SERVER_PORT / ..."]

    flags --> cm
    cm --> url
    url -->|yes| remote
    url -->|no| file
    remote --> unmarshal
    file --> defaults
    defaults --> env
    env --> unmarshal
    unmarshal --> validate
```

## Health & observability

```mermaid
flowchart TB
    subgraph HealthEndpoints["Health Endpoints"]
        h1["GET /health"]
        h2["GET /health/infrastructure"]
        h3["GET /health/dependencies"]
        h4["GET /health/resources"]
    end

    subgraph Metrics["Observability"]
        m1["Prometheus metrics<br/>GET /metrics"]
        m2["Metrics struct<br/>HTTPRequestsTotal / Duration / Size<br/>DB / Cache / CircuitBreaker<br/>Webhook / WebSocket / Batch"]
    end

    subgraph Sources["Data Sources"]
        s1["infraInitManager.IsReady()"]
        s2["infraInitManager.GetStatus()<br/>per-component progress + error"]
        s3["dependencies.GetAll()<br/>TTL-cached map"]
        s4["registry.GetServiceFactories()"]
        s5["utils.GetMemSelf() / GetRoutine()"]
    end

    h1 --> s1
    h1 --> s2
    h2 --> s2
    h3 --> s3
    h3 --> s4
    h4 --> s5
    m1 --> m2
```
