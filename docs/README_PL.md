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

### Pozostałości aplikacji
- **Osierocone preferencje** — pliki `.plist` w `~/Library/Preferences/` dla odinstalowanych aplikacji (ryzykowne)
- **Kopie zapasowe urządzeń iOS** — `~/Library/Application Support/MobileSync/Backup/` (ryzykowne)
- **Stare pobrania** — pliki w `~/Downloads/` starsze niż 90 dni (umiarkowane)

## Bezpieczeństwo

mac-cleaner został zaprojektowany z myślą o ochronie systemu:

- **Ścieżki chronione przez SIP są blokowane** — `/System`, `/usr`, `/bin`, `/sbin` nie są nigdy modyfikowane (`/usr/local` jest dozwolone)
- **Ochrona swap/VM** — `/private/var/vm` jest zawsze blokowany, aby zapobiec panikom jądra
- **Rozwiązywanie dowiązań symbolicznych** — wszystkie ścieżki są rozwiązywane przed usunięciem
- **Trzy poziomy ryzyka** — każda kategoria jest klasyfikowana jako **bezpieczna**, **umiarkowana** lub **ryzykowna**
- **Ponowna walidacja przed usunięciem** — kontrole bezpieczeństwa są uruchamiane ponownie podczas usuwania, nie tylko podczas skanowania
- **Tryb podglądu** — podgląd wszystkiego przed zatwierdzeniem z `--dry-run`
- **Interaktywne potwierdzenie** — wymagana jawna zgoda użytkownika przed usunięciem (chyba że użyto `--force`)

## Instalacja

### Wymagania

- **Go 1.25+**
- **macOS**

### Budowanie ze źródła

```bash
git clone https://github.com/gregor/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
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

## Flagi CLI

### Kategorie skanowania

| Flaga | Opis |
|-------|------|
| `--all` | Skanuj wszystkie kategorie |
| `--system-caches` | Skanuj pamięć podręczną aplikacji, logi i miniatury QuickLook |
| `--browser-data` | Skanuj pamięci podręczne Safari, Chrome i Firefox |
| `--dev-caches` | Skanuj pamięci podręczne Xcode, npm/yarn, Homebrew i Docker |
| `--app-leftovers` | Skanuj osierocone preferencje, kopie zapasowe iOS i stare pobrania |

### Wyjście i zachowanie

| Flaga | Opis |
|-------|------|
| `--dry-run` | Podgląd co zostałoby usunięte bez usuwania |
| `--json` | Wynik w formacie JSON |
| `--verbose` | Szczegółowa lista plików |
| `--force` | Pomiń monit o potwierdzenie |

### Flagi pomijania kategorii

| Flaga | Opis |
|-------|------|
| `--skip-system-caches` | Pomiń skanowanie pamięci podręcznych systemu |
| `--skip-browser-data` | Pomiń skanowanie danych przeglądarek |
| `--skip-dev-caches` | Pomiń skanowanie pamięci podręcznych deweloperskich |
| `--skip-app-leftovers` | Pomiń skanowanie pozostałości aplikacji |

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

## Licencja

Projekt nie zawiera obecnie pliku licencji.
