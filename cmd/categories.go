package cmd

// categoryDef describes a single scannable category within a scanner group.
type categoryDef struct {
	FlagName    string // targeted scan flag name, e.g. "npm" (empty if no per-item flag)
	CategoryID  string // engine category ID, e.g. "dev-npm"
	Description string // human-readable, e.g. "npm cache"
	SkipFlag    *bool  // pointer to skip flag variable (nil if no skip flag)
	ScanFlag    *bool  // pointer to targeted scan flag variable (nil if no targeted flag)
}

// groupDef describes a scanner group containing multiple categories.
type groupDef struct {
	FlagName    string        // group flag name, e.g. "dev-caches"
	ScannerID   string        // engine scanner ID, e.g. "developer"
	GroupName   string        // human-readable label, e.g. "Developer Caches"
	Description string        // flag help text
	ScanFlag    *bool         // pointer to group scan flag variable
	SkipFlag    *bool         // pointer to category-level skip flag variable
	Items       []categoryDef // categories produced by this scanner
}

// Targeted scan flag variables — registered on the scan subcommand only.
var (
	flagScanQuicklook         bool
	flagScanSafari            bool
	flagScanChrome            bool
	flagScanFirefox           bool
	flagScanDerivedData       bool
	flagScanNpm               bool
	flagScanYarn              bool
	flagScanHomebrew          bool
	flagScanDocker            bool
	flagScanSimulatorCaches   bool
	flagScanSimulatorLogs     bool
	flagScanXcodeDevSupport   bool
	flagScanXcodeArchives     bool
	flagScanPnpm              bool
	flagScanCocoapods         bool
	flagScanGradle            bool
	flagScanPip               bool
	flagScanOrphanedPrefs     bool
	flagScanIosBackups        bool
	flagScanOldDownloads      bool
	flagScanAdobe             bool
	flagScanAdobeMedia        bool
	flagScanSketch            bool
	flagScanFigma             bool
	flagScanSlack             bool
	flagScanDiscord           bool
	flagScanTeams             bool
	flagScanZoom              bool
	flagScanPhotosCaches      bool
	flagScanPhotosAnalysis    bool
	flagScanPhotosIcloudCache bool
	flagScanPhotosSyndication bool
	flagScanSpotlight         bool
	flagScanMail              bool
	flagScanMailDownloads     bool
	flagScanMessages          bool
	flagScanIOSUpdates        bool
	flagScanTimemachine       bool
	flagScanVMParallels       bool
	flagScanVMUTM             bool
	flagScanVMVMware          bool
)

// scanGroups is the central registry of all scanner groups and their
// categories. It is the single source of truth for flag names, category
// IDs, and group/item relationships used by the scan subcommand,
// help-json output, buildSkipSet, and flagForCategory.
var scanGroups = []groupDef{
	{
		FlagName:    "system-caches",
		ScannerID:   "system",
		GroupName:   "System Caches",
		Description: "user app caches, logs, and QuickLook thumbnails",
		ScanFlag:    &flagSystemCaches,
		SkipFlag:    &flagSkipSystemCaches,
		Items: []categoryDef{
			{CategoryID: "system-caches", Description: "user app caches"},
			{CategoryID: "system-logs", Description: "user logs"},
			{FlagName: "quicklook", CategoryID: "quicklook", Description: "QuickLook thumbnails", SkipFlag: &flagSkipQuicklook, ScanFlag: &flagScanQuicklook},
		},
	},
	{
		FlagName:    "browser-data",
		ScannerID:   "browser",
		GroupName:   "Browser Data",
		Description: "Safari, Chrome, and Firefox caches",
		ScanFlag:    &flagBrowserData,
		SkipFlag:    &flagSkipBrowserData,
		Items: []categoryDef{
			{FlagName: "safari", CategoryID: "browser-safari", Description: "Safari cache", SkipFlag: &flagSkipSafari, ScanFlag: &flagScanSafari},
			{FlagName: "chrome", CategoryID: "browser-chrome", Description: "Chrome cache", SkipFlag: &flagSkipChrome, ScanFlag: &flagScanChrome},
			{FlagName: "firefox", CategoryID: "browser-firefox", Description: "Firefox cache", SkipFlag: &flagSkipFirefox, ScanFlag: &flagScanFirefox},
		},
	},
	{
		FlagName:    "dev-caches",
		ScannerID:   "developer",
		GroupName:   "Developer Caches",
		Description: "Xcode, npm/yarn, Homebrew, Docker, and more",
		ScanFlag:    &flagDevCaches,
		SkipFlag:    &flagSkipDevCaches,
		Items: []categoryDef{
			{FlagName: "derived-data", CategoryID: "dev-xcode", Description: "Xcode DerivedData", SkipFlag: &flagSkipDerivedData, ScanFlag: &flagScanDerivedData},
			{FlagName: "npm", CategoryID: "dev-npm", Description: "npm cache", SkipFlag: &flagSkipNpm, ScanFlag: &flagScanNpm},
			{FlagName: "yarn", CategoryID: "dev-yarn", Description: "Yarn cache", SkipFlag: &flagSkipYarn, ScanFlag: &flagScanYarn},
			{FlagName: "homebrew", CategoryID: "dev-homebrew", Description: "Homebrew cache", SkipFlag: &flagSkipHomebrew, ScanFlag: &flagScanHomebrew},
			{FlagName: "docker", CategoryID: "dev-docker", Description: "Docker reclaimable space", SkipFlag: &flagSkipDocker, ScanFlag: &flagScanDocker},
			{FlagName: "pnpm", CategoryID: "dev-pnpm", Description: "pnpm store", SkipFlag: &flagSkipPnpm, ScanFlag: &flagScanPnpm},
			{FlagName: "cocoapods", CategoryID: "dev-cocoapods", Description: "CocoaPods cache", SkipFlag: &flagSkipCocoapods, ScanFlag: &flagScanCocoapods},
			{FlagName: "gradle", CategoryID: "dev-gradle", Description: "Gradle cache", SkipFlag: &flagSkipGradle, ScanFlag: &flagScanGradle},
			{FlagName: "pip", CategoryID: "dev-pip", Description: "pip cache", SkipFlag: &flagSkipPip, ScanFlag: &flagScanPip},
			{FlagName: "simulator-caches", CategoryID: "dev-simulator-caches", Description: "iOS Simulator caches", SkipFlag: &flagSkipSimulatorCaches, ScanFlag: &flagScanSimulatorCaches},
			{FlagName: "simulator-logs", CategoryID: "dev-simulator-logs", Description: "iOS Simulator logs", SkipFlag: &flagSkipSimulatorLogs, ScanFlag: &flagScanSimulatorLogs},
			{FlagName: "xcode-device-support", CategoryID: "dev-xcode-device-support", Description: "Xcode Device Support files", SkipFlag: &flagSkipXcodeDevSupport, ScanFlag: &flagScanXcodeDevSupport},
			{FlagName: "xcode-archives", CategoryID: "dev-xcode-archives", Description: "Xcode Archives", SkipFlag: &flagSkipXcodeArchives, ScanFlag: &flagScanXcodeArchives},
		},
	},
	{
		FlagName:    "app-leftovers",
		ScannerID:   "appleftovers",
		GroupName:   "App Leftovers",
		Description: "orphaned preferences, iOS backups, and old Downloads",
		ScanFlag:    &flagAppLeftovers,
		SkipFlag:    &flagSkipAppLeftovers,
		Items: []categoryDef{
			{FlagName: "orphaned-prefs", CategoryID: "app-orphaned-prefs", Description: "orphaned preferences", SkipFlag: &flagSkipOrphanedPrefs, ScanFlag: &flagScanOrphanedPrefs},
			{FlagName: "ios-backups", CategoryID: "app-ios-backups", Description: "iOS device backups", SkipFlag: &flagSkipIosBackups, ScanFlag: &flagScanIosBackups},
			{FlagName: "old-downloads", CategoryID: "app-old-downloads", Description: "old Downloads files", SkipFlag: &flagSkipOldDownloads, ScanFlag: &flagScanOldDownloads},
		},
	},
	{
		FlagName:    "creative-caches",
		ScannerID:   "creative",
		GroupName:   "Creative App Caches",
		Description: "Adobe, Sketch, and Figma caches",
		ScanFlag:    &flagCreativeCaches,
		SkipFlag:    &flagSkipCreativeCaches,
		Items: []categoryDef{
			{FlagName: "adobe", CategoryID: "creative-adobe", Description: "Adobe caches", SkipFlag: &flagSkipAdobe, ScanFlag: &flagScanAdobe},
			{FlagName: "adobe-media", CategoryID: "creative-adobe-media", Description: "Adobe media caches", SkipFlag: &flagSkipAdobeMedia, ScanFlag: &flagScanAdobeMedia},
			{FlagName: "sketch", CategoryID: "creative-sketch", Description: "Sketch cache", SkipFlag: &flagSkipSketch, ScanFlag: &flagScanSketch},
			{FlagName: "figma", CategoryID: "creative-figma", Description: "Figma cache", SkipFlag: &flagSkipFigma, ScanFlag: &flagScanFigma},
		},
	},
	{
		FlagName:    "messaging-caches",
		ScannerID:   "messaging",
		GroupName:   "Messaging App Caches",
		Description: "Slack, Discord, Teams, and Zoom caches",
		ScanFlag:    &flagMessagingCaches,
		SkipFlag:    &flagSkipMessagingCaches,
		Items: []categoryDef{
			{FlagName: "slack", CategoryID: "msg-slack", Description: "Slack cache", SkipFlag: &flagSkipSlack, ScanFlag: &flagScanSlack},
			{FlagName: "discord", CategoryID: "msg-discord", Description: "Discord cache", SkipFlag: &flagSkipDiscord, ScanFlag: &flagScanDiscord},
			{FlagName: "teams", CategoryID: "msg-teams", Description: "Microsoft Teams cache", SkipFlag: &flagSkipTeams, ScanFlag: &flagScanTeams},
			{FlagName: "zoom", CategoryID: "msg-zoom", Description: "Zoom cache", SkipFlag: &flagSkipZoom, ScanFlag: &flagScanZoom},
		},
	},
	{
		FlagName:    "unused-apps",
		ScannerID:   "unused",
		GroupName:   "Unused Applications",
		Description: "applications not opened in 180+ days",
		ScanFlag:    &flagUnusedApps,
		SkipFlag:    &flagSkipUnusedApps,
		Items: []categoryDef{
			{CategoryID: "unused-apps", Description: "applications not opened in 180+ days", SkipFlag: &flagSkipUnusedApps},
		},
	},
	{
		FlagName:    "photos",
		ScannerID:   "photos",
		GroupName:   "Photos & Media Caches",
		Description: "Photos app caches and media analysis data",
		ScanFlag:    &flagPhotos,
		SkipFlag:    &flagSkipPhotos,
		Items: []categoryDef{
			{FlagName: "photos-caches", CategoryID: "photos-caches", Description: "Photos app caches", SkipFlag: &flagSkipPhotosCaches, ScanFlag: &flagScanPhotosCaches},
			{FlagName: "photos-analysis", CategoryID: "photos-analysis", Description: "Photos analysis caches", SkipFlag: &flagSkipPhotosAnalysis, ScanFlag: &flagScanPhotosAnalysis},
			{FlagName: "photos-icloud-cache", CategoryID: "photos-icloud-cache", Description: "iCloud Photos sync cache", SkipFlag: &flagSkipPhotosIcloudCache, ScanFlag: &flagScanPhotosIcloudCache},
			{FlagName: "photos-syndication", CategoryID: "photos-syndication", Description: "Messages shared photos", SkipFlag: &flagSkipPhotosSyndication, ScanFlag: &flagScanPhotosSyndication},
		},
	},
	{
		FlagName:    "system-data",
		ScannerID:   "systemdata",
		GroupName:   "System Data",
		Description: "Spotlight, Mail, Messages, iOS updates, Time Machine, and VMs",
		ScanFlag:    &flagSystemData,
		SkipFlag:    &flagSkipSystemData,
		Items: []categoryDef{
			{FlagName: "spotlight", CategoryID: "sysdata-spotlight", Description: "CoreSpotlight metadata", SkipFlag: &flagSkipSpotlight, ScanFlag: &flagScanSpotlight},
			{FlagName: "mail", CategoryID: "sysdata-mail", Description: "Mail database", SkipFlag: &flagSkipMail, ScanFlag: &flagScanMail},
			{FlagName: "mail-downloads", CategoryID: "sysdata-mail-downloads", Description: "Mail attachment cache", SkipFlag: &flagSkipMailDownloads, ScanFlag: &flagScanMailDownloads},
			{FlagName: "messages", CategoryID: "sysdata-messages", Description: "Messages attachments", SkipFlag: &flagSkipMessages, ScanFlag: &flagScanMessages},
			{FlagName: "ios-updates", CategoryID: "sysdata-ios-updates", Description: "iOS software updates", SkipFlag: &flagSkipIOSUpdates, ScanFlag: &flagScanIOSUpdates},
			{FlagName: "timemachine", CategoryID: "sysdata-timemachine", Description: "Time Machine local snapshots", SkipFlag: &flagSkipTimemachine, ScanFlag: &flagScanTimemachine},
			{FlagName: "vm-parallels", CategoryID: "sysdata-vm-parallels", Description: "Parallels VMs", SkipFlag: &flagSkipVMParallels, ScanFlag: &flagScanVMParallels},
			{FlagName: "vm-utm", CategoryID: "sysdata-vm-utm", Description: "UTM VMs", SkipFlag: &flagSkipVMUTM, ScanFlag: &flagScanVMUTM},
			{FlagName: "vm-vmware", CategoryID: "sysdata-vm-vmware", Description: "VMware Fusion VMs", SkipFlag: &flagSkipVMVMware, ScanFlag: &flagScanVMVMware},
		},
	},
}

// groupForCategory returns the groupDef containing the given category ID.
// Returns nil if not found.
func groupForCategory(categoryID string) *groupDef {
	for i := range scanGroups {
		for _, item := range scanGroups[i].Items {
			if item.CategoryID == categoryID {
				return &scanGroups[i]
			}
		}
	}
	return nil
}
