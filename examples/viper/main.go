// Command viper configures srvr from a YAML file and environment variables
// with github.com/spf13/viper.
//
// Settings are read from config.yaml (embedded in the binary) under the
// "api." prefix, and any of them can be overridden by an environment variable
// named after its key, e.g. API_SRVR_EXTERNALPORT for api.srvr.externalPort.
//
//	go run ./examples/viper
//	curl localhost:8080/hello
//
//	API_SRVR_EXTERNALPORT=9090 go run ./examples/viper
//	curl localhost:9090/hello
//
// Press Ctrl+C to stop.
package main

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"github.com/tpyle/srvr"
)

//go:embed config.yaml
var defaultConfig []byte

// prefix is prepended to every srvr key, so several servers (or other
// components) can share one configuration.
const prefix = "api."

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	v, err := newConfig(bytes.NewReader(defaultConfig))
	if err != nil {
		logger.Error("reading config failed", "err", err)
		os.Exit(1)
	}
	if err := run(ctx, v, logger); err != nil {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
}

// newConfig returns a Viper instance holding the YAML in r, with environment
// variables taking precedence.
func newConfig(r io.Reader) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(r); err != nil {
		return nil, err
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	return v, nil
}

// run serves until ctx is cancelled, then stops the servers.
func run(ctx context.Context, v *viper.Viper, logger *slog.Logger) error {
	router := mux.NewRouter()
	router.HandleFunc("/hello", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Hello from a Viper-configured server!\n"))
	}).Methods(http.MethodGet)

	server := srvr.Create(
		srvr.WithLogger(logger),
		srvr.WithExternalRouter(router),
		// Options after WithViperPrefix override the configuration, and
		// options before it are overridden by any key that is set.
		srvr.WithViperPrefix(v, prefix),
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
