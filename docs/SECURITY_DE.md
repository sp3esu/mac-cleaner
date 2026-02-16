# Sicherheitsarchitektur

Dieses Dokument beschreibt die Sicherheitsarchitektur von mac-cleaner, einem CLI-Tool, das Dateien unter macOS scannt und loescht. Angesichts der destruktiven Natur von Dateiloeschungen implementiert das Tool mehrere Verteidigungsschichten, um versehentlichen Datenverlust oder Systemschaeden zu verhindern.

## Bedrohungsmodell

**Was das Tool tut:** Scannt bekannte Cache-, Log- und temporaere Dateispeicherorte unter macOS und loescht diese optional, um Speicherplatz freizugeben.

**Was schiefgehen koennte:**
- Loeschung von Systemdateien, wodurch macOS nicht mehr startfaehig wird
- Loeschung von Benutzerdaten ausserhalb vorgesehener Cache-Verzeichnisse
- Symlink-Angriffe, die Loeschungen auf unbeabsichtigte Ziele umleiten
- Path-Traversal-Angriffe, die vorgesehene Verzeichnisgrenzen ueberschreiten

**Angreifer-Annahmen:** Das Tool laeuft als aktueller Benutzer ohne erhoehte Berechtigungen. Das primaere Risiko sind Fehler in der Pfadkonstruktion oder -validierung, nicht externe Angreifer. Dennoch werden Symlink-basierte Angriffe durch andere Prozesse auf demselben System beruecksichtigt.

## Sicherheitsarchitektur

mac-cleaner verwendet eine mehrschichtige Verteidigungsstrategie. Jede Schicht ist unabhaengig -- ein Fehler in einer Schicht wird von der naechsten abgefangen.

### Schicht 1: Festcodierte Pfadkonstruktion

Alle Scan-Ziele sind in den Scanner-Implementierungen (`pkg/*/scanner.go`) fest codiert. Pfade werden mit `filepath.Join()` ausgehend vom Home-Verzeichnis des Benutzers konstruiert -- niemals aus Benutzereingaben, CLI-Argumenten oder Umgebungsvariablen (mit Ausnahme von `$TMPDIR` fuer QuickLook, das validiert wird).

### Schicht 2: Pfadvalidierung (`internal/safety/`)

Jeder Pfad wird von `safety.IsPathBlocked()` vor jeder Operation validiert. Diese Funktion:

1. **Normalisiert** den Pfad mit `filepath.Clean()`, um `..`-Komponenten zu entfernen
2. **Loest Symlinks auf** mit `filepath.EvalSymlinks()`, um den tatsaechlichen Dateisystempfad zu erhalten
3. **Prueft kritische Pfade** -- exakte Uebereinstimmungen mit `/`, `/Users`, `/Library`, `/Applications`, `/private`, `/var`, `/etc`, `/Volumes`, `/opt`, `/cores` werden immer blockiert
4. **Prueft Swap/VM-Pfade** -- `/private/var/vm` und Unterpfade werden immer blockiert, um Kernel Panics zu verhindern
5. **Prueft SIP-geschuetzte Pfade** -- `/System`, `/usr`, `/bin`, `/sbin` werden blockiert (mit `/usr/local` als Ausnahme)
6. **Erzwingt Home-Verzeichnis-Eingrenzung** -- alle loeschbaren Pfade muessen sich unter dem Home-Verzeichnis des Benutzers (`~/`) oder unter `/private/var/folders/` (fuer QuickLook-Caches) befinden. Alles andere wird blockiert

### Schicht 3: Erneute Validierung beim Loeschen

`cleanup.Execute()` prueft `safety.IsPathBlocked()` unmittelbar vor dem Aufruf von `os.RemoveAll()` fuer jeden Pfad erneut. Dies faengt Probleme ab, die zwischen Scan-Zeitpunkt und Loeschzeitpunkt auftreten koennten.

### Schicht 4: Benutzerbestaetigung

Vor jeder Loeschung muss der Benutzer die Operation explizit bestaetigen. Dies kann erfolgen durch:
- **Interaktiver Modus** (Standard) -- fuehrt durch jede Kategorie zur Genehmigung
- **Bestaetigungsabfrage** -- explizites Ja/Nein vor der Massenloeschung
- **Vorschau-Modus** (`--dry-run`) -- zeigt eine Vorschau der zu loeschenden Dateien, ohne tatsaechlich zu loeschen
- **Erzwungener Modus** (`--force`) -- umgeht die Bestaetigung (explizites Opt-in)

### Schicht 5: Risikoklassifizierung

Jeder Scan-Kategorie wird eine Risikostufe (`safe`, `moderate` oder `risky`) zugewiesen, die dem Benutzer vor der Bestaetigung angezeigt wird. Dies hilft Benutzern, fundierte Entscheidungen darueber zu treffen, was geloescht werden soll.

## Details zur Pfadvalidierung

### Umgang mit Symlinks

- **Beim Scannen** werden `os.Lstat()` und `filepath.WalkDir()` verwendet, die Symlinks NICHT folgen. Verlinkte Dateien werden nicht in die Groessenberechnung einbezogen.
- **Sicherheitspruefungen** verwenden `filepath.EvalSymlinks()`, um den tatsaechlichen Pfad aufzuloesen, bevor er gegen Blocklisten geprueft wird. Ein Symlink, der von `~/Library/Caches/safe-dir` auf `/System/Library` zeigt, wuerde erkannt und blockiert.
- Wenn die Symlink-Aufloesung fuer einen existierenden Pfad fehlschlaegt (nicht `IsNotExist`), wird der Pfad sicherheitshalber blockiert.

### Pfadgrenzen-Sicherheit

`pathHasPrefix()` prueft, ob ein Pfad gleich einem Praefix ist oder ein ordnungsgemaesses Kind davon (getrennt durch `/`). Dies verhindert falsche Uebereinstimmungen wie `/SystemVolume`, das mit `/System` uebereinstimmt.

### TMPDIR-Validierung

Der QuickLook-Scanner leitet ein Cache-Verzeichnis aus `$TMPDIR` ab. Vor der Verwendung dieses Pfads:
1. Wird validiert, dass `$TMPDIR` den Pfad `/var/folders/` enthaelt (macOS-Konvention)
2. Wird das abgeleitete Cache-Verzeichnis gegen `safety.IsPathBlocked()` geprueft
3. Werden einzelne Eintraege innerhalb des Cache-Verzeichnisses ebenfalls sicherheitsgeprueft

## Externe Befehle

Das Tool fuehrt zwei externe Befehle aus:
- `docker system df` -- zur Abfrage der Docker-Speichernutzung
- `/usr/libexec/PlistBuddy` -- zum Lesen von Bundle-Identifikatoren aus `.plist`-Dateien

Beide verwenden `exec.CommandContext()` mit Argumenten, die als separate Parameter uebergeben werden (nicht ueber eine Shell). Es besteht kein Risiko einer Shell-Injection. Befehlsbinaries werden vor der Ausfuehrung mit `exec.LookPath()` validiert.

## Was wir nicht tun

- **Kein Netzwerkzugriff** -- das Tool stellt niemals Netzwerkanfragen
- **Keine Rechteerhohung** -- kein `sudo`, kein setuid, keine Entitlements
- **Kein Dateischreiben** -- das Tool liest nur (Scannen) und loescht (Bereinigung)
- **Keine Systemaenderungen** -- keine Einstellungsaenderungen, keine Daemon-Verwaltung
- **Keine Benutzereingaben in Pfaden** -- alle Pfade werden aus festcodierten Basen und Dateisystem-Enumeration abgeleitet

## CI-Sicherheitswerkzeuge

Das Projekt verwendet folgende Sicherheitswerkzeuge in der CI:
- **gosec** -- statische Sicherheitsanalyse fuer Go (erkennt Path-Traversal, unbehandelte Fehler, Dateiberechtigungsprobleme)
- **govulncheck** -- Schwachstellen-Scanning von Abhaengigkeiten mit Erreichbarkeitsanalyse
- **Race Detector** -- `go test -race` erkennt Datenrennen in nebenlaeufigen Codepfaden
- **Fuzz-Testing** -- `FuzzIsPathBlocked` entdeckt Grenzfaelle in der Pfadvalidierung

## Schwachstellen melden

Wenn Sie eine Sicherheitsschwachstelle entdecken, melden Sie diese bitte verantwortungsvoll:

1. **Eroeffnen Sie KEIN oeffentliches Issue**
2. Schreiben Sie dem Maintainer eine E-Mail oder nutzen Sie die private Schwachstellenmeldung von GitHub
3. Fuegen Sie eine Beschreibung der Schwachstelle, Schritte zur Reproduktion und moegliche Auswirkungen bei
4. Geben Sie angemessene Zeit fuer eine Behebung, bevor Sie die Schwachstelle oeffentlich machen
