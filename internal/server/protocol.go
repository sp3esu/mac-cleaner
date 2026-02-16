// Package server implements a Unix domain socket IPC server using an
// NDJSON (newline-delimited JSON) protocol. It enables a Swift macOS app
// to control scanning and cleanup with real-time streaming progress.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Method constants for the NDJSON protocol.
const (
	MethodPing       = "ping"
	MethodShutdown   = "shutdown"
	MethodScan       = "scan"
	MethodCleanup    = "cleanup"
	MethodCategories = "categories"
)

// Request is the client-to-server NDJSON message.
type Request struct {
	// ID is a client-assigned identifier echoed in all responses.
	ID string `json:"id"`
	// Method is the RPC method name (ping, scan, cleanup, categories, shutdown).
	Method string `json:"method"`
	// Params holds method-specific parameters.
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is the server-to-client NDJSON message.
type Response struct {
	// ID echoes the request ID.
	ID string `json:"id"`
	// Type distinguishes message types: "result", "progress", "error".
	Type string `json:"type"`
	// Result holds method-specific result data (for "result" type).
	Result any `json:"result,omitempty"`
	// Error holds an error message (for "error" type).
	Error string `json:"error,omitempty"`
}

// Response types.
const (
	ResponseResult   = "result"
	ResponseProgress = "progress"
	ResponseError    = "error"
)

// ScanParams holds parameters for the scan method.
type ScanParams struct {
	// Skip lists category IDs to exclude from results.
	Skip []string `json:"skip,omitempty"`
}

// CleanupParams holds parameters for the cleanup method.
type CleanupParams struct {
	// Token is the scan token returned by a prior scan operation.
	// Required for cleanup (replay protection).
	Token string `json:"token"`
	// Categories lists the category IDs to clean up. Must match a prior scan.
	Categories []string `json:"categories,omitempty"`
}

// PingResult is the result of a ping request.
type PingResult struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// NDJSONWriter writes NDJSON responses to a writer. It is safe for
// concurrent use.
type NDJSONWriter struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewNDJSONWriter creates a new NDJSON writer.
func NewNDJSONWriter(w io.Writer) *NDJSONWriter {
	return &NDJSONWriter{enc: json.NewEncoder(w)}
}

// Write sends a single NDJSON response.
func (w *NDJSONWriter) Write(resp Response) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(resp)
}

// WriteResult sends a result response.
func (w *NDJSONWriter) WriteResult(id string, result any) error {
	return w.Write(Response{ID: id, Type: ResponseResult, Result: result})
}

// WriteProgress sends a progress response.
func (w *NDJSONWriter) WriteProgress(id string, progress any) error {
	return w.Write(Response{ID: id, Type: ResponseProgress, Result: progress})
}

// WriteError sends an error response.
func (w *NDJSONWriter) WriteError(id string, err error) error {
	return w.Write(Response{ID: id, Type: ResponseError, Error: err.Error()})
}

// WriteErrorMsg sends an error response with a string message.
func (w *NDJSONWriter) WriteErrorMsg(id, msg string) error {
	return w.Write(Response{ID: id, Type: ResponseError, Error: msg})
}

// NDJSONReader reads NDJSON requests from a reader.
type NDJSONReader struct {
	scanner *bufio.Scanner
}

// NewNDJSONReader creates a new NDJSON reader.
func NewNDJSONReader(r io.Reader) *NDJSONReader {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB per line
	return &NDJSONReader{scanner: s}
}

// Read reads the next NDJSON request. Returns io.EOF when the reader is closed.
func (r *NDJSONReader) Read() (Request, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return Request{}, fmt.Errorf("reading request: %w", err)
		}
		return Request{}, io.EOF
	}

	var req Request
	if err := json.Unmarshal(r.scanner.Bytes(), &req); err != nil {
		return Request{}, fmt.Errorf("decoding request: %w", err)
	}
	return req, nil
}
