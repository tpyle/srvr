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
//   - /logging: Log level management endpoint (requires [WithLogLevel])
//
// All logging goes through [log/slog]. By default srvr logs with [slog.Default];
// use [WithLogger] to supply a different logger.
//
// [Server.Start] binds both ports before returning, so bind errors are
// returned to the caller. [Server.Stop] and [Server.Shutdown] stop accepting
// new requests and let in-flight requests finish before returning.
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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	logmanager "github.com/tpyle/log-manager/v3"
	logmiddleware "github.com/tpyle/log-middleware/v3"
)

// ErrServerStarted is returned by [Server.Start] when the server is already running.
var ErrServerStarted = errors.New("srvr: server already started")

// ErrServerStopped is returned by [Server.Start] after the server has been
// stopped. A stopped Server cannot be restarted; create a new one.
var ErrServerStopped = errors.New("srvr: server stopped")

type state int

const (
	stateCreated state = iota
	stateStarted
	stateStopped
)

// Server represents a dual HTTP server instance with separate internal and external servers.
// The internal server handles observability endpoints (metrics, health checks, logging)
// while the external server handles application-specific API endpoints.
type Server struct {
	options        *Options
	internalServer *http.Server
	externalServer *http.Server
	waitGroup      *sync.WaitGroup

	// mu guards the fields below.
	mu               sync.Mutex
	state            state
	internalAddr     net.Addr
	externalAddr     net.Addr
	shutdownComplete chan struct{} // closed when Shutdown returns; nil until it is called
}

// logger returns the configured logger, or [slog.Default] if none was set.
// The default is looked up on every call so later calls to [slog.SetDefault]
// take effect.
func (s *Server) logger() *slog.Logger {
	return s.options.log()
}

// livenessHandler handles the liveness probe endpoint, which always returns 200 OK
// to indicate the server process is alive.
func (s *Server) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// readinessHandler handles the readiness probe endpoint, which runs all configured
// health checks and returns 200 OK if all checks pass, or 503 Service Unavailable
// if any check fails.
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	for name, check := range s.options.readinessChecks {
		if err := check(r.Context()); err != nil {
			s.logger().WarnContext(r.Context(), "Readiness check failed", "check", name, "err", err)
			http.Error(w, fmt.Sprintf("Readiness check '%s' failed: %v", name, err), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// Create initializes and returns a new Server instance with the provided options.
// If no options are provided, default values are used (internal port 8081, external port 8080).
// The server is not started automatically; call Start() or Run() to begin serving requests.
//
// Create adds routes and middleware to the routers it is given, so each
// router should be passed to only one Server.
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
		if o.logManager != nil && o.logLevel != nil {
			lm := logmanager.NewSlog(o.logLevel, logmanager.WithOnChange(logmanager.SlogChangeLogger(o.logger)))
			o.internalRouter.HandleFunc(o.logManager.Path, lm.GetLevelHandler).Methods("GET")
			o.internalRouter.HandleFunc(o.logManager.Path, lm.SetLevelHandler).Methods("POST")
		}
		if o.metricsManager != nil {
			o.internalRouter.Path(o.metricsManager.Path).Handler(promhttp.InstrumentMetricHandler(
				o.metricsRegisterer, promhttp.HandlerFor(o.metricsGatherer, promhttp.HandlerOpts{}),
			))
		}
		if o.livenessManager != nil {
			o.internalRouter.HandleFunc(o.livenessManager.Path, server.livenessHandler).Methods("GET")
		}
		if o.readinessManager != nil {
			o.internalRouter.HandleFunc(o.readinessManager.Path, server.readinessHandler).Methods("GET")
		}

		var handler http.Handler = o.internalRouter
		if o.doLogInternalRouter {
			handler = withRequestLogging(o, handler)
		}
		server.internalServer = server.newHTTPServer(o.internalPort, handler)
	}

	if o.externalRouter != nil {
		var handler http.Handler = o.externalRouter
		if o.metricsManager != nil {
			handler = newRequestMetrics(o.metricsRegisterer).instrument(o.externalRouter)
		}
		if o.doLogExternalRouter {
			handler = withRequestLogging(o, handler)
		}
		server.externalServer = server.newHTTPServer(o.externalPort, handler)
	}

	return server
}

// withRequestLogging wraps the whole handler, rather than adding router
// middleware, because mux runs middleware only for matched routes and 404
// and 405 responses should be logged too.
func withRequestLogging(o *Options, next http.Handler) http.Handler {
	return logmiddleware.NewSlog(o.logger).Handler(next)
}

func (s *Server) newHTTPServer(port int, handler http.Handler) *http.Server {
	o := s.options
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		ReadTimeout:       o.readTimeout,
		WriteTimeout:      o.writeTimeout,
		IdleTimeout:       o.idleTimeout,
		ReadHeaderTimeout: o.readHeaderTimeout,
		MaxHeaderBytes:    o.maxHeaderBytes,
	}
}

// Start binds the internal and external ports, then serves each in its own
// goroutine and returns. Use Wait() to block until the servers stop, or Run()
// to start and wait in a single call.
//
// If either port cannot be bound (for example, because it is in use), Start
// closes anything it opened and returns the error; neither server runs. It
// returns [ErrServerStarted] if the server is already running and
// [ErrServerStopped] if it has been stopped.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state {
	case stateStarted:
		return ErrServerStarted
	case stateStopped:
		return ErrServerStopped
	}

	var internalLn, externalLn net.Listener
	if s.internalServer != nil {
		ln, err := net.Listen("tcp", s.internalServer.Addr)
		if err != nil {
			return fmt.Errorf("srvr: starting internal server: %w", err)
		}
		internalLn = ln
	}
	if s.externalServer != nil {
		ln, err := net.Listen("tcp", s.externalServer.Addr)
		if err != nil {
			if internalLn != nil {
				_ = internalLn.Close()
			}
			return fmt.Errorf("srvr: starting external server: %w", err)
		}
		externalLn = ln
	}

	if internalLn != nil {
		s.internalAddr = internalLn.Addr()
		s.serve("Internal", s.internalServer, internalLn)
	}
	if externalLn != nil {
		s.externalAddr = externalLn.Addr()
		s.serve("External", s.externalServer, externalLn)
	}
	s.state = stateStarted
	return nil
}

// serve runs srv on ln in a goroutine tracked by the wait group.
func (s *Server) serve(name string, srv *http.Server, ln net.Listener) {
	s.waitGroup.Go(func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger().Error(name+" server error", "err", err)
		}
	})
	s.logger().Info(name+" server started", "addr", ln.Addr().String())
}

// InternalAddr returns the address the internal server is listening on, or
// nil if it is not running. With port 0 it reports the port that was chosen.
func (s *Server) InternalAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.internalAddr
}

// ExternalAddr returns the address the external server is listening on, or
// nil if it is not running. With port 0 it reports the port that was chosen.
func (s *Server) ExternalAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.externalAddr
}

// Run starts the HTTP servers and blocks the current goroutine until all servers stop.
// This is equivalent to calling Start() followed by Wait(). If Start fails, the
// error is logged and Run returns immediately; call Start directly to handle it.
func (s *Server) Run() {
	if err := s.Start(); err != nil {
		s.logger().Error("Failed to start server", "err", err)
		return
	}
	s.Wait()
}

// Wait blocks until both servers have stopped serving and, if Stop or
// Shutdown was called, until it has finished draining in-flight requests.
func (s *Server) Wait() {
	s.waitGroup.Wait()

	s.mu.Lock()
	done := s.shutdownComplete
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Stop gracefully shuts down both servers, waiting up to the shutdown timeout
// (see [WithShutdownTimeout], default 10 seconds) for in-flight requests to
// finish. It is [Server.Shutdown] with a context carrying that timeout.
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.options.shutdownTimeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// Shutdown gracefully shuts down both servers concurrently: they stop
// accepting connections, idle connections are closed, and Shutdown waits for
// in-flight requests to finish. If ctx ends first, the remaining connections
// are closed immediately and the context's error is returned.
//
// Both servers are always shut down; errors from each are joined. After
// Shutdown the Server cannot be started again.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.state = stateStopped
	if s.shutdownComplete == nil {
		s.shutdownComplete = make(chan struct{})
		defer close(s.shutdownComplete)
	}
	s.mu.Unlock()

	var wg sync.WaitGroup
	var internalErr, externalErr error
	if s.internalServer != nil {
		wg.Go(func() { internalErr = s.shutdown(ctx, "Internal", s.internalServer) })
	}
	if s.externalServer != nil {
		wg.Go(func() { externalErr = s.shutdown(ctx, "External", s.externalServer) })
	}
	wg.Wait()
	return errors.Join(internalErr, externalErr)
}

// shutdown drains srv until ctx ends, then closes whatever is left.
func (s *Server) shutdown(ctx context.Context, name string, srv *http.Server) error {
	err := srv.Shutdown(ctx)
	if err != nil {
		s.logger().Warn(name+" server did not shut down cleanly; closing remaining connections", "err", err)
		if closeErr := srv.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return fmt.Errorf("srvr: shutting down %s server: %w", strings.ToLower(name), err)
	}
	s.logger().Info(name + " server stopped")
	return nil
}
