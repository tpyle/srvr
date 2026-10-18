# Configuration

`srvr.Create` takes functional options. Options are applied in order, so a
later option overrides an earlier one that sets the same thing.

```go
server := srvr.Create(
    srvr.WithExternalRouter(router),
    srvr.WithExternalPort(3000),
    srvr.WithReadTimeout(30*time.Second),
)
```

## Ports

| Option | Default | |
|---|---|---|
| `WithExternalPort(int)` | `8080` | Port for your API |
| `WithInternalPort(int)` | `8081` | Port for the built-in endpoints |

Both servers listen on all interfaces (`:<port>`). The internal endpoints
have no authentication: `/logging` lets anyone who can reach it change your
log level, and a failed readiness check's error text is returned in the
response. Keep the internal port off the public network, for example by not
exposing it in your Kubernetes Service or load balancer, or by firewalling it.

## Routers

| Option | Default | |
|---|---|---|
| `WithExternalRouter(*mux.Router)` | none | Your API routes. Without one, no external server is started. |
| `WithInternalRouter(*mux.Router)` | a new router | Extra internal-only routes. The built-in endpoints are added to it. Passing `nil` disables the internal server and all built-in endpoints. |

`Create` adds routes, and to the external router a small middleware that
records the matched route for metrics. Give each server its own router, and
don't pass the same router to two `Create` calls.

## Built-in endpoint paths

| Option | Default |
|---|---|
| `WithLivenessManagement(path)` | `/live` |
| `WithReadinessManagement(path)` | `/ready` |
| `WithMetricsRoute(path)` | `/metrics` |
| `WithLogManagement(path)` | `/logging` |

See [Health Checks](Health-Checks.md), [Metrics](Metrics.md) and
[Logging](Logging.md) for what each endpoint does.

## Readiness checks

| Option | |
|---|---|
| `WithReadinessCheck(name, func(context.Context) error)` | Adds a check run by the readiness endpoint. Repeat for each dependency. Reusing a name replaces that check. |

## Logging

| Option | Default | |
|---|---|---|
| `WithLogger(*slog.Logger)` | `slog.Default()` | Logger for srvr's messages and request logs |
| `WithLogLevel(*slog.LevelVar)` | none | Level controlled by the log level endpoint. The endpoint is only served when this is set. |
| `WithExternalLogging(bool)` | `true` | Request logging on the external server |
| `WithInternalLogging(bool)` | `true` | Request logging on the internal server, including probe and metrics requests |

Probes and Prometheus scrapes are frequent, so consider
`WithInternalLogging(false)` if you log at debug level in production. See
[Logging](Logging.md).

## Timeouts and limits

These apply to both servers and map to the `net/http.Server` fields of the
same names.

| Option | Default | `http.Server` field |
|---|---|---|
| `WithReadTimeout(time.Duration)` | 5s | `ReadTimeout` |
| `WithWriteTimeout(time.Duration)` | 10s | `WriteTimeout` |
| `WithIdleTimeout(time.Duration)` | 120s | `IdleTimeout` |
| `WithReadHeaderTimeout(time.Duration)` | 2s | `ReadHeaderTimeout` |
| `WithMaxHeaderBytes(int)` | 1 MiB | `MaxHeaderBytes` |

| Option | Default | |
|---|---|---|
| `WithShutdownTimeout(time.Duration)` | 10s | How long `Stop` waits for in-flight requests before closing their connections. See [Server Lifecycle](Server-Lifecycle.md#stop-and-shutdown). |

`WriteTimeout` covers the whole handler, so raise it for slow endpoints such
as large downloads or streaming responses.

## Metrics

| Option | Default | |
|---|---|---|
| `WithMetricsRegistry(*prometheus.Registry)` | the Prometheus default registry | Where request metrics are registered, and what `/metrics` serves. See [Metrics](Metrics.md#using-your-own-registry). |

## Viper

| Option | |
|---|---|
| `WithViper(*viper.Viper)` | Read settings from keys under `srvr.` |
| `WithViperPrefix(*viper.Viper, prefix)` | Read settings from keys under `<prefix>srvr.` |

See [Configuring with Viper](Viper.md).
