package srvr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// newTestLogger returns a JSON logger writing to a buffer at debug level.
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

// logRecords decodes every JSON log line in buf.
func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("invalid log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

// findRecord returns the first record with the given message, or nil.
func findRecord(records []map[string]any, msg string) map[string]any {
	for _, r := range records {
		if r["msg"] == msg {
			return r
		}
	}
	return nil
}

func TestWithLogger(t *testing.T) {
	l, _ := newTestLogger()
	opts := defaultOptions()
	WithLogger(l)(opts)
	if opts.logger != l {
		t.Error("Expected logger to be set")
	}
	WithLogger(nil)(opts)
	if opts.logger != nil {
		t.Error("Expected logger to be cleared")
	}
}

func TestWithLogLevel(t *testing.T) {
	opts := defaultOptions()
	if opts.logLevel != nil {
		t.Error("Expected no default log level")
	}
	v := new(slog.LevelVar)
	WithLogLevel(v)(opts)
	if opts.logLevel != v {
		t.Error("Expected log level to be set")
	}
}

func TestServerLoggerDefaultsToSlogDefault(t *testing.T) {
	l, _ := newTestLogger()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(l)

	s := Create()
	if s.logger() != l {
		t.Error("Expected slog.Default to be used when no logger is set")
	}

	custom, _ := newTestLogger()
	s = Create(WithLogger(custom))
	if s.logger() != custom {
		t.Error("Expected configured logger to be used")
	}
}

func TestLogManagementEndpoint(t *testing.T) {
	l, buf := newTestLogger()
	level := new(slog.LevelVar)
	s := Create(WithLogger(l), WithLogLevel(level), WithInternalLogging(false))
	h := s.internalServer.Handler

	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		wantStatus  int
		wantLevel   string
	}{
		{"get current", http.MethodGet, "", "", http.StatusOK, "info"},
		{"set debug", http.MethodPost, "application/json", `{"level":"debug"}`, http.StatusOK, "debug"},
		{"get after set", http.MethodGet, "", "", http.StatusOK, "debug"},
		{"set warning alias", http.MethodPost, "application/json", `{"level":"WARNING"}`, http.StatusOK, "warn"},
		{"unknown level", http.MethodPost, "application/json", `{"level":"loud"}`, http.StatusBadRequest, ""},
		{"missing level", http.MethodPost, "application/json", `{}`, http.StatusBadRequest, ""},
		{"bad json", http.MethodPost, "application/json", `{`, http.StatusBadRequest, ""},
		{"wrong content type", http.MethodPost, "text/plain", `{"level":"error"}`, http.StatusUnsupportedMediaType, ""},
		{"wrong method", http.MethodPut, "application/json", `{"level":"error"}`, http.StatusMethodNotAllowed, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/logging", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("Expected status %d, got %d (%s)", tt.wantStatus, w.Code, w.Body.String())
			}
			if tt.wantLevel == "" {
				return
			}
			var msg struct{ Level string }
			if err := json.Unmarshal(w.Body.Bytes(), &msg); err != nil {
				t.Fatalf("invalid response body %q: %v", w.Body.String(), err)
			}
			if msg.Level != tt.wantLevel {
				t.Errorf("Expected level %q, got %q", tt.wantLevel, msg.Level)
			}
		})
	}

	if level.Level() != slog.LevelWarn {
		t.Errorf("Expected LevelVar to be warn, got %v", level.Level())
	}
	rec := findRecord(logRecords(t, buf), "log level changed")
	if rec == nil {
		t.Fatal("Expected level change to be logged with the configured logger")
	}
	if rec["oldLevel"] != "info" || rec["newLevel"] != "debug" {
		t.Errorf("Unexpected change record: %v", rec)
	}
}

func TestLogManagementEndpointCustomPath(t *testing.T) {
	s := Create(WithLogLevel(new(slog.LevelVar)), WithLogManagement("/admin/log"), WithInternalLogging(false))

	for path, want := range map[string]int{"/admin/log": http.StatusOK, "/logging": http.StatusNotFound} {
		w := httptest.NewRecorder()
		s.internalServer.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Errorf("GET %s: expected status %d, got %d", path, want, w.Code)
		}
	}
}

func TestLogManagementEndpointRequiresLevel(t *testing.T) {
	s := Create(WithInternalLogging(false))

	w := httptest.NewRecorder()
	s.internalServer.Handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/logging", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected endpoint to be absent without WithLogLevel, got status %d", w.Code)
	}
}

func TestRequestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		internalLog bool
		externalLog bool
	}{
		{"both enabled", true, true},
		{"internal only", true, false},
		{"external only", false, true},
		{"both disabled", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := newTestLogger()
			ext := mux.NewRouter()
			ext.HandleFunc("/hello", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			})
			s := Create(
				WithLogger(l),
				WithExternalRouter(ext),
				WithInternalLogging(tt.internalLog),
				WithExternalLogging(tt.externalLog),
			)

			for _, c := range []struct {
				h       http.Handler
				path    string
				status  float64
				enabled bool
			}{
				{s.internalServer.Handler, "/live", http.StatusOK, tt.internalLog},
				{s.externalServer.Handler, "/hello", http.StatusTeapot, tt.externalLog},
			} {
				buf.Reset()
				w := httptest.NewRecorder()
				c.h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, c.path, nil))

				gotID := w.Header().Get("X-Request-Id") != ""
				if gotID != c.enabled {
					t.Errorf("%s: expected request ID header present=%v, got %v", c.path, c.enabled, gotID)
				}
				rec := findRecord(logRecords(t, buf), "HTTP response sent")
				if !c.enabled {
					if rec != nil {
						t.Errorf("%s: expected no request log, got %v", c.path, rec)
					}
					continue
				}
				if rec == nil {
					t.Fatalf("%s: expected request to be logged, got %q", c.path, buf.String())
				}
				if rec["path"] != c.path || rec["statusCode"] != c.status || rec["requestId"] == "" {
					t.Errorf("%s: unexpected request log %v", c.path, rec)
				}
			}
		})
	}
}

func TestUnmatchedRequestsAreLogged(t *testing.T) {
	l, buf := newTestLogger()
	r := mux.NewRouter()
	r.HandleFunc("/x", func(http.ResponseWriter, *http.Request) {}).Methods(http.MethodGet)
	s := Create(WithLogger(l), WithExternalRouter(r))

	for _, c := range []struct {
		h      http.Handler
		method string
		path   string
		status float64
	}{
		{s.externalServer.Handler, http.MethodGet, "/nope", http.StatusNotFound},
		{s.externalServer.Handler, http.MethodPost, "/x", http.StatusMethodNotAllowed},
		{s.internalServer.Handler, http.MethodGet, "/nope", http.StatusNotFound},
		{s.internalServer.Handler, http.MethodPost, "/live", http.StatusMethodNotAllowed},
	} {
		buf.Reset()
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, httptest.NewRequest(c.method, c.path, nil))
		if w.Header().Get("X-Request-Id") == "" {
			t.Errorf("%s %s: missing request ID header", c.method, c.path)
		}
		rec := findRecord(logRecords(t, buf), "HTTP response sent")
		if rec == nil || rec["statusCode"] != c.status || rec["path"] != c.path {
			t.Errorf("%s %s: request log = %v, want status %v", c.method, c.path, rec, c.status)
		}
	}
}

func TestReadinessFailureIsLogged(t *testing.T) {
	l, buf := newTestLogger()
	s := Create(
		WithLogger(l),
		WithReadinessCheck("db", func(context.Context) error { return errors.New("down") }),
	)

	w := httptest.NewRecorder()
	s.readinessHandler(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected status 503, got %d", w.Code)
	}
	rec := findRecord(logRecords(t, buf), "Readiness check failed")
	if rec == nil {
		t.Fatalf("Expected readiness failure to be logged, got %q", buf.String())
	}
	if rec["level"] != "WARN" || rec["check"] != "db" || rec["err"] != "down" {
		t.Errorf("Unexpected readiness log %v", rec)
	}
}

func TestStartStopAreLogged(t *testing.T) {
	l, buf := newTestLogger()
	s := Create(WithLogger(l), WithInternalPort(0), WithExternalPort(0), WithExternalRouter(mux.NewRouter()))
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	s.Wait()

	records := logRecords(t, buf)
	for _, msg := range []string{"Internal server started", "External server started", "Internal server stopped", "External server stopped"} {
		if findRecord(records, msg) == nil {
			t.Errorf("Expected %q to be logged, got %q", msg, buf.String())
		}
	}
}
