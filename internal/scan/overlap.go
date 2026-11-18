package scan

import (
	"path/filepath"
	"sort"
	"strings"
)

// CountedPath is one filesystem path whose size a scan entry counts: the
// entry's own path, or one of its size components.
type CountedPath struct {
	// Path is the cleaned absolute path.
	Path string
	// Size is the number of bytes counted for Path.
	Size int64
	// Removed reports whether cleanup deletes Path.
	Removed bool
	// Category is the description of the category counting Path, for display.
	Category string
}

// Overlap is a counted path that is also covered by another counted path
// (the same path, or an ancestor directory), so its bytes are counted more
// than once in the totals.
type Overlap struct {
	Inner CountedPath
	Outer CountedPath
}

// SamePath reports whether Inner and Outer are the same path (as opposed to
// Inner lying inside Outer), compared the way overlaps are detected.
func (o Overlap) SamePath() bool {
	return PathKey(o.Inner.Path) == PathKey(o.Outer.Path)
}

// PathKey returns the key under which paths are compared for overlap and
// de-duplication: the cleaned path, lowercased. Lowercasing matches the
// default case-insensitive APFS volume, where "Application Support/Discord"
// and "Application Support/discord" are the same directory. On a
// case-sensitive volume it can only merge two size estimates into one; it
// never changes which paths cleanup deletes.
func PathKey(p string) string {
	return strings.ToLower(filepath.Clean(p))
}

// collectCountedPaths returns every absolute filesystem path counted in the
// results. Entries without a breakdown count their own path with their full
// size; entries with a breakdown count each component. Pseudo-paths such as
// "docker:Images", "git-gc:<repo>" and "tmutil:snapshot:<name>" are skipped
// (along with their components): their sizes are estimates, not the size of
// a filesystem path, so they cannot be compared with other entries.
func collectCountedPaths(results []CategoryResult) []CountedPath {
	var out []CountedPath
	for _, cat := range results {
		for _, e := range cat.Entries {
			if !filepath.IsAbs(e.Path) {
				continue
			}
			if len(e.Components) == 0 {
				out = append(out, CountedPath{filepath.Clean(e.Path), e.Size, true, cat.Description})
				continue
			}
			for _, c := range e.Components {
				if filepath.IsAbs(c.Path) {
					out = append(out, CountedPath{filepath.Clean(c.Path), c.Size, c.Removed, cat.Description})
				}
			}
		}
	}
	return out
}

// findCover returns the index of a counted path that covers paths[i] among
// those accepted by ok: an earlier occurrence of the same path, otherwise
// the nearest ancestor directory. It returns -1 when nothing covers it.
// byPath is keyed by PathKey.
func findCover(paths []CountedPath, byPath map[string][]int, i int, ok func(int) bool) int {
	key := PathKey(paths[i].Path)
	for _, j := range byPath[key] {
		if j >= i {
			break
		}
		if ok(j) {
			return j
		}
	}
	for p := key; ; {
		parent := filepath.Dir(p)
		if parent == p {
			return -1
		}
		p = parent
		for _, j := range byPath[p] {
			if ok(j) {
				return j
			}
		}
	}
}

// FindOverlaps reports every counted path covered by another counted path,
// plus the bytes counted more than once overall (dupCounted) and among paths
// cleanup removes (dupRemoved). Each covered path's bytes are already
// included in the path covering it, so subtracting them yields the total
// with every byte counted once. Paths are compared with PathKey.
func FindOverlaps(results []CategoryResult) (overlaps []Overlap, dupCounted, dupRemoved int64) {
	paths := collectCountedPaths(results)
	byPath := make(map[string][]int, len(paths))
	for i, p := range paths {
		k := PathKey(p.Path)
		byPath[k] = append(byPath[k], i)
	}
	all := func(int) bool { return true }
	removed := func(j int) bool { return paths[j].Removed }

	for i, p := range paths {
		if j := findCover(paths, byPath, i, all); j >= 0 {
			overlaps = append(overlaps, Overlap{Inner: p, Outer: paths[j]})
			dupCounted += p.Size
		}
		if p.Removed && findCover(paths, byPath, i, removed) >= 0 {
			dupRemoved += p.Size
		}
	}
	sort.SliceStable(overlaps, func(a, b int) bool { return overlaps[a].Inner.Size > overlaps[b].Inner.Size })
	return overlaps, dupCounted, dupRemoved
}

// Reclaimable returns the number of bytes cleanup deletes for the results:
// the removed size of every entry with a real filesystem path, with each
// path counted once. Pseudo-path entries (docker:, tmutil:, git-gc:) and
// components cleanup leaves in place contribute nothing.
func Reclaimable(results []CategoryResult) int64 {
	var removed int64
	for _, cat := range results {
		for _, e := range cat.Entries {
			removed += removedPathSize(e)
		}
	}
	_, _, dupRemoved := FindOverlaps(results)
	return removed - dupRemoved
}

// removedPathSize returns the bytes of e that cleanup deletes from real
// filesystem paths. It matches the paths FindOverlaps considers, so that
// subtracting dupRemoved never goes below zero.
func removedPathSize(e ScanEntry) int64 {
	if !filepath.IsAbs(e.Path) {
		return 0
	}
	if len(e.Components) == 0 {
		return e.Size
	}
	var n int64
	for _, c := range e.Components {
		if c.Removed && filepath.IsAbs(c.Path) {
			n += c.Size
		}
	}
	return n
}
