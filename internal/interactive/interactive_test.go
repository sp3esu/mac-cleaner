package interactive

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

func TestRunWalkthrough_RemovesMarked(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "item-a", Size: 1000},
				{Path: "/tmp/b", Description: "item-b", Size: 2000},
				{Path: "/tmp/c", Description: "item-c", Size: 3000},
			},
			TotalSize: 6000,
		},
	}

	in := strings.NewReader("r\nk\nr\n")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, results)

	if len(got) != 1 {
		t.Fatalf("expected 1 category, got %d", len(got))
	}
	if len(got[0].Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got[0].Entries))
	}
	if got[0].Entries[0].Path != "/tmp/a" {
		t.Errorf("expected first removed entry /tmp/a, got %s", got[0].Entries[0].Path)
	}
	if got[0].Entries[1].Path != "/tmp/c" {
		t.Errorf("expected second removed entry /tmp/c, got %s", got[0].Entries[1].Path)
	}
	if got[0].TotalSize != 4000 {
		t.Errorf("expected TotalSize 4000, got %d", got[0].TotalSize)
	}
}

func TestRunWalkthrough_AllKeep(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "item-a", Size: 1000},
				{Path: "/tmp/b", Description: "item-b", Size: 2000},
			},
			TotalSize: 3000,
		},
	}

	in := strings.NewReader("k\nk\n")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, results)

	if got != nil {
		t.Fatalf("expected nil when all items kept, got %v", got)
	}
	if !strings.Contains(out.String(), "Nothing marked for removal.") {
		t.Errorf("expected 'Nothing marked for removal.' in output, got:\n%s", out.String())
	}
}

func TestRunWalkthrough_AllRemove(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "item-a", Size: 1000},
				{Path: "/tmp/b", Description: "item-b", Size: 2000},
			},
			TotalSize: 3000,
		},
	}

	in := strings.NewReader("r\nr\n")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, results)

	if len(got) != 1 {
		t.Fatalf("expected 1 category, got %d", len(got))
	}
	if len(got[0].Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got[0].Entries))
	}
	if got[0].TotalSize != 3000 {
		t.Errorf("expected TotalSize 3000, got %d", got[0].TotalSize)
	}
}

func TestRunWalkthrough_EmptyResults(t *testing.T) {
	in := strings.NewReader("")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, []scan.CategoryResult{})

	if got != nil {
		t.Fatalf("expected nil for empty results, got %v", got)
	}
	if !strings.Contains(out.String(), "Nothing to clean.") {
		t.Errorf("expected 'Nothing to clean.' in output, got:\n%s", out.String())
	}
}

func TestRunWalkthrough_MultipleCategories(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "cat-a",
			Description: "Category A",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a1", Description: "a-item", Size: 1000},
			},
			TotalSize: 1000,
		},
		{
			Category:    "cat-b",
			Description: "Category B",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/b1", Description: "b-item", Size: 2000},
			},
			TotalSize: 2000,
		},
	}

	in := strings.NewReader("r\nr\n")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, results)

	if len(got) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(got))
	}
	if got[0].Category != "cat-a" {
		t.Errorf("expected first category cat-a, got %s", got[0].Category)
	}
	if got[1].Category != "cat-b" {
		t.Errorf("expected second category cat-b, got %s", got[1].Category)
	}
}

func TestRunWalkthrough_EOF(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "item-a", Size: 1000},
				{Path: "/tmp/b", Description: "item-b", Size: 2000},
				{Path: "/tmp/c", Description: "item-c", Size: 3000},
			},
			TotalSize: 6000,
		},
	}

	// Only one line of input; remaining entries get EOF -> default to keep.
	in := strings.NewReader("r\n")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, results)

	if len(got) != 1 {
		t.Fatalf("expected 1 category, got %d", len(got))
	}
	if len(got[0].Entries) != 1 {
		t.Fatalf("expected 1 entry (only first marked remove), got %d", len(got[0].Entries))
	}
	if got[0].Entries[0].Path != "/tmp/a" {
		t.Errorf("expected removed entry /tmp/a, got %s", got[0].Entries[0].Path)
	}
}

func TestRunWalkthrough_InvalidInputReprompts(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "item-a", Size: 1000},
			},
			TotalSize: 1000,
		},
	}

	// "x" is invalid, should re-prompt, then "r" is valid.
	in := strings.NewReader("x\nr\n")
	out := &bytes.Buffer{}

	got := RunWalkthrough(in, out, results)

	output := out.String()
	if !strings.Contains(output, "Please enter 'k' to keep or 'r' to remove:") {
		t.Errorf("expected re-prompt in output, got:\n%s", output)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 category after re-prompt, got %d", len(got))
	}
	if got[0].Entries[0].Path != "/tmp/a" {
		t.Errorf("expected removed entry /tmp/a, got %s", got[0].Entries[0].Path)
	}
}

func TestRunWalkthrough_ProgressIndicator(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category:    "test",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/a", Description: "item-a", Size: 1000},
				{Path: "/tmp/b", Description: "item-b", Size: 2000},
				{Path: "/tmp/c", Description: "item-c", Size: 3000},
			},
			TotalSize: 6000,
		},
	}

	in := strings.NewReader("k\nk\nk\n")
	out := &bytes.Buffer{}

	RunWalkthrough(in, out, results)

	output := out.String()
	if !strings.Contains(output, "[1/3]") {
		t.Errorf("expected [1/3] progress indicator, got:\n%s", output)
	}
	if !strings.Contains(output, "[3/3]") {
		t.Errorf("expected [3/3] progress indicator, got:\n%s", output)
	}
}

func TestRunWalkthrough_ShorthandInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // "remove" or "keep"
	}{
		{"k shorthand", "k\n", "keep"},
		{"r shorthand", "r\n", "remove"},
		{"keep full", "keep\n", "keep"},
		{"remove full", "remove\n", "remove"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := []scan.CategoryResult{
				{
					Category:    "test",
					Description: "Test Category",
					Entries: []scan.ScanEntry{
						{Path: "/tmp/a", Description: "item-a", Size: 1000},
					},
					TotalSize: 1000,
				},
			}

			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}

			got := RunWalkthrough(in, out, results)

			if tt.want == "keep" && got != nil {
				t.Errorf("expected nil (keep), got %v", got)
			}
			if tt.want == "remove" && (got == nil || len(got[0].Entries) != 1) {
				t.Errorf("expected 1 removed entry, got %v", got)
			}
		})
	}
}
