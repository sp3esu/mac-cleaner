package scan

import (
	"encoding/json"
	"testing"
)

// overlapResults returns categories with every overlap kind: a file listed in
// two categories (same path), a cache nested inside another category's cache
// directory, a removed path nested inside a counted-but-not-removed component,
// and pseudo-path entries that must be ignored.
func overlapResults() []CategoryResult {
	return []CategoryResult{
		{
			Category:    "home-large-files",
			Description: "Large Files",
			Entries:     []ScanEntry{{Path: "/h/Movies/a.mp4", Size: 1000}},
			TotalSize:   1000,
		},
		{
			Category:    "home-old-files",
			Description: "Old Files",
			Entries:     []ScanEntry{{Path: "/h/Movies/a.mp4", Size: 1000}},
			TotalSize:   1000,
		},
		{
			Category:    "system-caches",
			Description: "User App Caches",
			Entries:     []ScanEntry{{Path: "/h/Library/Caches/Google", Size: 500}},
			TotalSize:   500,
		},
		{
			Category:    "browser-chrome",
			Description: "Chrome Cache",
			Entries:     []ScanEntry{{Path: "/h/Library/Caches/Google/Chrome/Default/", Size: 400}},
			TotalSize:   400,
		},
		{
			Category:    "unused-apps",
			Description: "Unused Applications",
			Entries: []ScanEntry{{
				Path: "/Applications/Zoom.app",
				Size: 330,
				Components: []SizeComponent{
					{Path: "/Applications/Zoom.app", Size: 30},
					{Path: "/h/Library/Application Support/zoom.us", Size: 300},
				},
			}},
			TotalSize: 330,
		},
		{
			Category:    "msg-zoom",
			Description: "Zoom Cache",
			Entries:     []ScanEntry{{Path: "/h/Library/Application Support/zoom.us/data", Size: 200}},
			TotalSize:   200,
		},
		{
			Category:    "dev-docker",
			Description: "Docker",
			Entries: []ScanEntry{{
				Path:       "docker:Images",
				Size:       20,
				Components: []SizeComponent{{Path: "docker:Images", Size: 20}},
			}},
			TotalSize: 20,
		},
		{
			Category:    "git-repos",
			Description: "Git Repos",
			Entries: []ScanEntry{{
				Path:       "git-gc:/h/Movies",
				Size:       50,
				Components: []SizeComponent{{Path: "/h/Movies/.git/objects", Size: 50}},
			}},
			TotalSize: 50,
		},
	}
}

func TestFindOverlaps(t *testing.T) {
	overlaps, dupCounted, dupRemoved := FindOverlaps(overlapResults())

	if len(overlaps) != 3 {
		t.Fatalf("expected 3 overlaps, got %d: %+v", len(overlaps), overlaps)
	}
	// Sorted by size, largest first.
	want := []struct{ inner, outer, innerCat, outerCat string }{
		{"/h/Movies/a.mp4", "/h/Movies/a.mp4", "Old Files", "Large Files"},
		{"/h/Library/Caches/Google/Chrome/Default", "/h/Library/Caches/Google", "Chrome Cache", "User App Caches"},
		{"/h/Library/Application Support/zoom.us/data", "/h/Library/Application Support/zoom.us", "Zoom Cache", "Unused Applications"},
	}
	for i, w := range want {
		o := overlaps[i]
		if o.Inner.Path != w.inner || o.Outer.Path != w.outer || o.Inner.Category != w.innerCat || o.Outer.Category != w.outerCat {
			t.Errorf("overlap %d = %+v, want %+v", i, o, w)
		}
	}
	if !overlaps[0].SamePath() || overlaps[1].SamePath() {
		t.Errorf("SamePath: got %v/%v, want true/false", overlaps[0].SamePath(), overlaps[1].SamePath())
	}
	if dupCounted != 1000+400+200 {
		t.Errorf("dupCounted = %d, want 1600", dupCounted)
	}
	// Zoom data is removed, but the zoom.us component covering it is not,
	// so its bytes are removed only once and are not a removal duplicate.
	if dupRemoved != 1000+400 {
		t.Errorf("dupRemoved = %d, want 1400", dupRemoved)
	}
}

func TestFindOverlaps_CaseInsensitive(t *testing.T) {
	// unused-apps probes "Application Support/<AppName>" while the messaging
	// scanner uses the on-disk lowercase name. On case-insensitive APFS they
	// are the same directory.
	results := []CategoryResult{
		{Description: "Unused Applications", Entries: []ScanEntry{{
			Path: "/Applications/Discord.app", Size: 160,
			Components: []SizeComponent{
				{Path: "/Applications/Discord.app", Size: 100},
				{Path: "/h/Library/Application Support/Discord", Size: 60},
			},
		}}},
		{Description: "Discord Cache", Entries: []ScanEntry{{Path: "/h/Library/Application Support/discord/Cache", Size: 60}}},
	}
	overlaps, dupCounted, dupRemoved := FindOverlaps(results)
	if len(overlaps) != 1 || dupCounted != 60 {
		t.Fatalf("overlaps=%d dupCounted=%d, want 1/60: %+v", len(overlaps), dupCounted, overlaps)
	}
	if overlaps[0].Outer.Path != "/h/Library/Application Support/Discord" {
		t.Errorf("outer = %q, want the Discord component", overlaps[0].Outer.Path)
	}
	// The cache is removed; the component covering it is not.
	if dupRemoved != 0 {
		t.Errorf("dupRemoved = %d, want 0", dupRemoved)
	}

	// The same path in two cases is the same path.
	same := []CategoryResult{
		{Description: "A", Entries: []ScanEntry{{Path: "/h/Library/Caches/Yarn", Size: 10}}},
		{Description: "B", Entries: []ScanEntry{{Path: "/h/Library/Caches/yarn", Size: 10}}},
	}
	overlaps, dupCounted, dupRemoved = FindOverlaps(same)
	if len(overlaps) != 1 || !overlaps[0].SamePath() || dupCounted != 10 || dupRemoved != 10 {
		t.Errorf("Yarn/yarn: overlaps=%+v dupCounted=%d dupRemoved=%d, want 1 same-path overlap, 10/10", overlaps, dupCounted, dupRemoved)
	}
}

func TestPathKey(t *testing.T) {
	for in, want := range map[string]string{
		"/h/Library/Application Support/Discord/": "/h/library/application support/discord",
		"/h/x/../Y": "/h/y",
	} {
		if got := PathKey(in); got != want {
			t.Errorf("PathKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindOverlaps_ChainCountsEachCoveredPath(t *testing.T) {
	results := []CategoryResult{
		{Description: "A", Entries: []ScanEntry{{Path: "/x", Size: 100}}},
		{Description: "B", Entries: []ScanEntry{{Path: "/x/y", Size: 60}}},
		{Description: "C", Entries: []ScanEntry{{Path: "/x/y/z", Size: 10}}},
	}
	overlaps, dupCounted, dupRemoved := FindOverlaps(results)
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %+v", overlaps)
	}
	// /x/y/z reports its nearest covering ancestor.
	if overlaps[1].Inner.Path != "/x/y/z" || overlaps[1].Outer.Path != "/x/y" {
		t.Errorf("unexpected nearest ancestor: %+v", overlaps[1])
	}
	// Total 170, counted once 100.
	if dupCounted != 70 || dupRemoved != 70 {
		t.Errorf("dupCounted=%d dupRemoved=%d, want 70/70", dupCounted, dupRemoved)
	}
}

func TestFindOverlaps_PrefixIsNotAncestor(t *testing.T) {
	results := []CategoryResult{
		{Description: "A", Entries: []ScanEntry{{Path: "/h/Caches/pip", Size: 100}}},
		{Description: "B", Entries: []ScanEntry{{Path: "/h/Caches/pipx", Size: 60}}},
	}
	if overlaps, d, _ := FindOverlaps(results); len(overlaps) != 0 || d != 0 {
		t.Errorf("sibling with shared prefix reported as overlap: %+v", overlaps)
	}
}

func TestFindOverlaps_RemovedCoveredOnlyByRemoved(t *testing.T) {
	// A removed path inside a removed path is removed once; a removed path
	// inside only a non-removed component is not a removal duplicate.
	results := []CategoryResult{
		{Description: "A", Entries: []ScanEntry{{
			Path: "/a.app", Size: 100,
			Components: []SizeComponent{{Path: "/lib", Size: 100}},
		}}},
		{Description: "B", Entries: []ScanEntry{{Path: "/lib", Size: 100}}},
		{Description: "C", Entries: []ScanEntry{{Path: "/lib/sub", Size: 40}}},
	}
	_, dupCounted, dupRemoved := FindOverlaps(results)
	if dupCounted != 140 {
		t.Errorf("dupCounted = %d, want 140", dupCounted)
	}
	if dupRemoved != 40 {
		t.Errorf("dupRemoved = %d, want 40", dupRemoved)
	}
}

func TestFindOverlaps_NotRemovedAtOrInsideRemoved(t *testing.T) {
	// System caches runs before unused apps, so a removed cache entry can
	// cover an unused app's ~/Library component, which cleanup does not
	// remove. Those bytes are counted twice but removed only once.
	results := []CategoryResult{
		{Description: "User App Caches", Entries: []ScanEntry{{Path: "/h/Library/Caches/com.foo", Size: 100}}},
		{Description: "Unused Applications", Entries: []ScanEntry{{
			Path: "/Applications/Foo.app", Size: 140,
			Components: []SizeComponent{
				{Path: "/Applications/Foo.app", Size: 10},
				{Path: "/h/Library/Caches/com.foo", Size: 100},
				{Path: "/h/Library/Caches/com.foo/sub", Size: 30},
			},
		}}},
	}
	overlaps, dupCounted, dupRemoved := FindOverlaps(results)
	if len(overlaps) != 2 {
		t.Fatalf("expected 2 overlaps, got %+v", overlaps)
	}
	if dupCounted != 130 {
		t.Errorf("dupCounted = %d, want 130", dupCounted)
	}
	if dupRemoved != 0 {
		t.Errorf("dupRemoved = %d, want 0: paths cleanup does not remove are not removal duplicates", dupRemoved)
	}
}

func TestFindOverlaps_PseudoPathsIgnored(t *testing.T) {
	results := []CategoryResult{
		// git's .git/objects component is a count-objects estimate under a
		// pseudo-path entry: a real file inside it must not overlap it.
		{Description: "Git Repos", Entries: []ScanEntry{{
			Path: "git-gc:/h/r", Size: 50,
			Components: []SizeComponent{{Path: "/h/r/.git/objects", Size: 50}},
		}}},
		{Description: "Large Files", Entries: []ScanEntry{{Path: "/h/r/.git/objects/pack/p.pack", Size: 30}}},
		// Non-filesystem components never overlap, even when two entries
		// with real paths both carry the same one.
		{Description: "A", Entries: []ScanEntry{{
			Path: "/h/a", Size: 5,
			Components: []SizeComponent{{Path: "docker:Images", Size: 5}},
		}}},
		{Description: "B", Entries: []ScanEntry{{
			Path: "/h/b", Size: 5,
			Components: []SizeComponent{{Path: "docker:Images", Size: 5}},
		}}},
	}
	if overlaps, dupCounted, dupRemoved := FindOverlaps(results); len(overlaps) != 0 || dupCounted != 0 || dupRemoved != 0 {
		t.Errorf("spurious overlap %+v (dupCounted=%d dupRemoved=%d)", overlaps, dupCounted, dupRemoved)
	}
}

func TestReclaimable(t *testing.T) {
	// Removed: 1000+1000+500+400+200 = 3100, minus 1400 removal duplicates.
	// Docker (20), git (50) and the Zoom bundle/library (330) add nothing.
	if got := Reclaimable(overlapResults()); got != 1700 {
		t.Errorf("Reclaimable = %d, want 1700", got)
	}
}

func TestReclaimable_PseudoAndBlocked(t *testing.T) {
	results := []CategoryResult{
		// A pseudo-path entry without a breakdown: RemovedSize would be
		// its Size, but cleanup deletes no file for it.
		{Entries: []ScanEntry{{Path: "docker:BuildCache", Size: 20_000}}},
		{Entries: []ScanEntry{{Path: "tmutil:snapshot:x", Components: []SizeComponent{{Path: "tmutil:snapshot:x"}}}}},
		// An /Applications bundle that safety blocks: counted, not removed.
		{Entries: []ScanEntry{{
			Path: "/Applications/Docker.app", Size: 71_000,
			Components: []SizeComponent{
				{Path: "/Applications/Docker.app", Size: 2_000},
				{Path: "/h/Library/Containers/com.docker.docker", Size: 69_000},
			},
		}}},
		// A removed component under a non-filesystem path is not a file.
		{Entries: []ScanEntry{{
			Path: "/h/x", Size: 7,
			Components: []SizeComponent{{Path: "docker:Images", Size: 7, Removed: true}},
		}}},
	}
	if got := Reclaimable(results); got != 0 {
		t.Errorf("Reclaimable = %d, want 0", got)
	}
	if got := Reclaimable(nil); got != 0 {
		t.Errorf("Reclaimable(nil) = %d, want 0", got)
	}
}

func TestSummarize(t *testing.T) {
	results := overlapResults()
	results[0].PermissionIssues = []PermissionIssue{{Path: "/h/x", Description: "denied"}}
	s := Summarize(results)

	// Total 1000+1000+500+400+330+200+20+50 = 3500, duplicates 1600.
	if s.TotalSize != 3500 || s.UniqueSize != 1900 || s.RemovableSize != 1700 {
		t.Errorf("total=%d unique=%d removable=%d, want 3500/1900/1700", s.TotalSize, s.UniqueSize, s.RemovableSize)
	}
	if !(s.RemovableSize <= s.UniqueSize && s.UniqueSize <= s.TotalSize) {
		t.Errorf("want removable <= unique <= total, got %d/%d/%d", s.RemovableSize, s.UniqueSize, s.TotalSize)
	}
	wantCat := map[string]int64{
		"home-large-files": 1000, "home-old-files": 1000, "system-caches": 500, "browser-chrome": 400,
		"unused-apps": 0, "msg-zoom": 200, "dev-docker": 0, "git-repos": 0,
	}
	for _, c := range s.Categories {
		if c.RemovableSize != wantCat[c.Category] {
			t.Errorf("%s RemovableSize = %d, want %d", c.Category, c.RemovableSize, wantCat[c.Category])
		}
	}
	if len(s.PermissionIssues) != 1 {
		t.Errorf("expected 1 permission issue, got %d", len(s.PermissionIssues))
	}
	// The input is not modified.
	if results[0].RemovableSize != 0 {
		t.Error("Summarize modified its input")
	}

	// New fields are added to the JSON; total_size keeps its meaning.
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"total_size", "unique_size", "removable_size", "categories"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing %q in JSON: %s", k, b)
		}
	}
	cat := m["categories"].([]any)[0].(map[string]any)
	if _, ok := cat["removable_size"]; !ok {
		t.Errorf("missing category removable_size in JSON: %v", cat)
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := Summarize(nil)
	if s.TotalSize != 0 || s.UniqueSize != 0 || s.RemovableSize != 0 || len(s.Categories) != 0 {
		t.Errorf("unexpected summary for no results: %+v", s)
	}
}
