# Logging

srvr logs through the standard library's
[`log/slog`](https://pkg.go.dev/log/slog), using
[log-middleware](https://github.com/tpyle/log-middleware) for request logs and
[log-manager](https://github.com/tpyle/log-manager) for the runtime log level
endpoint.

## Choosing the logger

By default srvr logs with `slog.Default()`, looked up each time it logs, so
calling `slog.SetDefault` after `Create` still takes effect. To use a
different logger:

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "orders")
server := srvr.Create(srvr.WithLogger(logger))
```

The logger is used for:

| Message | Level | Attributes |
|---|---|---|
| `Internal server started` / `External server started` | INFO | `addr` |
| `Internal server stopped` / `External server stopped` | INFO | |
| `Internal server did not shut down cleanly; closing remaining connections` (and the external equivalent) | WARN | `err` |
| `Internal server error` / `External server error` (the server failed while running) | ERROR | `err` |
| `Failed to start server` (from `Run` only; `Start` returns the error) | ERROR | `err` |
| `Ignoring invalid duration in configuration` | WARN | `key`, `err` |
| `Readiness check failed` | WARN | `check`, `err` |
| `HTTP response sent` (one per request) | DEBUG | `requestId`, `method`, `path`, `statusCode`, `duration` |
| `log level changed` | INFO | `oldLevel`, `newLevel` |

## Request logs

Each request to a server with logging enabled (both are, by default),
including requests that match no route:

- gets a random request ID, returned in the `X-Request-Id` response header,
- gets a request-scoped logger carrying `requestId`, `method` and `path`, and
- is logged at **debug** level when it completes.

Request logs are at debug level, so with slog's default of info they don't
appear until you lower the level. Turn them off per server with
`WithInternalLogging(false)` or `WithExternalLogging(false)`.

## Logging from handlers

Get the request-scoped logger from the context:

```go
import logmiddleware "github.com/tpyle/log-middleware/v3"

func getOrder(w http.ResponseWriter, r *http.Request) {
    log := logmiddleware.SlogFromContext(r.Context())
    log.Info("loading order", "id", mux.Vars(r)["id"])
    // {"level":"INFO","msg":"loading order","requestId":"…","method":"GET","path":"/orders/7","id":"7"}
}
```

`logmiddleware.GetRequestIdFromRequest(r)` returns just the ID, for example
to pass to a downstream service.

To get request fields on plain `slog.InfoContext(ctx, …)` calls, including in
packages that don't import log-middleware, wrap your handler in
`logmiddleware.NewContextHandler`:

```go
handler := logmiddleware.NewContextHandler(slog.NewJSONHandler(os.Stdout, nil))
slog.SetDefault(slog.New(handler))
```

See log-middleware's
[Automatic Request Fields](https://github.com/tpyle/log-middleware/wiki/Automatic-Request-Fields)
page for details.

## Changing the level at runtime

slog has no global level: each handler filters on the level in its
`HandlerOptions`. To change it at runtime, create a `*slog.LevelVar`, give it
to your handler, and give the same `LevelVar` to srvr:

```go
level := new(slog.LevelVar) // starts at info
slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

server := srvr.Create(srvr.WithLogLevel(level))
```

srvr then serves the log level endpoint on the internal server (default
`/logging`, change it with `WithLogManagement`). Without `WithLogLevel` the
endpoint is not registered.

```bash
$ curl localhost:8081/logging
{"level":"info"}

$ curl -X POST localhost:8081/logging -H 'Content-Type: application/json' -d '{"level":"debug"}'
{"level":"debug"}
```

| Request | Response |
|---|---|
| `GET` | 200, `{"level":"<current>"}` |
| `POST` `{"level":"<name>"}` | 200 with the new level |
| `POST` with an unknown, missing or malformed level | 400 |
| `POST` without `Content-Type: application/json` | 415 |
| `POST` with a body over 64 KiB | 413 |
| any other method | 405 |

Accepted levels are `debug`, `info`, `warn` (or `warning`) and `error`,
case-insensitively, optionally with an offset such as `debug-4` or `info+2`.

Each change is logged at info level (`log level changed`) with the old and
new levels. A change to `warn` or `error` hides its own message.

To control several handlers at once, for example a JSON handler to stdout and
a text handler to a file, pass the same `LevelVar` to each.

The endpoint has no authentication. Keep the internal port private; see
[Configuration](Configuration.md#ports).
