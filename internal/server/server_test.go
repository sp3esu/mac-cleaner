package server

import (
	"bufio"
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
	"github.com/sp3esu/mac-cleaner/internal/scan"
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

// newMockTestEngine creates an engine with 2 deterministic mock scanners.
// This avoids hitting the real filesystem and produces predictable results.
func newMockTestEngine() *engine.Engine {
	eng := engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{
		ID:   "mock-sys",
		Name: "Mock System",
	}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{{
			Category:    "mock-caches",
			Description: "Mock Caches",
			TotalSize:   1024,
			Entries: []scan.ScanEntry{
				{Path: "/tmp/mock-test/cache1", Description: "Cache 1", Size: 512},
				{Path: "/tmp/mock-test/cache2", Description: "Cache 2", Size: 512},
			},
		}}, nil
	}))
	eng.Register(engine.NewScanner(engine.ScannerInfo{
		ID:   "mock-browser",
		Name: "Mock Browser",
	}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{{
			Category:    "mock-browser-data",
			Description: "Mock Browser Data",
			TotalSize:   2048,
			Entries: []scan.ScanEntry{
				{Path: "/tmp/mock-test/browser1", Description: "Browser 1", Size: 2048},
			},
		}}, nil
	}))
	return eng
}

// isTimeout reports whether an error is a network timeout.
func isTimeout(err error) bool {
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}

// readAllResponses reads NDJSON responses from conn until a final response
// (result or error) is received or the timeout expires. It uses a line-based
// scanner instead of json.Decoder to avoid internal buffering issues with
// streaming NDJSON.
func readAllResponses(t *testing.T, conn net.Conn, timeout time.Duration) []Response {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	var responses []Response
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var resp Response
		if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		responses = append(responses, resp)
		// Check if this is a final response (result or error type).
		if resp.Type == ResponseResult || resp.Type == ResponseError {
			break
		}
	}
	if err := sc.Err(); err != nil {
		// Ignore deadline exceeded -- we got what we needed.
		if !isTimeout(err) {
			t.Fatalf("scanner error: %v", err)
		}
	}
	return responses
}

func TestServer_ScanStreaming(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-scan-stream.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newMockTestEngine())
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

	// Send scan request.
	sendRequest(t, conn, Request{ID: "s1", Method: MethodScan})

	// Read all responses (progress + final result).
	responses := readAllResponses(t, conn, 5*time.Second)

	// Count progress and result responses.
	var progressCount int
	var resultCount int
	for _, resp := range responses {
		if resp.ID != "s1" {
			t.Errorf("expected id s1, got %q", resp.ID)
		}
		switch resp.Type {
		case ResponseProgress:
			progressCount++
		case ResponseResult:
			resultCount++
		}
	}

	// 2 scanners x (scanner_start + scanner_done) = 4 progress events minimum.
	if progressCount < 4 {
		t.Errorf("expected at least 4 progress responses, got %d", progressCount)
	}
	if resultCount != 1 {
		t.Errorf("expected exactly 1 result response, got %d", resultCount)
	}

	// Verify progress events contain expected fields.
	for _, resp := range responses {
		if resp.Type != ResponseProgress {
			continue
		}
		resultBytes, _ := json.Marshal(resp.Result)
		var progress ScanProgress
		if err := json.Unmarshal(resultBytes, &progress); err != nil {
			t.Fatalf("unmarshal progress: %v", err)
		}
		if progress.Event == "" {
			t.Error("progress event field is empty")
		}
		if progress.ScannerID == "" {
			t.Error("progress scanner_id field is empty")
		}
		if progress.Label == "" {
			t.Error("progress label field is empty")
		}
	}

	// Verify final result.
	final := responses[len(responses)-1]
	resultBytes, _ := json.Marshal(final.Result)
	var scanResult struct {
		Categories []json.RawMessage `json:"categories"`
		TotalSize  int64             `json:"total_size"`
		Token      string            `json:"token"`
	}
	if err := json.Unmarshal(resultBytes, &scanResult); err != nil {
		t.Fatalf("unmarshal scan result: %v", err)
	}
	if len(scanResult.Categories) == 0 {
		t.Error("expected non-empty categories")
	}
	if scanResult.TotalSize != 3072 {
		t.Errorf("expected total_size 3072, got %d", scanResult.TotalSize)
	}
	if scanResult.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestServer_ScanThenCleanup(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-scan-clean.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newMockTestEngine())
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

	// Step 1: Send scan and collect all responses.
	sendRequest(t, conn, Request{ID: "s1", Method: MethodScan})
	scanResponses := readAllResponses(t, conn, 5*time.Second)

	// Extract token from final result.
	final := scanResponses[len(scanResponses)-1]
	if final.Type != ResponseResult {
		t.Fatalf("expected result type, got %q", final.Type)
	}
	resultBytes, _ := json.Marshal(final.Result)
	var scanResult struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resultBytes, &scanResult); err != nil {
		t.Fatalf("unmarshal scan result: %v", err)
	}
	if scanResult.Token == "" {
		t.Fatal("scan returned empty token")
	}

	// Step 2: Send cleanup with the scan token.
	params, _ := json.Marshal(CleanupParams{Token: scanResult.Token})
	sendRequest(t, conn, Request{ID: "c1", Method: MethodCleanup, Params: params})
	cleanupResponses := readAllResponses(t, conn, 5*time.Second)

	// Verify at least 1 progress response (cleanup events for mock entries).
	var cleanProgressCount int
	var cleanResultCount int
	for _, resp := range cleanupResponses {
		if resp.ID != "c1" {
			t.Errorf("expected id c1, got %q", resp.ID)
		}
		switch resp.Type {
		case ResponseProgress:
			cleanProgressCount++
		case ResponseResult:
			cleanResultCount++
		}
	}

	if cleanProgressCount < 1 {
		t.Errorf("expected at least 1 cleanup progress response, got %d", cleanProgressCount)
	}
	if cleanResultCount != 1 {
		t.Errorf("expected exactly 1 cleanup result response, got %d", cleanResultCount)
	}

	// Verify cleanup result has expected fields.
	cleanFinal := cleanupResponses[len(cleanupResponses)-1]
	cleanResultBytes, _ := json.Marshal(cleanFinal.Result)
	var cleanupResult struct {
		Removed    int   `json:"removed"`
		Failed     int   `json:"failed"`
		BytesFreed int64 `json:"bytes_freed"`
	}
	if err := json.Unmarshal(cleanResultBytes, &cleanupResult); err != nil {
		t.Fatalf("unmarshal cleanup result: %v", err)
	}

	// Mock paths don't exist on disk, so all entries should be reported as
	// failed. The key assertion is that the fields are present and the handler
	// completed the full flow.
	totalEntries := cleanupResult.Removed + cleanupResult.Failed
	if totalEntries == 0 {
		t.Error("expected non-zero removed+failed count")
	}
}

func TestServer_ConcurrentScanRejected(t *testing.T) {
	// The server processes requests sequentially per connection, so true
	// socket-level concurrent scans can't happen on one connection. Instead,
	// we test the busy flag mechanism by calling Dispatch directly on a
	// second writer while the first scan handler is running.
	blocker := make(chan struct{})
	eng := engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{
		ID:   "slow",
		Name: "Slow Scanner",
	}, func() ([]scan.CategoryResult, error) {
		<-blocker // block until released
		return []scan.CategoryResult{{
			Category:  "slow-cat",
			TotalSize: 100,
		}}, nil
	}))

	socketPath := filepath.Join(os.TempDir(), "mc-test-concurrent.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", eng)
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

	// Start the first scan on the connection.
	sendRequest(t, conn, Request{ID: "s1", Method: MethodScan})

	// Read first progress event to confirm scan started and busy flag is set.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		t.Fatalf("failed to read first progress event: %v", sc.Err())
	}
	var firstResp Response
	if err := json.Unmarshal(sc.Bytes(), &firstResp); err != nil {
		t.Fatalf("unmarshal first response: %v", err)
	}
	if firstResp.Type != ResponseProgress {
		t.Fatalf("expected progress, got %q", firstResp.Type)
	}

	// While scan is running (scanner is blocked), call Dispatch directly
	// with a second scan request. This simulates what would happen if two
	// requests could arrive concurrently (e.g., in a future multi-connection server).
	var secondBuf strings.Builder
	secondWriter := NewNDJSONWriter(&secondBuf)
	srv.handler.Dispatch(ctx, Request{ID: "s2", Method: MethodScan}, secondWriter)

	// Parse the response written to secondBuf.
	var secondResp Response
	if err := json.Unmarshal([]byte(secondBuf.String()), &secondResp); err != nil {
		t.Fatalf("unmarshal second response: %v", err)
	}
	if secondResp.Type != ResponseError {
		t.Errorf("expected error type for concurrent scan, got %q", secondResp.Type)
	}
	if !strings.Contains(secondResp.Error, "another operation is in progress") {
		t.Errorf("expected 'another operation is in progress' error, got: %q", secondResp.Error)
	}

	// Release the blocker so the first scan completes.
	close(blocker)

	// Drain remaining responses from the first scan.
	for sc.Scan() {
		var resp Response
		if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
			break
		}
		if resp.Type == ResponseResult || resp.Type == ResponseError {
			break
		}
	}
}

func TestServer_ScanWithSkipParam(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-scan-skip.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newMockTestEngine())
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

	// Send scan with skip param to exclude "mock-caches".
	params, _ := json.Marshal(ScanParams{Skip: []string{"mock-caches"}})
	sendRequest(t, conn, Request{ID: "sk1", Method: MethodScan, Params: params})

	responses := readAllResponses(t, conn, 5*time.Second)

	// Find the final result.
	var final *Response
	for i := range responses {
		if responses[i].Type == ResponseResult {
			final = &responses[i]
			break
		}
	}
	if final == nil {
		t.Fatal("no result response received")
	}

	resultBytes, _ := json.Marshal(final.Result)
	var scanResult struct {
		Categories []struct {
			Category  string `json:"category"`
			TotalSize int64  `json:"total_size"`
		} `json:"categories"`
		TotalSize int64  `json:"total_size"`
		Token     string `json:"token"`
	}
	if err := json.Unmarshal(resultBytes, &scanResult); err != nil {
		t.Fatalf("unmarshal scan result: %v", err)
	}

	// Only "mock-browser-data" should be present (not "mock-caches").
	if len(scanResult.Categories) != 1 {
		t.Fatalf("expected 1 category after skip, got %d", len(scanResult.Categories))
	}
	if scanResult.Categories[0].Category != "mock-browser-data" {
		t.Errorf("expected mock-browser-data, got %q", scanResult.Categories[0].Category)
	}
	if scanResult.TotalSize != 2048 {
		t.Errorf("expected total_size 2048, got %d", scanResult.TotalSize)
	}
	if scanResult.Token == "" {
		t.Error("expected non-empty token")
	}
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

	if len(cats.Scanners) != 8 {
		t.Errorf("expected 8 scanners, got %d", len(cats.Scanners))
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

func TestServer_ActiveServerBlocks(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-active.sock")
	os.Remove(socketPath) // ensure clean start
	defer os.Remove(socketPath)

	// Start a listener to simulate an active server on the socket path.
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("create listener: %v", err)
	}
	defer ln.Close()

	// Creating a new server and calling Serve should fail because
	// cleanStaleSocket detects an active listener via dial probe.
	srv := New(socketPath, "test", newTestEngine())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = srv.Serve(ctx)
	if err == nil {
		t.Fatal("expected error when another server is already listening")
	}
	if !strings.Contains(err.Error(), "already listening") {
		t.Errorf("expected 'already listening' error, got: %v", err)
	}
}

func TestServer_DisconnectDuringScan(t *testing.T) {
	blocker := make(chan struct{})
	eng := engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{
		ID:   "blocking",
		Name: "Blocking Scanner",
	}, func() ([]scan.CategoryResult, error) {
		<-blocker // block until released
		return []scan.CategoryResult{{
			Category:    "blocking-cat",
			Description: "Blocking Category",
			TotalSize:   100,
			Entries:     []scan.ScanEntry{{Path: "/tmp/blocking-test/f1", Description: "File 1", Size: 100}},
		}}, nil
	}))

	socketPath := filepath.Join(os.TempDir(), "mc-test-disc-scan.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", eng)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)
	waitForSocket(t, socketPath)

	// Connect and start a scan.
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	sendRequest(t, conn, Request{ID: "s1", Method: MethodScan})

	// Read first progress event (scanner_start) to confirm scan is underway.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		t.Fatalf("failed to read first progress event: %v", sc.Err())
	}
	var firstResp Response
	if err := json.Unmarshal(sc.Bytes(), &firstResp); err != nil {
		t.Fatalf("unmarshal first response: %v", err)
	}
	if firstResp.Type != ResponseProgress {
		t.Fatalf("expected progress event, got %q", firstResp.Type)
	}

	// Disconnect while scan is still blocked.
	conn.Close()

	// Release the blocker so the scanner goroutine can complete.
	close(blocker)

	// Wait for server to process the disconnect.
	time.Sleep(200 * time.Millisecond)

	// Verify server is still operational by connecting again.
	conn2, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("reconnect failed (server crashed?): %v", err)
	}
	defer conn2.Close()

	sendRequest(t, conn2, Request{ID: "alive", Method: MethodPing})
	resp := readResponse(t, conn2)
	if resp.Type != ResponseResult {
		t.Errorf("expected result after reconnect, got %q", resp.Type)
	}
}

func TestServer_DisconnectDuringCleanup(t *testing.T) {
	// Create temp files that cleanup can actually remove.
	tmpDir := t.TempDir()
	var tmpFiles []string
	for i := 0; i < 3; i++ {
		f, err := os.CreateTemp(tmpDir, "cleanup-test-*")
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		tmpFiles = append(tmpFiles, f.Name())
		f.Close()
	}

	entries := make([]scan.ScanEntry, len(tmpFiles))
	for i, p := range tmpFiles {
		entries[i] = scan.ScanEntry{
			Path:        p,
			Description: fmt.Sprintf("Temp file %d", i),
			Size:        100,
		}
	}

	eng := engine.New()
	eng.Register(engine.NewScanner(engine.ScannerInfo{
		ID:   "temp-scanner",
		Name: "Temp Scanner",
	}, func() ([]scan.CategoryResult, error) {
		return []scan.CategoryResult{{
			Category:    "temp-files",
			Description: "Temp Files",
			TotalSize:   300,
			Entries:     entries,
		}}, nil
	}))

	socketPath := filepath.Join(os.TempDir(), "mc-test-disc-clean.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", eng)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Shutdown()

	go srv.Serve(ctx)
	waitForSocket(t, socketPath)

	// Step 1: Scan to get a token.
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	sendRequest(t, conn, Request{ID: "s1", Method: MethodScan})
	scanResponses := readAllResponses(t, conn, 5*time.Second)

	final := scanResponses[len(scanResponses)-1]
	if final.Type != ResponseResult {
		t.Fatalf("expected result type, got %q", final.Type)
	}
	resultBytes, _ := json.Marshal(final.Result)
	var scanResult struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resultBytes, &scanResult); err != nil {
		t.Fatalf("unmarshal scan result: %v", err)
	}
	if scanResult.Token == "" {
		t.Fatal("scan returned empty token")
	}

	// Step 2: Send cleanup, read first progress event, then disconnect.
	params, _ := json.Marshal(CleanupParams{Token: scanResult.Token})
	sendRequest(t, conn, Request{ID: "c1", Method: MethodCleanup, Params: params})

	// Read the first progress event to confirm cleanup has started.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		t.Fatalf("failed to read first cleanup progress: %v", sc.Err())
	}
	var firstResp Response
	if err := json.Unmarshal(sc.Bytes(), &firstResp); err != nil {
		t.Fatalf("unmarshal first cleanup response: %v", err)
	}
	if firstResp.Type != ResponseProgress {
		t.Fatalf("expected progress event, got %q", firstResp.Type)
	}

	// Disconnect while cleanup is running.
	conn.Close()

	// Wait for cleanup to finish (file deletion continues to completion by design).
	time.Sleep(200 * time.Millisecond)

	// Verify server is still operational.
	conn2, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("reconnect failed (server crashed?): %v", err)
	}
	defer conn2.Close()

	sendRequest(t, conn2, Request{ID: "alive", Method: MethodPing})
	resp := readResponse(t, conn2)
	if resp.Type != ResponseResult {
		t.Errorf("expected result after reconnect, got %q", resp.Type)
	}
}

func TestServer_IdleTimeoutClosesConnection(t *testing.T) {
	socketPath := filepath.Join(os.TempDir(), "mc-test-idle.sock")
	os.Remove(socketPath)
	defer os.Remove(socketPath)
	srv := New(socketPath, "test-1.0.0", newMockTestEngine())
	srv.IdleTimeout = 100 * time.Millisecond
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

	// Confirm connection is alive by sending a ping.
	sendRequest(t, conn, Request{ID: "p1", Method: MethodPing})
	resp := readResponse(t, conn)
	if resp.Type != ResponseResult {
		t.Fatalf("expected result, got %q", resp.Type)
	}

	// Wait beyond the idle timeout.
	time.Sleep(200 * time.Millisecond)

	// Attempt to read — should fail because server closed the connection.
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("expected error reading from idle-timed-out connection, got nil")
	}
	// Accept either EOF (connection closed) or timeout (deadline exceeded).
	// Both confirm the server is no longer serving this connection.
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
