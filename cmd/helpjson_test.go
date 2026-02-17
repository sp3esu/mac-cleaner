package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/sp3esu/mac-cleaner/internal/safety"
)

func TestBuildHelpJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	printHelpJSON(&buf)

	var h helpJSON
	if err := json.Unmarshal(buf.Bytes(), &h); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
}

func TestBuildHelpJSON_HasVersion(t *testing.T) {
	h := buildHelpJSON()
	if h.Version == "" {
		t.Error("expected non-empty version")
	}
}

func TestBuildHelpJSON_HasAllCommands(t *testing.T) {
	h := buildHelpJSON()
	for _, name := range []string{"root", "scan", "serve"} {
		if _, ok := h.Commands[name]; !ok {
			t.Errorf("expected command %q in help JSON", name)
		}
	}
}

func TestBuildHelpJSON_HasAllScannerGroups(t *testing.T) {
	h := buildHelpJSON()
	if len(h.ScannerGroups) != len(scanGroups) {
		t.Fatalf("expected %d scanner groups, got %d", len(scanGroups), len(h.ScannerGroups))
	}
	for i, g := range scanGroups {
		if h.ScannerGroups[i].ID != g.ScannerID {
			t.Errorf("group %d: expected ID %q, got %q", i, g.ScannerID, h.ScannerGroups[i].ID)
		}
		if h.ScannerGroups[i].GroupFlag != "--"+g.FlagName {
			t.Errorf("group %d: expected GroupFlag --%s, got %s", i, g.FlagName, h.ScannerGroups[i].GroupFlag)
		}
		if h.ScannerGroups[i].SkipFlag != "--skip-"+g.FlagName {
			t.Errorf("group %d: expected SkipFlag --skip-%s, got %s", i, g.FlagName, h.ScannerGroups[i].SkipFlag)
		}
	}
}

func TestBuildHelpJSON_CategoriesMatchItems(t *testing.T) {
	h := buildHelpJSON()
	for i, g := range scanGroups {
		hg := h.ScannerGroups[i]
		if len(hg.Categories) != len(g.Items) {
			t.Errorf("group %q: expected %d categories, got %d", g.ScannerID, len(g.Items), len(hg.Categories))
			continue
		}
		for j, item := range g.Items {
			hc := hg.Categories[j]
			if hc.ID != item.CategoryID {
				t.Errorf("group %q category %d: expected ID %q, got %q", g.ScannerID, j, item.CategoryID, hc.ID)
			}
			if hc.Description != item.Description {
				t.Errorf("group %q category %q: expected Description %q, got %q", g.ScannerID, item.CategoryID, item.Description, hc.Description)
			}
		}
	}
}

func TestBuildHelpJSON_CategoriesHaveRiskLevels(t *testing.T) {
	h := buildHelpJSON()
	for _, hg := range h.ScannerGroups {
		for _, hc := range hg.Categories {
			if hc.RiskLevel == "" {
				t.Errorf("category %q has empty risk level", hc.ID)
			}
			expected := safety.RiskForCategory(hc.ID)
			if hc.RiskLevel != expected {
				t.Errorf("category %q: expected risk %q, got %q", hc.ID, expected, hc.RiskLevel)
			}
		}
	}
}

func TestBuildHelpJSON_ItemsWithFlagsHaveScanAndSkipFlags(t *testing.T) {
	h := buildHelpJSON()
	for _, hg := range h.ScannerGroups {
		for _, hc := range hg.Categories {
			// Find the corresponding item in scanGroups.
			g := groupForCategory(hc.ID)
			if g == nil {
				t.Errorf("category %q not found in scanGroups", hc.ID)
				continue
			}
			for _, item := range g.Items {
				if item.CategoryID == hc.ID {
					if item.FlagName != "" {
						if hc.ScanFlag != "--"+item.FlagName {
							t.Errorf("category %q: expected ScanFlag --%s, got %s", hc.ID, item.FlagName, hc.ScanFlag)
						}
						if hc.SkipFlag != "--skip-"+item.FlagName {
							t.Errorf("category %q: expected SkipFlag --skip-%s, got %s", hc.ID, item.FlagName, hc.SkipFlag)
						}
					} else {
						if hc.ScanFlag != "" {
							t.Errorf("category %q: expected empty ScanFlag for item without FlagName, got %q", hc.ID, hc.ScanFlag)
						}
						if hc.SkipFlag != "" {
							t.Errorf("category %q: expected empty SkipFlag for item without FlagName, got %q", hc.ID, hc.SkipFlag)
						}
					}
					break
				}
			}
		}
	}
}

func TestBuildHelpJSON_HasGlobalFlags(t *testing.T) {
	h := buildHelpJSON()
	if len(h.GlobalFlags) == 0 {
		t.Error("expected at least one global flag")
	}
	found := false
	for _, f := range h.GlobalFlags {
		if f.Flag == "--dry-run" {
			found = true
		}
	}
	if !found {
		t.Error("expected --dry-run in global flags")
	}
}

func TestBuildHelpJSON_HasOutputFlags(t *testing.T) {
	h := buildHelpJSON()
	expectedFlags := map[string]bool{"--json": false, "--verbose": false, "--force": false}
	for _, f := range h.OutputFlags {
		if _, ok := expectedFlags[f.Flag]; ok {
			expectedFlags[f.Flag] = true
		}
	}
	for flag, found := range expectedFlags {
		if !found {
			t.Errorf("expected %s in output flags", flag)
		}
	}
}

func TestBuildHelpJSON_HasExamples(t *testing.T) {
	h := buildHelpJSON()
	if len(h.Examples) == 0 {
		t.Error("expected at least one example")
	}
	for _, ex := range h.Examples {
		if ex.Command == "" {
			t.Error("example has empty command")
		}
		if ex.Description == "" {
			t.Error("example has empty description")
		}
	}
}

func TestPrintHelpJSON_OutputIsValidJSON(t *testing.T) {
	var buf bytes.Buffer
	printHelpJSON(&buf)

	if buf.Len() == 0 {
		t.Fatal("expected non-empty output")
	}

	// Verify it's valid JSON by re-parsing.
	var raw json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}
