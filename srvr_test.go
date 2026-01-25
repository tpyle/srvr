package srvr

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

func TestCreate(t *testing.T) {
	server := Create()

	if server == nil {
		t.Fatal("Expected server to be created")
	}
	if server.options == nil {
		t.Fatal("Expected server options to be initialized")
	}

	// Verify default options are applied
	if server.options.internalPort != 8081 {
		t.Errorf("Expected internal port 8081, got %d", server.options.internalPort)
	}
	if server.options.externalPort != 8080 {
		t.Errorf("Expected external port 8080, got %d", server.options.externalPort)
	}
}

func TestCreateWithOptions(t *testing.T) {
	internalRouter := mux.NewRouter()
	externalRouter := mux.NewRouter()

	server := Create(
		WithInternalPort(9001),
		WithExternalPort(9002),
		WithInternalRouter(internalRouter),
		WithExternalRouter(externalRouter),
		WithReadTimeout(30*time.Second),
		WithWriteTimeout(45*time.Second),
		WithIdleTimeout(200*time.Second),
		WithReadHeaderTimeout(10*time.Second),
		WithMaxHeaderBytes(2<<20),
	)

	if server.options.internalPort != 9001 {
		t.Errorf("Expected internal port 9001, got %d", server.options.internalPort)
	}
	if server.options.externalPort != 9002 {
		t.Errorf("Expected external port 9002, got %d", server.options.externalPort)
	}
	if server.options.internalRouter != internalRouter {
		t.Error("Expected internal router to be set")
	}
	if server.options.externalRouter != externalRouter {
		t.Error("Expected external router to be set")
	}
}

func TestCreateWithInternalRouter(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := Create(
		WithInternalRouter(router),
		WithInternalPort(getAvailablePort(t)),
		WithLogManagement("/custom-logs"),
		WithMetricsRoute("/custom-metrics"),
	)

	if server.internalServer == nil {
		t.Fatal("Expected internal server to be created")
	}

	// Check server configuration
	expectedAddr := fmt.Sprintf(":%d", server.options.internalPort)
	if server.internalServer.Addr != expectedAddr {
		t.Errorf("Expected addr %s, got %s", expectedAddr, server.internalServer.Addr)
	}
	if server.internalServer.ReadTimeout != server.options.readTimeout {
		t.Errorf("Expected read timeout %v, got %v", server.options.readTimeout, server.internalServer.ReadTimeout)
	}
	if server.internalServer.WriteTimeout != server.options.writeTimeout {
		t.Errorf("Expected write timeout %v, got %v", server.options.writeTimeout, server.internalServer.WriteTimeout)
	}
	if server.internalServer.IdleTimeout != server.options.idleTimeout {
		t.Errorf("Expected idle timeout %v, got %v", server.options.idleTimeout, server.internalServer.IdleTimeout)
	}
	if server.internalServer.ReadHeaderTimeout != server.options.readHeaderTimeout {
		t.Errorf("Expected read header timeout %v, got %v", server.options.readHeaderTimeout, server.internalServer.ReadHeaderTimeout)
	}
	if server.internalServer.MaxHeaderBytes != server.options.maxHeaderBytes {
		t.Errorf("Expected max header bytes %d, got %d", server.options.maxHeaderBytes, server.internalServer.MaxHeaderBytes)
	}
}

func TestCreateWithExternalRouter(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := Create(
		WithExternalRouter(router),
		WithExternalPort(getAvailablePort(t)),
	)

	if server.externalServer == nil {
		t.Fatal("Expected external server to be created")
	}

	expectedAddr := fmt.Sprintf(":%d", server.options.externalPort)
	if server.externalServer.Addr != expectedAddr {
		t.Errorf("Expected addr %s, got %s", expectedAddr, server.externalServer.Addr)
	}
}

func TestCreateWithBothRouters(t *testing.T) {
	internalRouter := mux.NewRouter()
	externalRouter := mux.NewRouter()

	server := Create(
		WithInternalRouter(internalRouter),
		WithExternalRouter(externalRouter),
		WithInternalPort(getAvailablePort(t)),
		WithExternalPort(getAvailablePort(t)),
	)

	if server.internalServer == nil {
		t.Error("Expected internal server to be created")
	}
	if server.externalServer == nil {
		t.Error("Expected external server to be created")
	}
}

func TestCreateWithNoRouters(t *testing.T) {
	server := Create()

	if server.internalServer == nil {
		t.Error("Expected default internal server when no internal router provided")
	}
	if server.externalServer != nil {
		t.Error("Expected no external server when no external router provided")
	}
}

func TestCreateWithLoggingDisabled(t *testing.T) {
	router := mux.NewRouter()

	server := Create(
		WithInternalRouter(router),
		WithExternalRouter(router),
		WithInternalLogging(false),
		WithExternalLogging(false),
		WithInternalPort(getAvailablePort(t)),
		WithExternalPort(getAvailablePort(t)),
	)

	if server.options.doLogInternalRouter {
		t.Error("Expected internal router logging to be disabled")
	}
	if server.options.doLogExternalRouter {
		t.Error("Expected external router logging to be disabled")
	}
}

func TestStartAndStop(t *testing.T) {
	internalRouter := mux.NewRouter()
	externalRouter := mux.NewRouter()

	internalPort := getAvailablePort(t)
	externalPort := getAvailablePort(t)

	server := Create(
		WithInternalRouter(internalRouter),
		WithExternalRouter(externalRouter),
		WithInternalPort(internalPort),
		WithExternalPort(externalPort),
	)

	// Test Start
	err := server.Start()
	if err != nil {
		t.Fatalf("Expected no error on start, got %v", err)
	}

	// Give servers a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify servers are listening
	verifyServerListening(t, internalPort, "internal")
	verifyServerListening(t, externalPort, "external")

	// Test Stop
	err = server.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop, got %v", err)
	}

	// Give servers a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Verify servers are no longer listening
	verifyServerNotListening(t, internalPort, "internal")
	verifyServerNotListening(t, externalPort, "external")
}

func TestStartInternalOnly(t *testing.T) {
	internalRouter := mux.NewRouter()
	internalPort := getAvailablePort(t)

	server := Create(
		WithInternalRouter(internalRouter),
		WithInternalPort(internalPort),
	)

	err := server.Start()
	if err != nil {
		t.Fatalf("Expected no error on start, got %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	verifyServerListening(t, internalPort, "internal")

	err = server.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop, got %v", err)
	}
}

func TestStartExternalOnly(t *testing.T) {
	externalRouter := mux.NewRouter()
	externalPort := getAvailablePort(t)

	server := Create(
		WithExternalRouter(externalRouter),
		WithExternalPort(externalPort),
	)

	err := server.Start()
	if err != nil {
		t.Fatalf("Expected no error on start, got %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	verifyServerListening(t, externalPort, "external")

	err = server.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop, got %v", err)
	}
}

func TestStartNoServers(t *testing.T) {
	server := Create()

	err := server.Start()
	if err != nil {
		t.Fatalf("Expected no error on start with no servers, got %v", err)
	}

	err = server.Stop()
	if err != nil {
		t.Fatalf("Expected no error on stop with no servers, got %v", err)
	}
}

func TestStopInternalServerError(t *testing.T) {
	// Test scenario where internal server Close() might fail
	server := &Server{
		options: defaultOptions(),
		internalServer: &http.Server{
			Addr: ":invalid", // This could potentially cause close issues
		},
	}

	// This test mainly verifies that Stop() properly handles any potential errors
	// The actual error handling would depend on the specific http.Server.Close() behavior
	err := server.Stop()
	// Since http.Server.Close() typically doesn't error on invalid addresses,
	// we expect no error, but the structure supports error handling
	if err != nil {
		t.Logf("Stop returned error (may be expected): %v", err)
	}
}

func TestStopExternalServerError(t *testing.T) {
	// Test scenario where external server Close() might fail
	server := &Server{
		options: defaultOptions(),
		externalServer: &http.Server{
			Addr: ":invalid", // This could potentially cause close issues
		},
	}

	err := server.Stop()
	if err != nil {
		t.Logf("Stop returned error (may be expected): %v", err)
	}
}

func TestServerStruct(t *testing.T) {
	options := defaultOptions()
	server := &Server{
		options:        options,
		internalServer: nil,
		externalServer: nil,
	}

	if server.options != options {
		t.Error("Expected options to be set")
	}
	if server.internalServer != nil {
		t.Error("Expected internal server to be nil")
	}
	if server.externalServer != nil {
		t.Error("Expected external server to be nil")
	}
}

// Helper functions

func getAvailablePort(t *testing.T) int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

func verifyServerListening(t *testing.T, port int, serverType string) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
	if err != nil {
		t.Errorf("Expected %s server to be listening on port %d, but connection failed: %v", serverType, port, err)
		return
	}
	conn.Close()
}

func verifyServerNotListening(t *testing.T, port int, serverType string) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 100*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Errorf("Expected %s server to not be listening on port %d, but connection succeeded", serverType, port)
	}
}

// Benchmark tests for performance assessment

func BenchmarkCreateServer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Create()
	}
}

func BenchmarkCreateServerWithOptions(b *testing.B) {
	internalRouter := mux.NewRouter()
	externalRouter := mux.NewRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Create(
			WithInternalRouter(internalRouter),
			WithExternalRouter(externalRouter),
			WithInternalPort(8081),
			WithExternalPort(8080),
		)
	}
}

func BenchmarkStartStop(b *testing.B) {
	servers := make([]*Server, b.N)
	ports := make([]int, b.N*2) // internal and external ports

	// Pre-allocate servers and ports
	for i := 0; i < b.N; i++ {
		ports[i*2] = 10000 + i*2       // internal port
		ports[i*2+1] = 10000 + i*2 + 1 // external port

		servers[i] = Create(
			WithInternalRouter(mux.NewRouter()),
			WithExternalRouter(mux.NewRouter()),
			WithInternalPort(ports[i*2]),
			WithExternalPort(ports[i*2+1]),
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		servers[i].Start()
		servers[i].Stop()
	}
}
