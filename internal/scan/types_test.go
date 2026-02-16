package scan

import "testing"

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
