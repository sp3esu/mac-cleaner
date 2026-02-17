# Wykrywanie nieużywanych aplikacji

## Jak to działa

macOS Spotlight zapisuje `kMDItemLastUsedDate` w każdym pakiecie `.app` za każdym razem, gdy użytkownik go otwiera. mac-cleaner odpytuje te metadane, aby zidentyfikować aplikacje nieotwierane przez konfigurowalny okres (domyślnie: 180 dni).

```bash
mdls -name kMDItemLastUsedDate -raw /Applications/SomeApp.app
# Zwraca: "2024-05-14 09:23:41 +0000" lub "(null)" jeśli nigdy nie otwarto
```

Nie są wymagane żadne specjalne uprawnienia. Działa na wszystkich nowoczesnych wersjach macOS (APFS/HFS+).

## Co obejmuje „całkowite zajmowane miejsce"

Raportowany rozmiar każdej nieużywanej aplikacji obejmuje:

1. **Sam pakiet `.app`** — pakiet aplikacji w `/Applications` lub `~/Applications`
2. **Powiązane katalogi `~/Library/`** — dane przechowywane przez aplikację w następujących lokalizacjach:

| Lokalizacja | Dopasowanie według |
|-------------|-------------------|
| `~/Library/Application Support/<id lub nazwa>/` | Identyfikator pakietu lub nazwa aplikacji |
| `~/Library/Caches/<id>/` | Identyfikator pakietu |
| `~/Library/Containers/<id>/` | Identyfikator pakietu |
| `~/Library/Group Containers/*<id>*/` | Identyfikator pakietu (glob) |
| `~/Library/Preferences/<id>.plist` | Identyfikator pakietu |
| `~/Library/Preferences/ByHost/<id>.*.plist` | Identyfikator pakietu (glob) |
| `~/Library/Saved Application State/<id>.savedState/` | Identyfikator pakietu |
| `~/Library/HTTPStorages/<id>/` | Identyfikator pakietu |
| `~/Library/WebKit/<id>/` | Identyfikator pakietu |
| `~/Library/Logs/<id lub nazwa>/` | Identyfikator pakietu lub nazwa aplikacji |
| `~/Library/Cookies/<id>.binarycookies` | Identyfikator pakietu |
| `~/Library/LaunchAgents/<id>*.plist` | Identyfikator pakietu (glob) |

## Domyślny próg

Aplikacje nieotwierane przez **180 dni** (około 6 miesięcy) są oznaczane jako nieużywane. Aplikacje otwierane w tym czasie są wykluczane z wyników.

## Skanowane katalogi

| Katalog | Uwzględniany |
|---------|--------------|
| `/Applications` | Tak |
| `/Applications/Utilities` | Tak |
| `~/Applications` | Tak |
| `/System/Applications` | Nie (aplikacje systemowe nigdy nie są oznaczane) |

## Dlaczego aplikacje w /Applications wymagają ręcznego usunięcia

Istniejący mechanizm `safety.IsPathBlocked()` blokuje usuwanie ścieżek spoza `~/`. Ponieważ `/Applications` znajduje się poza katalogiem domowym użytkownika, moduł czyszczenia naturalnie pomija te pozycje. Aplikacje w `~/Applications` są w obrębie `~` i można je czyścić normalnie.

Aby usunąć aplikację z `/Applications`, przeciągnij ją do Kosza ręcznie lub użyj:
```bash
sudo rm -rf /Applications/SomeApp.app
```

## Użycie CLI

```bash
# Skanuj tylko nieużywane aplikacje
mac-cleaner --unused-apps --dry-run

# Uwzględnij nieużywane aplikacje w pełnym skanowaniu
mac-cleaner --all --dry-run

# Pełne skanowanie z pominięciem nieużywanych aplikacji
mac-cleaner --all --skip-unused-apps

# Wyjście w formacie JSON
mac-cleaner --unused-apps --json
```

## Poziom ryzyka

Wszystkie pozycje w kategorii `unused-apps` są klasyfikowane jako **ryzykowne**, ponieważ usuwanie aplikacji nie jest łatwo odwracalne.
