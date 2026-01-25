package srvr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

func TestDefaultOptionsIncludeLivenessReadiness(t *testing.T) {
	opts := defaultOptions()

	if opts.livenessManager == nil || opts.livenessManager.Path != "/live" {
		t.Errorf("Expected default liveness manager with path '/live', got %v", opts.livenessManager)
	}
	if opts.readinessManager == nil || opts.readinessManager.Path != "/ready" {
		t.Errorf("Expected default readiness manager with path '/ready', got %v", opts.readinessManager)
	}
}

func TestLivenessHandler(t *testing.T) {
	server := &Server{
		options: defaultOptions(),
	}

	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()

	server.livenessHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", body)
	}
}

func TestReadinessHandlerNoChecks(t *testing.T) {
	opts := defaultOptions()
	// Clear any existing readiness checks
	opts.readinessChecks = map[string]ReadinessCheckFunc{}

	server := &Server{
		options: opts,
	}

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	server.readinessHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", body)
	}
}

func TestReadinessHandlerWithPassingChecks(t *testing.T) {
	opts := defaultOptions()
	opts.readinessChecks = map[string]ReadinessCheckFunc{
		"database": func(ctx context.Context) error {
			return nil // Healthy
		},
		"cache": func(ctx context.Context) error {
			return nil // Healthy
		},
	}

	server := &Server{
		options: opts,
	}

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	server.readinessHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", body)
	}
}

func TestReadinessHandlerWithFailingCheck(t *testing.T) {
	opts := defaultOptions()
	opts.readinessChecks = map[string]ReadinessCheckFunc{
		"database": func(ctx context.Context) error {
			return nil // Healthy
		},
		"failing_service": func(ctx context.Context) error {
			return errors.New("connection timeout")
		},
	}

	server := &Server{
		options: opts,
	}

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	server.readinessHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	body := w.Body.String()
	if !strings.Contains(body, "failing_service") {
		t.Errorf("Expected body to contain 'failing_service', got '%s'", body)
	}
	if !strings.Contains(body, "connection timeout") {
		t.Errorf("Expected body to contain 'connection timeout', got '%s'", body)
	}
}

func TestReadinessHandlerWithMultipleFailingChecks(t *testing.T) {
	opts := defaultOptions()
	opts.readinessChecks = map[string]ReadinessCheckFunc{
		"first_fail": func(ctx context.Context) error {
			return errors.New("first error")
		},
		"second_fail": func(ctx context.Context) error {
			return errors.New("second error")
		},
	}

	server := &Server{
		options: opts,
	}

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	server.readinessHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Should fail on the first failing check encountered
	body := w.Body.String()
	if !strings.Contains(body, "failed:") {
		t.Errorf("Expected body to contain 'failed:', got '%s'", body)
	}
}

func TestReadinessHandlerWithContext(t *testing.T) {
	opts := defaultOptions()

	// Test with a check that uses the context
	opts.readinessChecks = map[string]ReadinessCheckFunc{
		"context_check": func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	}

	server := &Server{
		options: opts,
	}

	// Test with normal context
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()

	server.readinessHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req = httptest.NewRequestWithContext(ctx, "GET", "/ready", nil)
	w = httptest.NewRecorder()

	server.readinessHandler(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestCreateWithLivenessManager(t *testing.T) {
	internalRouter := mux.NewRouter()

	server := Create(
		WithInternalRouter(internalRouter),
		WithInternalPort(getAvailablePort(t)),
		WithLivenessManagement("/health/live"),
	)

	if server.options.livenessManager == nil {
		t.Fatal("Expected liveness manager to be set")
	}
	if server.options.livenessManager.Path != "/health/live" {
		t.Errorf("Expected liveness path '/health/live', got '%s'", server.options.livenessManager.Path)
	}
	if server.internalServer == nil {
		t.Fatal("Expected internal server to be created")
	}
}

func TestCreateWithReadinessManager(t *testing.T) {
	internalRouter := mux.NewRouter()

	server := Create(
		WithInternalRouter(internalRouter),
		WithInternalPort(getAvailablePort(t)),
		WithReadinessManagement("/health/ready"),
	)

	if server.options.readinessManager == nil {
		t.Fatal("Expected readiness manager to be set")
	}
	if server.options.readinessManager.Path != "/health/ready" {
		t.Errorf("Expected readiness path '/health/ready', got '%s'", server.options.readinessManager.Path)
	}
	if server.internalServer == nil {
		t.Fatal("Expected internal server to be created")
	}
}

func TestCreateWithBothHealthManagers(t *testing.T) {
	internalRouter := mux.NewRouter()

	server := Create(
		WithInternalRouter(internalRouter),
		WithInternalPort(getAvailablePort(t)),
		WithLivenessManagement("/custom-live"),
		WithReadinessManagement("/custom-ready"),
		WithReadinessCheck("test", func(ctx context.Context) error {
			return nil
		}),
	)

	if server.options.livenessManager == nil {
		t.Fatal("Expected liveness manager to be set")
	}
	if server.options.readinessManager == nil {
		t.Fatal("Expected readiness manager to be set")
	}
	if len(server.options.readinessChecks) != 1 {
		t.Errorf("Expected 1 readiness check, got %d", len(server.options.readinessChecks))
	}
}

func TestHealthEndpointsIntegration(t *testing.T) {
	internalRouter := mux.NewRouter()
	port := getAvailablePort(t)

	server := Create(
		WithInternalRouter(internalRouter),
		WithInternalPort(port),
		WithLivenessManagement("/health/live"),
		WithReadinessManagement("/health/ready"),
		WithReadinessCheck("database", func(ctx context.Context) error {
			// Simulate database health check
			return nil
		}),
		WithReadinessCheck("cache", func(ctx context.Context) error {
			// Simulate cache health check
			return nil
		}),
	)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond) // Let server start

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Test liveness endpoint
	resp, err := http.Get(baseURL + "/health/live")
	if err != nil {
		t.Fatalf("Failed to call liveness endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected liveness status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Test readiness endpoint
	resp, err = http.Get(baseURL + "/health/ready")
	if err != nil {
		t.Fatalf("Failed to call readiness endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected readiness status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHealthEndpointsWithFailingReadinessCheck(t *testing.T) {
	internalRouter := mux.NewRouter()
	port := getAvailablePort(t)

	server := Create(
		WithInternalRouter(internalRouter),
		WithInternalPort(port),
		WithLivenessManagement("/health/live"),
		WithReadinessManagement("/health/ready"),
		WithReadinessCheck("database", func(ctx context.Context) error {
			return nil // Healthy
		}),
		WithReadinessCheck("failing_service", func(ctx context.Context) error {
			return errors.New("service unavailable")
		}),
	)

	err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond) // Let server start

	baseURL := fmt.Sprintf("http://localhost:%d", port)

	// Liveness should still work
	resp, err := http.Get(baseURL + "/health/live")
	if err != nil {
		t.Fatalf("Failed to call liveness endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected liveness status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Readiness should fail
	resp, err = http.Get(baseURL + "/health/ready")
	if err != nil {
		t.Fatalf("Failed to call readiness endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected readiness status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// Benchmark tests for health endpoints
func BenchmarkLivenessHandler(b *testing.B) {
	server := &Server{
		options: defaultOptions(),
	}

	req := httptest.NewRequest("GET", "/live", nil)

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.livenessHandler(w, req)
	}
}

func BenchmarkReadinessHandlerNoChecks(b *testing.B) {
	opts := defaultOptions()
	opts.readinessChecks = map[string]ReadinessCheckFunc{}

	server := &Server{
		options: opts,
	}

	req := httptest.NewRequest("GET", "/ready", nil)

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.readinessHandler(w, req)
	}
}

func BenchmarkReadinessHandlerWithChecks(b *testing.B) {
	opts := defaultOptions()
	opts.readinessChecks = map[string]ReadinessCheckFunc{
		"check1": func(ctx context.Context) error { return nil },
		"check2": func(ctx context.Context) error { return nil },
		"check3": func(ctx context.Context) error { return nil },
	}

	server := &Server{
		options: opts,
	}

	req := httptest.NewRequest("GET", "/ready", nil)

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.readinessHandler(w, req)
	}
}
