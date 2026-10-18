package srvr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

// newLifecycleServer returns a Server on random ports whose external router
// serves /block, which signals entered and then waits for release (or for the
// request to be cancelled).
func newLifecycleServer(t *testing.T, opts ...Option) (s *Server, entered <-chan struct{}, release chan<- struct{}) {
	t.Helper()
	in := make(chan struct{}, 10)
	rel := make(chan struct{})
	r := mux.NewRouter()
	r.HandleFunc("/block", func(w http.ResponseWriter, req *http.Request) {
		in <- struct{}{}
		select {
		case <-rel:
			_, _ = w.Write([]byte("done"))
		case <-req.Context().Done():
		}
	})
	l, _ := newTestLogger()
	base := []Option{
		WithLogger(l),
		WithInternalPort(0),
		WithExternalPort(0),
		WithExternalRouter(r),
		WithMetricsRegistry(prometheus.NewRegistry()),
	}
	s = Create(append(base, opts...)...)
	return s, in, rel
}

// startBlockedRequest starts a GET /block and waits for the handler to run.
// The result is sent on the returned channel.
func startBlockedRequest(t *testing.T, s *Server, entered <-chan struct{}) <-chan error {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		resp, err := http.Get(fmt.Sprintf("http://%s/block", s.ExternalAddr()))
		if err != nil {
			result <- err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err == nil && string(body) != "done" {
			err = fmt.Errorf("unexpected body %q", body)
		}
		result <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("handler was not reached")
	}
	return result
}

func TestStartReturnsBindErrors(t *testing.T) {
	for _, busy := range []string{"internal", "external"} {
		t.Run(busy, func(t *testing.T) {
			occupied, err := net.Listen("tcp", ":0")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = occupied.Close() }()
			busyPort := occupied.Addr().(*net.TCPAddr).Port
			freePort := getAvailablePort(t)

			internal, external := freePort, busyPort
			if busy == "internal" {
				internal, external = busyPort, freePort
			}
			l, _ := newTestLogger()
			s := Create(WithLogger(l), WithInternalPort(internal), WithExternalPort(external), WithExternalRouter(mux.NewRouter()))

			err = s.Start()
			if !errors.Is(err, syscall.EADDRINUSE) {
				t.Fatalf("Start() = %v, want address in use", err)
			}
			if !strings.Contains(err.Error(), busy+" server") {
				t.Errorf("error %q should name the %s server", err, busy)
			}
			if s.InternalAddr() != nil || s.ExternalAddr() != nil {
				t.Error("addresses should be nil after a failed start")
			}
			// The port that could be bound must have been released.
			ln, err := net.Listen("tcp", fmt.Sprintf(":%d", freePort))
			if err != nil {
				t.Fatalf("port %d was not released: %v", freePort, err)
			}
			_ = ln.Close()

			// A failed Start leaves the server startable.
			_ = occupied.Close()
			if err := s.Start(); err != nil {
				t.Fatalf("Start after freeing the port: %v", err)
			}
			if err := s.Stop(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStartStateErrors(t *testing.T) {
	s, _, _ := newLifecycleServer(t)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); !errors.Is(err, ErrServerStarted) {
		t.Errorf("second Start() = %v, want %v", err, ErrServerStarted)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); !errors.Is(err, ErrServerStopped) {
		t.Errorf("Start() after Stop = %v, want %v", err, ErrServerStopped)
	}
}

func TestShutdownBeforeStart(t *testing.T) {
	s, _, _ := newLifecycleServer(t)
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}
	if err := s.Start(); !errors.Is(err, ErrServerStopped) {
		t.Errorf("Start() after Shutdown = %v, want %v", err, ErrServerStopped)
	}
	done := make(chan struct{})
	go func() { s.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return")
	}
}

func TestAddrs(t *testing.T) {
	s, _, _ := newLifecycleServer(t)
	if s.InternalAddr() != nil || s.ExternalAddr() != nil {
		t.Fatal("addresses should be nil before Start")
	}
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Stop() }()

	for name, addr := range map[string]net.Addr{"internal": s.InternalAddr(), "external": s.ExternalAddr()} {
		tcp, ok := addr.(*net.TCPAddr)
		if !ok || tcp.Port == 0 {
			t.Errorf("%s address = %v, want a bound TCP port", name, addr)
		}
	}
	resp, err := http.Get(fmt.Sprintf("http://%s/live", s.InternalAddr()))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /live = %d", resp.StatusCode)
	}
}

func TestAddrsWithoutExternalServer(t *testing.T) {
	l, _ := newTestLogger()
	s := Create(WithLogger(l), WithInternalPort(0))
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Stop() }()
	if s.ExternalAddr() != nil {
		t.Errorf("ExternalAddr() = %v, want nil", s.ExternalAddr())
	}
	if s.InternalAddr() == nil {
		t.Error("InternalAddr() = nil")
	}
}

func TestShutdownDrainsInFlightRequests(t *testing.T) {
	s, entered, release := newLifecycleServer(t)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	result := startBlockedRequest(t, s, entered)

	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- s.Shutdown(context.Background()) }()
	waited := make(chan struct{})
	go func() { s.Wait(); close(waited) }()

	// New connections are refused while the in-flight request drains.
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.Dial("tcp", s.ExternalAddr().String())
		if err != nil {
			break
		}
		_ = conn.Close()
		if time.Now().After(deadline) {
			t.Fatal("listener still accepting connections during shutdown")
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case err := <-shutdownErr:
		t.Fatalf("Shutdown returned (%v) before the in-flight request finished", err)
	case <-waited:
		t.Fatal("Wait returned before the in-flight request finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-result; err != nil {
		t.Errorf("in-flight request failed: %v", err)
	}
	if err := <-shutdownErr; err != nil {
		t.Errorf("Shutdown() = %v", err)
	}
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return after shutdown")
	}
}

func TestShutdownDeadlineClosesConnections(t *testing.T) {
	s, entered, _ := newLifecycleServer(t)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	result := startBlockedRequest(t, s, entered)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := s.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown() = %v, want %v", err, context.DeadlineExceeded)
	}
	if !strings.Contains(fmt.Sprint(err), "external server") {
		t.Errorf("error %q should name the external server", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Error("in-flight request should fail when its connection is closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request was not cut off")
	}
	s.Wait()
}

func TestStopUsesShutdownTimeout(t *testing.T) {
	s, entered, _ := newLifecycleServer(t, WithShutdownTimeout(50*time.Millisecond))
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	result := startBlockedRequest(t, s, entered)

	start := time.Now()
	if err := s.Stop(); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Stop() = %v, want %v", err, context.DeadlineExceeded)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Stop took %v, want about 50ms", elapsed)
	}
	<-result
	s.Wait()
}

func TestRunReturnsWhenStartFails(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()

	l, buf := newTestLogger()
	s := Create(WithLogger(l), WithInternalPort(occupied.Addr().(*net.TCPAddr).Port))

	done := make(chan struct{})
	go func() { s.Run(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after Start failed")
	}
	if rec := findRecord(logRecords(t, buf), "Failed to start server"); rec == nil {
		t.Errorf("start failure not logged, got %q", buf.String())
	}
}

func TestRunStopsOnShutdown(t *testing.T) {
	s, _, _ := newLifecycleServer(t)
	done := make(chan struct{})
	go func() { s.Run(); close(done) }()

	deadline := time.Now().Add(5 * time.Second)
	for s.ExternalAddr() == nil {
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after Stop")
	}
}
