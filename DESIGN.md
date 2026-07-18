# stackyrd — Workflow Design

Modular service framework on Echo v4. Registration-based, `init()` auto-discovery,
Viper config, async infrastructure init, dependency-injection bag, sandboxed plugin runtime.

All workflows below are expressed as diagrams. Code references point at the implementing file.

---

## 1. System Context

```mermaid
flowchart TB
    CLI["CLI / Config URL<br/>(flags: -c -port -env -verbose)"] --> CM["ConfigManager<br/>(cmd/app/config_manager.go)"]
    CM -->|"config.yaml + env"| CFG["config.Config<br/>(config/config.go)"]
    CFG --> APP["Application.Run<br/>(cmd/app/application.go)"]
    APP --> SRV["server.Start<br/>(internal/server/server.go)"]

    subgraph RUNTIME["Echo HTTP Server
        SRV --> INF["Infrastructure Layer<br/>(pkg/infrastructure/*)"]
        SRV --> PLG["Plugin Runtime<br/>(pkg/plugin/*)"]
        SRV --> MW["Middleware Chain<br/>(internal/middleware/*)"]
        SRV --> SVC["Service Registry<br/>(pkg/registry/*)"]
    end

    INF --> EXT["Redis · Postgres · Mongo · Kafka · MinIO · Grafana · Cron"]
    SVC --> API["/api/v1/* routes"]
    SRV --> HC["/health · /health/infrastructure<br/>/health/dependencies · /health/resources"]
```

---

## 2. Startup Workflow (Boot Sequence)

```mermaid
sequenceDiagram
    participant M as main()
    participant A as Application
    participant CF as ConfigManager
    participant S as Server
    participant IIM as InfraInitManager
    participant D as Dependencies
    participant PL as PluginBridge
    participant MR as MiddlewareRegistry
    participant SR as ServiceRegistry

    M->>A: NewApplication(configManager)
    A->>A: Run() — execute 6 steps
    A->>CF: LoadConfig()
    CF-->>A: config.Config
    A->>CF: ValidateConfig()
    A->>CF: LoadBanner()
    A->>A: CheckPortAvailability()
    A->>A: InitLogger()
    A->>S: Start()

    par Non-blocking infra warmup
        S->>IIM: StartAsyncInitialization()
        IIM->>INF[Infrastructure components]: concurrent init + health check (10s timeout)
        INF-->>IIM: status/progress
    end

    S->>D: NewDependencies()
    IIM-->>D: Set(name, component) for each
    D->>D: Set("postgres.default", conn)
    D->>D: Set("mongo.default", conn)

    S->>PL: plugin.Init()
    PL-->>D: Set("plugins", bridge)

    S->>MR: ApplyConfig() + AutoDiscoverMiddlewares()
    MR-->>S: []echo.MiddlewareFunc → e.Use()

    S->>S: mount infra RouteRegistrar routes
    S->>SR: NewServiceRegistry()
    S->>SR: AutoDiscoverServices()
    SR-->>S: []Service (enabled only)
    S->>S: serviceRegistry.Boot(e) → RegisterRoutes(/api/v1)
    S->>S: e.Start(port)
```

---

## 3. Registration & Auto-Discovery

```mermaid
flowchart LR
    subgraph REG["Registration (compile-time, init())"]
        S1["internal/services/modules/*<br/>init() → RegisterService(name, factory)"] --> SF["serviceFactories<br/>(sync.Map)"]
        M1["internal/middleware/*<br/>init() → RegisterMiddleware(name, factory)"] --> MF["globalMiddlewareRegistry"]
        I1["pkg/infrastructure/*<br/>RegisterComponent(name, factory)"] --> IF["ComponentRegistry<br/>(global)"]
        P1["plugin manifests<br/>loaded from store"] --> PF["Plugin runtime"]
    end

    subgraph BOOT["Discovery (runtime, server.Start)"]
        SF -->|"range + config.Services.IsEnabled"| AD["AutoDiscoverServices()"]
        MF -->|"ApplyConfig + IsEnabled"| AM["AutoDiscoverMiddlewares()"]
        IF -->|"Initialize()"| AI["StartAsyncInitialization()"]
    end

    AD --> SVCS["enabled Service instances"]
    AM --> CHAIN["Echo middleware chain"]
    AI --> DEPS["Dependencies bag"]
```

**Enable rule:** `ServicesConfig.IsEnabled(name)` defaults to **true** when unset (config/config.go:25). Middleware defaults to enabled unless `middleware.{name}` is false.

---

## 4. Dependency Injection Flow

```mermaid
flowchart TB
    IIM["InfraInitManager"] -->|"Set(name, component)"| D["Dependencies<br/>(sync.Map + 2s TTL cache)"]
    PL["PluginBridge"] -->|"Set('plugins', bridge)"| D

    D -->|"Get('postgres.default')"| SVC1["Service A"]
    D -->|"Get('redis')"| SVC2["Service B"]
    D -->|"Get('plugins')"| SVC3["Service C"]

    subgraph SERVICE["Service factory signature"]
        F["func(cfg, logger, deps *Dependencies) interfaces.Service"]
    end
    SVC1 --> F
    SVC2 --> F
    SVC3 --> F

    F -->|"deps.Get(name)"| D
```

`GetTyped[T]` provides type-safe retrieval. `GetAll()` returns a TTL-cached snapshot (2s) to avoid per-request map copies on `/health/dependencies`.

---

## 5. Request Lifecycle

```mermaid
sequenceDiagram
    participant R as HTTP Request
    participant MW as Middleware Chain
    participant RT as Route Handler
    participant S as Service
    participant D as Dependencies
    participant INF as Infra Component

    R->>MW: Recover → RequestID → Logger → PermissionCheck → custom
    MW->>RT: next()
    RT->>S: handler (echo.Context)
    S->>D: Get("postgres.default")
    D->>INF: connection manager
    INF-->>S: *sql.DB / client
    S-->>RT: response.Success / response.Error
    RT-->>MW: echo.Context
    MW-->>R: JSON + X-Request-ID
```

**Service contract** (`pkg/interfaces/service.go`):
`Name() · WireName() · Enabled() · Endpoints() · RegisterRoutes(g *echo.Group) · Get()`

---

## 6. Plugin Execution

```mermaid
flowchart TB
    REQ["/api/v1 plugin call"] --> BR["PluginBridge<br/>(InfrastructureComponent 'plugins')"]
    BR --> RT["Runtime dispatch"]
    RT -->|"TS / Lua"| VM["goja / gopher-lua sandbox"]
    RT -->|"Python / Go"| GRPC["gRPC subprocess"]
    VM --> EX["Plugin.Execute(ctx, args)"]
    GRPC --> EX
    EX -->|"ResourceLimits<br/>max_memory_bytes / max_timeout_ms"| GUARD["enforced limits"]
    EX -->|"Result{Success,Data,Error}"| BR
    BR -->|"via Dependencies"| SVC["calling Service"]
```

Plugins access infrastructure through `Context.Registry (*ComponentRegistry)` and `Context.Limits`.

---

## 7. Graceful Shutdown

```mermaid
flowchart TB
    SIG["SIGINT / SIGTERM<br/>or ShutdownChan"] --> SD["server.Shutdown(ctx)"]
    SD --> LOOP["for name, component in deps.GetAll()"]
    LOOP --> CLOSE["component.Close() with 10s timeout"]
    CLOSE -->|"ok"| OK["logged: shut down successfully"]
    CLOSE -->|"timeout/error"| ERR["logged: forced shutdown"]
    OK --> EXIT["os.Exit(0)"]
    ERR --> EXIT
```

---

## 8. Extension Points

```mermaid
flowchart TB
    subgraph ADD["Add a capability"]
        A1["New Service<br/>internal/services/modules/x_service.go<br/>init() → RegisterService"] --> T1["toggle: services.x_service in config.yaml"]
        A2["New Middleware<br/>internal/middleware/x.go<br/>init() → RegisterMiddleware"] --> T2["toggle: middleware.x in config.yaml"]
        A3["New Infra Component<br/>pkg/infrastructure/x.go<br/>RegisterComponent + RouteRegistrar"] --> T3["toggle: x.enabled in config.yaml"]
        A4["New Plugin<br/>manifest + entrypoint<br/>sandbox runtime"] --> T4["load via plugin store"]
    end

    T1 --> DS["AutoDiscoverServices filters by IsEnabled"]
    T2 --> DM["AutoDiscoverMiddlewares filters by IsEnabled"]
    T3 --> DI["StartAsyncInitialization + route mount"]
    T4 --> DP["PluginBridge.Execute at runtime"]
```

**Naming:** `{name}_service.go` · `{name}.go` (infra) · `{package}_test.go`

---

## 9. Health & Observability Surface

```mermaid
flowchart LR
    H["GET /health"] -->|"server_ready + infra status"| IIM
    HI["GET /health/infrastructure"] --> IIM["InfraInitManager.GetStatus()"]
    HD["GET /health/dependencies"] --> D["Dependencies.GetAll() + GetServiceFactories()"]
    HR["GET /health/resources"] -->|"memory + goroutines"| SYS["utils.GetMemSelf / GetRoutine"]
    M["GET /metrics"] -->|"if metrics.enabled"| PR["Prometheus handler"]
    SW["GET /swagger"] -->|"if swagger.enabled"| UI["Swagger UI"]
```
