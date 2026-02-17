package cmd

import (
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/safety"
)

// --- scanGroups registry tests ---

func TestScanGroups_AllGroupsPresent(t *testing.T) {
	expectedGroups := []struct {
		flagName  string
		scannerID string
	}{
		{"system-caches", "system"},
		{"browser-data", "browser"},
		{"dev-caches", "developer"},
		{"app-leftovers", "appleftovers"},
		{"creative-caches", "creative"},
		{"messaging-caches", "messaging"},
		{"unused-apps", "unused"},
		{"photos", "photos"},
		{"system-data", "systemdata"},
	}

	if len(scanGroups) != len(expectedGroups) {
		t.Fatalf("expected %d groups, got %d", len(expectedGroups), len(scanGroups))
	}

	for i, exp := range expectedGroups {
		g := scanGroups[i]
		if g.FlagName != exp.flagName {
			t.Errorf("group %d: expected FlagName %q, got %q", i, exp.flagName, g.FlagName)
		}
		if g.ScannerID != exp.scannerID {
			t.Errorf("group %d: expected ScannerID %q, got %q", i, exp.scannerID, g.ScannerID)
		}
	}
}

func TestScanGroups_AllGroupsHaveRequiredFields(t *testing.T) {
	for _, g := range scanGroups {
		if g.FlagName == "" {
			t.Errorf("group %q has empty FlagName", g.ScannerID)
		}
		if g.ScannerID == "" {
			t.Errorf("group %q has empty ScannerID", g.FlagName)
		}
		if g.GroupName == "" {
			t.Errorf("group %q has empty GroupName", g.ScannerID)
		}
		if g.Description == "" {
			t.Errorf("group %q has empty Description", g.ScannerID)
		}
		if g.ScanFlag == nil {
			t.Errorf("group %q has nil ScanFlag", g.ScannerID)
		}
		if g.SkipFlag == nil {
			t.Errorf("group %q has nil SkipFlag", g.ScannerID)
		}
		if len(g.Items) == 0 {
			t.Errorf("group %q has no items", g.ScannerID)
		}
	}
}

func TestScanGroups_AllItemsHaveCategoryID(t *testing.T) {
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.CategoryID == "" {
				t.Errorf("group %q has item with empty CategoryID", g.ScannerID)
			}
			if item.Description == "" {
				t.Errorf("group %q item %q has empty Description", g.ScannerID, item.CategoryID)
			}
		}
	}
}

func TestScanGroups_TargetedFlagsCount(t *testing.T) {
	count := 0
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.FlagName != "" && item.ScanFlag != nil {
				count++
			}
		}
	}
	if count != 41 {
		t.Errorf("expected 41 targeted scan flags, got %d", count)
	}
}

func TestScanGroups_SkipFlagsCount(t *testing.T) {
	count := 0
	seen := map[*bool]bool{}
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.SkipFlag != nil && !seen[item.SkipFlag] {
				seen[item.SkipFlag] = true
				count++
			}
		}
	}
	// 41 item-level skip flags + 1 dual-purpose (unused-apps group skip == item skip)
	// = 42 unique skip mappings, but unused-apps shares the pointer with the group skip
	// so unique SkipFlag pointers across items = 42
	if count != 42 {
		t.Errorf("expected 42 unique skip flag pointers across items, got %d", count)
	}
}

func TestScanGroups_ItemsWithFlagNameHaveScanFlag(t *testing.T) {
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.FlagName != "" && item.ScanFlag == nil {
				t.Errorf("group %q item %q has FlagName but nil ScanFlag", g.ScannerID, item.CategoryID)
			}
		}
	}
}

func TestScanGroups_NoDuplicateFlagNames(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range scanGroups {
		if seen[g.FlagName] {
			t.Errorf("duplicate group flag name: %q", g.FlagName)
		}
		seen[g.FlagName] = true
	}
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.FlagName == "" {
				continue
			}
			if seen[item.FlagName] {
				t.Errorf("duplicate item flag name: %q (group %q)", item.FlagName, g.ScannerID)
			}
			seen[item.FlagName] = true
		}
	}
}

func TestScanGroups_NoDuplicateCategoryIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if seen[item.CategoryID] {
				t.Errorf("duplicate category ID: %q", item.CategoryID)
			}
			seen[item.CategoryID] = true
		}
	}
}

func TestScanGroups_AllCategoryIDsHaveRisk(t *testing.T) {
	for _, g := range scanGroups {
		for _, item := range g.Items {
			risk := safety.RiskForCategory(item.CategoryID)
			if risk == "" {
				t.Errorf("item %q has empty risk level", item.CategoryID)
			}
		}
	}
}

// --- groupForCategory tests ---

func TestGroupForCategory_Found(t *testing.T) {
	tests := []struct {
		categoryID  string
		wantGroupID string
	}{
		{"dev-npm", "developer"},
		{"browser-safari", "browser"},
		{"quicklook", "system"},
		{"system-caches", "system"},
		{"unused-apps", "unused"},
		{"photos-caches", "photos"},
		{"sysdata-spotlight", "systemdata"},
		{"msg-slack", "messaging"},
		{"creative-adobe", "creative"},
		{"app-orphaned-prefs", "appleftovers"},
	}
	for _, tt := range tests {
		g := groupForCategory(tt.categoryID)
		if g == nil {
			t.Errorf("groupForCategory(%q) returned nil, want group %q", tt.categoryID, tt.wantGroupID)
			continue
		}
		if g.ScannerID != tt.wantGroupID {
			t.Errorf("groupForCategory(%q).ScannerID = %q, want %q", tt.categoryID, g.ScannerID, tt.wantGroupID)
		}
	}
}

func TestGroupForCategory_NotFound(t *testing.T) {
	if g := groupForCategory("unknown-thing"); g != nil {
		t.Errorf("expected nil for unknown category, got group %q", g.ScannerID)
	}
	if g := groupForCategory(""); g != nil {
		t.Errorf("expected nil for empty category, got group %q", g.ScannerID)
	}
}

// --- flagForCategory additional test cases ---

func TestFlagForCategory_PhotosAndSystemData(t *testing.T) {
	tests := []struct {
		categoryID string
		want       string
	}{
		{"unused-apps", "--unused-apps"},
		{"photos-caches", "--photos"},
		{"photos-analysis", "--photos"},
		{"photos-icloud-cache", "--photos"},
		{"photos-syndication", "--photos"},
		{"sysdata-spotlight", "--system-data"},
		{"sysdata-mail", "--system-data"},
		{"sysdata-mail-downloads", "--system-data"},
		{"sysdata-messages", "--system-data"},
		{"sysdata-ios-updates", "--system-data"},
		{"sysdata-timemachine", "--system-data"},
		{"sysdata-vm-parallels", "--system-data"},
		{"sysdata-vm-utm", "--system-data"},
		{"sysdata-vm-vmware", "--system-data"},
	}
	for _, tt := range tests {
		t.Run(tt.categoryID, func(t *testing.T) {
			got := flagForCategory(tt.categoryID)
			if got != tt.want {
				t.Errorf("flagForCategory(%q) = %q, want %q", tt.categoryID, got, tt.want)
			}
		})
	}
}

// --- buildSkipSet regression tests ---

func TestBuildSkipSet_NoFlagsSet(t *testing.T) {
	// Reset all skip flags.
	resetSkipFlags()
	defer resetSkipFlags()

	skip := buildSkipSet()
	if len(skip) != 0 {
		t.Errorf("expected empty skip set, got %d entries", len(skip))
	}
}

func TestBuildSkipSet_SingleItemSkip(t *testing.T) {
	resetSkipFlags()
	defer resetSkipFlags()

	flagSkipNpm = true
	skip := buildSkipSet()
	if !skip["dev-npm"] {
		t.Error("expected dev-npm in skip set")
	}
	if len(skip) != 1 {
		t.Errorf("expected 1 skip entry, got %d", len(skip))
	}
}

func TestBuildSkipSet_MultipleItemSkips(t *testing.T) {
	resetSkipFlags()
	defer resetSkipFlags()

	flagSkipSafari = true
	flagSkipDocker = true
	flagSkipSlack = true
	skip := buildSkipSet()
	expected := map[string]bool{"browser-safari": true, "dev-docker": true, "msg-slack": true}
	for id := range expected {
		if !skip[id] {
			t.Errorf("expected %q in skip set", id)
		}
	}
	if len(skip) != 3 {
		t.Errorf("expected 3 skip entries, got %d", len(skip))
	}
}

func TestBuildSkipSet_UnusedApps(t *testing.T) {
	resetSkipFlags()
	defer resetSkipFlags()

	flagSkipUnusedApps = true
	skip := buildSkipSet()
	if !skip["unused-apps"] {
		t.Error("expected unused-apps in skip set")
	}
}

// --- scan command help (no flags) ---

func TestScanCmd_NoFlags_ShowsHelp(t *testing.T) {
	// The scan command should show help when no flags are provided.
	// We verify the usage function is set.
	if scanCmd.UsageFunc() == nil {
		t.Error("expected custom usage function on scan command")
	}
}

func TestScanCmd_HasExpectedFlags(t *testing.T) {
	// Verify key flags are registered on the scan command.
	expectedFlags := []string{
		"all", "json", "verbose", "force",
		"system-caches", "browser-data", "dev-caches",
		"npm", "safari", "docker", "homebrew",
		"skip-npm", "skip-safari", "skip-dev-caches",
	}
	for _, name := range expectedFlags {
		if scanCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q on scan command", name)
		}
	}
}

func TestScanCmd_InheritsRootPersistentFlags(t *testing.T) {
	// --dry-run is a persistent flag on root, should be available on scan.
	f := scanCmd.Flags().Lookup("dry-run")
	if f == nil {
		// Check inherited flags.
		f = scanCmd.InheritedFlags().Lookup("dry-run")
	}
	if f == nil {
		t.Error("expected --dry-run available on scan command (inherited from root)")
	}
}

// resetSkipFlags sets all item-level skip flags to false.
func resetSkipFlags() {
	for _, g := range scanGroups {
		for _, item := range g.Items {
			if item.SkipFlag != nil {
				*item.SkipFlag = false
			}
		}
	}
}
