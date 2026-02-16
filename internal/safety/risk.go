package safety

// Risk level constants used as ScanEntry.RiskLevel values.
const (
	RiskSafe     = "safe"
	RiskModerate = "moderate"
	RiskRisky    = "risky"
)

// categoryRisk maps known category IDs to their deletion risk level.
var categoryRisk = map[string]string{
	"system-caches":      RiskSafe,
	"system-logs":        RiskSafe,
	"quicklook":          RiskSafe,
	"browser-safari":     RiskModerate,
	"browser-chrome":     RiskModerate,
	"browser-firefox":    RiskModerate,
	"dev-xcode":          RiskRisky,
	"dev-npm":            RiskModerate,
	"dev-yarn":           RiskModerate,
	"dev-homebrew":       RiskModerate,
	"dev-docker":         RiskRisky,
	"app-orphaned-prefs": RiskRisky,
	"app-ios-backups":    RiskRisky,
	"app-old-downloads":  RiskModerate,
}

// RiskForCategory returns the risk level for a known category ID.
// Unknown categories default to moderate.
func RiskForCategory(categoryID string) string {
	if level, ok := categoryRisk[categoryID]; ok {
		return level
	}
	return RiskModerate
}
