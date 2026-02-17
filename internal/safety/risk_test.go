package safety

import "testing"

func TestRiskForCategory(t *testing.T) {
	tests := []struct {
		categoryID string
		want       string
	}{
		// Safe categories.
		{"system-caches", RiskSafe},
		{"system-logs", RiskSafe},
		{"quicklook", RiskSafe},

		// Moderate categories.
		{"browser-safari", RiskModerate},
		{"browser-chrome", RiskModerate},
		{"browser-firefox", RiskModerate},
		{"dev-npm", RiskModerate},
		{"dev-yarn", RiskModerate},
		{"dev-homebrew", RiskModerate},
		{"app-old-downloads", RiskModerate},

		// Risky categories.
		{"dev-xcode", RiskRisky},
		{"dev-docker", RiskRisky},
		{"app-orphaned-prefs", RiskRisky},
		{"app-ios-backups", RiskRisky},
		{"unused-apps", RiskRisky},

		// Unknown and empty default to moderate.
		{"unknown-category", RiskModerate},
		{"", RiskModerate},
	}

	for _, tt := range tests {
		t.Run(tt.categoryID, func(t *testing.T) {
			got := RiskForCategory(tt.categoryID)
			if got != tt.want {
				t.Errorf("RiskForCategory(%q) = %q, want %q", tt.categoryID, got, tt.want)
			}
		})
	}
}
