# mac-cleaner

Szybkie i bezpieczne narzędzie CLI do odzyskiwania miejsca na dysku w macOS.

[English](../README.md) | Polski | [Deutsch](README_DE.md) | [Українська](README_UA.md) | [Русский](README_RU.md) | [Français](README_FR.md)

## Funkcje

### Pamięci podręczne systemu
- **Pamięć podręczna aplikacji** — `~/Library/Caches/` (bezpieczne)
- **Logi użytkownika** — `~/Library/Logs/` (bezpieczne)
- **Miniatury QuickLook** — pamięć podręczna QuickLook użytkownika (bezpieczne)

### Dane przeglądarek
- **Pamięć podręczna Safari** — `~/Library/Caches/com.apple.Safari/` (umiarkowane)
- **Pamięć podręczna Chrome** — `~/Library/Caches/Google/Chrome/` dla wszystkich profili (umiarkowane)
- **Pamięć podręczna Firefox** — `~/Library/Caches/Firefox/` (umiarkowane)

### Pamięci podręczne deweloperskie
- **Xcode DerivedData** — `~/Library/Developer/Xcode/DerivedData/` (ryzykowne)
- **Pamięć podręczna npm** — `~/.npm/` (umiarkowane)
- **Pamięć podręczna Yarn** — `~/Library/Caches/yarn/` (umiarkowane)
- **Pamięć podręczna Homebrew** — `~/Library/Caches/Homebrew/` (umiarkowane)
- **Docker — zasoby do odzyskania** — kontenery, obrazy, pamięć podręczna budowania, wolumeny (ryzykowne)
- **Pamięć podręczna symulatora iOS** — `~/Library/Developer/CoreSimulator/Caches/` (bezpieczne)
- **Logi symulatora iOS** — `~/Library/Logs/CoreSimulator/` (bezpieczne)
- **Xcode Device Support** — `~/Library/Developer/Xcode/iOS DeviceSupport/` (umiarkowane)
- **Xcode Archives** — `~/Library/Developer/Xcode/Archives/` (ryzykowne)
- **Magazyn pnpm** — `~/Library/pnpm/store/` (umiarkowane)
- **Pamięć podręczna CocoaPods** — `~/Library/Caches/CocoaPods/` (umiarkowane)
- **Pamięć podręczna Gradle** — `~/.gradle/caches/` (umiarkowane)
- **Pamięć podręczna pip** — `~/Library/Caches/pip/` (bezpieczne)

### Pozostałości aplikacji
- **Osierocone preferencje** — pliki `.plist` w `~/Library/Preferences/` dla odinstalowanych aplikacji (ryzykowne)
- **Kopie zapasowe urządzeń iOS** — `~/Library/Application Support/MobileSync/Backup/` (ryzykowne)
- **Stare pobrania** — pliki w `~/Downloads/` starsze niż 90 dni (umiarkowane)

### Pamięci podręczne aplikacji kreatywnych
- **Pamięć podręczna Adobe** — `~/Library/Caches/Adobe/` (bezpieczne)
- **Pamięć podręczna multimediów Adobe** — `~/Library/Application Support/Adobe/Common/Media Cache Files/` + `Media Cache/` (umiarkowane)
- **Pamięć podręczna Sketch** — `~/Library/Caches/com.bohemiancoding.sketch3/` (bezpieczne)
- **Pamięć podręczna Figma** — `~/Library/Application Support/Figma/` (bezpieczne)

### Pamięci podręczne komunikatorów
- **Pamięć podręczna Slack** — `~/Library/Application Support/Slack/Cache/` + `Service Worker/CacheStorage/` (bezpieczne)
- **Pamięć podręczna Discord** — `~/Library/Application Support/discord/Cache/` + `Code Cache/` (bezpieczne)
- **Pamięć podręczna Microsoft Teams** — `~/Library/Application Support/Microsoft/Teams/Cache/` + `~/Library/Caches/com.microsoft.teams2/` (bezpieczne)
- **Pamięć podręczna Zoom** — `~/Library/Application Support/zoom.us/data/` (bezpieczne)

### Pamięci podręczne Zdjęć i multimediów
- **Pamięć podręczna aplikacji Zdjęcia** — `~/Library/Containers/com.apple.Photos/` pamięci podręczne (bezpieczne)
- **Pamięć podręczna analizy Zdjęć** — `~/Library/Containers/com.apple.photoanalysisd/` dane modeli ML (bezpieczne)
- **Pamięć podręczna synchronizacji iCloud Zdjęcia** — `~/Library/Caches/com.apple.cloudd/` (umiarkowane)
- **Udostępnione zdjęcia z Wiadomości** — `~/Library/Messages/Attachments/` zsynchronizowane multimedia (ryzykowne)

### Dane systemowe
- **Metadane CoreSpotlight** — `~/Library/Caches/com.apple.Spotlight/` (bezpieczne)
- **Baza danych Mail** — `~/Library/Mail/` indeks kopert i dane (ryzykowne)
- **Pamięć podręczna załączników Mail** — `~/Library/Mail Downloads/` (umiarkowane)
- **Załączniki Wiadomości** — `~/Library/Messages/` multimedia i załączniki (ryzykowne)
- **Aktualizacje oprogramowania iOS** — `~/Library/iTunes/iPhone Software Updates/` (bezpieczne)
- **Lokalne snapshoty Time Machine** — lokalne metadane snapshotów TM (ryzykowne)
- **Maszyny wirtualne Parallels** — `~/Parallels/` obrazy dysków maszyn wirtualnych (ryzykowne)
- **Maszyny wirtualne UTM** — `~/Library/Containers/com.utmapp.UTM/` maszyny wirtualne (ryzykowne)
- **Maszyny wirtualne VMware Fusion** — `~/Virtual Machines.localized/` obrazy dysków (ryzykowne)

### Nieużywane aplikacje
- **Nieużywane aplikacje** — aplikacje w `/Applications` i `~/Applications` nieotwierane od ponad 180 dni, z całkowitym zajmowanym miejscem włącznie z danymi `~/Library/` (ryzykowne)

Szczegóły w dokumentacji [Wykrywanie nieużywanych aplikacji](unused-apps_PL.md).

## Bezpieczeństwo

mac-cleaner został zaprojektowany z myślą o ochronie systemu:

- **Ścieżki chronione przez SIP są blokowane** — `/System`, `/usr`, `/bin`, `/sbin` nie są nigdy modyfikowane (`/usr/local` jest dozwolone)
- **Ochrona swap/VM** — `/private/var/vm` jest zawsze blokowany, aby zapobiec panikom jądra
- **Rozwiązywanie dowiązań symbolicznych** — wszystkie ścieżki są rozwiązywane przed usunięciem
- **Trzy poziomy ryzyka** — każda kategoria jest klasyfikowana jako **bezpieczna**, **umiarkowana** lub **ryzykowna**
- **Ponowna walidacja przed usunięciem** — kontrole bezpieczeństwa są uruchamiane ponownie podczas usuwania, nie tylko podczas skanowania
- **Tryb podglądu** — podgląd wszystkiego przed zatwierdzeniem z `--dry-run`
- **Interaktywne potwierdzenie** — wymagana jawna zgoda użytkownika przed usunięciem (chyba że użyto `--force`)

Szczegółową analizę bezpieczeństwa znajdziesz w dokumencie [Architektura bezpieczeństwa](SECURITY_PL.md).

## Instalacja

### Homebrew

```bash
brew install sp3esu/tap/mac-cleaner
```

### Budowanie ze źródła

**Wymagania:** Go 1.25+, macOS

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Autouzupełnianie powłoki

Generowanie skryptów autouzupełniania dla uzupełniania flag i podkomend klawiszem Tab.

**Bash:**
```bash
# Załaduj w bieżącej sesji:
source <(mac-cleaner completion bash)

# Zainstaluj na stałe:
mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner
```

**Zsh:**
```bash
mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"
# Następnie zrestartuj powłokę lub uruchom: compinit
```

**Fish:**
```bash
mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish
```

**PowerShell:**
```powershell
mac-cleaner completion powershell | Out-String | Invoke-Expression
```

## Użycie

**Tryb interaktywny** (domyślny — prowadzi przez każdą kategorię):
```bash
./mac-cleaner
```

**Skanuj wszystko, tylko podgląd:**
```bash
./mac-cleaner --all --dry-run
```

**Wyczyść pamięci podręczne systemu bez potwierdzenia:**
```bash
./mac-cleaner --system-caches --force
```

**Skanuj wszystko, wyjście JSON:**
```bash
./mac-cleaner --all --json
```

**Skanuj wszystko, ale pomiń Docker i kopie zapasowe iOS:**
```bash
./mac-cleaner --all --skip-docker --skip-ios-backups
```

**Ukierunkowane skanowanie — tylko wybrane elementy (przez podkomendę `scan`):**
```bash
./mac-cleaner scan --npm --safari --dry-run
```

**Ukierunkowane skanowanie — pełna grupa plus pojedyncze elementy:**
```bash
./mac-cleaner scan --dev-caches --safari
```

**Ukierunkowane skanowanie — grupa bez wybranych elementów:**
```bash
./mac-cleaner scan --dev-caches --skip-docker
```

**Strukturalna pomoc dla agentów AI:**
```bash
./mac-cleaner --help-json
```

## Flagi CLI

### Kategorie skanowania

| Flaga | Opis |
|-------|------|
| `--all` | Skanuj wszystkie kategorie |
| `--system-caches` | Skanuj pamięć podręczną aplikacji, logi i miniatury QuickLook |
| `--browser-data` | Skanuj pamięci podręczne Safari, Chrome i Firefox |
| `--dev-caches` | Skanuj pamięci podręczne Xcode, npm/yarn, Homebrew i Docker |
| `--app-leftovers` | Skanuj osierocone preferencje, kopie zapasowe iOS i stare pobrania |
| `--creative-caches` | Skanuj pamięci podręczne Adobe, Sketch i Figma |
| `--messaging-caches` | Skanuj pamięci podręczne Slack, Discord, Teams i Zoom |
| `--unused-apps` | Skanuj aplikacje nieotwierane od ponad 180 dni |
| `--photos` | Skanuj pamięci podręczne aplikacji Zdjęcia i dane analizy multimediów |
| `--system-data` | Skanuj Spotlight, Mail, Wiadomości, aktualizacje iOS, Time Machine i maszyny wirtualne |

### Wyjście i zachowanie

| Flaga | Opis |
|-------|------|
| `--dry-run` | Podgląd co zostałoby usunięte bez usuwania |
| `--json` | Wynik w formacie JSON |
| `--verbose` | Szczegółowa lista plików |
| `--force` | Pomiń monit o potwierdzenie |
| `--help-json` | Wynik strukturalnej pomocy w formacie JSON dla agentów AI |

### Flagi pomijania kategorii

| Flaga | Opis |
|-------|------|
| `--skip-system-caches` | Pomiń skanowanie pamięci podręcznych systemu |
| `--skip-browser-data` | Pomiń skanowanie danych przeglądarek |
| `--skip-dev-caches` | Pomiń skanowanie pamięci podręcznych deweloperskich |
| `--skip-app-leftovers` | Pomiń skanowanie pozostałości aplikacji |
| `--skip-creative-caches` | Pomiń skanowanie pamięci podręcznych aplikacji kreatywnych |
| `--skip-messaging-caches` | Pomiń skanowanie pamięci podręcznych komunikatorów |
| `--skip-unused-apps` | Pomiń skanowanie nieużywanych aplikacji |
| `--skip-photos` | Pomiń skanowanie pamięci podręcznych Zdjęć |
| `--skip-system-data` | Pomiń skanowanie danych systemowych |

### Flagi pomijania elementów

| Flaga | Opis |
|-------|------|
| `--skip-derived-data` | Pomiń Xcode DerivedData |
| `--skip-npm` | Pomiń pamięć podręczną npm |
| `--skip-yarn` | Pomiń pamięć podręczną Yarn |
| `--skip-homebrew` | Pomiń pamięć podręczną Homebrew |
| `--skip-docker` | Pomiń odzyskiwalne zasoby Docker |
| `--skip-safari` | Pomiń pamięć podręczną Safari |
| `--skip-chrome` | Pomiń pamięć podręczną Chrome |
| `--skip-firefox` | Pomiń pamięć podręczną Firefox |
| `--skip-quicklook` | Pomiń miniatury QuickLook |
| `--skip-orphaned-prefs` | Pomiń osierocone preferencje |
| `--skip-ios-backups` | Pomiń kopie zapasowe urządzeń iOS |
| `--skip-old-downloads` | Pomiń stare pobrania |
| `--skip-simulator-caches` | Pomiń pamięć podręczną symulatora iOS |
| `--skip-simulator-logs` | Pomiń logi symulatora iOS |
| `--skip-xcode-device-support` | Pomiń pliki Xcode Device Support |
| `--skip-xcode-archives` | Pomiń Xcode Archives |
| `--skip-pnpm` | Pomiń magazyn pnpm |
| `--skip-cocoapods` | Pomiń pamięć podręczną CocoaPods |
| `--skip-gradle` | Pomiń pamięć podręczną Gradle |
| `--skip-pip` | Pomiń pamięć podręczną pip |
| `--skip-adobe` | Pomiń pamięć podręczną Adobe |
| `--skip-adobe-media` | Pomiń pamięć podręczną multimediów Adobe |
| `--skip-sketch` | Pomiń pamięć podręczną Sketch |
| `--skip-figma` | Pomiń pamięć podręczną Figma |
| `--skip-slack` | Pomiń pamięć podręczną Slack |
| `--skip-discord` | Pomiń pamięć podręczną Discord |
| `--skip-teams` | Pomiń pamięć podręczną Microsoft Teams |
| `--skip-zoom` | Pomiń pamięć podręczną Zoom |
| `--skip-photos-caches` | Pomiń pamięć podręczną aplikacji Zdjęcia |
| `--skip-photos-analysis` | Pomiń pamięć podręczną analizy Zdjęć |
| `--skip-photos-icloud-cache` | Pomiń pamięć podręczną synchronizacji iCloud Zdjęcia |
| `--skip-photos-syndication` | Pomiń udostępnione zdjęcia z Wiadomości |
| `--skip-spotlight` | Pomiń metadane CoreSpotlight |
| `--skip-mail` | Pomiń bazę danych Mail |
| `--skip-mail-downloads` | Pomiń pamięć podręczną załączników Mail |
| `--skip-messages` | Pomiń załączniki Wiadomości |
| `--skip-ios-updates` | Pomiń aktualizacje oprogramowania iOS |
| `--skip-timemachine` | Pomiń lokalne snapshoty Time Machine |
| `--skip-vm-parallels` | Pomiń maszyny wirtualne Parallels |
| `--skip-vm-utm` | Pomiń maszyny wirtualne UTM |
| `--skip-vm-vmware` | Pomiń maszyny wirtualne VMware Fusion |

### Podkomenda scan

Podkomenda `scan` umożliwia ukierunkowane skanowanie na poziomie elementów. W przeciwieństwie do komendy głównej (która domyślnie uruchamia tryb interaktywny), `scan` wymaga jawnych flag i obsługuje celowanie w poszczególne elementy.

```bash
# Skanuj tylko pamięci podręczne npm i yarn
mac-cleaner scan --npm --yarn --dry-run

# Skanuj wszystkie pamięci podręczne deweloperskie plus Safari
mac-cleaner scan --dev-caches --safari

# Skanuj wszystko z wyjątkiem Docker
mac-cleaner scan --all --skip-docker

# Wynik w formacie JSON do automatyzacji
mac-cleaner scan --npm --json
```

Uruchom `mac-cleaner scan --help`, aby zobaczyć pełną listę flag ukierunkowanych pogrupowanych według kategorii.

## Licencja

MIT

## Zbudowano z pomocą

Ten projekt został zbudowany przy użyciu [Claude Code](https://claude.com/product/claude-code) i wtyczki [Get Shit Done](https://github.com/gsd-build/get-shit-done).
