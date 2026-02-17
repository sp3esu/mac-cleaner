package engine

import (
	"github.com/sp3esu/mac-cleaner/pkg/appleftovers"
	"github.com/sp3esu/mac-cleaner/pkg/browser"
	"github.com/sp3esu/mac-cleaner/pkg/creative"
	"github.com/sp3esu/mac-cleaner/pkg/developer"
	"github.com/sp3esu/mac-cleaner/pkg/messaging"
	"github.com/sp3esu/mac-cleaner/pkg/system"
	"github.com/sp3esu/mac-cleaner/pkg/unused"
)

// Register adds a scanner to the engine's registry.
func (e *Engine) Register(s Scanner) {
	e.scanners = append(e.scanners, s)
}

// Categories returns metadata for all registered scanners.
func (e *Engine) Categories() []ScannerInfo {
	infos := make([]ScannerInfo, len(e.scanners))
	for i, s := range e.scanners {
		infos[i] = s.Info()
	}
	return infos
}

// RegisterDefaults registers all built-in scanner groups with the engine.
// Each scanner wraps an existing pkg/*/Scan() function via the adapter pattern.
func RegisterDefaults(e *Engine) {
	e.Register(NewScanner(ScannerInfo{
		ID:          "system",
		Name:        "System Caches",
		Description: "User caches, logs, and QuickLook thumbnails",
		CategoryIDs: []string{"system-caches", "system-logs", "quicklook"},
	}, system.Scan))

	e.Register(NewScanner(ScannerInfo{
		ID:          "browser",
		Name:        "Browser Data",
		Description: "Safari, Chrome, and Firefox caches",
		CategoryIDs: []string{"browser-safari", "browser-chrome", "browser-firefox"},
	}, browser.Scan))

	e.Register(NewScanner(ScannerInfo{
		ID:          "developer",
		Name:        "Developer Caches",
		Description: "Xcode, npm, yarn, Homebrew, Docker, and more",
		CategoryIDs: []string{
			"dev-xcode", "dev-npm", "dev-yarn", "dev-homebrew", "dev-docker",
			"dev-pnpm", "dev-cocoapods", "dev-gradle", "dev-pip",
			"dev-simulator-caches", "dev-simulator-logs",
			"dev-xcode-device-support", "dev-xcode-archives",
		},
	}, developer.Scan))

	e.Register(NewScanner(ScannerInfo{
		ID:          "appleftovers",
		Name:        "App Leftovers",
		Description: "Orphaned preferences, iOS backups, and old Downloads",
		CategoryIDs: []string{"app-orphaned-prefs", "app-ios-backups", "app-old-downloads"},
	}, appleftovers.Scan))

	e.Register(NewScanner(ScannerInfo{
		ID:          "creative",
		Name:        "Creative App Caches",
		Description: "Adobe, Sketch, and Figma caches",
		CategoryIDs: []string{"creative-adobe", "creative-adobe-media", "creative-sketch", "creative-figma"},
	}, creative.Scan))

	e.Register(NewScanner(ScannerInfo{
		ID:          "messaging",
		Name:        "Messaging App Caches",
		Description: "Slack, Discord, Teams, and Zoom caches",
		CategoryIDs: []string{"msg-slack", "msg-discord", "msg-teams", "msg-zoom"},
	}, messaging.Scan))

	e.Register(NewScanner(ScannerInfo{
		ID:          "unused",
		Name:        "Unused Applications",
		Description: "Applications not opened in 180+ days",
		CategoryIDs: []string{"unused-apps"},
	}, unused.Scan))
}
