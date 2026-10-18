package srvr

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newMetricsServer returns a Server using its own registry, with routes
// covering path templates, subrouters and routes without a path.
func newMetricsServer(t *testing.T, opts ...Option) (*Server, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	r := mux.NewRouter()
	r.HandleFunc("/users/{id}", func(http.ResponseWriter, *http.Request) {}).Methods(http.MethodGet)
	r.HandleFunc("/fail", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	r.PathPrefix("/api").Subrouter().HandleFunc("/items/{id}", func(http.ResponseWriter, *http.Request) {})
	r.Host("named.example").Name("named-host").HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	r.Host("anon.example").HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	l, _ := newTestLogger()
	base := []Option{WithLogger(l), WithExternalRouter(r), WithMetricsRegistry(reg)}
	return Create(append(base, opts...)...), reg
}

func serve(h http.Handler, method, target string) int {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w.Code
}

func TestRequestMetricsLabels(t *testing.T) {
	s, _ := newMetricsServer(t)
	h := s.externalServer.Handler

	tests := []struct {
		method, target string
		wantCode       int
		route, code    string
	}{
		{http.MethodGet, "/users/1", 200, "/users/{id}", "200"},
		{http.MethodGet, "/users/2", 200, "/users/{id}", "200"},
		{http.MethodGet, "/fail", 500, "/fail", "500"},
		{http.MethodGet, "/api/items/9", 200, "/api/items/{id}", "200"},
		{http.MethodGet, "http://named.example/x", 200, "named-host", "200"},
		{http.MethodGet, "http://anon.example/x", 200, "unnamed", "200"},
		{http.MethodGet, "/nope", 404, UnmatchedRoute, "404"},
		{http.MethodPost, "/users/1", 405, UnmatchedRoute, "405"},
		{"PURGE", "/nope", 404, UnmatchedRoute, "404"},
	}
	for _, tt := range tests {
		if code := serve(h, tt.method, tt.target); code != tt.wantCode {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.target, code, tt.wantCode)
		}
	}

	m := newRequestMetrics(s.options.metricsRegisterer) // returns the registered collectors
	counts := map[[3]string]float64{
		{"/users/{id}", "get", "200"}:      2,
		{"/fail", "get", "500"}:            1,
		{"/api/items/{id}", "get", "200"}:  1,
		{"named-host", "get", "200"}:       1,
		{"unnamed", "get", "200"}:          1,
		{UnmatchedRoute, "get", "404"}:     1,
		{UnmatchedRoute, "post", "405"}:    1,
		{UnmatchedRoute, "unknown", "404"}: 1,
	}
	for labels, want := range counts {
		if got := testutil.ToFloat64(m.requests.WithLabelValues(labels[:]...)); got != want {
			t.Errorf("requests_total%v = %v, want %v", labels, got, want)
		}
	}
	if got := testutil.CollectAndCount(m.duration); got != len(counts) {
		t.Errorf("duration series = %d, want %d", got, len(counts))
	}
	if got := testutil.ToFloat64(m.inFlight); got != 0 {
		t.Errorf("requests_in_flight = %v, want 0", got)
	}
}

func TestMetricsEndpointServesRegistry(t *testing.T) {
	s, _ := newMetricsServer(t)
	serve(s.externalServer.Handler, http.MethodGet, "/users/1")

	w := httptest.NewRecorder()
	s.internalServer.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()

	if !strings.Contains(body, `srvr_http_requests_total{code="200",method="get",route="/users/{id}"} 1`) {
		t.Errorf("request counter missing from /metrics:\n%s", body)
	}
	// The scrape counter counts scrapes only; this is the first one, recorded
	// after it is served.
	if !strings.Contains(body, `promhttp_metric_handler_requests_total{code="200"} 0`) {
		t.Errorf("scrape counter should not count API requests:\n%s", body)
	}
	if strings.Contains(body, "go_goroutines") {
		t.Error("a custom registry should not include the default Go collector")
	}
}

func TestInternalRequestsAreNotCounted(t *testing.T) {
	s, _ := newMetricsServer(t)
	serve(s.internalServer.Handler, http.MethodGet, "/live")
	m := newRequestMetrics(s.options.metricsRegisterer)
	if got := testutil.CollectAndCount(m.requests); got != 0 {
		t.Errorf("internal requests produced %d series", got)
	}
}

func TestServersShareRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	l, _ := newTestLogger()
	newServer := func() *Server {
		r := mux.NewRouter()
		r.HandleFunc("/x", func(http.ResponseWriter, *http.Request) {})
		return Create(WithLogger(l), WithExternalRouter(r), WithMetricsRegistry(reg))
	}
	a, b := newServer(), newServer()
	serve(a.externalServer.Handler, http.MethodGet, "/x")
	serve(b.externalServer.Handler, http.MethodGet, "/x")

	m := newRequestMetrics(reg)
	if got := testutil.ToFloat64(m.requests.WithLabelValues("/x", "get", "200")); got != 2 {
		t.Errorf("shared counter = %v, want 2", got)
	}
}

func TestConflictingMetricPanics(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewCounter(prometheus.CounterOpts{Name: "srvr_http_requests_total", Help: "conflict"}))

	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "srvr: registering request metrics") {
			t.Errorf("recover() = %v, want a srvr registration panic", r)
		}
	}()
	Create(WithExternalRouter(mux.NewRouter()), WithMetricsRegistry(reg))
}

func TestWithMetricsRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	opts := defaultOptions()
	if opts.metricsRegisterer != prometheus.DefaultRegisterer || opts.metricsGatherer != prometheus.DefaultGatherer {
		t.Fatal("expected the default registry by default")
	}
	WithMetricsRegistry(reg)(opts)
	if opts.metricsRegisterer != reg || opts.metricsGatherer != reg {
		t.Error("expected the custom registry")
	}
	WithMetricsRegistry(nil)(opts)
	if opts.metricsRegisterer != prometheus.DefaultRegisterer || opts.metricsGatherer != prometheus.DefaultGatherer {
		t.Error("nil should restore the default registry")
	}
}

func TestDefaultRegistryIncludesRequestMetrics(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/default-registry-test", func(http.ResponseWriter, *http.Request) {})
	l, _ := newTestLogger()
	s := Create(WithLogger(l), WithExternalRouter(r))
	serve(s.externalServer.Handler, http.MethodGet, "/default-registry-test")

	w := httptest.NewRecorder()
	s.internalServer.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	for _, want := range []string{`route="/default-registry-test"`, "go_goroutines"} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics missing %q", want)
		}
	}
}

func TestMetricsDisabledWithoutMetricsManager(t *testing.T) {
	reg := prometheus.NewRegistry()
	r := mux.NewRouter()
	r.HandleFunc("/x", func(http.ResponseWriter, *http.Request) {})
	opts := []Option{WithExternalRouter(r), WithMetricsRegistry(reg), func(o *Options) { o.metricsManager = nil }}
	s := Create(opts...)
	serve(s.externalServer.Handler, http.MethodGet, "/x")
	if n, err := testutil.GatherAndCount(reg); err != nil || n != 0 {
		t.Errorf("GatherAndCount() = %d, %v; want no metrics", n, err)
	}
}
