package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

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

func TestDatabasePing(t *testing.T) {
	db := &database{}
	if err := db.Ping(context.Background()); err != errUnavailable {
		t.Errorf("Ping while down = %v, want %v", err, errUnavailable)
	}
	db.up.Store(true)
	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("Ping while up = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := db.Ping(ctx); err != context.Canceled {
		t.Errorf("Ping with cancelled context = %v, want %v", err, context.Canceled)
	}
}

func TestRun(t *testing.T) {
	db := &database{}
	ext, internal := freePort(t), freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, db, ext, internal) }()

	extURL := fmt.Sprintf("http://localhost:%d", ext)
	intURL := fmt.Sprintf("http://localhost:%d", internal)

	tests := []struct {
		name     string
		up       bool
		url      string
		wantCode int
		wantBody string
	}{
		{"live while db down", false, intURL + "/healthz/live", http.StatusOK, "OK"},
		{"ready while db down", false, intURL + "/healthz/ready", http.StatusServiceUnavailable, "Readiness check 'database' failed: database unavailable\n"},
		{"api while db down", false, extURL + "/items", http.StatusServiceUnavailable, "database unavailable\n"},
		{"admin while db down", false, intURL + "/admin/status", http.StatusOK, "database up: false\n"},
		{"default ready path unused", false, intURL + "/ready", http.StatusNotFound, ""},
		{"admin not on external port", false, extURL + "/admin/status", http.StatusNotFound, ""},
		{"ready while db up", true, intURL + "/healthz/ready", http.StatusOK, "OK"},
		{"api while db up", true, extURL + "/items", http.StatusOK, "[\"apple\",\"banana\"]\n"},
		{"admin while db up", true, intURL + "/admin/status", http.StatusOK, "database up: true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.up.Store(tt.up)
			code, body := get(t, tt.url)
			if code != tt.wantCode {
				t.Errorf("status = %d, want %d", code, tt.wantCode)
			}
			if tt.wantBody != "" && body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
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
