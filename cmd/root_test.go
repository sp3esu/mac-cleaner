package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/sp3esu/mac-cleaner/internal/cleanup"
)

func TestPrintCleanupSummary_NoFailures(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	result := cleanup.CleanupResult{Removed: 5, BytesFreed: 1000000, Failed: 0}
	printCleanupSummary(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "5 items removed") {
		t.Errorf("expected removed count, got: %s", out)
	}
	if strings.Contains(out, "failed") {
		t.Errorf("should not mention failures when none exist, got: %s", out)
	}
}

func TestPrintCleanupSummary_WithFailures(t *testing.T) {
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var buf bytes.Buffer
	result := cleanup.CleanupResult{
		Removed:    3,
		BytesFreed: 500000,
		Failed:     2,
		Errors: []error{
			errors.New("skip non-filesystem path: docker:BuildCache"),
			errors.New("remove /tmp/locked: permission denied"),
		},
	}
	printCleanupSummary(&buf, result)

	out := buf.String()
	if !strings.Contains(out, "3 items removed") {
		t.Errorf("expected removed count, got: %s", out)
	}
	if !strings.Contains(out, "2 items failed:") {
		t.Errorf("expected failure header, got: %s", out)
	}
	if !strings.Contains(out, "  - skip non-filesystem path: docker:BuildCache") {
		t.Errorf("expected first error listed, got: %s", out)
	}
	if !strings.Contains(out, "  - remove /tmp/locked: permission denied") {
		t.Errorf("expected second error listed, got: %s", out)
	}
	if strings.Contains(out, "see warnings above") {
		t.Errorf("should not contain old message, got: %s", out)
	}
}
