package srvr

import (
	"context"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

// Option is a functional option for configuring a Server instance.
// Options are passed to the Create function to customize server behavior.
type Option func(*Options)

// ReadinessCheckFunc is a function that performs a health check.
// It receives a context and returns an error if the check fails.
// Readiness checks are executed by the /ready endpoint.
type ReadinessCheckFunc func(context.Context) error

// LogManager configures the log level management endpoint.
type LogManager struct {
	// Path is the HTTP path where the log management endpoint will be served.
	Path string
}

// MetricsManager configures the Prometheus metrics endpoint.
type MetricsManager struct {
	// Path is the HTTP path where Prometheus metrics will be exposed.
	Path string
}

// LivenessManager configures the liveness probe endpoint.
type LivenessManager struct {
	// Path is the HTTP path for the liveness probe.
	Path string
}

// ReadinessManager configures the readiness probe endpoint.
type ReadinessManager struct {
	// Path is the HTTP path for the readiness probe.
	// Path is the HTTP path for the readiness probe.
	Path string
}

// Options holds all configuration options for a Server instance.
// Use the With* functions to set these values via the functional options pattern.
type Options struct {
	internalPort int
	externalPort int

	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
	readHeaderTimeout time.Duration
	maxHeaderBytes    int

	logManager *LogManager

	metricsManager *MetricsManager

	livenessManager  *LivenessManager
	readinessManager *ReadinessManager

	internalRouter      *mux.Router
	doLogInternalRouter bool
	externalRouter      *mux.Router
	doLogExternalRouter bool
	readinessChecks     map[string]ReadinessCheckFunc

	viperRef *viper.Viper // Optional reference to a Viper instance for config from viper
}

func defaultOptions() *Options {
	return &Options{
		internalPort:        8081,
		externalPort:        8080,
		readTimeout:         5 * time.Second,
		writeTimeout:        10 * time.Second,
		idleTimeout:         120 * time.Second,
		readHeaderTimeout:   2 * time.Second,
		maxHeaderBytes:      1 << 20, // 1 MB
		logManager:          &LogManager{Path: "/logging"},
		metricsManager:      &MetricsManager{Path: "/metrics"},
		livenessManager:     &LivenessManager{Path: "/live"},
		readinessManager:    &ReadinessManager{Path: "/ready"},
		readinessChecks:     map[string]ReadinessCheckFunc{},
		doLogInternalRouter: true,
		doLogExternalRouter: true,
		viperRef:            viper.GetViper(),
		internalRouter:      mux.NewRouter(),
	}
}

// WithReadinessCheck adds a named health check to the readiness probe.
// The check function will be called when the /ready endpoint is accessed.
// If any check returns an error, the readiness endpoint will return 503.
//
// Example:
//
//	srvr.WithReadinessCheck("database", func(ctx context.Context) error {
//		return db.PingContext(ctx)
//	})
func WithReadinessCheck(name string, check ReadinessCheckFunc) Option {
	return func(o *Options) {
		if o.readinessChecks == nil {
			o.readinessChecks = make(map[string]ReadinessCheckFunc)
		}
		o.readinessChecks[name] = check
	}
}

// WithInternalPort sets the port for the internal server.
// The internal server hosts observability endpoints (metrics, health checks, logging).
// Default: 8081
func WithInternalPort(port int) Option {
	return func(o *Options) {
		o.internalPort = port
	}
}

// WithExternalPort sets the port for the external server.
// The external server hosts your application's API endpoints.
// Default: 8080
func WithExternalPort(port int) Option {
	return func(o *Options) {
		o.externalPort = port
	}
}

// WithInternalRouter sets a custom router for the internal server.
// The internal server will still automatically register observability endpoints.
// If not provided, a default router will be created.
func WithInternalRouter(r *mux.Router) Option {
	return func(o *Options) {
		o.internalRouter = r
	}
}

// WithExternalRouter sets the router for the external server.
// This router should contain your application's API endpoints.
func WithExternalRouter(r *mux.Router) Option {
	return func(o *Options) {
		o.externalRouter = r
	}
}

// WithReadTimeout sets the maximum duration for reading the entire request,
// including the body. Default: 5 seconds
func WithReadTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.readTimeout = d
	}
}

// WithWriteTimeout sets the maximum duration before timing out writes of the response.
// Default: 10 seconds
func WithWriteTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.writeTimeout = d
	}
}

// WithIdleTimeout sets the maximum amount of time to wait for the next request
// when keep-alives are enabled. Default: 120 seconds
func WithIdleTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.idleTimeout = d
	}
}

// WithReadHeaderTimeout sets the amount of time allowed to read request headers.
// Default: 2 seconds
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.readHeaderTimeout = d
	}
}

// WithMaxHeaderBytes sets the maximum number of bytes the server will read
// parsing the request header's keys and values. Default: 1 MB
func WithMaxHeaderBytes(n int) Option {
	return func(o *Options) {
		o.maxHeaderBytes = n
	}
}

// WithLogManagement configures the log level management endpoint path.
// This endpoint allows runtime adjustment of log levels.
// Default: "/logging"
func WithLogManagement(route string) Option {
	return func(o *Options) {
		o.logManager = &LogManager{
			route,
		}
	}
}

// WithMetricsRoute configures the Prometheus metrics endpoint path.
// Default: "/metrics"
func WithMetricsRoute(route string) Option {
	return func(o *Options) {
		o.metricsManager = &MetricsManager{
			route,
		}
	}
}

// WithInternalLogging enables or disables request logging middleware
// for the internal server. Default: true
func WithInternalLogging(enabled bool) Option {
	return func(o *Options) {
		o.doLogInternalRouter = enabled
	}
}

// WithExternalLogging enables or disables request logging middleware
// for the external server. Default: true
func WithExternalLogging(enabled bool) Option {
	return func(o *Options) {
		o.doLogExternalRouter = enabled
	}
}

// WithLivenessManagement configures the liveness probe endpoint path.
// The liveness probe always returns 200 OK when the server is running.
// Default: "/live"
func WithLivenessManagement(route string) Option {
	return func(o *Options) {
		o.livenessManager = &LivenessManager{
			route,
		}
	}
}

// WithReadinessManagement configures the readiness probe endpoint path.
// The readiness probe runs health checks before returning status.
// Default: "/ready"
func WithReadinessManagement(route string) Option {
	return func(o *Options) {
		o.readinessManager = &ReadinessManager{
			route,
		}
	}
}

// WithViper configures the server to read settings from a Viper instance.
// This is equivalent to calling WithViperPrefix with an empty prefix.
// Configuration keys will be prefixed with "srvr." (e.g., "srvr.internalPort").
func WithViper(viper *viper.Viper) Option {
	return WithViperPrefix(viper, "")
}

// WithViperPrefix configures the server to read settings from a Viper instance
// with a custom prefix. This allows multiple server instances to be configured
// from the same Viper instance with different prefixes.
//
// Configuration keys will be: prefix + "srvr." + setting name
// For example, with prefix "api.", the internal port key would be "api.srvr.internalPort"
//
// Supported configuration keys:
//   - srvr.internalPort (int)
//   - srvr.externalPort (int)
//   - srvr.readTimeout (seconds as int)
//   - srvr.writeTimeout (seconds as int)
//   - srvr.idleTimeout (seconds as int)
//   - srvr.readHeaderTimeout (seconds as int)
//   - srvr.maxHeaderBytes (int)
//   - srvr.doLogInternalRouter (bool)
//   - srvr.doLogExternalRouter (bool)
func WithViperPrefix(viper *viper.Viper, prefix string) Option {
	return func(o *Options) {
		o.viperRef = viper

		if viper != nil {
			viper.SetDefault(prefix+"srvr.internalPort", o.internalPort)
			viper.SetDefault(prefix+"srvr.externalPort", o.externalPort)
			viper.SetDefault(prefix+"srvr.readTimeout", o.readTimeout.Seconds())
			viper.SetDefault(prefix+"srvr.writeTimeout", o.writeTimeout.Seconds())
			viper.SetDefault(prefix+"srvr.idleTimeout", o.idleTimeout.Seconds())
			viper.SetDefault(prefix+"srvr.readHeaderTimeout", o.readHeaderTimeout.Seconds())
			viper.SetDefault(prefix+"srvr.maxHeaderBytes", o.maxHeaderBytes)
			viper.SetDefault(prefix+"srvr.doLogInternalRouter", o.doLogInternalRouter)
			viper.SetDefault(prefix+"srvr.doLogExternalRouter", o.doLogExternalRouter)

			o.internalPort = viper.GetInt(prefix + "srvr.internalPort")
			o.externalPort = viper.GetInt(prefix + "srvr.externalPort")
			o.readTimeout = time.Duration(viper.GetInt(prefix+"srvr.readTimeout")) * time.Second
			o.writeTimeout = time.Duration(viper.GetInt(prefix+"srvr.writeTimeout")) * time.Second
			o.idleTimeout = time.Duration(viper.GetInt(prefix+"srvr.idleTimeout")) * time.Second
			o.readHeaderTimeout = time.Duration(viper.GetInt(prefix+"srvr.readHeaderTimeout")) * time.Second
			o.maxHeaderBytes = viper.GetInt(prefix + "srvr.maxHeaderBytes")
			o.doLogInternalRouter = viper.GetBool(prefix + "srvr.doLogInternalRouter")
			o.doLogExternalRouter = viper.GetBool(prefix + "srvr.doLogExternalRouter")
		}
	}
}
