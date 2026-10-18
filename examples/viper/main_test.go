package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe for the concurrent writes made by
// server goroutines.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// get polls url until the server answers, then returns the status and body.
func get(t *testing.T, url string) (int, string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return resp.StatusCode, string(b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s: %v", url, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNewConfig(t *testing.T) {
	v, err := newConfig(bytes.NewReader(defaultConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got := v.GetInt(prefix + "srvr.externalPort"); got != 8080 {
		t.Errorf("externalPort = %d, want 8080", got)
	}
	if got := v.GetBool(prefix + "srvr.doLogInternalRouter"); got {
		t.Error("doLogInternalRouter = true, want false")
	}

	t.Setenv("API_SRVR_EXTERNALPORT", "9090")
	if got := v.GetInt(prefix + "srvr.externalPort"); got != 9090 {
		t.Errorf("externalPort with env override = %d, want 9090", got)
	}
}

func TestNewConfigInvalidYAML(t *testing.T) {
	if _, err := newConfig(strings.NewReader("api: [")); err == nil {
		t.Error("expected an error for invalid YAML")
	}
}

func TestRun(t *testing.T) {
	ext, internal := freePort(t), freePort(t)
	t.Setenv("API_SRVR_EXTERNALPORT", fmt.Sprint(ext))
	t.Setenv("API_SRVR_INTERNALPORT", fmt.Sprint(internal))

	v, err := newConfig(bytes.NewReader(defaultConfig))
	if err != nil {
		t.Fatal(err)
	}
	var logs syncBuffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, v, logger) }()

	if code, body := get(t, fmt.Sprintf("http://localhost:%d/hello", ext)); code != http.StatusOK || body != "Hello from a Viper-configured server!\n" {
		t.Errorf("GET /hello = %d %q", code, body)
	}
	if code, _ := get(t, fmt.Sprintf("http://localhost:%d/live", internal)); code != http.StatusOK {
		t.Errorf("GET /live = %d", code)
	}

	// config.yaml enables request logging only on the external server.
	if !strings.Contains(logs.String(), `"path":"/hello"`) {
		t.Errorf("external request should be logged, got:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), `"path":"/live"`) {
		t.Errorf("internal request should not be logged, got:\n%s", logs.String())
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}
