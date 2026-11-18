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

Pfade werden in der Schreibweise angezeigt, die sie auf der Festplatte haben (zum Beispiel `Application Support/discord` für eine App namens Discord), und ein Verzeichnis, das sowohl über die Bundle-ID als auch über den App-Namen gefunden wird, wird nur einmal gezählt.

## Wann eine App nicht gemeldet wird

Eine App, deren letztes Nutzungsdatum älter als der Schwellenwert ist oder fehlt, wird trotzdem ausgelassen, wenn eine dieser Prüfungen zeigt, dass sie verwendet wird:

| Prüfung | Regel |
|---------|-------|
| Läuft | Eine ausführbare Datei innerhalb des Bundles steht in der Prozessliste (`ps -axo comm=`, einmal pro Scan gelesen) |
| launchd-Job | Eine plist in `~/Library/LaunchAgents`, `/Library/LaunchAgents` oder `/Library/LaunchDaemons`, deren `Program`, `ProgramArguments[0]` oder `BundleProgram` in das Bundle zeigt oder deren `AssociatedBundleIdentifiers` die Bundle-ID der App enthält (gelesen mit `plutil -convert json`) |
| Aktuelle App-Daten | Etwas innerhalb des Schwellenwerts geändert in `Preferences/<id>.plist`, `Preferences/ByHost/<id>.*.plist`, `Group Containers/<id>` oder `Group Containers/<präfix>.<id>`, `Containers/<id>` und `Containers/<id>/Data`, `Application Support/<id or name>`, `Caches/<id>`, `Saved Application State`, `HTTPStorages`, `WebKit` oder `Logs`. Verzeichnisse werden nach ihrem neuesten Eintrag durchsucht, bis zu 4 Ebenen tief und je höchstens 5.000 Einträge, sodass auch ein tief in einem alten Ordner beschriebenes Disk-Image zählt |
| Kürzlich installiert | `kMDItemLastUsedDate` ist `(null)` und `kMDItemDateAdded` liegt innerhalb des Schwellenwerts |
| Erweiterungs-Host | Das Bundle enthält `Contents/PlugIns/*.appex`, `Contents/Library/QuickLook`, `Contents/Library/Spotlight` oder `Contents/Library/SystemExtensions`. Solche Apps arbeiten über Finder, Quick Look, Safari oder das System, ohne geöffnet zu werden |
| Apple-App | Die Bundle-ID beginnt mit `com.apple.` |

Kann die Prozessliste nicht gelesen werden, wird in diesem Scan keine App gemeldet, und ein Hinweis erklärt den Grund. Eine App ohne letztes Nutzungsdatum und ohne Hinzufügedatum wird nur dann als `(no usage history)` gemeldet, wenn keine der obigen Prüfungen zutrifft.

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
