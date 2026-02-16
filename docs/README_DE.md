# mac-cleaner

Ein schnelles, sicheres CLI-Tool zur Rückgewinnung von Speicherplatz unter macOS.

[English](../README.md) | [Polski](README_PL.md) | Deutsch | [Українська](README_UA.md) | [Русский](README_RU.md) | [Français](README_FR.md)

## Funktionen

### System-Caches
- **App-Caches** — `~/Library/Caches/` (sicher)
- **Benutzer-Logs** — `~/Library/Logs/` (sicher)
- **QuickLook-Miniaturbilder** — QuickLook-Cache des Benutzers (sicher)

### Browser-Daten
- **Safari-Cache** — `~/Library/Caches/com.apple.Safari/` (moderat)
- **Chrome-Cache** — `~/Library/Caches/Google/Chrome/` für alle Profile (moderat)
- **Firefox-Cache** — `~/Library/Caches/Firefox/` (moderat)

### Entwickler-Caches
- **Xcode DerivedData** — `~/Library/Developer/Xcode/DerivedData/` (riskant)
- **npm-Cache** — `~/.npm/` (moderat)
- **Yarn-Cache** — `~/Library/Caches/yarn/` (moderat)
- **Homebrew-Cache** — `~/Library/Caches/Homebrew/` (moderat)
- **Docker — rückgewinnbar** — Container, Images, Build-Cache, Volumes (riskant)
- **iOS-Simulator-Caches** — `~/Library/Developer/CoreSimulator/Caches/` (sicher)
- **iOS-Simulator-Logs** — `~/Library/Logs/CoreSimulator/` (sicher)
- **Xcode Device Support** — `~/Library/Developer/Xcode/iOS DeviceSupport/` (moderat)
- **Xcode Archives** — `~/Library/Developer/Xcode/Archives/` (riskant)
- **pnpm Store** — `~/Library/pnpm/store/` (moderat)
- **CocoaPods-Cache** — `~/Library/Caches/CocoaPods/` (moderat)
- **Gradle-Cache** — `~/.gradle/caches/` (moderat)
- **pip-Cache** — `~/Library/Caches/pip/` (sicher)

### App-Überbleibsel
- **Verwaiste Einstellungen** — `.plist`-Dateien in `~/Library/Preferences/` für deinstallierte Apps (riskant)
- **iOS-Gerätesicherungen** — `~/Library/Application Support/MobileSync/Backup/` (riskant)
- **Alte Downloads** — Dateien in `~/Downloads/` älter als 90 Tage (moderat)

### Kreativ-App-Caches
- **Adobe-Caches** — `~/Library/Caches/Adobe/` (sicher)
- **Adobe Media Cache** — `~/Library/Application Support/Adobe/Common/Media Cache Files/` + `Media Cache/` (moderat)
- **Sketch-Cache** — `~/Library/Caches/com.bohemiancoding.sketch3/` (sicher)
- **Figma-Cache** — `~/Library/Application Support/Figma/` (sicher)

### Messaging-App-Caches
- **Slack-Cache** — `~/Library/Application Support/Slack/Cache/` + `Service Worker/CacheStorage/` (sicher)
- **Discord-Cache** — `~/Library/Application Support/discord/Cache/` + `Code Cache/` (sicher)
- **Microsoft Teams-Cache** — `~/Library/Application Support/Microsoft/Teams/Cache/` + `~/Library/Caches/com.microsoft.teams2/` (sicher)
- **Zoom-Cache** — `~/Library/Application Support/zoom.us/data/` (sicher)

## Sicherheit

mac-cleaner wurde zum Schutz Ihres Systems entwickelt:

- **SIP-geschützte Pfade werden blockiert** — `/System`, `/usr`, `/bin`, `/sbin` werden nie verändert (`/usr/local` ist erlaubt)
- **Swap/VM-Schutz** — `/private/var/vm` wird immer blockiert, um Kernel Panics zu verhindern
- **Symlink-Auflösung** — alle Pfade werden vor dem Löschen aufgelöst
- **Drei Risikostufen** — jede Kategorie ist als **sicher**, **moderat** oder **riskant** eingestuft
- **Erneute Validierung vor dem Löschen** — Sicherheitsprüfungen werden beim Löschen erneut durchgeführt, nicht nur beim Scannen
- **Vorschau-Modus** — alles vor der Ausführung mit `--dry-run` prüfen
- **Interaktive Bestätigung** — explizite Benutzerzustimmung vor dem Löschen erforderlich (es sei denn, `--force` wird verwendet)

## Installation

### Voraussetzungen

- **Go 1.25+**
- **macOS**

### Aus dem Quellcode bauen

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Verwendung

**Interaktiver Modus** (Standard — führt durch jede Kategorie):
```bash
./mac-cleaner
```

**Alles scannen, nur Vorschau:**
```bash
./mac-cleaner --all --dry-run
```

**System-Caches ohne Bestätigung bereinigen:**
```bash
./mac-cleaner --system-caches --force
```

**Alles scannen, JSON-Ausgabe:**
```bash
./mac-cleaner --all --json
```

**Alles scannen, aber Docker und iOS-Backups überspringen:**
```bash
./mac-cleaner --all --skip-docker --skip-ios-backups
```

## CLI-Flags

### Scan-Kategorien

| Flag | Beschreibung |
|------|-------------|
| `--all` | Alle Kategorien scannen |
| `--system-caches` | App-Caches, Logs und QuickLook-Miniaturbilder scannen |
| `--browser-data` | Safari-, Chrome- und Firefox-Caches scannen |
| `--dev-caches` | Xcode-, npm/yarn-, Homebrew- und Docker-Caches scannen |
| `--app-leftovers` | Verwaiste Einstellungen, iOS-Backups und alte Downloads scannen |
| `--creative-caches` | Adobe-, Sketch- und Figma-Caches scannen |
| `--messaging-caches` | Slack-, Discord-, Teams- und Zoom-Caches scannen |

### Ausgabe & Verhalten

| Flag | Beschreibung |
|------|-------------|
| `--dry-run` | Vorschau der zu löschenden Dateien ohne tatsächliches Löschen |
| `--json` | Ergebnisse als JSON ausgeben |
| `--verbose` | Detaillierte Dateiliste anzeigen |
| `--force` | Bestätigungsabfrage überspringen |

### Kategorie-Skip-Flags

| Flag | Beschreibung |
|------|-------------|
| `--skip-system-caches` | System-Cache-Scan überspringen |
| `--skip-browser-data` | Browser-Daten-Scan überspringen |
| `--skip-dev-caches` | Entwickler-Cache-Scan überspringen |
| `--skip-app-leftovers` | App-Überbleibsel-Scan überspringen |
| `--skip-creative-caches` | Kreativ-App-Cache-Scan überspringen |
| `--skip-messaging-caches` | Messaging-App-Cache-Scan überspringen |

### Element-Skip-Flags

| Flag | Beschreibung |
|------|-------------|
| `--skip-derived-data` | Xcode DerivedData überspringen |
| `--skip-npm` | npm-Cache überspringen |
| `--skip-yarn` | Yarn-Cache überspringen |
| `--skip-homebrew` | Homebrew-Cache überspringen |
| `--skip-docker` | Docker-rückgewinnbaren Speicher überspringen |
| `--skip-safari` | Safari-Cache überspringen |
| `--skip-chrome` | Chrome-Cache überspringen |
| `--skip-firefox` | Firefox-Cache überspringen |
| `--skip-quicklook` | QuickLook-Miniaturbilder überspringen |
| `--skip-orphaned-prefs` | Verwaiste Einstellungen überspringen |
| `--skip-ios-backups` | iOS-Gerätesicherungen überspringen |
| `--skip-old-downloads` | Alte Downloads überspringen |
| `--skip-simulator-caches` | iOS-Simulator-Caches überspringen |
| `--skip-simulator-logs` | iOS-Simulator-Logs überspringen |
| `--skip-xcode-device-support` | Xcode Device Support überspringen |
| `--skip-xcode-archives` | Xcode Archives überspringen |
| `--skip-pnpm` | pnpm Store überspringen |
| `--skip-cocoapods` | CocoaPods-Cache überspringen |
| `--skip-gradle` | Gradle-Cache überspringen |
| `--skip-pip` | pip-Cache überspringen |
| `--skip-adobe` | Adobe-Caches überspringen |
| `--skip-adobe-media` | Adobe Media Cache überspringen |
| `--skip-sketch` | Sketch-Cache überspringen |
| `--skip-figma` | Figma-Cache überspringen |
| `--skip-slack` | Slack-Cache überspringen |
| `--skip-discord` | Discord-Cache überspringen |
| `--skip-teams` | Microsoft Teams-Cache überspringen |
| `--skip-zoom` | Zoom-Cache überspringen |

## Lizenz

Dieses Projekt enthält derzeit keine Lizenzdatei.
