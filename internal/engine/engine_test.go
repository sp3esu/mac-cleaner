package engine

import (
	"errors"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

func mockScanner(id, label string, results []scan.CategoryResult, err error) Scanner {
	return Scanner{
		ID:    id,
		Label: label,
		ScanFn: func() ([]scan.CategoryResult, error) {
			return results, err
		},
	}
}

func TestDefaultScanners_Count(t *testing.T) {
	scanners := DefaultScanners()
	if len(scanners) != 6 {
		t.Errorf("expected 6 default scanners, got %d", len(scanners))
	}
}

func TestDefaultScanners_UniqueIDs(t *testing.T) {
	scanners := DefaultScanners()
	seen := map[string]bool{}
	for _, s := range scanners {
		if seen[s.ID] {
			t.Errorf("duplicate scanner ID: %s", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestDefaultScanners_HaveLabels(t *testing.T) {
	for _, s := range DefaultScanners() {
		if s.Label == "" {
			t.Errorf("scanner %q has empty label", s.ID)
		}
	}
}

func TestDefaultScanners_HaveScanFn(t *testing.T) {
	for _, s := range DefaultScanners() {
		if s.ScanFn == nil {
			t.Errorf("scanner %q has nil ScanFn", s.ID)
		}
	}
}

func TestScanAll_AggregatesResults(t *testing.T) {
	scanners := []Scanner{
		mockScanner("a", "A", []scan.CategoryResult{
			{Category: "a-1", TotalSize: 100},
		}, nil),
		mockScanner("b", "B", []scan.CategoryResult{
			{Category: "b-1", TotalSize: 200},
			{Category: "b-2", TotalSize: 300},
		}, nil),
	}

	results := ScanAll(scanners, nil, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Category != "a-1" || results[1].Category != "b-1" || results[2].Category != "b-2" {
		t.Errorf("unexpected result order: %v", results)
	}
}

func TestScanAll_SkipsErroredScanners(t *testing.T) {
	scanners := []Scanner{
		mockScanner("ok", "OK", []scan.CategoryResult{
			{Category: "ok-1", TotalSize: 100},
		}, nil),
		mockScanner("fail", "Fail", nil, errors.New("boom")),
		mockScanner("ok2", "OK2", []scan.CategoryResult{
			{Category: "ok2-1", TotalSize: 50},
		}, nil),
	}

	results := ScanAll(scanners, nil, nil)
	if len(results) != 2 {
		t.Fatalf("expected 2 results (skipping errored), got %d", len(results))
	}
}

func TestScanAll_AppliesSkipSet(t *testing.T) {
	scanners := []Scanner{
		mockScanner("a", "A", []scan.CategoryResult{
			{Category: "keep-me", TotalSize: 100},
			{Category: "skip-me", TotalSize: 200},
		}, nil),
	}

	results := ScanAll(scanners, map[string]bool{"skip-me": true}, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after skip, got %d", len(results))
	}
	if results[0].Category != "keep-me" {
		t.Errorf("expected keep-me, got %q", results[0].Category)
	}
}

func TestScanAll_ProgressCallbacks(t *testing.T) {
	scanners := []Scanner{
		mockScanner("a", "A", []scan.CategoryResult{
			{Category: "a-1"},
		}, nil),
		mockScanner("b", "B", nil, errors.New("fail")),
	}

	var events []ScanEvent
	ScanAll(scanners, nil, func(e ScanEvent) {
		events = append(events, e)
	})

	// Expect: start_a, done_a, start_b, error_b
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	expected := []struct {
		typ string
		id  string
	}{
		{EventScannerStart, "a"},
		{EventScannerDone, "a"},
		{EventScannerStart, "b"},
		{EventScannerError, "b"},
	}

	for i, exp := range expected {
		if events[i].Type != exp.typ {
			t.Errorf("event[%d]: expected type %q, got %q", i, exp.typ, events[i].Type)
		}
		if events[i].ScannerID != exp.id {
			t.Errorf("event[%d]: expected scanner %q, got %q", i, exp.id, events[i].ScannerID)
		}
	}

	// Done event should carry results.
	if len(events[1].Results) != 1 {
		t.Errorf("done event should have 1 result, got %d", len(events[1].Results))
	}

	// Error event should carry error.
	if events[3].Err == nil {
		t.Error("error event should carry non-nil Err")
	}
}

func TestScanAll_NilProgressIsOK(t *testing.T) {
	scanners := []Scanner{
		mockScanner("a", "A", []scan.CategoryResult{{Category: "a-1"}}, nil),
	}
	// Should not panic.
	results := ScanAll(scanners, nil, nil)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestScanAll_EmptyScanners(t *testing.T) {
	results := ScanAll(nil, nil, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results from nil scanners, got %d", len(results))
	}
}

func TestFilterSkipped_EmptySkip(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}}
	got := FilterSkipped(results, nil)
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_FiltersMatching(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}, {Category: "c"}}
	got := FilterSkipped(results, map[string]bool{"b": true})
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	for _, r := range got {
		if r.Category == "b" {
			t.Error("category 'b' should have been filtered")
		}
	}
}

func TestFilterSkipped_NonMatchingSkip(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}}
	got := FilterSkipped(results, map[string]bool{"z": true})
	if len(got) != 2 {
		t.Errorf("expected 2 results, got %d", len(got))
	}
}

func TestFilterSkipped_NilResults(t *testing.T) {
	got := FilterSkipped(nil, map[string]bool{"a": true})
	if len(got) != 0 {
		t.Errorf("expected 0 results, got %d", len(got))
	}
}

func TestFilterSkipped_AllSkipped(t *testing.T) {
	results := []scan.CategoryResult{{Category: "a"}, {Category: "b"}}
	got := FilterSkipped(results, map[string]bool{"a": true, "b": true})
	if len(got) != 0 {
		t.Errorf("expected 0 results after skipping all, got %d", len(got))
	}
}
