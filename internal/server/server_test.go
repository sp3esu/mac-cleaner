package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/engine"
)

// newTestEngine creates an engine with all default scanners registered.
func newTestEngine() *engine.Engine {
	eng := engine.New()
	engine.RegisterDefaults(eng)
	return eng
}

// waitForSocket blocks until the socket file exists or timeout.
func waitForSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %s did not appear within timeout", path)
}

// sendRequest sends a Request over the connection.
func sendRequest(t *testing.T, conn net.Conn, req Request) {
	t.Helper()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("send %s: %v", req.Method, err)
	}
}

// readResponse reads one Response from the connection.
func readResponse(t *testing.T, conn net.Conn) Response {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp
}

func TestServer_PingIntegration(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx)
	}()

	// Wait for server to start.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send ping request.
	enc := json.NewEncoder(conn)
	if err := enc.Encode(Request{ID: "p1", Method: MethodPing}); err != nil {
		t.Fatalf("send ping: %v", err)
	}

	// Read response.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	dec := json.NewDecoder(conn)
	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("read ping response: %v", err)
	}

	if resp.ID != "p1" {
		t.Errorf("expected id p1, got %q", resp.ID)
	}
	if resp.Type != ResponseResult {
		t.Errorf("expected type result, got %q", resp.Type)
	}

	// Check result contains version.
	resultBytes, _ := json.Marshal(resp.Result)
	var ping PingResult
	if err := json.Unmarshal(resultBytes, &ping); err != nil {
		t.Fatalf("unmarshal ping result: %v", err)
	}
	if ping.Status != "ok" {
		t.Errorf("expected status ok, got %q", ping.Status)
	}
	if ping.Version != "test-1.0.0" {
		t.Errorf("expected version test-1.0.0, got %q", ping.Version)
	}

	srv.Shutdown()
}

func TestServer_ShutdownViaMethod(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- srv.Serve(ctx)
	}()

	// Wait for server.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send shutdown.
	enc := json.NewEncoder(conn)
	if err := enc.Encode(Request{ID: "s1", Method: MethodShutdown}); err != nil {
		t.Fatalf("send shutdown: %v", err)
	}

	// Server should exit.
	select {
	case err := <-doneCh:
		if err != nil {
			t.Errorf("server returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not shut down within timeout")
	}

	// Socket file should be cleaned up.
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after shutdown")
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	_ = enc.Encode(Request{ID: "u1", Method: "nonexistent"})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	dec := json.NewDecoder(conn)
	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("read response: %v", err)
	}

	if resp.Type != ResponseError {
		t.Errorf("expected error type, got %q", resp.Type)
	}
	if resp.Error == "" {
		t.Error("expected error message for unknown method")
	}
}

func TestServer_StaleSocketCleanup(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-stale.sock")
	defer os.Remove(socketPath)
	os.Remove(socketPath) // ensure clean start

	// Create a stale socket by listening and then stopping.
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	ln.Close()

	// On macOS, Close may remove the file. Skip if so.
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skip("platform removes socket file on Close(); cannot test stale cleanup")
	}

	// Starting a new server should clean up the stale socket and start.
	srv := New(socketPath, "test", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go srv.Serve(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	srv.Shutdown()
}

func TestServer_CategoriesMethod(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	_ = enc.Encode(Request{ID: "c1", Method: MethodCategories})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	dec := json.NewDecoder(conn)
	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("read response: %v", err)
	}

	if resp.Type != ResponseResult {
		t.Errorf("expected result type, got %q", resp.Type)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var cats CategoriesResult
	if err := json.Unmarshal(resultBytes, &cats); err != nil {
		t.Fatalf("unmarshal categories: %v", err)
	}

	if len(cats.Scanners) != 6 {
		t.Errorf("expected 6 scanners, got %d", len(cats.Scanners))
	}
}

func TestServer_MultipleRequestsSameConnection(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-multi.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)
	waitForSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send three pings on the same connection.
	for i := 1; i <= 3; i++ {
		sendRequest(t, conn, Request{ID: fmt.Sprintf("p%d", i), Method: MethodPing})
		resp := readResponse(t, conn)
		if resp.ID != fmt.Sprintf("p%d", i) {
			t.Errorf("ping %d: expected id p%d, got %q", i, i, resp.ID)
		}
		if resp.Type != ResponseResult {
			t.Errorf("ping %d: expected result type, got %q", i, resp.Type)
		}
	}
}

func TestServer_ClientDisconnectHandledGracefully(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-disc.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)
	waitForSocket(t, socketPath)

	// Connect and immediately disconnect.
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()

	// Give server time to process the disconnect.
	time.Sleep(50 * time.Millisecond)

	// Server should still be running — verify by connecting again.
	conn2, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("second dial failed (server crashed?): %v", err)
	}
	defer conn2.Close()

	sendRequest(t, conn2, Request{ID: "alive", Method: MethodPing})
	resp := readResponse(t, conn2)
	if resp.Type != ResponseResult {
		t.Errorf("expected result after reconnect, got %q", resp.Type)
	}
}

func TestServer_ContextCancellation(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- srv.Serve(ctx)
	}()
	waitForSocket(t, socketPath)

	// Cancel context should shut down the server.
	cancel()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Errorf("server returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not stop after context cancellation")
	}
}

func TestServer_NonSocketFileBlocks(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-socket")

	// Create a regular file at the socket path.
	if err := os.WriteFile(filePath, []byte("not a socket"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	srv := New(filePath, "test", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := srv.Serve(ctx)
	if err == nil {
		t.Fatal("expected error when socket path is a regular file")
	}
	if !strings.Contains(err.Error(), "not a socket") {
		t.Errorf("expected 'not a socket' error, got: %v", err)
	}
}

func TestServer_CleanupWithoutScan(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send cleanup without a token.
	enc := json.NewEncoder(conn)
	_ = enc.Encode(Request{ID: "cl1", Method: MethodCleanup})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	dec := json.NewDecoder(conn)
	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("read response: %v", err)
	}

	if resp.Type != ResponseError {
		t.Errorf("expected error type, got %q", resp.Type)
	}
	if resp.Error == "" {
		t.Error("expected error about missing token")
	}
	if !strings.Contains(resp.Error, "token is required") {
		t.Errorf("expected 'token is required' error, got: %q", resp.Error)
	}
}

func TestServer_CleanupWithInvalidToken(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-badtoken.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)
	waitForSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send cleanup with an invalid token.
	params, _ := json.Marshal(CleanupParams{Token: "bogus-token"})
	sendRequest(t, conn, Request{ID: "cl1", Method: MethodCleanup, Params: params})

	resp := readResponse(t, conn)
	if resp.Type != ResponseError {
		t.Errorf("expected error type, got %q", resp.Type)
	}
	if !strings.Contains(resp.Error, "invalid token") {
		t.Errorf("expected 'invalid token' error, got: %q", resp.Error)
	}
}
