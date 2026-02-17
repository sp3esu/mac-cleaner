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
	"app-orphaned-prefs":       RiskRisky,
	"app-ios-backups":          RiskRisky,
	"app-old-downloads":        RiskModerate,
	"dev-simulator-caches":     RiskSafe,
	"dev-simulator-logs":       RiskSafe,
	"dev-xcode-device-support": RiskModerate,
	"dev-xcode-archives":       RiskRisky,
	"dev-pnpm":                 RiskModerate,
	"dev-cocoapods":            RiskModerate,
	"dev-gradle":               RiskModerate,
	"dev-pip":                  RiskSafe,
	"creative-adobe":           RiskSafe,
	"creative-adobe-media":     RiskModerate,
	"creative-sketch":          RiskSafe,
	"creative-figma":           RiskSafe,
	"msg-slack":                RiskSafe,
	"msg-discord":              RiskSafe,
	"msg-teams":                RiskSafe,
	"msg-zoom":                 RiskSafe,
	"unused-apps":              RiskRisky,
	"photos-caches":            RiskSafe,
	"photos-analysis":          RiskSafe,
	"photos-icloud-cache":      RiskModerate,
	"photos-syndication":       RiskRisky,
	"sysdata-spotlight":        RiskSafe,
	"sysdata-mail":             RiskRisky,
	"sysdata-mail-downloads":   RiskModerate,
	"sysdata-messages":         RiskRisky,
	"sysdata-ios-updates":      RiskSafe,
	"sysdata-timemachine":      RiskRisky,
	"sysdata-vm-parallels":     RiskRisky,
	"sysdata-vm-utm":           RiskRisky,
	"sysdata-vm-vmware":        RiskRisky,
}

// RiskForCategory returns the risk level for a known category ID.
// Unknown categories default to moderate.
func RiskForCategory(categoryID string) string {
	if level, ok := categoryRisk[categoryID]; ok {
		return level
	}
	return RiskModerate
}
