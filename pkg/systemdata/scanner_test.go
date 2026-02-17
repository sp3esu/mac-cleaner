package systemdata

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/scan"
)

// writeFile is a test helper that creates a file with the given size,
// creating parent directories as needed.
func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// --- Spotlight tests ---

func TestScanSpotlightMissing(t *testing.T) {
	home := t.TempDir()
	result := scanSpotlight(home)
	if result != nil {
		t.Fatal("expected nil for missing Spotlight metadata")
	}
}

func TestScanSpotlightEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Metadata", "CoreSpotlight")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanSpotlight(home)
	if result != nil {
		t.Fatal("expected nil for empty Spotlight directory")
	}
}

func TestScanSpotlightWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Metadata", "CoreSpotlight")
	writeFile(t, filepath.Join(dir, "index-1", "store.db"), 5000)
	writeFile(t, filepath.Join(dir, "index-2", "store.db"), 3000)

	result := scanSpotlight(home)
	if result == nil {
		t.Fatal("expected non-nil result for Spotlight with data")
	}
	if result.Category != "sysdata-spotlight" {
		t.Errorf("expected category 'sysdata-spotlight', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 8000 {
		t.Errorf("expected total size 8000, got %d", result.TotalSize)
	}
}

func TestScanSpotlightPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Metadata", "CoreSpotlight")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Metadata")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanSpotlight(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Mail tests ---

func TestScanMailMissing(t *testing.T) {
	home := t.TempDir()
	result := scanMail(home)
	if result != nil {
		t.Fatal("expected nil for missing Mail directory")
	}
}

func TestScanMailEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Mail")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanMail(home)
	if result != nil {
		t.Fatal("expected nil for empty Mail directory")
	}
}

func TestScanMailWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Mail")
	writeFile(t, filepath.Join(dir, "V10", "Mailboxes", "INBOX.mbox", "messages.db"), 10000)

	result := scanMail(home)
	if result == nil {
		t.Fatal("expected non-nil result for Mail with data")
	}
	if result.Category != "sysdata-mail" {
		t.Errorf("expected category 'sysdata-mail', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 10000 {
		t.Errorf("expected total size 10000, got %d", result.TotalSize)
	}
}

func TestScanMailPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Mail")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanMail(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Mail Downloads tests ---

func TestScanMailDownloadsMissing(t *testing.T) {
	home := t.TempDir()
	result := scanMailDownloads(home)
	if result != nil {
		t.Fatal("expected nil for missing Mail Downloads")
	}
}

func TestScanMailDownloadsEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanMailDownloads(home)
	if result != nil {
		t.Fatal("expected nil for empty Mail Downloads directory")
	}
}

func TestScanMailDownloadsWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads")
	writeFile(t, filepath.Join(dir, "attachment.pdf"), 7000)

	result := scanMailDownloads(home)
	if result == nil {
		t.Fatal("expected non-nil result for Mail Downloads with data")
	}
	if result.Category != "sysdata-mail-downloads" {
		t.Errorf("expected category 'sysdata-mail-downloads', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 7000 {
		t.Errorf("expected total size 7000, got %d", result.TotalSize)
	}
}

func TestScanMailDownloadsPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.apple.mail", "Data", "Library", "Mail Downloads")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Containers", "com.apple.mail", "Data", "Library")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanMailDownloads(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Messages tests ---

func TestScanMessagesMissing(t *testing.T) {
	home := t.TempDir()
	result := scanMessages(home)
	if result != nil {
		t.Fatal("expected nil for missing Messages Attachments")
	}
}

func TestScanMessagesEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Messages", "Attachments")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanMessages(home)
	if result != nil {
		t.Fatal("expected nil for empty Messages Attachments directory")
	}
}

func TestScanMessagesWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Messages", "Attachments")
	writeFile(t, filepath.Join(dir, "ab", "photo.heic"), 4000)
	writeFile(t, filepath.Join(dir, "cd", "video.mov"), 6000)

	result := scanMessages(home)
	if result == nil {
		t.Fatal("expected non-nil result for Messages with data")
	}
	if result.Category != "sysdata-messages" {
		t.Errorf("expected category 'sysdata-messages', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry (single blob), got %d", len(result.Entries))
	}
	if result.TotalSize != 10000 {
		t.Errorf("expected total size 10000, got %d", result.TotalSize)
	}
}

func TestScanMessagesPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Messages", "Attachments")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Messages")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanMessages(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- iOS Updates tests ---

func TestScanIOSUpdatesMissing(t *testing.T) {
	home := t.TempDir()
	result := scanIOSUpdates(home)
	if result != nil {
		t.Fatal("expected nil for missing iOS update directories")
	}
}

func TestScanIOSUpdatesEmpty(t *testing.T) {
	home := t.TempDir()
	dir1 := filepath.Join(home, "Library", "iTunes", "iPhone Software Updates")
	dir2 := filepath.Join(home, "Library", "iTunes", "iPad Software Updates")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanIOSUpdates(home)
	if result != nil {
		t.Fatal("expected nil for empty iOS update directories")
	}
}

func TestScanIOSUpdatesWithData(t *testing.T) {
	home := t.TempDir()
	dir1 := filepath.Join(home, "Library", "iTunes", "iPhone Software Updates")
	dir2 := filepath.Join(home, "Library", "iTunes", "iPad Software Updates")
	writeFile(t, filepath.Join(dir1, "iOS17.ipsw"), 8000)
	writeFile(t, filepath.Join(dir2, "iPadOS17.ipsw"), 4000)

	result := scanIOSUpdates(home)
	if result == nil {
		t.Fatal("expected non-nil result for iOS updates with data")
	}
	if result.Category != "sysdata-ios-updates" {
		t.Errorf("expected category 'sysdata-ios-updates', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 12000 {
		t.Errorf("expected total size 12000, got %d", result.TotalSize)
	}
}

func TestScanIOSUpdatesPartial(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "iTunes", "iPhone Software Updates")
	writeFile(t, filepath.Join(dir, "iOS17.ipsw"), 6000)

	result := scanIOSUpdates(home)
	if result == nil {
		t.Fatal("expected non-nil result for partial iOS updates")
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.TotalSize != 6000 {
		t.Errorf("expected total size 6000, got %d", result.TotalSize)
	}
}

func TestScanIOSUpdatesPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "iTunes", "iPhone Software Updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "iTunes")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanIOSUpdates(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Time Machine tests ---

func TestScanTimeMachineNotInstalled(t *testing.T) {
	// Use a runner that returns an error (simulating tmutil not found).
	// The actual LookPath check happens first, so we test via runner error path.
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("exit status 1")
	}
	result := scanTimeMachine(runner)
	if result != nil {
		t.Fatal("expected nil when tmutil returns error")
	}
}

func TestScanTimeMachineNoSnapshots(t *testing.T) {
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	result := scanTimeMachine(runner)
	if result != nil {
		t.Fatal("expected nil for no snapshots")
	}
}

func TestScanTimeMachineWithSnapshots(t *testing.T) {
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("com.apple.TimeMachine.2024-01-15-120000.local\ncom.apple.TimeMachine.2024-01-16-120000.local\n"), nil
	}
	result := scanTimeMachine(runner)
	if result == nil {
		t.Fatal("expected non-nil result for snapshots")
	}
	if result.Category != "sysdata-timemachine" {
		t.Errorf("expected category 'sysdata-timemachine', got %q", result.Category)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	if result.TotalSize != 0 {
		t.Errorf("expected total size 0, got %d", result.TotalSize)
	}
	// Verify pseudo-path format.
	for _, e := range result.Entries {
		if e.Path[:len("tmutil:snapshot:")] != "tmutil:snapshot:" {
			t.Errorf("expected pseudo-path prefix 'tmutil:snapshot:', got %q", e.Path)
		}
	}
}

func TestScanTimeMachineError(t *testing.T) {
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("tmutil: Operation not permitted")
	}
	result := scanTimeMachine(runner)
	if result != nil {
		t.Fatal("expected nil when tmutil returns error")
	}
}

// --- parseTmutilSnapshots tests ---

func TestParseTmutilSnapshots(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name:   "empty output",
			output: "",
			want:   0,
		},
		{
			name:   "no matching lines",
			output: "Snapshots for disk /:\n---\n",
			want:   0,
		},
		{
			name:   "single snapshot bare",
			output: "com.apple.TimeMachine.2024-01-15-120000.local\n",
			want:   1,
		},
		{
			name:   "multiple snapshots",
			output: "com.apple.TimeMachine.2024-01-15-120000.local\ncom.apple.TimeMachine.2024-01-16-120000.local\ncom.apple.TimeMachine.2024-01-17-120000.local\n",
			want:   3,
		},
		{
			name:   "prefixed format",
			output: "Snapshot: com.apple.TimeMachine.2024-01-15-120000.local\nSnapshot: com.apple.TimeMachine.2024-01-16-120000.local\n",
			want:   2,
		},
		{
			name:   "mixed with header lines",
			output: "Snapshots for disk /:\n---\ncom.apple.TimeMachine.2024-01-15-120000.local\n",
			want:   1,
		},
		{
			name:   "whitespace handling",
			output: "  com.apple.TimeMachine.2024-01-15-120000.local  \n\n  \n",
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTmutilSnapshots(tt.output)
			if len(got) != tt.want {
				t.Errorf("parseTmutilSnapshots() returned %d snapshots, want %d", len(got), tt.want)
			}
		})
	}
}

// --- Parallels VM tests ---

func TestScanVMParallelsMissing(t *testing.T) {
	home := t.TempDir()
	result := scanVMParallels(home)
	if result != nil {
		t.Fatal("expected nil for missing Parallels directory")
	}
}

func TestScanVMParallelsEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Parallels")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanVMParallels(home)
	if result != nil {
		t.Fatal("expected nil for empty Parallels directory")
	}
}

func TestScanVMParallelsWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Parallels")
	writeFile(t, filepath.Join(dir, "Windows 11.pvm", "disk.hdd"), 50000)

	result := scanVMParallels(home)
	if result == nil {
		t.Fatal("expected non-nil result for Parallels with data")
	}
	if result.Category != "sysdata-vm-parallels" {
		t.Errorf("expected category 'sysdata-vm-parallels', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.TotalSize != 50000 {
		t.Errorf("expected total size 50000, got %d", result.TotalSize)
	}
}

func TestScanVMParallelsPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Parallels")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	result := scanVMParallels(home)
	// ScanTopLevel should return permission issues.
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- UTM VM tests ---

func TestScanVMUTMMissing(t *testing.T) {
	home := t.TempDir()
	result := scanVMUTM(home)
	if result != nil {
		t.Fatal("expected nil for missing UTM directory")
	}
}

func TestScanVMUTMEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.utmapp.UTM", "Data", "Documents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanVMUTM(home)
	if result != nil {
		t.Fatal("expected nil for empty UTM directory")
	}
}

func TestScanVMUTMWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.utmapp.UTM", "Data", "Documents")
	writeFile(t, filepath.Join(dir, "Ubuntu.utm", "disk.qcow2"), 30000)

	result := scanVMUTM(home)
	if result == nil {
		t.Fatal("expected non-nil result for UTM with data")
	}
	if result.Category != "sysdata-vm-utm" {
		t.Errorf("expected category 'sysdata-vm-utm', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.TotalSize != 30000 {
		t.Errorf("expected total size 30000, got %d", result.TotalSize)
	}
}

func TestScanVMUTMPermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Containers", "com.utmapp.UTM", "Data", "Documents")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(home, "Library", "Containers", "com.utmapp.UTM", "Data")
	if err := os.Chmod(parent, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })

	result := scanVMUTM(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- VMware Fusion VM tests ---

func TestScanVMVMwareMissing(t *testing.T) {
	home := t.TempDir()
	result := scanVMVMware(home)
	if result != nil {
		t.Fatal("expected nil for missing VMware directory")
	}
}

func TestScanVMVMwareEmpty(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Virtual Machines.localized")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	result := scanVMVMware(home)
	if result != nil {
		t.Fatal("expected nil for empty VMware directory")
	}
}

func TestScanVMVMwareWithData(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Virtual Machines.localized")
	writeFile(t, filepath.Join(dir, "Windows.vmwarevm", "disk.vmdk"), 40000)

	result := scanVMVMware(home)
	if result == nil {
		t.Fatal("expected non-nil result for VMware with data")
	}
	if result.Category != "sysdata-vm-vmware" {
		t.Errorf("expected category 'sysdata-vm-vmware', got %q", result.Category)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Entries))
	}
	if result.TotalSize != 40000 {
		t.Errorf("expected total size 40000, got %d", result.TotalSize)
	}
}

func TestScanVMVMwarePermission(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Virtual Machines.localized")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	result := scanVMVMware(home)
	if result == nil {
		t.Fatal("expected non-nil result for permission denied")
	}
	if len(result.PermissionIssues) == 0 {
		t.Fatal("expected permission issues")
	}
}

// --- Integration test ---

func TestScanIntegration(t *testing.T) {
	home := t.TempDir()

	// Create Spotlight data.
	spotlightDir := filepath.Join(home, "Library", "Metadata", "CoreSpotlight")
	writeFile(t, filepath.Join(spotlightDir, "index-1", "store.db"), 1000)

	// Create Messages data.
	messagesDir := filepath.Join(home, "Library", "Messages", "Attachments")
	writeFile(t, filepath.Join(messagesDir, "photo.jpg"), 2000)

	// No Mail, no iOS updates, no VMs -- should be silently skipped.

	var results []scan.CategoryResult
	if cr := scanSpotlight(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanMail(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanMailDownloads(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanMessages(home); cr != nil {
		results = append(results, *cr)
	}
	if cr := scanIOSUpdates(home); cr != nil {
		results = append(results, *cr)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (Spotlight + Messages), got %d", len(results))
	}
	if results[0].Category != "sysdata-spotlight" {
		t.Errorf("expected first result 'sysdata-spotlight', got %q", results[0].Category)
	}
	if results[1].Category != "sysdata-messages" {
		t.Errorf("expected second result 'sysdata-messages', got %q", results[1].Category)
	}
}
