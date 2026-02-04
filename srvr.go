// Package srvr provides a dual-server HTTP architecture with built-in observability,
// health checks, and structured logging for production-ready Go applications.
//
// The library creates two separate HTTP servers:
//   - External Server: For public API endpoints (default port 8080)
//   - Internal Server: For observability and management endpoints (default port 8081)
//
// The internal server automatically provides:
//   - /live: Liveness probe endpoint
//   - /ready: Readiness probe endpoint with health checks
//   - /metrics: Prometheus metrics endpoint
//   - /logging: Log level management endpoint
//
// Example usage:
//
//	router := mux.NewRouter()
//	router.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("Hello, World!"))
//	})
//
//	server := srvr.Create(
//		srvr.WithExternalRouter(router),
//		srvr.WithReadTimeout(30 * time.Second),
//	)
//
//	if err := server.Start(); err != nil {
//		panic(err)
//	}
//	server.Wait()
package srvr

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	logmanager "github.com/tpyle/log-manager/v2"
	logmiddleware "github.com/tpyle/log-middleware/v2"
)

// Server represents a dual HTTP server instance with separate internal and external servers.
// The internal server handles observability endpoints (metrics, health checks, logging)
// while the external server handles application-specific API endpoints.
type Server struct {
	options        *Options
	internalServer *http.Server
	externalServer *http.Server
	waitGroup      *sync.WaitGroup
}

// livenessHandler handles the liveness probe endpoint, which always returns 200 OK
// to indicate the server process is alive.
func (s *Server) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readinessHandler handles the readiness probe endpoint, which runs all configured
// health checks and returns 200 OK if all checks pass, or 503 Service Unavailable
// if any check fails.
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	for name, check := range s.options.readinessChecks {
		if err := check(r.Context()); err != nil {
			log.Warn().Err(err).Msgf("Readiness check '%s' failed", name)
			http.Error(w, fmt.Sprintf("Readiness check '%s' failed: %v", name, err), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Create initializes and returns a new Server instance with the provided options.
// If no options are provided, default values are used (internal port 8081, external port 8080).
// The server is not started automatically; call Start() or Run() to begin serving requests.
//
// Example:
//
//	server := srvr.Create(
//		srvr.WithExternalPort(3000),
//		srvr.WithExternalRouter(myRouter),
//	)
func Create(opts ...Option) *Server {
	o := defaultOptions()

	for _, opt := range opts {
		opt(o)
	}

	server := &Server{
		options:   o,
		waitGroup: &sync.WaitGroup{},
	}

	if o.internalRouter != nil {
		if o.doLogInternalRouter {
			o.internalRouter.Use(logmiddleware.NewLogMiddleware(&log.Logger).Handler)
		}

		if o.logManager != nil {
			o.internalRouter.HandleFunc(o.logManager.Path, logmanager.HandleLogCall).Methods("POST", "GET")
		}
		if o.metricsManager != nil {
			o.internalRouter.Path(o.metricsManager.Path).Handler(promhttp.Handler())
		}
		if o.livenessManager != nil {
			o.internalRouter.HandleFunc(o.livenessManager.Path, server.livenessHandler).Methods("GET")
		}
		if o.readinessManager != nil {
			o.internalRouter.HandleFunc(o.readinessManager.Path, server.readinessHandler).Methods("GET")
		}

		server.internalServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", o.internalPort),
			Handler:           o.internalRouter,
			ReadTimeout:       o.readTimeout,
			WriteTimeout:      o.writeTimeout,
			IdleTimeout:       o.idleTimeout,
			ReadHeaderTimeout: o.readHeaderTimeout,
			MaxHeaderBytes:    o.maxHeaderBytes,
		}
	}

	if o.externalRouter != nil {
		if o.doLogExternalRouter {
			o.externalRouter.Use(logmiddleware.NewLogMiddleware(&log.Logger).Handler)
		}

		if o.metricsManager != nil {
			o.externalRouter.Use(func(next http.Handler) http.Handler {
				return promhttp.InstrumentMetricHandler(
					prometheus.DefaultRegisterer, next,
				)
			})
		}

		server.externalServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", o.externalPort),
			Handler:           o.externalRouter,
			ReadTimeout:       o.readTimeout,
			WriteTimeout:      o.writeTimeout,
			IdleTimeout:       o.idleTimeout,
			ReadHeaderTimeout: o.readHeaderTimeout,
			MaxHeaderBytes:    o.maxHeaderBytes,
		}
	}

	return server
}

// Start begins serving HTTP requests on both the internal and external servers in separate goroutines.
// This method returns immediately after starting the servers. Use Wait() to block until servers stop,
// or use Run() if you want to start and wait in a single call.
//
// Returns an error if the servers cannot be started (though currently always returns nil).
func (s *Server) Start() error {
	if s.internalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Internal server error")
			}
		})
		log.Info().Msgf("Internal server started on port %d", s.options.internalPort)
	}

	if s.externalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.externalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("External server error")
			}
		})
		log.Info().Msgf("External server started on port %d", s.options.externalPort)
	}

	return nil
}

// Run starts the HTTP servers and blocks the current goroutine until all servers stop.
// This is equivalent to calling Start() followed by Wait(). Use this method when you
// want the main goroutine to wait for the servers to finish.
func (s *Server) Run() {
	if s.internalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Internal server error")
			}
		})
		log.Info().Msgf("Internal server started on port %d", s.options.internalPort)
	}

	if s.externalServer != nil {
		s.waitGroup.Go(func() {
			if err := s.externalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("External server error")
			}
		})
		log.Info().Msgf("External server started on port %d", s.options.externalPort)
	}

	s.waitGroup.Wait()
}

// Wait blocks until all server goroutines have completed.
// This is useful when Start() has been called and you want to wait for the servers to stop.
func (s *Server) Wait() {
	s.waitGroup.Wait()
}

// Stop gracefully shuts down both the internal and external servers.
// It closes all active connections and stops accepting new requests.
// Returns an error if either server fails to shut down properly.
func (s *Server) Stop() error {
	if s.internalServer != nil {
		if err := s.internalServer.Close(); err != nil {
			return err
		}
		log.Info().Msg("Internal server stopped")
	}

	if s.externalServer != nil {
		if err := s.externalServer.Close(); err != nil {
			return err
		}
		log.Info().Msg("External server stopped")
	}

	return nil
}
