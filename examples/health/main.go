// Command health shows readiness checks, custom probe paths, and extra routes
// on the internal server.
//
// A simulated database becomes reachable a few seconds after startup, so the
// readiness probe fails at first and then passes. Liveness passes throughout:
// the process is healthy, it just can't serve traffic yet.
//
//	go run ./examples/health
//	curl -i localhost:8081/healthz/live
//	curl -i localhost:8081/healthz/ready   # 503 at first, 200 after ~5s
//	curl localhost:8081/admin/status
//	curl localhost:8080/items
//
// Press Ctrl+C to stop.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/tpyle/srvr"
)

// database stands in for a real dependency, such as a *sql.DB whose
// PingContext would be used as the readiness check.
type database struct {
	up atomic.Bool
}

// errUnavailable is returned by Ping while the database is down.
var errUnavailable = errors.New("database unavailable")

// Ping reports whether the database is reachable.
func (d *database) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !d.up.Load() {
		return errUnavailable
	}
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db := &database{}
	time.AfterFunc(5*time.Second, func() {
		db.up.Store(true)
		slog.Info("database is up")
	})

	if err := run(ctx, db, 8080, 8081); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// run serves until ctx is cancelled, then stops the servers.
func run(ctx context.Context, db *database, externalPort, internalPort int) error {
	api := mux.NewRouter()
	api.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`["apple","banana"]` + "\n"))
	}).Methods(http.MethodGet)

	// Routes added here are served next to srvr's built-in endpoints, on the
	// internal port only.
	internal := mux.NewRouter()
	internal.HandleFunc("/admin/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "database up: %t\n", db.up.Load())
	}).Methods(http.MethodGet)

	server := srvr.Create(
		srvr.WithExternalRouter(api),
		srvr.WithInternalRouter(internal),
		srvr.WithExternalPort(externalPort),
		srvr.WithInternalPort(internalPort),
		srvr.WithLivenessManagement("/healthz/live"),
		srvr.WithReadinessManagement("/healthz/ready"),
		srvr.WithReadinessCheck("database", func(ctx context.Context) error {
			// Bound each check so a hung dependency can't hang the probe.
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			return db.Ping(ctx)
		}),
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
