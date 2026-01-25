package srvr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()

	if opts.internalPort != 8081 {
		t.Errorf("Expected internal port 8081, got %d", opts.internalPort)
	}
	if opts.externalPort != 8080 {
		t.Errorf("Expected external port 8080, got %d", opts.externalPort)
	}
	if opts.readTimeout != 5*time.Second {
		t.Errorf("Expected read timeout 5s, got %v", opts.readTimeout)
	}
	if opts.writeTimeout != 10*time.Second {
		t.Errorf("Expected write timeout 10s, got %v", opts.writeTimeout)
	}
	if opts.idleTimeout != 120*time.Second {
		t.Errorf("Expected idle timeout 120s, got %v", opts.idleTimeout)
	}
	if opts.readHeaderTimeout != 2*time.Second {
		t.Errorf("Expected read header timeout 2s, got %v", opts.readHeaderTimeout)
	}
	if opts.maxHeaderBytes != 1<<20 {
		t.Errorf("Expected max header bytes %d, got %d", 1<<20, opts.maxHeaderBytes)
	}
	if opts.logManager == nil || opts.logManager.Path != "/logging" {
		t.Errorf("Expected log manager with path '/logging', got %v", opts.logManager)
	}
	if opts.metricsManager == nil || opts.metricsManager.Path != "/metrics" {
		t.Errorf("Expected metrics manager with path '/metrics', got %v", opts.metricsManager)
	}
	if opts.livenessManager == nil || opts.livenessManager.Path != "/live" {
		t.Errorf("Expected liveness manager with path '/live', got %v", opts.livenessManager)
	}
	if opts.readinessManager == nil || opts.readinessManager.Path != "/ready" {
		t.Errorf("Expected readiness manager with path '/ready', got %v", opts.readinessManager)
	}
	if opts.readinessChecks == nil {
		t.Error("Expected readiness checks map to be initialized")
	}
	if !opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be true")
	}
	if !opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be true")
	}
}

func TestWithReadinessCheck(t *testing.T) {
	opts := defaultOptions()
	testCheck := func(ctx context.Context) error {
		return nil
	}

	option := WithReadinessCheck("test", testCheck)
	option(opts)

	if len(opts.readinessChecks) != 1 {
		t.Errorf("Expected 1 readiness check, got %d", len(opts.readinessChecks))
	}
	if _, exists := opts.readinessChecks["test"]; !exists {
		t.Error("Expected 'test' readiness check to exist")
	}
}

func TestWithReadinessCheckNilMap(t *testing.T) {
	opts := &Options{
		readinessChecks: nil,
	}
	testCheck := func(ctx context.Context) error {
		return errors.New("test error")
	}

	option := WithReadinessCheck("test", testCheck)
	option(opts)

	if opts.readinessChecks == nil {
		t.Error("Expected readiness checks map to be initialized")
	}
	if len(opts.readinessChecks) != 1 {
		t.Errorf("Expected 1 readiness check, got %d", len(opts.readinessChecks))
	}
	if check, exists := opts.readinessChecks["test"]; !exists {
		t.Error("Expected 'test' readiness check to exist")
	} else if check == nil {
		t.Error("Expected readiness check function to be set")
	}
}

func TestWithInternalPort(t *testing.T) {
	opts := defaultOptions()
	option := WithInternalPort(9000)
	option(opts)

	if opts.internalPort != 9000 {
		t.Errorf("Expected internal port 9000, got %d", opts.internalPort)
	}
}

func TestWithExternalPort(t *testing.T) {
	opts := defaultOptions()
	option := WithExternalPort(3000)
	option(opts)

	if opts.externalPort != 3000 {
		t.Errorf("Expected external port 3000, got %d", opts.externalPort)
	}
}

func TestWithInternalRouter(t *testing.T) {
	opts := defaultOptions()
	router := mux.NewRouter()
	option := WithInternalRouter(router)
	option(opts)

	if opts.internalRouter != router {
		t.Error("Expected internal router to be set")
	}
}

func TestWithExternalRouter(t *testing.T) {
	opts := defaultOptions()
	router := mux.NewRouter()
	option := WithExternalRouter(router)
	option(opts)

	if opts.externalRouter != router {
		t.Error("Expected external router to be set")
	}
}

func TestWithReadTimeout(t *testing.T) {
	opts := defaultOptions()
	timeout := 30 * time.Second
	option := WithReadTimeout(timeout)
	option(opts)

	if opts.readTimeout != timeout {
		t.Errorf("Expected read timeout %v, got %v", timeout, opts.readTimeout)
	}
}

func TestWithWriteTimeout(t *testing.T) {
	opts := defaultOptions()
	timeout := 60 * time.Second
	option := WithWriteTimeout(timeout)
	option(opts)

	if opts.writeTimeout != timeout {
		t.Errorf("Expected write timeout %v, got %v", timeout, opts.writeTimeout)
	}
}

func TestWithIdleTimeout(t *testing.T) {
	opts := defaultOptions()
	timeout := 300 * time.Second
	option := WithIdleTimeout(timeout)
	option(opts)

	if opts.idleTimeout != timeout {
		t.Errorf("Expected idle timeout %v, got %v", timeout, opts.idleTimeout)
	}
}

func TestWithReadHeaderTimeout(t *testing.T) {
	opts := defaultOptions()
	timeout := 5 * time.Second
	option := WithReadHeaderTimeout(timeout)
	option(opts)

	if opts.readHeaderTimeout != timeout {
		t.Errorf("Expected read header timeout %v, got %v", timeout, opts.readHeaderTimeout)
	}
}

func TestWithMaxHeaderBytes(t *testing.T) {
	opts := defaultOptions()
	maxBytes := 2 << 20 // 2 MB
	option := WithMaxHeaderBytes(maxBytes)
	option(opts)

	if opts.maxHeaderBytes != maxBytes {
		t.Errorf("Expected max header bytes %d, got %d", maxBytes, opts.maxHeaderBytes)
	}
}

func TestWithLogManagement(t *testing.T) {
	opts := defaultOptions()
	route := "/custom-logs"
	option := WithLogManagement(route)
	option(opts)

	if opts.logManager == nil || opts.logManager.Path != route {
		t.Errorf("Expected log manager with path '%s', got %v", route, opts.logManager)
	}
}

func TestWithMetricsRoute(t *testing.T) {
	opts := defaultOptions()
	route := "/custom-metrics"
	option := WithMetricsRoute(route)
	option(opts)

	if opts.metricsManager == nil || opts.metricsManager.Path != route {
		t.Errorf("Expected metrics manager with path '%s', got %v", route, opts.metricsManager)
	}
}

func TestWithInternalLogging(t *testing.T) {
	opts := defaultOptions()

	// Test disabling
	option := WithInternalLogging(false)
	option(opts)
	if opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be false")
	}

	// Test enabling
	option = WithInternalLogging(true)
	option(opts)
	if !opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be true")
	}
}

func TestWithExternalLogging(t *testing.T) {
	opts := defaultOptions()

	// Test disabling
	option := WithExternalLogging(false)
	option(opts)
	if opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be false")
	}

	// Test enabling
	option = WithExternalLogging(true)
	option(opts)
	if !opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be true")
	}
}

func TestWithLivenessManagement(t *testing.T) {
	opts := defaultOptions()
	route := "/custom-liveness"
	option := WithLivenessManagement(route)
	option(opts)

	if opts.livenessManager == nil || opts.livenessManager.Path != route {
		t.Errorf("Expected liveness manager with path '%s', got %v", route, opts.livenessManager)
	}
}

func TestWithReadinessManagement(t *testing.T) {
	opts := defaultOptions()
	route := "/custom-readiness"
	option := WithReadinessManagement(route)
	option(opts)

	if opts.readinessManager == nil || opts.readinessManager.Path != route {
		t.Errorf("Expected readiness manager with path '%s', got %v", route, opts.readinessManager)
	}
}

func TestLogManagerStruct(t *testing.T) {
	lm := &LogManager{
		Path: "/test-logging",
	}

	if lm.Path != "/test-logging" {
		t.Errorf("Expected path '/test-logging', got '%s'", lm.Path)
	}
}

func TestMetricsManagerStruct(t *testing.T) {
	mm := &MetricsManager{
		Path: "/test-metrics",
	}

	if mm.Path != "/test-metrics" {
		t.Errorf("Expected path '/test-metrics', got '%s'", mm.Path)
	}
}

func TestLivenessManagerStruct(t *testing.T) {
	lm := &LivenessManager{
		Path: "/test-liveness",
	}

	if lm.Path != "/test-liveness" {
		t.Errorf("Expected path '/test-liveness', got '%s'", lm.Path)
	}
}

func TestReadinessManagerStruct(t *testing.T) {
	rm := &ReadinessManager{
		Path: "/test-readiness",
	}

	if rm.Path != "/test-readiness" {
		t.Errorf("Expected path '/test-readiness', got '%s'", rm.Path)
	}
}

func TestReadinessCheckFunc(t *testing.T) {
	// Test a readiness check function that returns no error
	successCheck := func(_ context.Context) error {
		return nil
	}

	// Test a readiness check function that returns an error
	failCheck := func(_ context.Context) error {
		return errors.New("service unavailable")
	}

	ctx := context.Background()

	if err := successCheck(ctx); err != nil {
		t.Errorf("Expected success check to return no error, got %v", err)
	}

	if err := failCheck(ctx); err == nil {
		t.Error("Expected fail check to return an error")
	} else if err.Error() != "service unavailable" {
		t.Errorf("Expected error 'service unavailable', got '%s'", err.Error())
	}
}

func TestMultipleOptions(t *testing.T) {
	opts := defaultOptions()
	internalRouter := mux.NewRouter()
	externalRouter := mux.NewRouter()

	options := []Option{
		WithInternalPort(7000),
		WithExternalPort(7001),
		WithInternalRouter(internalRouter),
		WithExternalRouter(externalRouter),
		WithReadTimeout(15 * time.Second),
		WithWriteTimeout(25 * time.Second),
		WithInternalLogging(false),
		WithExternalLogging(false),
		WithLogManagement("/custom-log"),
		WithMetricsRoute("/custom-metrics"),
		WithLivenessManagement("/custom-live"),
		WithReadinessManagement("/custom-ready"),
	}

	for _, opt := range options {
		opt(opts)
	}

	if opts.internalPort != 7000 {
		t.Errorf("Expected internal port 7000, got %d", opts.internalPort)
	}
	if opts.externalPort != 7001 {
		t.Errorf("Expected external port 7001, got %d", opts.externalPort)
	}
	if opts.internalRouter != internalRouter {
		t.Error("Expected internal router to be set")
	}
	if opts.externalRouter != externalRouter {
		t.Error("Expected external router to be set")
	}
	if opts.readTimeout != 15*time.Second {
		t.Errorf("Expected read timeout 15s, got %v", opts.readTimeout)
	}
	if opts.writeTimeout != 25*time.Second {
		t.Errorf("Expected write timeout 25s, got %v", opts.writeTimeout)
	}
	if opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be false")
	}
	if opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be false")
	}
	if opts.logManager.Path != "/custom-log" {
		t.Errorf("Expected log manager path '/custom-log', got '%s'", opts.logManager.Path)
	}
	if opts.metricsManager.Path != "/custom-metrics" {
		t.Errorf("Expected metrics manager path '/custom-metrics', got '%s'", opts.metricsManager.Path)
	}
	if opts.livenessManager.Path != "/custom-live" {
		t.Errorf("Expected liveness manager path '/custom-live', got '%s'", opts.livenessManager.Path)
	}
	if opts.readinessManager.Path != "/custom-ready" {
		t.Errorf("Expected readiness manager path '/custom-ready', got '%s'", opts.readinessManager.Path)
	}
}
