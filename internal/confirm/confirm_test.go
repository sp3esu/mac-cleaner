package confirm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

func sampleResults() []scan.CategoryResult {
	return []scan.CategoryResult{
		{
			Category:    "test-category",
			Description: "Test Category",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/testdir/foo", Description: "foo", Size: 1500},
				{Path: "/tmp/testdir/bar", Description: "bar", Size: 3000},
			},
			TotalSize: 4500,
		},
	}
}

func TestConfirmationYes(t *testing.T) {
	in := strings.NewReader("yes\n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, sampleResults())
	if !got {
		t.Fatal("expected true for 'yes' input")
	}
}

func TestConfirmationNo(t *testing.T) {
	in := strings.NewReader("no\n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, sampleResults())
	if got {
		t.Fatal("expected false for 'no' input")
	}
}

func TestConfirmationEmpty(t *testing.T) {
	in := strings.NewReader("\n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, sampleResults())
	if got {
		t.Fatal("expected false for empty input")
	}
}

func TestConfirmationYUppercase(t *testing.T) {
	in := strings.NewReader("Yes\n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, sampleResults())
	if got {
		t.Fatal("expected false for 'Yes' (case-sensitive)")
	}
}

func TestConfirmationJustY(t *testing.T) {
	in := strings.NewReader("y\n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, sampleResults())
	if got {
		t.Fatal("expected false for 'y'")
	}
}

func TestConfirmationWithWhitespace(t *testing.T) {
	in := strings.NewReader("  yes  \n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, sampleResults())
	if !got {
		t.Fatal("expected true for '  yes  ' (whitespace-trimmed)")
	}
}

func TestConfirmationOutputContainsPath(t *testing.T) {
	in := strings.NewReader("no\n")
	out := &bytes.Buffer{}
	PromptConfirmation(in, out, sampleResults())

	output := out.String()
	if !strings.Contains(output, "/tmp/testdir/foo") {
		t.Errorf("output should contain path /tmp/testdir/foo, got:\n%s", output)
	}
	if !strings.Contains(output, "/tmp/testdir/bar") {
		t.Errorf("output should contain path /tmp/testdir/bar, got:\n%s", output)
	}
}

func TestConfirmationOutputContainsSize(t *testing.T) {
	in := strings.NewReader("no\n")
	out := &bytes.Buffer{}
	results := []scan.CategoryResult{
		{
			Category:    "sized",
			Description: "Sized Items",
			Entries: []scan.ScanEntry{
				{Path: "/tmp/big", Description: "big", Size: 5000000},
			},
			TotalSize: 5000000,
		},
	}
	PromptConfirmation(in, out, results)

	output := out.String()
	if !strings.Contains(output, "5.0 MB") {
		t.Errorf("output should contain formatted size '5.0 MB', got:\n%s", output)
	}
}

func TestConfirmationEmptyResults(t *testing.T) {
	in := strings.NewReader("yes\n")
	out := &bytes.Buffer{}
	got := PromptConfirmation(in, out, []scan.CategoryResult{})
	if !got {
		t.Fatal("expected true for 'yes' input even with empty results")
	}
}
