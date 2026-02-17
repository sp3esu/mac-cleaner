// Package systemdata provides scanners for macOS "System Data" contributors
// including Spotlight metadata, Mail, Messages, iOS software updates,
// Time Machine local snapshots, and virtual machine disk images.
package systemdata

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sp3esu/mac-cleaner/internal/safety"
	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// CmdRunner executes an external command and returns its combined stdout output.
// It is used for dependency injection so tmutil calls can be mocked in tests.
type CmdRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// defaultRunner is the production CmdRunner that uses os/exec.
func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- all command names and arguments are hardcoded string literals, no user input
	return cmd.Output()
}

// Scan discovers and sizes System Data contributors including Spotlight metadata,
// Mail data, Messages attachments, iOS software updates, Time Machine snapshots,
// and virtual machine disk images. Missing directories are silently skipped.
// No files are modified.
func Scan() ([]scan.CategoryResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	var results []scan.CategoryResult

	if cr := scanSpotlight(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanMail(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanMailDownloads(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanMessages(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanIOSUpdates(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanTimeMachine(defaultRunner); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanVMParallels(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanVMUTM(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}
	if cr := scanVMVMware(home); cr != nil {
		cr.SetRiskLevels(safety.RiskForCategory)
		results = append(results, *cr)
	}

	return results, nil
}

// scanSpotlight scans ~/Library/Metadata/CoreSpotlight/.
// Returns nil if the directory does not exist.
func scanSpotlight(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Metadata", "CoreSpotlight")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "sysdata-spotlight",
				Description: "CoreSpotlight Metadata",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "CoreSpotlight Metadata (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(dir, "sysdata-spotlight", "CoreSpotlight Metadata")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanMail scans ~/Library/Mail/.
// Returns nil if the directory does not exist.
func scanMail(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Mail")
	return scanSingleDir(dir, "sysdata-mail", "Mail Database")
}

// scanMailDownloads scans ~/Library/Containers/com.apple.mail/Data/Library/Mail Downloads/.
// Returns nil if the directory does not exist.
func scanMailDownloads(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads")
	return scanSingleDir(dir, "sysdata-mail-downloads", "Mail Attachment Cache")
}

// scanMessages scans ~/Library/Messages/Attachments/.
// Returns nil if the directory does not exist.
func scanMessages(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Messages", "Attachments")
	return scanSingleDir(dir, "sysdata-messages", "Messages Attachments")
}

// scanIOSUpdates scans iOS/iPad software update directories:
//   - ~/Library/iTunes/iPhone Software Updates/
//   - ~/Library/iTunes/iPad Software Updates/
//
// Returns nil if neither directory exists.
func scanIOSUpdates(home string) *scan.CategoryResult {
	paths := []string{
		filepath.Join(home, "Library", "iTunes", "iPhone Software Updates"),
		filepath.Join(home, "Library", "iTunes", "iPad Software Updates"),
	}
	return scanMultiDir(paths, "sysdata-ios-updates", "iOS Software Updates")
}

// scanTimeMachine queries tmutil for local APFS snapshots.
// Snapshots use pseudo-paths (tmutil:snapshot:<name>) since they are not
// regular filesystem entries. Size is reported as 0 because per-snapshot
// size is unavailable without root privileges.
// Returns nil if tmutil is not installed or no snapshots exist.
func scanTimeMachine(runner CmdRunner) *scan.CategoryResult {
	if _, err := exec.LookPath("tmutil"); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := runner(ctx, "tmutil", "listlocalsnapshots", "/")
	if err != nil {
		return nil
	}

	snapshots := parseTmutilSnapshots(string(out))
	if len(snapshots) == 0 {
		return nil
	}

	var entries []scan.ScanEntry
	for _, name := range snapshots {
		entries = append(entries, scan.ScanEntry{
			Path:        "tmutil:snapshot:" + name,
			Description: name,
			Size:        0,
		})
	}

	return &scan.CategoryResult{
		Category:    "sysdata-timemachine",
		Description: fmt.Sprintf("Time Machine Local Snapshots (%d snapshots)", len(snapshots)),
		Entries:     entries,
		TotalSize:   0,
	}
}

// parseTmutilSnapshots extracts snapshot names from tmutil listlocalsnapshots output.
// Each relevant line contains "com.apple.TimeMachine" — the snapshot name is
// extracted after the last ":" or used as-is if there is no colon.
func parseTmutilSnapshots(output string) []string {
	var snapshots []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "com.apple.TimeMachine") {
			continue
		}
		// Lines may be "com.apple.TimeMachine.2024-01-15-123456.local"
		// or prefixed like "Snapshot: com.apple.TimeMachine.2024-01-15-123456.local"
		if idx := strings.LastIndex(line, ": "); idx != -1 {
			line = line[idx+2:]
		}
		snapshots = append(snapshots, line)
	}
	return snapshots
}

// scanVMParallels scans ~/Parallels/.
// Returns nil if the directory does not exist.
func scanVMParallels(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Parallels")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "sysdata-vm-parallels",
				Description: "Parallels VMs",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "Parallels VMs (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(dir, "sysdata-vm-parallels", "Parallels VMs")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanVMUTM scans ~/Library/Containers/com.utmapp.UTM/Data/Documents/.
// Returns nil if the directory does not exist.
func scanVMUTM(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Library", "Containers", "com.utmapp.UTM", "Data", "Documents")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "sysdata-vm-utm",
				Description: "UTM VMs",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "UTM VMs (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(dir, "sysdata-vm-utm", "UTM VMs")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanVMVMware scans ~/Virtual Machines.localized/.
// Returns nil if the directory does not exist.
func scanVMVMware(home string) *scan.CategoryResult {
	dir := filepath.Join(home, "Virtual Machines.localized")

	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    "sysdata-vm-vmware",
				Description: "VMware Fusion VMs",
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: "VMware Fusion VMs (permission denied)",
				}},
			}
		}
		return nil
	}

	cr, err := scan.ScanTopLevel(dir, "sysdata-vm-vmware", "VMware Fusion VMs")
	if err != nil {
		return nil
	}

	if len(cr.Entries) == 0 && len(cr.PermissionIssues) == 0 {
		return nil
	}

	return cr
}

// scanSingleDir scans a single directory and returns it as a blob entry.
// Returns nil if the directory does not exist or is empty.
func scanSingleDir(dir, category, description string) *scan.CategoryResult {
	if _, err := os.Stat(dir); err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    category,
				Description: description,
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: description + " (permission denied)",
				}},
			}
		}
		return nil
	}

	size, err := scan.DirSize(dir)
	if err != nil {
		if os.IsPermission(err) {
			return &scan.CategoryResult{
				Category:    category,
				Description: description,
				PermissionIssues: []scan.PermissionIssue{{
					Path:        dir,
					Description: description + " (permission denied)",
				}},
			}
		}
		return nil
	}

	if size == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:    category,
		Description: description,
		Entries: []scan.ScanEntry{
			{
				Path:        dir,
				Description: description,
				Size:        size,
			},
		},
		TotalSize: size,
	}
}

// scanMultiDir scans multiple directories and combines them into a single
// CategoryResult. Each existing directory becomes a single blob entry with
// its total size. Returns nil if no directories exist or all are empty.
func scanMultiDir(paths []string, category, description string) *scan.CategoryResult {
	var entries []scan.ScanEntry
	var permIssues []scan.PermissionIssue
	var totalSize int64

	for _, dir := range paths {
		if _, err := os.Stat(dir); err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        dir,
					Description: description + " (permission denied)",
				})
			}
			continue
		}

		size, err := scan.DirSize(dir)
		if err != nil {
			if os.IsPermission(err) {
				permIssues = append(permIssues, scan.PermissionIssue{
					Path:        dir,
					Description: description + " (permission denied)",
				})
			}
			continue
		}

		if size == 0 {
			continue
		}

		entries = append(entries, scan.ScanEntry{
			Path:        dir,
			Description: filepath.Base(dir),
			Size:        size,
		})
		totalSize += size
	}

	if len(entries) == 0 && len(permIssues) == 0 {
		return nil
	}

	return &scan.CategoryResult{
		Category:         category,
		Description:      description,
		Entries:          entries,
		TotalSize:        totalSize,
		PermissionIssues: permIssues,
	}
}
