package confirm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// render runs the prompt with "no" and returns its plain-text output.
func render(t *testing.T, results []scan.CategoryResult) string {
	t.Helper()
	old := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = old })
	out := &bytes.Buffer{}
	PromptConfirmation(strings.NewReader("no\n"), out, results)
	return out.String()
}

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

func TestConfirmation_TotalIsWhatCleanupDeletes(t *testing.T) {
	results := []scan.CategoryResult{
		{
			Category: "dev-docker", Description: "Docker Reclaimable",
			Entries: []scan.ScanEntry{{
				Path: "docker:Images", Size: 20_000_000_000, RiskLevel: safety.RiskRisky,
				Components: []scan.SizeComponent{{Path: "docker:Images", Size: 20_000_000_000, Note: "not a file path"}},
			}},
			TotalSize: 20_000_000_000,
		},
		{
			Category: "sysdata-timemachine", Description: "Time Machine",
			Entries: []scan.ScanEntry{{
				Path:       "tmutil:snapshot:com.apple.TimeMachine.x",
				Components: []scan.SizeComponent{{Path: "tmutil:snapshot:com.apple.TimeMachine.x"}},
			}},
		},
		{
			Category: "git-repos", Description: "Git Repositories",
			Entries: []scan.ScanEntry{{
				Path: "git-gc:/Users/x/repo", Size: 300_000_000, RiskLevel: safety.RiskModerate,
				Components: []scan.SizeComponent{{Path: "/Users/x/repo/.git/objects", Size: 300_000_000}},
			}},
			TotalSize: 300_000_000,
		},
		{
			Category: "unused-apps", Description: "Unused Applications",
			Entries: []scan.ScanEntry{{
				Path: "/Applications/Docker.app", Size: 71_000_000_000, RiskLevel: safety.RiskRisky,
				Components: []scan.SizeComponent{
					{Path: "/Applications/Docker.app", Size: 2_600_000_000, Note: "app bundle; cleanup is blocked by safety check: outside home directory"},
					{Path: "/Users/x/Library/Containers/com.docker.docker", Size: 68_400_000_000, Note: "app data in ~/Library"},
				},
			}},
			TotalSize: 71_000_000_000,
		},
	}
	out := render(t, results)

	deletedIdx := strings.Index(out, "The following items will be permanently deleted:")
	gitIdx := strings.Index(out, "git gc will repack (estimate ~300.0 MB):")
	listedIdx := strings.Index(out, "Listed but not deleted:")
	if deletedIdx < 0 || gitIdx < 0 || listedIdx < 0 || !(deletedIdx < gitIdx && gitIdx < listedIdx) {
		t.Fatalf("expected deleted, git gc and listed sections in order:\n%s", out)
	}
	deletedSection := out[deletedIdx:gitIdx]
	for _, p := range []string{"docker:", "tmutil:", "Docker.app", "repo"} {
		if strings.Contains(deletedSection, p) {
			t.Errorf("%q must not be listed as deleted:\n%s", p, out)
		}
	}
	if !strings.Contains(deletedSection, "(nothing)") {
		t.Errorf("empty deleted section should say (nothing):\n%s", out)
	}
	for _, want := range []string{
		"/Users/x/repo [moderate]  (~300.0 MB)",
		"docker:Images [risky]  (20.0 GB counted) - not a file path",
		"tmutil:snapshot:com.apple.TimeMachine.x  (0 B counted) - not a filesystem path",
		"/Applications/Docker.app [risky]  (71.0 GB counted) - app bundle; cleanup is blocked by safety check",
		"Total: 0 B will be permanently deleted.",
		"WARNING: Selection includes risky items",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestConfirmation_PartlyRemovedEntry(t *testing.T) {
	results := []scan.CategoryResult{{
		Category: "unused-apps", Description: "Unused Applications",
		Entries: []scan.ScanEntry{{
			Path: "/Users/x/Applications/X.app", Size: 1000, RiskLevel: safety.RiskRisky,
			Components: []scan.SizeComponent{
				{Path: "/Users/x/Applications/X.app", Size: 100, Removed: true, Note: "app bundle"},
				{Path: "/Users/x/Library/Application Support/X", Size: 900, Note: "app data in ~/Library"},
			},
		}},
		TotalSize: 1000,
	}}
	out := render(t, results)
	listedIdx := strings.Index(out, "Listed but not deleted:")
	if listedIdx < 0 {
		t.Fatalf("expected listed section:\n%s", out)
	}
	if !strings.Contains(out[:listedIdx], "/Users/x/Applications/X.app [risky]  (100 B)") {
		t.Errorf("bundle should be deleted with its removed size:\n%s", out)
	}
	if !strings.Contains(out[listedIdx:], "/Users/x/Library/Application Support/X  left in place (900 B) - app data in ~/Library") {
		t.Errorf("library data should be listed as left in place:\n%s", out)
	}
	if !strings.Contains(out, "Total: 100 B will be permanently deleted.") {
		t.Errorf("total should be the removed size:\n%s", out)
	}
}

func TestConfirmation_TotalCountsSharedPathsOnce(t *testing.T) {
	results := []scan.CategoryResult{
		{Category: "system-caches", Description: "User App Caches", Entries: []scan.ScanEntry{{Path: "/h/Library/Caches/Google", Size: 1000}}, TotalSize: 1000},
		{Category: "browser-chrome", Description: "Chrome", Entries: []scan.ScanEntry{{Path: "/h/Library/Caches/Google/Chrome/Default", Size: 400}}, TotalSize: 400},
	}
	out := render(t, results)
	if !strings.Contains(out, "Total: 1.0 kB will be permanently deleted.") {
		t.Errorf("nested path must be counted once in the total:\n%s", out)
	}
	if strings.Contains(out, "Listed but not deleted:") || strings.Contains(out, "git gc") {
		t.Errorf("unexpected sections for plain entries:\n%s", out)
	}
}

func TestConfirmation_NoRiskyWarningWithoutRisky(t *testing.T) {
	out := render(t, sampleResults())
	if strings.Contains(out, "WARNING") {
		t.Errorf("unexpected risky warning:\n%s", out)
	}
	if !strings.Contains(out, "Total: 4.5 kB will be permanently deleted.") {
		t.Errorf("unexpected total:\n%s", out)
	}
}

func TestEntryNote_Fallbacks(t *testing.T) {
	if got := entryNote(scan.ScanEntry{Path: "/a", Components: []scan.SizeComponent{{Path: "/b", Note: "first"}}}); got != "first" {
		t.Errorf("entryNote = %q, want first note", got)
	}
	if got := entryNote(scan.ScanEntry{Path: "/a", Components: []scan.SizeComponent{{Path: "/a"}}}); got != "" {
		t.Errorf("entryNote = %q, want empty", got)
	}
}
