// Command basic runs an API on srvr's external server and the built-in
// observability endpoints on its internal server. It logs JSON with log/slog
// and lets the log level be changed at runtime.
//
//	go run ./examples/basic
//	curl 'localhost:8080/hello?name=gopher'
//	curl localhost:8081/live
//	curl localhost:8081/ready
//	curl localhost:8081/metrics
//	curl localhost:8081/logging
//	curl -X POST localhost:8081/logging -H 'Content-Type: application/json' -d '{"level":"debug"}'
//	curl 'localhost:8080/hello?name=gopher'   # request logs now appear too
//
// Press Ctrl+C to stop.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
	logmiddleware "github.com/tpyle/log-middleware/v3"
	"github.com/tpyle/srvr"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The same LevelVar must be given to the handler and to srvr, so that
	// the /logging endpoint changes the level this handler filters on.
	level := new(slog.LevelVar)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	if err := run(ctx, logger, level, 8080, 8081); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// run serves until ctx is cancelled, then stops the servers.
func run(ctx context.Context, logger *slog.Logger, level *slog.LevelVar, externalPort, internalPort int) error {
	router := mux.NewRouter()
	router.HandleFunc("/hello", hello).Methods(http.MethodGet)

	server := srvr.Create(
		srvr.WithExternalRouter(router),
		srvr.WithExternalPort(externalPort),
		srvr.WithInternalPort(internalPort),
		srvr.WithLogger(logger),
		srvr.WithLogLevel(level),
	)
	if err := server.Start(); err != nil {
		return err
	}

	<-ctx.Done()
	if err := server.Stop(); err != nil {
		return err
	}
	server.Wait()
	return nil
}

// hello greets the caller. The logger from SlogFromContext already carries
// the requestId, method and path added by srvr's request logging middleware.
func hello(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}
	logmiddleware.SlogFromContext(r.Context()).Info("saying hello", "name", name)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "Hello, %s!\n", name)
}
