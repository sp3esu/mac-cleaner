// Package developer provides scanners for macOS developer tool cache directories.
package developer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// CmdRunner executes an external command and returns its combined stdout output.
// It is used for dependency injection so Docker CLI calls can be mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// Scan discovers and sizes developer cache directories for Xcode DerivedData,
// npm cache, yarn cache, Homebrew cache, and Docker artifacts. Missing tools
// are silently skipped. No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanXcodeDerivedData(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanNpmCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanYarnCache(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanHomebrew(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanDocker(defaultRunner); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanXcodeDerivedData scans ~/Library/Developer/Xcode/DerivedData/.
// Returns nil if the directory does not exist.
func scanXcodeDerivedData(home string) *scan.CategoryResult {
	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")

	if _, err := os.Stat(derivedData); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "dev-xcode",
				Description: "Xcode DerivedData",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        derivedData,
					Description: "Xcode DerivedData (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(derivedData, "dev-xcode", "Xcode DerivedData")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanNpmCache scans ~/.npm/ (the npm cache directory).
// Returns nil if the directory does not exist.
func scanNpmCache(home string) *scan.CategoryResult {
	npmDir := filepath.Join(home, ".npm")

	if _, err := os.Stat(npmDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "dev-npm",
				Description: "npm Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        npmDir,
					Description: "npm cache (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(npmDir, "dev-npm", "npm Cache")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanYarnCache scans ~/Library/Caches/yarn/.
// Returns nil if the directory does not exist. Uses DirSize since
// yarn cache is treated as a single blob rather than individual entries.
func scanYarnCache(home string) *scan.CategoryResult {
	yarnDir := filepath.Join(home, "Library", "Caches", "yarn")

	if _, err := os.Stat(yarnDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "dev-yarn",
				Description: "Yarn Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        yarnDir,
					Description: "Yarn cache (permission denied)",
				}},
			}
		}
		return nil
	}

	size, err := scan.DirSize(yarnDir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "dev-yarn",
				Description: "Yarn Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        yarnDir,
					Description: "Yarn cache (permission denied)",
				}},
			}
		}
		return nil
	}

	if size == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:    "dev-yarn",
		Description: "Yarn Cache",
		Entries: []scan.ScanEntry{
			{
				Path:        yarnDir,
				Description: "yarn",
				Size:        size,
			},
		},
		TotalSize: size,
	}
}

// scanHomebrew scans ~/Library/Caches/Homebrew/.
// Returns nil if the directory does not exist.
func scanHomebrew(home string) *scan.CategoryResult {
	brewDir := filepath.Join(home, "Library", "Caches", "Homebrew")

	if _, err := os.Stat(brewDir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "dev-homebrew",
				Description: "Homebrew Cache",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        brewDir,
					Description: "Homebrew cache (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(brewDir, "dev-homebrew", "Homebrew Cache")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// dockerDFRow represents one row from docker system df --format '{{json .}}'.
type dockerDFRow struct {
	Type        string `json:"Type"`
	Reclaimable string `json:"Reclaimable"`
}

// scanDocker queries Docker for reclaimable space using docker system df.
// Returns nil if Docker is not installed or not running. Uses a 10-second
// timeout to prevent hangs when the Docker daemon is unresponsive.
func scanDocker(runner CmdRunner) *scan.CategoryResult {
	// Check if docker binary is available.
	if _, err := exec.LookPath("docker"); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runner(ctx, "docker", "system", "df", "--format", "{{json .}}")
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var entries []scan.ScanEntry
	var totalSize int64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var row dockerDFRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}

		size := parseDockerSize(row.Reclaimable)
		if size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        "docker:" + row.Type,
			Description: "Docker " + row.Type,
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &scan.CategoryResult{
		Category:    "dev-docker",
		Description: "Docker Reclaimable",
		Entries:     entries,
		TotalSize:   totalSize,
	}
}

// parseDockerSize parses Docker's human-readable size strings like "16.43MB",
// "2.3GB", "1.5kB", "0B". The Reclaimable field may include a percentage
// suffix like "1.2GB (52%)" which is stripped before parsing.
func parseDockerSize(s string) int64 {
	// Strip percentage suffix: "1.2GB (52%)" -> "1.2GB"
	if idx := strings.Index(s, " ("); idx != -1 {
		s = s[:idx]
	}

	s = strings.TrimSpace(s)
	if s == "" || s == "0B" {
		return 0
	}

	// Check longer suffixes first to avoid "B" matching "GB", "MB", etc.
	type unitEntry struct {
		suffix     string
		multiplier float64
	}
	units := []unitEntry{
		{"TB", 1000 * 1000 * 1000 * 1000},
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"kB", 1000},
		{"KB", 1000},
		{"B", 1},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numStr := strings.TrimSuffix(s, u.suffix)
			numStr = strings.TrimSpace(numStr)
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0
			}
			return int64(val * u.multiplier)
		}
	}

	return 0
}
