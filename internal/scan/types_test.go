package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetRiskLevels_AppliesRiskToAllEntries(t *testing.T) {
	cr := CategoryResult{
		Category: "test-cat",
		Entries: []ScanEntry{
			{Path: "/a", Description: "a", Size: 100},
			{Path: "/b", Description: "b", Size: 200},
			{Path: "/c", Description: "c", Size: 300},
		},
	}

	called := false
	cr.SetRiskLevels(func(catID string) string {
		called = true
		if catID != "test-cat" {
			t.Errorf("expected category 'test-cat', got %q", catID)
		}
		return "safe"
	})

	if !called {
		t.Error("risk function was never called")
	}
	for i, e := range cr.Entries {
		if e.RiskLevel != "safe" {
			t.Errorf("entry %d: expected risk 'safe', got %q", i, e.RiskLevel)
		}
	}
}

func TestSetRiskLevels_EmptyEntries(t *testing.T) {
	cr := CategoryResult{Category: "empty"}
	cr.SetRiskLevels(func(string) string { return "risky" })
	if len(cr.Entries) != 0 {
		t.Errorf("expected no entries, got %d", len(cr.Entries))
	}
}

func TestSetRiskLevels_UsesCategory(t *testing.T) {
	cr := CategoryResult{
		Category: "dev-xcode",
		Entries: []ScanEntry{
			{Path: "/x", Description: "x", Size: 50},
		},
	}

	var receivedID string
	cr.SetRiskLevels(func(catID string) string {
		receivedID = catID
		return "risky"
	})

	if receivedID != "dev-xcode" {
		t.Errorf("expected riskFn called with 'dev-xcode', got %q", receivedID)
	}
	if cr.Entries[0].RiskLevel != "risky" {
		t.Errorf("expected risk 'risky', got %q", cr.Entries[0].RiskLevel)
	}
}

func TestScanEntryJSON_ComponentsOmittedWhenEmpty(t *testing.T) {
	b, err := json.Marshal(ScanEntry{Path: "/a", Description: "a", Size: 1, RiskLevel: "safe"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"path":"/a","description":"a","size":1,"risk_level":"safe"}`
	if string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}

func TestScanEntryJSON_ComponentsEncoded(t *testing.T) {
	e := ScanEntry{
		Path: "/a.app", Size: 3,
		Components: []SizeComponent{
			{Path: "/a.app", Size: 1, Removed: true},
			{Path: "/lib", Size: 2, Note: "counted"},
		},
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `"components":[{"path":"/a.app","size":1,"removed":true},{"path":"/lib","size":2,"removed":false,"note":"counted"}]`
	if !strings.Contains(string(b), want) {
		t.Errorf("got %s, want it to contain %s", b, want)
	}

	// Old payloads without components still decode.
	var old ScanEntry
	if err := json.Unmarshal([]byte(`{"path":"/x","description":"x","size":5,"risk_level":"safe"}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Components != nil || old.RemovedSize() != 5 {
		t.Errorf("unexpected decode %+v", old)
	}
}

func TestRemovedSize(t *testing.T) {
	tests := []struct {
		name  string
		entry ScanEntry
		want  int64
	}{
		{"no components", ScanEntry{Size: 10}, 10},
		{"all removed", ScanEntry{Size: 10, Components: []SizeComponent{{Size: 4, Removed: true}, {Size: 6, Removed: true}}}, 10},
		{"partially removed", ScanEntry{Size: 10, Components: []SizeComponent{{Size: 4, Removed: true}, {Size: 6}}}, 4},
		{"none removed", ScanEntry{Size: 10, Components: []SizeComponent{{Size: 10}}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.RemovedSize(); got != tt.want {
				t.Errorf("RemovedSize() = %d, want %d", got, tt.want)
			}
		})
	}
}
