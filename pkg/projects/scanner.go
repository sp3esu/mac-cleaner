// Package projects scans for project build cache directories (node_modules,
// .build, target, venv/.venv, build) across the home directory. Each
// project type produces a separate CategoryResult with manifest validation
// to avoid false positives.
package projects

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
	"github.com/sp3esu/mac-cleaner/internal/walk"
)

// projectType defines a project cache directory pattern.
type projectType struct {
	CategoryID   string   // e.g. "proj-node"
	CacheDirName string   // e.g. "node_modules"
	Markers      []string // manifest file names to validate against
	MarkerInside bool     // true if marker lives inside cache dir (e.g. pyvenv.cfg)
	Label        string   // human-readable label for category description
}

// projectTypes defines all supported project cache types.
var projectTypes = []projectType{
	{
		CategoryID:   "proj-node",
		CacheDirName: "node_modules",
		Markers:      []string{"package.json"},
		MarkerInside: false,
		Label:        "node_modules",
	},
	{
		CategoryID:   "proj-swift",
		CacheDirName: ".build",
		Markers:      []string{"Package.swift"},
		MarkerInside: false,
		Label:        ".build (Swift)",
	},
	{
		CategoryID:   "proj-rust",
		CacheDirName: "target",
		Markers:      []string{"Cargo.toml"},
		MarkerInside: false,
		Label:        "target (Rust)",
	},
	{
		CategoryID:   "proj-python",
		CacheDirName: "venv",
		Markers:      []string{"pyvenv.cfg"},
		MarkerInside: true,
		Label:        "venv (Python)",
	},
	{
		CategoryID:   "proj-python",
		CacheDirName: ".venv",
		Markers:      []string{"pyvenv.cfg"},
		MarkerInside: true,
		Label:        "venv (Python)",
	},
	{
		CategoryID:   "proj-java",
		CacheDirName: "build",
		Markers:      []string{"build.gradle", "build.gradle.kts", "pom.xml"},
		MarkerInside: false,
		Label:        "build (Java/Kotlin)",
	},
}

// Scan discovers project build caches under the user's home directory.
// It returns one CategoryResult per project type that has at least one entry.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("projects: resolve home dir: %w", err)
	}
	return scanFrom(home)
}

// scanFrom is the internal implementation that accepts a custom root,
// enabling tests to use temporary directories.
func scanFrom(root string) ([]scan.CategoryResult, error) {
	// Build lookup: cache dir name -> list of project types.
	cacheLookup := make(map[string][]projectType)
	knownCacheDirs := make(map[string]bool)
	for _, pt := range projectTypes {
		cacheLookup[pt.CacheDirName] = append(cacheLookup[pt.CacheDirName], pt)
		knownCacheDirs[pt.CacheDirName] = true
	}

	// Collect entries grouped by category ID.
	type entryInfo struct {
		entry scan.ScanEntry
	}
	categoryEntries := make(map[string][]scan.ScanEntry)

	err := walk.Walk(walk.Options{
		Root:     root,
		MaxDepth: 5,
		Exclude:  []string{".git"},
		ExcludeAbs: []string{
			filepath.Join(root, "Library"),
			filepath.Join(root, "Applications"),
		},
		OnDir: func(path string, d fs.DirEntry, depth int) bool {
			name := d.Name()

			// Check if this is a known cache dir name.
			pts, ok := cacheLookup[name]
			if !ok {
				return true // continue descent
			}

			// It's a known cache dir name. Try to validate against each
			// matching project type.
			for _, pt := range pts {
				manifestPath, found := hasManifest(path, pt)
				if !found {
					continue
				}

				// Valid project cache found.
				size, sizeErr := scan.DirSize(path)
				if sizeErr != nil {
					continue
				}

				// Get manifest mtime for staleness check.
				info, statErr := os.Stat(manifestPath)
				if statErr != nil {
					continue
				}
				mtime := info.ModTime()

				// Parse project name.
				projName := projectNameFromManifest(manifestPath)
				if projName == "" {
					projName = filepath.Base(filepath.Dir(path))
				}

				// Build description with age annotation.
				desc := projName + "  " + formatAge(mtime)
				if isStale(mtime) {
					desc = projName + "  \u26a0 " + formatAge(mtime)
				}

				categoryEntries[pt.CategoryID] = append(categoryEntries[pt.CategoryID], scan.ScanEntry{
					Path:        path,
					Description: desc,
					Size:        size,
				})
				break // matched one type, no need to check others
			}

			// Don't descend into known cache dirs (even without manifest).
			return false
		},
	})
	if err != nil {
		return nil, fmt.Errorf("projects: walk: %w", err)
	}

	// Build CategoryResults for each type with entries.
	// Maintain stable order based on projectTypes definition.
	seen := make(map[string]bool)
	var results []scan.CategoryResult
	for _, pt := range projectTypes {
		entries, ok := categoryEntries[pt.CategoryID]
		if !ok || seen[pt.CategoryID] {
			continue
		}
		seen[pt.CategoryID] = true

		// Sort entries by size descending.
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Size > entries[j].Size
		})

		// Calculate total size.
		var totalSize int64
		for _, e := range entries {
			totalSize += e.Size
		}

		// Build aggregate description.
		desc := fmt.Sprintf("%s (%d dirs, %s)", pt.Label, len(entries), scan.FormatSize(totalSize))

		cr := scan.CategoryResult{
			Category:    pt.CategoryID,
			Description: desc,
			Entries:     entries,
			TotalSize:   totalSize,
		}
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, cr)
	}

	return results, nil
}

// hasManifest checks whether a cache directory has a valid manifest marker.
// For MarkerInside types, it checks inside the cache dir itself.
// For sibling markers, it checks the parent directory.
// Returns the manifest path and whether it was found.
func hasManifest(cacheDirPath string, pt projectType) (string, bool) {
	for _, marker := range pt.Markers {
		var markerPath string
		if pt.MarkerInside {
			markerPath = filepath.Join(cacheDirPath, marker)
		} else {
			markerPath = filepath.Join(filepath.Dir(cacheDirPath), marker)
		}
		if _, err := os.Stat(markerPath); err == nil {
			return markerPath, true
		}
	}
	return "", false
}

// projectNameFromManifest attempts to parse the project name from a manifest file.
// Supports package.json, Cargo.toml, and Package.swift.
// Returns empty string if parsing fails.
func projectNameFromManifest(manifestPath string) string {
	base := filepath.Base(manifestPath)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}

	switch base {
	case "package.json":
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pkg) == nil && pkg.Name != "" {
			return pkg.Name
		}
	case "Cargo.toml":
		// Simple line-by-line parse for name = "..."
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					val = strings.Trim(val, `"'`)
					if val != "" {
						return val
					}
				}
			}
		}
	case "Package.swift":
		re := regexp.MustCompile(`name:\s*"([^"]+)"`)
		if m := re.FindSubmatch(data); len(m) > 1 {
			return string(m[1])
		}
	}

	return ""
}

// formatAge formats a time as a human-readable age string.
func formatAge(mtime time.Time) string {
	d := time.Since(mtime)
	days := int(d.Hours() / 24)
	if days < 1 {
		return "<1d"
	}
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	years := months / 12
	return fmt.Sprintf("%dy", years)
}

// isStale returns true if the manifest was last modified more than 6 months ago.
func isStale(mtime time.Time) bool {
	return time.Since(mtime) > 180*24*time.Hour
}
