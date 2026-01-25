package srvr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
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
	if opts.viperRef != viper.GetViper() {
		t.Error("Expected viperRef to be set to global viper instance")
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

func TestWithViper(t *testing.T) {
	opts := defaultOptions()
	v := viper.New()
	option := WithViper(v)
	option(opts)

	if opts.viperRef != v {
		t.Error("Expected viper reference to be set")
	}

	// Verify defaults were set
	if v.GetInt("srvr.internalPort") != 8081 {
		t.Errorf("Expected viper default internal port 8081, got %d", v.GetInt("srvr.internalPort"))
	}
	if v.GetInt("srvr.externalPort") != 8080 {
		t.Errorf("Expected viper default external port 8080, got %d", v.GetInt("srvr.externalPort"))
	}
	if v.GetInt("srvr.readTimeout") != 5 {
		t.Errorf("Expected viper default read timeout 5, got %d", v.GetInt("srvr.readTimeout"))
	}
	if v.GetInt("srvr.writeTimeout") != 10 {
		t.Errorf("Expected viper default write timeout 10, got %d", v.GetInt("srvr.writeTimeout"))
	}
	if v.GetInt("srvr.idleTimeout") != 120 {
		t.Errorf("Expected viper default idle timeout 120, got %d", v.GetInt("srvr.idleTimeout"))
	}
	if v.GetInt("srvr.readHeaderTimeout") != 2 {
		t.Errorf("Expected viper default read header timeout 2, got %d", v.GetInt("srvr.readHeaderTimeout"))
	}
	if v.GetInt("srvr.maxHeaderBytes") != 1<<20 {
		t.Errorf("Expected viper default max header bytes %d, got %d", 1<<20, v.GetInt("srvr.maxHeaderBytes"))
	}
	if !v.GetBool("srvr.doLogInternalRouter") {
		t.Error("Expected viper default doLogInternalRouter to be true")
	}
	if !v.GetBool("srvr.doLogExternalRouter") {
		t.Error("Expected viper default doLogExternalRouter to be true")
	}
}

func TestWithViperCustomValues(t *testing.T) {
	opts := defaultOptions()
	v := viper.New()

	// Set custom values in viper before applying the option
	v.Set("srvr.internalPort", 9090)
	v.Set("srvr.externalPort", 8888)
	v.Set("srvr.readTimeout", 15)
	v.Set("srvr.writeTimeout", 30)
	v.Set("srvr.idleTimeout", 180)
	v.Set("srvr.readHeaderTimeout", 3)
	v.Set("srvr.maxHeaderBytes", 2048)
	v.Set("srvr.doLogInternalRouter", false)
	v.Set("srvr.doLogExternalRouter", false)

	option := WithViper(v)
	option(opts)

	// Verify the options were updated with viper values
	if opts.internalPort != 9090 {
		t.Errorf("Expected internal port 9090, got %d", opts.internalPort)
	}
	if opts.externalPort != 8888 {
		t.Errorf("Expected external port 8888, got %d", opts.externalPort)
	}
	if opts.readTimeout != 15*time.Second {
		t.Errorf("Expected read timeout 15s, got %v", opts.readTimeout)
	}
	if opts.writeTimeout != 30*time.Second {
		t.Errorf("Expected write timeout 30s, got %v", opts.writeTimeout)
	}
	if opts.idleTimeout != 180*time.Second {
		t.Errorf("Expected idle timeout 180s, got %v", opts.idleTimeout)
	}
	if opts.readHeaderTimeout != 3*time.Second {
		t.Errorf("Expected read header timeout 3s, got %v", opts.readHeaderTimeout)
	}
	if opts.maxHeaderBytes != 2048 {
		t.Errorf("Expected max header bytes 2048, got %d", opts.maxHeaderBytes)
	}
	if opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be false")
	}
	if opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be false")
	}
}

func TestWithViperDefaults(t *testing.T) {
	opts := defaultOptions()
	v := viper.New()

	option := WithViper(v)
	option(opts)

	// Verify the options maintain default values when viper has no custom config
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
	if !opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be true")
	}
	if !opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be true")
	}
}

func TestWithViperNil(t *testing.T) {
	opts := defaultOptions()
	option := WithViper(nil)
	option(opts)

	// Verify that viperRef is set to nil
	if opts.viperRef != nil {
		t.Error("Expected viper reference to be nil")
	}

	// The function should handle nil gracefully and not panic,
	// but the options should remain with their original default values
	if opts.internalPort != 8081 {
		t.Errorf("Expected internal port to remain 8081, got %d", opts.internalPort)
	}
	if opts.externalPort != 8080 {
		t.Errorf("Expected external port to remain 8080, got %d", opts.externalPort)
	}
}

func TestWithViperGlobalInstance(t *testing.T) {
	// Set some values in the global viper instance
	globalViper := viper.GetViper()
	originalInternalPort := globalViper.GetInt("srvr.internalPort")
	originalExternalPort := globalViper.GetInt("srvr.externalPort")

	// Set custom values in global viper
	globalViper.Set("srvr.internalPort", 7777)
	globalViper.Set("srvr.externalPort", 6666)

	// Restore original values after test
	defer func() {
		if originalInternalPort != 0 {
			globalViper.Set("srvr.internalPort", originalInternalPort)
		} else {
			// If it was unset, unset it again
			globalViper.Set("srvr.internalPort", nil)
		}
		if originalExternalPort != 0 {
			globalViper.Set("srvr.externalPort", originalExternalPort)
		} else {
			globalViper.Set("srvr.externalPort", nil)
		}
	}()

	opts := defaultOptions()
	option := WithViper(globalViper)
	option(opts)

	// Verify that values from global viper are used
	if opts.internalPort != 7777 {
		t.Errorf("Expected internal port 7777 from global viper, got %d", opts.internalPort)
	}
	if opts.externalPort != 6666 {
		t.Errorf("Expected external port 6666 from global viper, got %d", opts.externalPort)
	}

	// Verify viper reference is the global instance
	if opts.viperRef != globalViper {
		t.Error("Expected viperRef to be the global viper instance")
	}
}

func TestWithViperPrefix(t *testing.T) {
	opts := defaultOptions()
	v := viper.New()
	prefix := "myapp."
	option := WithViperPrefix(v, prefix)
	option(opts)

	if opts.viperRef != v {
		t.Error("Expected viper reference to be set")
	}

	// Verify defaults were set with prefix
	if v.GetInt(prefix+"srvr.internalPort") != 8081 {
		t.Errorf("Expected viper default internal port 8081 with prefix, got %d", v.GetInt(prefix+"srvr.internalPort"))
	}
	if v.GetInt(prefix+"srvr.externalPort") != 8080 {
		t.Errorf("Expected viper default external port 8080 with prefix, got %d", v.GetInt(prefix+"srvr.externalPort"))
	}
	if v.GetInt(prefix+"srvr.readTimeout") != 5 {
		t.Errorf("Expected viper default read timeout 5 with prefix, got %d", v.GetInt(prefix+"srvr.readTimeout"))
	}
	if v.GetInt(prefix+"srvr.writeTimeout") != 10 {
		t.Errorf("Expected viper default write timeout 10 with prefix, got %d", v.GetInt(prefix+"srvr.writeTimeout"))
	}
	if v.GetInt(prefix+"srvr.idleTimeout") != 120 {
		t.Errorf("Expected viper default idle timeout 120 with prefix, got %d", v.GetInt(prefix+"srvr.idleTimeout"))
	}
	if v.GetInt(prefix+"srvr.readHeaderTimeout") != 2 {
		t.Errorf("Expected viper default read header timeout 2 with prefix, got %d", v.GetInt(prefix+"srvr.readHeaderTimeout"))
	}
	if v.GetInt(prefix+"srvr.maxHeaderBytes") != 1<<20 {
		t.Errorf("Expected viper default max header bytes %d with prefix, got %d", 1<<20, v.GetInt(prefix+"srvr.maxHeaderBytes"))
	}
	if !v.GetBool(prefix + "srvr.doLogInternalRouter") {
		t.Error("Expected viper default doLogInternalRouter to be true with prefix")
	}
	if !v.GetBool(prefix + "srvr.doLogExternalRouter") {
		t.Error("Expected viper default doLogExternalRouter to be true with prefix")
	}
}

func TestWithViperPrefixEmptyString(t *testing.T) {
	opts := defaultOptions()
	v := viper.New()
	option := WithViperPrefix(v, "")
	option(opts)

	if opts.viperRef != v {
		t.Error("Expected viper reference to be set")
	}

	// Verify defaults were set without prefix (should behave like WithViper)
	if v.GetInt("srvr.internalPort") != 8081 {
		t.Errorf("Expected viper default internal port 8081 without prefix, got %d", v.GetInt("srvr.internalPort"))
	}
	if v.GetInt("srvr.externalPort") != 8080 {
		t.Errorf("Expected viper default external port 8080 without prefix, got %d", v.GetInt("srvr.externalPort"))
	}
}

func TestWithViperPrefixCustomValues(t *testing.T) {
	opts := defaultOptions()
	v := viper.New()
	prefix := "webapp."

	// Set custom values in viper with prefix before applying the option
	v.Set(prefix+"srvr.internalPort", 9191)
	v.Set(prefix+"srvr.externalPort", 8989)
	v.Set(prefix+"srvr.readTimeout", 25)
	v.Set(prefix+"srvr.writeTimeout", 45)
	v.Set(prefix+"srvr.idleTimeout", 240)
	v.Set(prefix+"srvr.readHeaderTimeout", 5)
	v.Set(prefix+"srvr.maxHeaderBytes", 4096)
	v.Set(prefix+"srvr.doLogInternalRouter", false)
	v.Set(prefix+"srvr.doLogExternalRouter", false)

	option := WithViperPrefix(v, prefix)
	option(opts)

	// Verify the options were updated with prefixed viper values
	if opts.internalPort != 9191 {
		t.Errorf("Expected internal port 9191, got %d", opts.internalPort)
	}
	if opts.externalPort != 8989 {
		t.Errorf("Expected external port 8989, got %d", opts.externalPort)
	}
	if opts.readTimeout != 25*time.Second {
		t.Errorf("Expected read timeout 25s, got %v", opts.readTimeout)
	}
	if opts.writeTimeout != 45*time.Second {
		t.Errorf("Expected write timeout 45s, got %v", opts.writeTimeout)
	}
	if opts.idleTimeout != 240*time.Second {
		t.Errorf("Expected idle timeout 240s, got %v", opts.idleTimeout)
	}
	if opts.readHeaderTimeout != 5*time.Second {
		t.Errorf("Expected read header timeout 5s, got %v", opts.readHeaderTimeout)
	}
	if opts.maxHeaderBytes != 4096 {
		t.Errorf("Expected max header bytes 4096, got %d", opts.maxHeaderBytes)
	}
	if opts.doLogInternalRouter {
		t.Error("Expected doLogInternalRouter to be false")
	}
	if opts.doLogExternalRouter {
		t.Error("Expected doLogExternalRouter to be false")
	}
}

func TestWithViperPrefixNil(t *testing.T) {
	opts := defaultOptions()
	prefix := "test."
	option := WithViperPrefix(nil, prefix)
	option(opts)

	// Verify that viperRef is set to nil
	if opts.viperRef != nil {
		t.Error("Expected viper reference to be nil")
	}

	// The function should handle nil gracefully and not panic,
	// but the options should remain with their original default values
	if opts.internalPort != 8081 {
		t.Errorf("Expected internal port to remain 8081, got %d", opts.internalPort)
	}
	if opts.externalPort != 8080 {
		t.Errorf("Expected external port to remain 8080, got %d", opts.externalPort)
	}
}

func TestWithViperPrefixKeyIsolation(t *testing.T) {
	opts1 := defaultOptions()
	opts2 := defaultOptions()
	v := viper.New()

	// Set values with different prefixes
	v.Set("app1.srvr.internalPort", 8181)
	v.Set("app1.srvr.externalPort", 8080)
	v.Set("app2.srvr.internalPort", 8282)
	v.Set("app2.srvr.externalPort", 8181)

	// Apply options with different prefixes
	option1 := WithViperPrefix(v, "app1.")
	option1(opts1)

	option2 := WithViperPrefix(v, "app2.")
	option2(opts2)

	// Verify that each option uses its own prefixed values
	if opts1.internalPort != 8181 {
		t.Errorf("Expected app1 internal port 8181, got %d", opts1.internalPort)
	}
	if opts1.externalPort != 8080 {
		t.Errorf("Expected app1 external port 8080, got %d", opts1.externalPort)
	}

	if opts2.internalPort != 8282 {
		t.Errorf("Expected app2 internal port 8282, got %d", opts2.internalPort)
	}
	if opts2.externalPort != 8181 {
		t.Errorf("Expected app2 external port 8181, got %d", opts2.externalPort)
	}
}
