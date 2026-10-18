package srvr

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cast"
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
//
// GET on Path responds with {"level":"<current>"}; POST with a JSON body
// {"level":"<new>"} and Content-Type application/json changes it. The
// endpoint is only registered when a level is supplied with [WithLogLevel].
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
	shutdownTimeout   time.Duration

	logger     *slog.Logger   // nil means slog.Default, resolved at log time
	logLevel   *slog.LevelVar // level controlled by the log management endpoint
	logManager *LogManager

	metricsManager *MetricsManager
	// metricsRegisterer receives srvr's metrics; metricsGatherer is served
	// by the metrics endpoint.
	metricsRegisterer prometheus.Registerer
	metricsGatherer   prometheus.Gatherer

	livenessManager  *LivenessManager
	readinessManager *ReadinessManager

	internalRouter      *mux.Router
	doLogInternalRouter bool
	externalRouter      *mux.Router
	doLogExternalRouter bool
	readinessChecks     map[string]ReadinessCheckFunc
}

// log returns the configured logger, or [slog.Default] if none is set yet.
func (o *Options) log() *slog.Logger {
	if o.logger != nil {
		return o.logger
	}
	return slog.Default()
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
		shutdownTimeout:     10 * time.Second,
		logManager:          &LogManager{Path: "/logging"},
		metricsManager:      &MetricsManager{Path: "/metrics"},
		metricsRegisterer:   prometheus.DefaultRegisterer,
		metricsGatherer:     prometheus.DefaultGatherer,
		livenessManager:     &LivenessManager{Path: "/live"},
		readinessManager:    &ReadinessManager{Path: "/ready"},
		readinessChecks:     map[string]ReadinessCheckFunc{},
		doLogInternalRouter: true,
		doLogExternalRouter: true,
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

// WithShutdownTimeout sets how long [Server.Stop] waits for in-flight
// requests to finish before closing their connections. Default: 10 seconds
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.shutdownTimeout = d
	}
}

// WithMetricsRegistry registers srvr's request metrics with r and serves r,
// instead of the Prometheus default registry, on the metrics endpoint.
//
// A new [prometheus.Registry] has no Go runtime or process collectors; add
// them with the collectors package if you want them. Passing nil restores the
// default registry.
func WithMetricsRegistry(r *prometheus.Registry) Option {
	return func(o *Options) {
		if r == nil {
			o.metricsRegisterer = prometheus.DefaultRegisterer
			o.metricsGatherer = prometheus.DefaultGatherer
			return
		}
		o.metricsRegisterer = r
		o.metricsGatherer = r
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

// WithLogger sets the logger used for srvr's own messages, request logging
// middleware, and log level change messages. If not set (or set to nil),
// [slog.Default] is used, looked up each time a message is logged so later
// calls to [slog.SetDefault] take effect.
func WithLogger(l *slog.Logger) Option {
	return func(o *Options) {
		o.logger = l
	}
}

// WithLogLevel sets the level controlled by the log level management endpoint.
//
// slog has no global level, so v must be the same LevelVar passed as
// [slog.HandlerOptions.Level] to the handler(s) whose level should change at
// runtime. Without this option the log management endpoint is not registered.
//
// Example:
//
//	level := new(slog.LevelVar)
//	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
//	server := srvr.Create(srvr.WithLogLevel(level))
func WithLogLevel(v *slog.LevelVar) Option {
	return func(o *Options) {
		o.logLevel = v
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
// from the same Viper instance with different prefixes. Viper is only read
// when this option (or [WithViper]) is given; nothing is read from the global
// Viper instance otherwise.
//
// Configuration keys will be: prefix + "srvr." + setting name
// For example, with prefix "api.", the internal port key would be "api.srvr.internalPort"
//
// Supported configuration keys:
//   - srvr.internalPort (int)
//   - srvr.externalPort (int)
//   - srvr.readTimeout (duration)
//   - srvr.writeTimeout (duration)
//   - srvr.idleTimeout (duration)
//   - srvr.readHeaderTimeout (duration)
//   - srvr.shutdownTimeout (duration)
//   - srvr.maxHeaderBytes (int)
//   - srvr.doLogInternalRouter (bool)
//   - srvr.doLogExternalRouter (bool)
//
// Durations are a number of seconds, which may be fractional (15, 0.5), or a
// Go duration string ("1m30s", "500ms"). An invalid duration is logged and
// ignored.
//
// The current option values are registered as Viper defaults, so options
// given before this one apply unless the key is set, and options given after
// it override Viper.
func WithViperPrefix(viper *viper.Viper, prefix string) Option {
	return func(o *Options) {
		if viper == nil {
			return
		}
		key := func(name string) string { return prefix + "srvr." + name }

		viper.SetDefault(key("internalPort"), o.internalPort)
		viper.SetDefault(key("externalPort"), o.externalPort)
		viper.SetDefault(key("maxHeaderBytes"), o.maxHeaderBytes)
		viper.SetDefault(key("doLogInternalRouter"), o.doLogInternalRouter)
		viper.SetDefault(key("doLogExternalRouter"), o.doLogExternalRouter)

		o.internalPort = viper.GetInt(key("internalPort"))
		o.externalPort = viper.GetInt(key("externalPort"))
		o.maxHeaderBytes = viper.GetInt(key("maxHeaderBytes"))
		o.doLogInternalRouter = viper.GetBool(key("doLogInternalRouter"))
		o.doLogExternalRouter = viper.GetBool(key("doLogExternalRouter"))

		for name, d := range map[string]*time.Duration{
			"readTimeout":       &o.readTimeout,
			"writeTimeout":      &o.writeTimeout,
			"idleTimeout":       &o.idleTimeout,
			"readHeaderTimeout": &o.readHeaderTimeout,
			"shutdownTimeout":   &o.shutdownTimeout,
		} {
			// Registered in seconds, the unit used in config files.
			viper.SetDefault(key(name), d.Seconds())
			parsed, err := parseDuration(viper.Get(key(name)))
			if err != nil {
				o.log().Warn("Ignoring invalid duration in configuration", "key", key(name), "err", err)
				continue
			}
			*d = parsed
		}
	}
}

// parseDuration converts a configuration value to a duration. Numbers and
// numeric strings are seconds; other strings are parsed with
// [time.ParseDuration].
func parseDuration(v any) (time.Duration, error) {
	switch v := v.(type) {
	case time.Duration:
		return v, nil
	case string:
		v = strings.TrimSpace(v)
		if secs, err := strconv.ParseFloat(v, 64); err == nil {
			return secondsToDuration(secs), nil
		}
		return time.ParseDuration(v)
	}
	secs, err := cast.ToFloat64E(v)
	if err != nil {
		return 0, err
	}
	return secondsToDuration(secs), nil
}

func secondsToDuration(secs float64) time.Duration {
	return time.Duration(secs * float64(time.Second))
}
