# Erkennung unbenutzter Anwendungen

## Funktionsweise

macOS Spotlight zeichnet `kMDItemLastUsedDate` für jedes `.app`-Bundle auf, sobald der Benutzer es öffnet. mac-cleaner fragt diese Metadaten ab, um Anwendungen zu identifizieren, die seit einem konfigurierbaren Zeitraum nicht geöffnet wurden (Standard: 180 Tage).

```bash
mdls -name kMDItemLastUsedDate -raw /Applications/SomeApp.app
# Returns: "2024-05-14 09:23:41 +0000" or "(null)" if never opened
```

Es sind keine besonderen Berechtigungen erforderlich. Dies funktioniert auf allen modernen macOS-Versionen (APFS/HFS+).

## Was der „Gesamte Speicherverbrauch" umfasst

Der gemeldete Speicherverbrauch jeder ungenutzten App beinhaltet:

1. **Das `.app`-Bundle selbst** — das Anwendungspaket in `/Applications` oder `~/Applications`
2. **Zugehörige `~/Library/`-Verzeichnisse** — von der App gespeicherte Daten an folgenden Orten:

| Ort | Übereinstimmung nach |
|-----|----------------------|
| `~/Library/Application Support/<id or name>/` | Bundle-ID oder App-Name |
| `~/Library/Caches/<id>/` | Bundle-ID |
| `~/Library/Containers/<id>/` | Bundle-ID |
| `~/Library/Group Containers/*<id>*/` | Bundle-ID (Glob) |
| `~/Library/Preferences/<id>.plist` | Bundle-ID |
| `~/Library/Preferences/ByHost/<id>.*.plist` | Bundle-ID (Glob) |
| `~/Library/Saved Application State/<id>.savedState/` | Bundle-ID |
| `~/Library/HTTPStorages/<id>/` | Bundle-ID |
| `~/Library/WebKit/<id>/` | Bundle-ID |
| `~/Library/Logs/<id or name>/` | Bundle-ID oder App-Name |
| `~/Library/Cookies/<id>.binarycookies` | Bundle-ID |
| `~/Library/LaunchAgents/<id>*.plist` | Bundle-ID (Glob) |

## Standard-Schwellenwert

Anwendungen, die seit **180 Tagen** (ca. 6 Monate) nicht geöffnet wurden, werden als unbenutzt markiert. Kürzlich geöffnete Apps werden aus den Ergebnissen ausgeschlossen.

## Durchsuchte Verzeichnisse

| Verzeichnis | Eingeschlossen |
|-------------|----------------|
| `/Applications` | Ja |
| `/Applications/Utilities` | Ja |
| `~/Applications` | Ja |
| `/System/Applications` | Nein (System-Apps werden nie markiert) |

## Warum Apps in /Applications manuell entfernt werden müssen

Der vorhandene Mechanismus `safety.IsPathBlocked()` blockiert das Löschen von Pfaden außerhalb von `~/`. Da `/Applications` außerhalb des Benutzerverzeichnisses liegt, überspringt das Bereinigungsmodul diese Einträge automatisch. Apps in `~/Applications` befinden sich unter `~` und können normal bereinigt werden.

Um eine App aus `/Applications` zu entfernen, ziehen Sie sie manuell in den Papierkorb oder verwenden Sie:
```bash
sudo rm -rf /Applications/SomeApp.app
```

## CLI-Verwendung

```bash
# Scan for unused apps only
mac-cleaner --unused-apps --dry-run

# Include unused apps in a full scan
mac-cleaner --all --dry-run

# Full scan but skip unused apps
mac-cleaner --all --skip-unused-apps

# JSON output
mac-cleaner --unused-apps --json
```

## Risikostufe

Alle Einträge in der Kategorie `unused-apps` werden als **riskant** eingestuft, da das Entfernen von Anwendungen nicht ohne Weiteres rückgängig gemacht werden kann.
