package srvr

import (
	"context"
	"time"

	"github.com/gorilla/mux"
)

type Option func(*Options)

type ReadinessCheckFunc func(context.Context) error

type LogManager struct {
	Path string
}

type MetricsManager struct {
	Path string
}

type LivenessManager struct {
	Path string
}

type ReadinessManager struct {
	Path string
}

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
	}
}

func WithReadinessCheck(name string, check ReadinessCheckFunc) Option {
	return func(o *Options) {
		if o.readinessChecks == nil {
			o.readinessChecks = make(map[string]ReadinessCheckFunc)
		}
		o.readinessChecks[name] = check
	}
}

func WithInternalPort(port int) Option {
	return func(o *Options) {
		o.internalPort = port
	}
}

func WithExternalPort(port int) Option {
	return func(o *Options) {
		o.externalPort = port
	}
}

func WithInternalRouter(r *mux.Router) Option {
	return func(o *Options) {
		o.internalRouter = r
	}
}

func WithExternalRouter(r *mux.Router) Option {
	return func(o *Options) {
		o.externalRouter = r
	}
}

func WithReadTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.readTimeout = d
	}
}

func WithWriteTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.writeTimeout = d
	}
}

func WithIdleTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.idleTimeout = d
	}
}

func WithReadHeaderTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.readHeaderTimeout = d
	}
}

func WithMaxHeaderBytes(n int) Option {
	return func(o *Options) {
		o.maxHeaderBytes = n
	}
}

func WithLogManagement(route string) Option {
	return func(o *Options) {
		o.logManager = &LogManager{
			route,
		}
	}
}

func WithMetricsRoute(route string) Option {
	return func(o *Options) {
		o.metricsManager = &MetricsManager{
			route,
		}
	}
}

func WithInternalLogging(enabled bool) Option {
	return func(o *Options) {
		o.doLogInternalRouter = enabled
	}
}

func WithExternalLogging(enabled bool) Option {
	return func(o *Options) {
		o.doLogExternalRouter = enabled
	}
}

func WithLivenessManagement(route string) Option {
	return func(o *Options) {
		o.livenessManager = &LivenessManager{
			route,
		}
	}
}

func WithReadinessManagement(route string) Option {
	return func(o *Options) {
		o.readinessManager = &ReadinessManager{
			route,
		}
	}
}
