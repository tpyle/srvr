# Migrating to slog

srvr v0.1 logged with [zerolog](https://github.com/rs/zerolog) through
`log-manager/v2` and `log-middleware/v2`. Later versions log with the standard
library's `log/slog` through `log-manager/v3` and `log-middleware/v3`, and no
longer depend on zerolog.

## Requirements

Go 1.27 or newer.

## What changes

| | v0.1 (zerolog) | Now (slog) |
|---|---|---|
| Logger | zerolog's global `log.Logger` | `slog.Default()`, or `WithLogger(l)` |
| Log level endpoint | Always served, controlled zerolog's global level | Only served with `WithLogLevel(levelVar)` |
| Level names | `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic` | `debug`, `info`, `warn`/`warning`, `error`, with optional offsets such as `debug-4` |
| Request logger in handlers | `zerolog.Ctx(r.Context())` | `logmiddleware.SlogFromContext(r.Context())` |
| Request log level | debug | debug |
| Status for handlers that never call `WriteHeader` | logged as `0` | logged as `200` |

No srvr function or option was removed or changed signature, so code that
compiles today keeps compiling. But srvr's logs now go to slog, and the
`/logging` endpoint disappears until you pass `WithLogLevel`.

## Other behavior changes in the same release

- **`Start` returns bind errors.** It binds both ports before returning, so
  "address already in use" is returned instead of only logged. Calling
  `Start` on a running or stopped server returns `ErrServerStarted` or
  `ErrServerStopped`.
- **`Stop` is graceful.** It waits up to 10 seconds (`WithShutdownTimeout`)
  for in-flight requests instead of cutting them off, so it can now take that
  long to return. `Shutdown(ctx)` is new, for a caller-controlled deadline.
- **New request metrics.** External requests were counted in
  `promhttp_metric_handler_requests_total` and `..._in_flight`, which now
  count only scrapes of `/metrics`. Use `srvr_http_requests_total`,
  `srvr_http_request_duration_seconds` and `srvr_http_requests_in_flight`,
  which have `route`, `method` and `code` labels. Update dashboards and
  alerts. See [Metrics](Metrics.md).
- **Unmatched requests are logged and counted.** 404 and 405 responses from
  the router now get a request ID, a log line and metrics.
- **Fractional and string durations in Viper.** A sub-second timeout set with
  an option before `WithViper` was truncated to 0 (no timeout) before; it is
  now kept. Durations also accept strings like `"500ms"`.
- **Started log lines** carry `addr` (e.g. `[::]:8080`) instead of `port`.

## Steps

1. Update the module:

   ```bash
   go get github.com/tpyle/srvr@latest
   go mod tidy
   ```

2. Set up slog with a `LevelVar` and pass it to srvr:

   ```diff
   -zerolog.SetGlobalLevel(zerolog.InfoLevel)
   -log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
   +level := new(slog.LevelVar)
   +slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

    server := srvr.Create(
        srvr.WithExternalRouter(router),
   +    srvr.WithLogLevel(level),
    )
   ```

3. Update handlers that use the request logger:

   ```diff
   -import "github.com/rs/zerolog"
   +import logmiddleware "github.com/tpyle/log-middleware/v3"

   -zerolog.Ctx(r.Context()).Info().Str("id", id).Msg("loading order")
   +logmiddleware.SlogFromContext(r.Context()).Info("loading order", "id", id)
   ```

4. Update scripts that set the level to `trace`, `fatal` or `panic`; slog
   rejects those names with 400. `debug-4` is the closest equivalent of
   `trace`.

## Keeping zerolog

If the rest of your program still logs with zerolog, you can send slog's
output to it with a bridge handler such as
[`slog-zerolog`](https://github.com/samber/slog-zerolog), then pass that
logger to `WithLogger`. Runtime level changes through `/logging` then need a
`LevelVar` on the bridge handler. Changing zerolog's global level has no
effect on srvr.
