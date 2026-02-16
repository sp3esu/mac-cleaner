package server

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestNDJSONWriter_WriteResult(t *testing.T) {
	var buf bytes.Buffer
	w := NewNDJSONWriter(&buf)

	err := w.WriteResult("req-1", map[string]string{"status": "ok"})
	if err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "req-1" {
		t.Errorf("expected id req-1, got %q", resp.ID)
	}
	if resp.Type != ResponseResult {
		t.Errorf("expected type result, got %q", resp.Type)
	}
	if resp.Error != "" {
		t.Errorf("expected no error, got %q", resp.Error)
	}
}

func TestNDJSONWriter_WriteProgress(t *testing.T) {
	var buf bytes.Buffer
	w := NewNDJSONWriter(&buf)

	err := w.WriteProgress("req-2", map[string]int{"done": 5})
	if err != nil {
		t.Fatalf("WriteProgress: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != ResponseProgress {
		t.Errorf("expected type progress, got %q", resp.Type)
	}
}

func TestNDJSONWriter_WriteError(t *testing.T) {
	var buf bytes.Buffer
	w := NewNDJSONWriter(&buf)

	err := w.WriteErrorMsg("req-3", "something went wrong")
	if err != nil {
		t.Fatalf("WriteErrorMsg: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != ResponseError {
		t.Errorf("expected type error, got %q", resp.Type)
	}
	if resp.Error != "something went wrong" {
		t.Errorf("expected error message, got %q", resp.Error)
	}
}

func TestNDJSONWriter_MultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	w := NewNDJSONWriter(&buf)

	_ = w.WriteResult("1", "first")
	_ = w.WriteResult("2", "second")
	_ = w.WriteResult("3", "third")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), buf.String())
	}

	for i, line := range lines {
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Errorf("line %d: unmarshal: %v", i, err)
		}
	}
}

func TestNDJSONReader_Read(t *testing.T) {
	input := `{"id":"1","method":"ping"}` + "\n" +
		`{"id":"2","method":"scan","params":{"skip":["dev-docker"]}}` + "\n"

	reader := NewNDJSONReader(strings.NewReader(input))

	req1, err := reader.Read()
	if err != nil {
		t.Fatalf("read 1: %v", err)
	}
	if req1.ID != "1" || req1.Method != "ping" {
		t.Errorf("req1: got id=%q method=%q", req1.ID, req1.Method)
	}

	req2, err := reader.Read()
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if req2.ID != "2" || req2.Method != "scan" {
		t.Errorf("req2: got id=%q method=%q", req2.ID, req2.Method)
	}
	if req2.Params == nil {
		t.Error("req2: expected params")
	}
}

func TestNDJSONReader_EOF(t *testing.T) {
	reader := NewNDJSONReader(strings.NewReader(""))
	_, err := reader.Read()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestNDJSONReader_InvalidJSON(t *testing.T) {
	reader := NewNDJSONReader(strings.NewReader("not json\n"))
	_, err := reader.Read()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decoding request") {
		t.Errorf("expected decoding error, got: %v", err)
	}
}

func TestNDJSONReader_EmptyObject(t *testing.T) {
	reader := NewNDJSONReader(strings.NewReader("{}\n"))
	req, err := reader.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if req.ID != "" || req.Method != "" {
		t.Errorf("expected empty fields, got id=%q method=%q", req.ID, req.Method)
	}
}

func TestRequestResponseRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	// Write a request.
	req := Request{ID: "test-1", Method: MethodScan}
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	// Read it back.
	reader := NewNDJSONReader(&buf)
	got, err := reader.Read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.ID != req.ID || got.Method != req.Method {
		t.Errorf("roundtrip mismatch: got id=%q method=%q", got.ID, got.Method)
	}
}
