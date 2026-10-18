# srvr

Run a Go HTTP service as two servers: an **external** one for your API and an
**internal** one for operational endpoints that shouldn't be public. The
internal server comes with liveness and readiness probes, Prometheus metrics,
and a runtime log level endpoint, and both servers log every request through
the standard library's `log/slog`.

```bash
go get github.com/tpyle/srvr
```

srvr requires Go 1.27 or newer.

## Features

- **Two servers**: your API on one port (default 8080), operational endpoints
  on another (default 8081) that you keep off the public network.
- **Health probes**: `/live` for liveness and `/ready` for readiness, which
  runs the checks you register for your dependencies.
- **Metrics**: `/metrics` serves Prometheus metrics, including request
  counts, latency histograms and in-flight requests for the external server,
  labelled by route template, method and status code.
- **Logging with `log/slog`**: request IDs in an `X-Request-Id` header and in
  every request log line, a request-scoped logger in each request context, and
  `/logging` to change the level of a running process.
- **Graceful lifecycle**: `Start` reports port bind errors, and `Stop` lets
  in-flight requests finish before shutting down.
- **Configuration**: functional options, plus optional
  [Viper](https://github.com/spf13/viper) support for config files and
  environment variables.

## Quick start

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

// Share one LevelVar between your slog handler and srvr so /logging can change it.
level := new(slog.LevelVar)
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

router := mux.NewRouter()
router.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
    logmiddleware.SlogFromContext(r.Context()).Info("saying hello") // carries requestId
    w.Write([]byte("Hello, World!\n"))
}).Methods(http.MethodGet)

server := srvr.Create(
    srvr.WithExternalRouter(router),
    srvr.WithLogLevel(level),
    srvr.WithReadinessCheck("database", db.PingContext),
)
if err := server.Start(); err != nil {
    return err
}

<-ctx.Done()
if err := server.Stop(); err != nil {
    return err
}
server.Wait()
```

```bash
curl localhost:8080/hello
curl localhost:8081/live
curl localhost:8081/ready
curl localhost:8081/metrics
curl -X POST localhost:8081/logging -H 'Content-Type: application/json' -d '{"level":"debug"}'
```

## Options

| Option | Default |
|---|---|
| `WithExternalPort`, `WithInternalPort` | 8080, 8081 |
| `WithExternalRouter`, `WithInternalRouter` | none (no external server), a new router |
| `WithReadinessCheck(name, check)` | no checks |
| `WithLivenessManagement`, `WithReadinessManagement`, `WithMetricsRoute`, `WithLogManagement` | `/live`, `/ready`, `/metrics`, `/logging` |
| `WithLogger(*slog.Logger)` | `slog.Default()` |
| `WithLogLevel(*slog.LevelVar)` | none (`/logging` not served) |
| `WithExternalLogging`, `WithInternalLogging` | `true`, `true` |
| `WithReadTimeout`, `WithWriteTimeout`, `WithIdleTimeout`, `WithReadHeaderTimeout` | 5s, 10s, 120s, 2s |
| `WithMaxHeaderBytes` | 1 MiB |
| `WithShutdownTimeout` | 10s |
| `WithMetricsRegistry(*prometheus.Registry)` | the Prometheus default registry |
| `WithViper`, `WithViperPrefix` | Viper not used |

## Documentation

The [wiki](https://github.com/tpyle/srvr/wiki) covers:

- [Getting Started](wiki/Getting-Started.md)
- [Configuration](wiki/Configuration.md): every option and how options combine
- [Configuring with Viper](wiki/Viper.md): config keys, files, environment variables
- [Logging](wiki/Logging.md): request logs, request-scoped loggers, runtime log level
- [Health Checks](wiki/Health-Checks.md): liveness, readiness, writing checks
- [Metrics](wiki/Metrics.md): request metrics, example queries, custom registries
- [Server Lifecycle](wiki/Server-Lifecycle.md): `Start`, `Run`, `Wait`, graceful `Stop`/`Shutdown`
- [Migrating to slog](wiki/Migrating-to-slog.md): upgrading from v0.1 (slog, graceful shutdown, new metric names)

API reference: [pkg.go.dev/github.com/tpyle/srvr](https://pkg.go.dev/github.com/tpyle/srvr).

## Examples

Runnable programs are in [`examples/`](examples):

| Example | Shows |
|---|---|
| [`basic`](examples/basic) | An API, all built-in endpoints, changing the log level at runtime |
| [`health`](examples/health) | Readiness checks on a dependency, custom probe paths, extra internal routes |
| [`viper`](examples/viper) | Configuration from YAML with environment variable overrides |

```bash
go run ./examples/basic
```

## Dependencies

- [gorilla/mux](https://github.com/gorilla/mux): routing
- [Prometheus Go client](https://github.com/prometheus/client_golang): metrics
- [Viper](https://github.com/spf13/viper): optional configuration
- [log-middleware](https://github.com/tpyle/log-middleware): request logging
- [log-manager](https://github.com/tpyle/log-manager): runtime log level

## License

MIT. See [LICENSE](LICENSE).
