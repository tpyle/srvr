# Getting Started

## Install

```bash
go get github.com/tpyle/srvr
```

srvr requires Go 1.27 or newer.

## A minimal server

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/gorilla/mux"
    "github.com/tpyle/srvr"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    level := new(slog.LevelVar)
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

    router := mux.NewRouter()
    router.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!\n"))
    }).Methods(http.MethodGet)

    server := srvr.Create(
        srvr.WithExternalRouter(router),
        srvr.WithLogLevel(level),
    )
    if err := server.Start(); err != nil {
        slog.Error("start failed", "err", err)
        os.Exit(1)
    }

    <-ctx.Done()
    if err := server.Stop(); err != nil {
        slog.Error("stop failed", "err", err)
    }
    server.Wait()
}
```

Without `WithExternalRouter`, only the internal server runs. That suits a
worker that has no API but still needs probes and metrics.

## Try it

```bash
curl localhost:8080/hello
curl localhost:8081/live
curl localhost:8081/ready
curl localhost:8081/metrics
curl localhost:8081/logging
curl -X POST localhost:8081/logging -H 'Content-Type: application/json' -d '{"level":"debug"}'
```

After the last command, requests are logged too:

```json
{"time":"…","level":"DEBUG","msg":"HTTP response sent","requestId":"0b6c…","method":"GET","path":"/hello","statusCode":200,"duration":41250}
```

Every response carries the same ID in its `X-Request-Id` header.

## Next steps

- Add [readiness checks](Health-Checks.md) for your dependencies.
- Load ports and timeouts from a [config file or the environment](Viper.md).
- Use the [request-scoped logger](Logging.md#logging-from-handlers) in handlers.
- Don't expose the internal port publicly. See [Configuration](Configuration.md#ports).
