# srvr

`github.com/tpyle/srvr` runs a Go HTTP service as two servers:

- an **external server** (default port 8080) for your API, and
- an **internal server** (default port 8081) for operational endpoints that
  should not be exposed to the public: liveness and readiness probes,
  Prometheus metrics, and runtime log level control.

Both servers log every request through the standard library's `log/slog`,
with a request ID on every line.

```go
import "github.com/tpyle/srvr"
```

## Built-in endpoints

All are served on the internal port. Each path can be changed.

| Endpoint | Default path | Purpose | Page |
|---|---|---|---|
| Liveness | `/live` | Always 200 while the process is serving | [Health Checks](Health-Checks.md) |
| Readiness | `/ready` | 200 when every readiness check passes, 503 otherwise | [Health Checks](Health-Checks.md) |
| Metrics | `/metrics` | Prometheus metrics, including per-route request metrics | [Metrics](Metrics.md) |
| Log level | `/logging` | Read or change the slog level (requires `WithLogLevel`) | [Logging](Logging.md) |

## Pages

| Page | Contents |
|---|---|
| [Getting Started](Getting-Started.md) | Install, a minimal server, trying the endpoints |
| [Configuration](Configuration.md) | Every option, its default, and how options combine |
| [Configuring with Viper](Viper.md) | Config files, environment variables, prefixes |
| [Logging](Logging.md) | slog setup, request logs, request-scoped loggers, runtime log level |
| [Health Checks](Health-Checks.md) | Liveness, readiness checks, writing good checks |
| [Metrics](Metrics.md) | Request metrics, example queries, custom registries |
| [Server Lifecycle](Server-Lifecycle.md) | `Start`, `Run`, `Wait`, graceful `Stop`/`Shutdown`, signal handling, Kubernetes |
| [Migrating to slog](Migrating-to-slog.md) | Upgrading from v0.1: slog, graceful shutdown, new metric names |

Runnable programs are in the repository's
[`examples/`](https://github.com/tpyle/srvr/tree/trunk/examples) directory:

- `basic`: an API, all built-in endpoints, and runtime log level changes
- `health`: readiness checks on a dependency, custom probe paths, extra internal routes
- `viper`: configuration from YAML and environment variables
