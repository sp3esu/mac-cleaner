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

### Projekt-Caches
- **node_modules** — `node_modules/`-Verzeichnisse in JavaScript/TypeScript-Projekten (sicher)
- **.build (Swift)** — `.build/`-Verzeichnisse in Swift-Package-Manager-Projekten (sicher)
- **target (Rust)** — `target/`-Verzeichnisse in Cargo/Rust-Projekten (sicher)
- **venv (Python)** — `venv/`- und `.venv/`-Verzeichnisse in Python-Projekten (moderat)
- **build (Java/Kotlin)** — `build/`-Verzeichnisse in Gradle/Maven-Projekten (sicher)

Projekt-Caches werden durch Durchsuchen Ihres Home-Verzeichnisses erkannt und gegen die jeweilige Manifest-Datei validiert (z.B. `package.json` fuer node_modules, `Cargo.toml` fuer target). Projekte, die seit ueber 6 Monaten inaktiv sind, werden mit einem Inaktivitaetsindikator markiert.

### Home-Verzeichnis-Dateien
- **Grosse Dateien (>100 MB)** — Dateien groesser als 100 MB in Ihrem Home-Verzeichnis (riskant)
- **Alte Dateien (>180 Tage)** — Dateien aelter als 180 Tage und groesser als 10 MB in Ihrem Home-Verzeichnis (riskant)

Die Home-Verzeichnis-Suche schliesst `~/Library`, `~/Applications`, `~/.Trash`, `~/Downloads` und Dotfile-Verzeichnisse (z.B. `~/.config`) aus. Ergebnisse werden als "riskant" markiert, da es sich um persoenliche Dateien handelt, nicht um regenerierbare Caches.

> **Hinweis:** iCloud-Drive-Platzhalterdateien koennen ihre urspruengliche Groesse anzeigen, auch wenn sie nicht lokal gespeichert sind.

### Git-Repository-Bereinigung
- **Git-Repositorys** — `.git/`-Verzeichnisse im Home-Verzeichnis gefunden (moderat)

Die Git-Repository-Bereinigung entdeckt `.git`-Verzeichnisse unter Ihrem Home-Ordner, zeigt deren Groesse und geschaetzten rueckgewinnbaren Speicherplatz via `git count-objects` an und bietet `git gc --aggressive`-Bereinigung an. Fuer Repositorys mit Git LFS (erkannt durch `.git/lfs`) wird `git lfs prune` automatisch ausgefuehrt, wenn `git-lfs` installiert ist. Repositorys mit aktiven Operationen (`index.lock` vorhanden) werden sicher uebersprungen.

> **Hinweis:** Die Bereinigung fuehrt `git gc --aggressive` und `git lfs prune` aus — es werden keine Repository-Dateien geloescht. Dies ist fuer alle Repositorys sicher.

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

### Fotos- und Medien-Caches
- **Fotos-App-Caches** — `~/Library/Containers/com.apple.Photos/`-Caches (sicher)
- **Fotos-Analyse-Caches** — `~/Library/Containers/com.apple.photoanalysisd/` ML-Modelldaten (sicher)
- **iCloud-Fotos-Sync-Cache** — `~/Library/Caches/com.apple.cloudd/` (moderat)
- **Geteilte Fotos aus Nachrichten** — `~/Library/Messages/Attachments/` synchronisierte Medien (riskant)

### Systemdaten
- **CoreSpotlight-Metadaten** — `~/Library/Caches/com.apple.Spotlight/` (sicher)
- **Mail-Datenbank** — `~/Library/Mail/` Envelope-Index und Daten (riskant)
- **Mail-Anhang-Cache** — `~/Library/Mail Downloads/` (moderat)
- **Nachrichten-Anhänge** — `~/Library/Messages/` Medien und Anhänge (riskant)
- **iOS-Softwareaktualisierungen** — `~/Library/iTunes/iPhone Software Updates/` (sicher)
- **Lokale Time-Machine-Snapshots** — lokale TM-Snapshot-Metadaten (riskant)
- **Parallels-VMs** — `~/Parallels/` Disk-Images virtueller Maschinen (riskant)
- **UTM-VMs** — `~/Library/Containers/com.utmapp.UTM/` virtuelle Maschinen (riskant)
- **VMware Fusion-VMs** — `~/Virtual Machines.localized/` Disk-Images (riskant)

### Unbenutzte Anwendungen
- **Unbenutzte Apps** — Anwendungen in `/Applications` und `~/Applications`, die seit über 180 Tagen nicht geöffnet wurden, mit gesamtem Speicherverbrauch einschließlich `~/Library/`-Daten (riskant)

Details finden Sie in der Dokumentation [Erkennung unbenutzter Anwendungen](unused-apps_DE.md).

## Sicherheit

mac-cleaner wurde zum Schutz Ihres Systems entwickelt:

- **SIP-geschützte Pfade werden blockiert** — `/System`, `/usr`, `/bin`, `/sbin` werden nie verändert (`/usr/local` ist erlaubt)
- **Swap/VM-Schutz** — `/private/var/vm` wird immer blockiert, um Kernel Panics zu verhindern
- **Symlink-Auflösung** — alle Pfade werden vor dem Löschen aufgelöst
- **Drei Risikostufen** — jede Kategorie ist als **sicher**, **moderat** oder **riskant** eingestuft
- **Erneute Validierung vor dem Löschen** — Sicherheitsprüfungen werden beim Löschen erneut durchgeführt, nicht nur beim Scannen
- **Vorschau-Modus** — alles vor der Ausführung mit `--dry-run` prüfen
- **Interaktive Bestätigung** — explizite Benutzerzustimmung vor dem Löschen erforderlich (es sei denn, `--force` wird verwendet)

Eine detaillierte Sicherheitsanalyse finden Sie in der [Sicherheitsarchitektur](SECURITY_DE.md).

## Installation

### Homebrew

```bash
brew install sp3esu/tap/mac-cleaner
```

### Aus dem Quellcode bauen

**Voraussetzungen:** Go 1.25+, macOS

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Shell-Vervollständigung

Generieren Sie Shell-Vervollständigungsskripte für die Tab-Vervollständigung von Flags und Unterbefehlen.

**Bash:**
```bash
# In der aktuellen Sitzung laden:
source <(mac-cleaner completion bash)

# Dauerhaft installieren:
mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner
```

**Zsh:**
```bash
mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"
# Dann Shell neu starten oder ausführen: compinit
```

**Fish:**
```bash
mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish
```

**PowerShell:**
```powershell
mac-cleaner completion powershell | Out-String | Invoke-Expression
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

**Projekt-Build-Caches scannen (node_modules, .build, target, venv, build):**
```bash
./mac-cleaner --project-caches --dry-run
```

**Grosse und alte Dateien im Home-Verzeichnis scannen:**
```bash
./mac-cleaner --home-files --dry-run
```

**Git-Repositorys nach rueckgewinnbarem Cache-Speicherplatz scannen:**
```bash
./mac-cleaner --git-repos --dry-run
```

**Nur node_modules-Verzeichnisse scannen:**
```bash
./mac-cleaner scan --node-modules --dry-run
```

**Gezielter Scan — nur bestimmte Elemente (über den `scan`-Unterbefehl):**
```bash
./mac-cleaner scan --npm --safari --dry-run
```

**Gezielter Scan — komplette Gruppe plus einzelne Elemente:**
```bash
./mac-cleaner scan --dev-caches --safari
```

**Gezielter Scan — Gruppe ohne bestimmte Elemente:**
```bash
./mac-cleaner scan --dev-caches --skip-docker
```

**Strukturierte Hilfe für KI-Agenten:**
```bash
./mac-cleaner --help-json
```

## CLI-Flags

### Scan-Kategorien

| Flag | Beschreibung |
|------|-------------|
| `--all` | Alle Kategorien scannen |
| `--system-caches` | App-Caches, Logs und QuickLook-Miniaturbilder scannen |
| `--browser-data` | Safari-, Chrome- und Firefox-Caches scannen |
| `--dev-caches` | Xcode-, npm/yarn-, Homebrew- und Docker-Caches scannen |
| `--project-caches` | Projekt-Build-Caches scannen (node_modules, .build, target, venv, build) |
| `--home-files` | Grosse und alte Dateien im Home-Verzeichnis scannen |
| `--git-repos` | Git-Repository-Caches scannen |
| `--app-leftovers` | Verwaiste Einstellungen, iOS-Backups und alte Downloads scannen |
| `--creative-caches` | Adobe-, Sketch- und Figma-Caches scannen |
| `--messaging-caches` | Slack-, Discord-, Teams- und Zoom-Caches scannen |
| `--unused-apps` | Anwendungen scannen, die seit über 180 Tagen nicht geöffnet wurden |
| `--photos` | Fotos-App-Caches und Medienanalysedaten scannen |
| `--system-data` | Spotlight, Mail, Nachrichten, iOS-Updates, Time Machine und VMs scannen |

### Ausgabe & Verhalten

| Flag | Beschreibung |
|------|-------------|
| `--dry-run` | Vorschau der zu löschenden Dateien ohne tatsächliches Löschen |
| `--json` | Ergebnisse als JSON ausgeben |
| `--verbose` | Detaillierte Dateiliste anzeigen |
| `--force` | Bestätigungsabfrage überspringen |
| `--help-json` | Strukturierte Hilfe als JSON für KI-Agenten ausgeben |

### Kategorie-Skip-Flags

| Flag | Beschreibung |
|------|-------------|
| `--skip-system-caches` | System-Cache-Scan überspringen |
| `--skip-browser-data` | Browser-Daten-Scan überspringen |
| `--skip-dev-caches` | Entwickler-Cache-Scan überspringen |
| `--skip-project-caches` | Projekt-Cache-Scan überspringen |
| `--skip-home-files` | Home-Verzeichnis-Dateien-Scan überspringen |
| `--skip-git-repos` | Git-Repository-Scan überspringen |
| `--skip-app-leftovers` | App-Überbleibsel-Scan überspringen |
| `--skip-creative-caches` | Kreativ-App-Cache-Scan überspringen |
| `--skip-messaging-caches` | Messaging-App-Cache-Scan überspringen |
| `--skip-unused-apps` | Scan unbenutzter Anwendungen überspringen |
| `--skip-photos` | Fotos-Cache-Scan überspringen |
| `--skip-system-data` | Systemdaten-Scan überspringen |

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
| `--skip-node-modules` | node_modules-Verzeichnisse überspringen |
| `--skip-swift-build` | .build (Swift)-Verzeichnisse überspringen |
| `--skip-rust-target` | target (Rust)-Verzeichnisse überspringen |
| `--skip-python-venv` | venv (Python)-Verzeichnisse überspringen |
| `--skip-java-build` | build (Java/Kotlin)-Verzeichnisse überspringen |
| `--skip-large-files` | Grosse Dateien im Home-Verzeichnis überspringen |
| `--skip-old-files` | Alte Dateien im Home-Verzeichnis überspringen |
| `--skip-adobe` | Adobe-Caches überspringen |
| `--skip-adobe-media` | Adobe Media Cache überspringen |
| `--skip-sketch` | Sketch-Cache überspringen |
| `--skip-figma` | Figma-Cache überspringen |
| `--skip-slack` | Slack-Cache überspringen |
| `--skip-discord` | Discord-Cache überspringen |
| `--skip-teams` | Microsoft Teams-Cache überspringen |
| `--skip-zoom` | Zoom-Cache überspringen |
| `--skip-photos-caches` | Fotos-App-Caches überspringen |
| `--skip-photos-analysis` | Fotos-Analyse-Caches überspringen |
| `--skip-photos-icloud-cache` | iCloud-Fotos-Sync-Cache überspringen |
| `--skip-photos-syndication` | Geteilte Fotos aus Nachrichten überspringen |
| `--skip-spotlight` | CoreSpotlight-Metadaten überspringen |
| `--skip-mail` | Mail-Datenbank überspringen |
| `--skip-mail-downloads` | Mail-Anhang-Cache überspringen |
| `--skip-messages` | Nachrichten-Anhänge überspringen |
| `--skip-ios-updates` | iOS-Softwareaktualisierungen überspringen |
| `--skip-timemachine` | Lokale Time-Machine-Snapshots überspringen |
| `--skip-vm-parallels` | Parallels-VMs überspringen |
| `--skip-vm-utm` | UTM-VMs überspringen |
| `--skip-vm-vmware` | VMware Fusion-VMs überspringen |

### Scan-Unterbefehl

Der `scan`-Unterbefehl ermöglicht gezieltes Scannen auf Elementebene. Im Gegensatz zum Hauptbefehl (der standardmäßig den interaktiven Modus startet) erfordert `scan` explizite Flags und unterstützt das gezielte Ansprechen einzelner Elemente.

```bash
# Nur npm- und Yarn-Caches scannen
mac-cleaner scan --npm --yarn --dry-run

# Alle Entwickler-Caches plus Safari scannen
mac-cleaner scan --dev-caches --safari

# Alles außer Docker scannen
mac-cleaner scan --all --skip-docker

# Ausgabe als JSON für Automatisierung
mac-cleaner scan --npm --json
```

Führen Sie `mac-cleaner scan --help` aus, um die vollständige Liste der gezielten Flags nach Kategorien gruppiert anzuzeigen.

## Geplante Bereinigung

`mac-cleaner schedule` installiert einen benutzerbezogenen launchd-Agenten (`~/Library/LaunchAgents/com.mac-cleaner.plist`), der Entwickler-Caches automatisch bereinigt — standardmäßig Xcode DerivedData, jede Nacht um 03:00 Uhr. Er läuft unter Ihrem Benutzer, daher ist kein `sudo` erforderlich.

```bash
# Xcode DerivedData täglich um 03:00 Uhr bereinigen
mac-cleaner schedule install

# Zusätzlich iOS-Simulator-Caches bereinigen, montags und freitags um 22:30 Uhr
mac-cleaner schedule install --at 22:30 --weekdays mon,fri --simulator-caches

# Vorschau ohne Installation
mac-cleaner schedule install --dry-run

mac-cleaner schedule status      # Zeitplan, nächster Lauf, letzter Exit-Code, Log-Pfad
mac-cleaner schedule run         # Job sofort ausführen
mac-cleaner schedule uninstall   # Agenten entfernen
```

| Flag | Beschreibung |
|------|--------------|
| `--at HH:MM` | Uhrzeit im 24-Stunden-Format (Standard `03:00`) |
| `--weekdays` | Kommagetrennte Tage, z. B. `mon,wed,fri` (Standard: jeden Tag) |
| `--targets` | Zu bereinigende Elemente, als `scan`-Flag-Namen (Standard `derived-data`) |
| `--simulator-caches` | Zusätzlich iOS-Simulator-Caches bereinigen |
| `--even-if-running` | Auch bereinigen, während Xcode läuft (standardmäßig wird der Lauf übersprungen) |

Hinweise:
- Schläft der Mac zur geplanten Zeit, wird der Job beim Aufwachen ausgeführt; ist er ausgeschaltet, entfällt dieser Lauf.
- Die Ausgabe wird an `~/Library/Logs/mac-cleaner/scheduled.log` angehängt.
- Ein erneutes `install` ersetzt den bestehenden Zeitplan. Bei Installation über Homebrew wird der stabile Pfad `/opt/homebrew/bin/mac-cleaner` verwendet, damit Upgrades weiter funktionieren.

## Lizenz

MIT

## Erstellt mit

Dieses Projekt wurde mit [Claude Code](https://claude.com/product/claude-code) und dem [Get Shit Done](https://github.com/gsd-build/get-shit-done)-Plugin erstellt.
