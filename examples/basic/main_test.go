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
	return do(t, http.MethodGet, url, "", "")
}

func do(t *testing.T, method, url, contentType, body string) (int, string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		req, err := http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return resp.StatusCode, string(b)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s %s: %v", method, url, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRun(t *testing.T) {
	var logs syncBuffer
	level := new(slog.LevelVar)
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: level}))
	ext, internal := freePort(t), freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, logger, level, ext, internal) }()

	extURL := fmt.Sprintf("http://localhost:%d", ext)
	intURL := fmt.Sprintf("http://localhost:%d", internal)

	if code, body := get(t, extURL+"/hello?name=gopher"); code != http.StatusOK || body != "Hello, gopher!\n" {
		t.Errorf("GET /hello?name=gopher = %d %q", code, body)
	}
	if code, body := get(t, extURL+"/hello"); code != http.StatusOK || body != "Hello, world!\n" {
		t.Errorf("GET /hello = %d %q", code, body)
	}
	for _, path := range []string{"/live", "/ready", "/metrics"} {
		if code, _ := get(t, intURL+path); code != http.StatusOK {
			t.Errorf("GET %s = %d", path, code)
		}
	}
	if strings.Contains(logs.String(), "HTTP response sent") {
		t.Error("request logs should be hidden at the default info level")
	}

	if code, body := do(t, http.MethodPost, intURL+"/logging", "application/json", `{"level":"debug"}`); code != http.StatusOK || !strings.Contains(body, `"debug"`) {
		t.Errorf("POST /logging = %d %q", code, body)
	}
	get(t, extURL+"/hello")
	if !strings.Contains(logs.String(), "HTTP response sent") {
		t.Errorf("request logs should appear at debug level, got:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), `"msg":"saying hello"`) {
		t.Errorf("handler log missing, got:\n%s", logs.String())
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
