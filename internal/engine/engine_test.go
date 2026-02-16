package engine

import (
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// Tests are being migrated to the new struct-based API.
// FilterSkipped tests remain unchanged as it is still package-level.

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
