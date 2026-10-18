package srvr

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// UnmatchedRoute is the route label value for requests that matched no route,
// such as those answered 404 or 405 by the router.
const UnmatchedRoute = "unmatched"

// requestMetrics holds the external server's request metrics.
type requestMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

// newRequestMetrics creates the request metrics and registers them with reg.
// If identical metrics are already registered, for example by another Server
// sharing the registry, those are reused. It panics if a different metric
// with the same name is registered.
func newRequestMetrics(reg prometheus.Registerer) *requestMetrics {
	labels := []string{"route", "method", "code"}
	return &requestMetrics{
		requests: register(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "srvr",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Requests completed by the external server.",
		}, labels)),
		duration: register(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "srvr",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Time the external server spent serving requests.",
			Buckets:   prometheus.DefBuckets,
		}, labels)),
		inFlight: register(reg, prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "srvr",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Requests the external server is serving.",
		})),
	}
}

// register registers c with reg, or returns the equivalent collector that is
// already registered.
func register[T prometheus.Collector](reg prometheus.Registerer, c T) T {
	err := reg.Register(c)
	if err == nil {
		return c
	}
	if are, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
		if existing, ok := are.ExistingCollector.(T); ok {
			return existing
		}
	}
	panic(fmt.Sprintf("srvr: registering request metrics: %v", err))
}

// instrument wraps router, which must be the handler next wraps, so that each
// request is recorded. The route label is the matched route's path template
// (e.g. "/users/{id}"), which keeps label cardinality bounded, or
// [UnmatchedRoute].
func (m *requestMetrics) instrument(router *mux.Router) http.Handler {
	router.Use(recordRoute)

	routeOpt := promhttp.WithLabelFromCtx("route", func(ctx context.Context) string {
		if rt, ok := ctx.Value(routeKey{}).(*matchedRoute); ok && rt.template != "" {
			return rt.template
		}
		return UnmatchedRoute
	})
	h := promhttp.InstrumentHandlerInFlight(m.inFlight, router)
	h = promhttp.InstrumentHandlerCounter(m.requests, h, routeOpt)
	h = promhttp.InstrumentHandlerDuration(m.duration, h, routeOpt)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, &matchedRoute{})))
	})
}

// routeLabel returns route's path template, or for routes without a path
// matcher, its name or "unnamed".
func routeLabel(route *mux.Route) string {
	if tmpl, err := route.GetPathTemplate(); err == nil {
		return tmpl
	}
	if name := route.GetName(); name != "" {
		return name
	}
	return "unnamed"
}

type routeKey struct{}

// matchedRoute carries the matched route's template out of the router. The
// router passes handlers a derived request, so the template can't be read
// from the outer request's context directly.
type matchedRoute struct {
	template string
}

// recordRoute is router middleware that stores the matched route's template.
// mux runs middleware only for matched routes, so unmatched requests leave
// the template empty.
func recordRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rt, ok := r.Context().Value(routeKey{}).(*matchedRoute); ok {
			if route := mux.CurrentRoute(r); route != nil {
				rt.template = routeLabel(route)
			}
		}
		next.ServeHTTP(w, r)
	})
}
