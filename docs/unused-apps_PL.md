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

Ścieżki są pokazywane z wielkością liter taką, jaką mają na dysku (na przykład `Application Support/discord` dla aplikacji o nazwie Discord), a katalog dopasowany zarówno po identyfikatorze pakietu, jak i po nazwie aplikacji jest liczony raz.

## Kiedy aplikacja nie jest zgłaszana

Aplikacja, której data ostatniego użycia jest starsza niż próg lub jej brak, jest mimo to pomijana, gdy którakolwiek z tych kontroli wskazuje, że jest używana:

| Kontrola | Reguła |
|----------|--------|
| Działa | Plik wykonywalny z wnętrza pakietu jest na liście procesów (`ps -axo comm=`, odczytywanej raz na skanowanie) |
| Zadanie launchd | Plik plist w `~/Library/LaunchAgents`, `/Library/LaunchAgents` lub `/Library/LaunchDaemons`, którego `Program`, `ProgramArguments[0]` lub `BundleProgram` wskazuje do wnętrza pakietu albo którego `AssociatedBundleIdentifiers` zawiera identyfikator pakietu aplikacji (odczytywany przez `plutil -convert json`) |
| Świeże dane aplikacji | Coś zmienionego w okresie progu w `Preferences/<id>.plist`, `Preferences/ByHost/<id>.*.plist`, `Group Containers/<id>` lub `Group Containers/<prefiks>.<id>`, `Containers/<id>` i `Containers/<id>/Data`, `Application Support/<id lub nazwa>`, `Caches/<id>`, `Saved Application State`, `HTTPStorages`, `WebKit` lub `Logs`. W katalogach szukany jest najnowszy wpis, do 4 poziomów w głąb i najwyżej 5000 wpisów w każdym, więc liczy się też obraz dysku zapisywany głęboko w starym folderze |
| Niedawno zainstalowana | `kMDItemLastUsedDate` to `(null)`, a `kMDItemDateAdded` mieści się w okresie progu |
| Host rozszerzeń | Pakiet zawiera `Contents/PlugIns/*.appex`, `Contents/Library/QuickLook`, `Contents/Library/Spotlight` lub `Contents/Library/SystemExtensions`. Takie aplikacje działają przez Findera, Szybki przegląd, Safari lub system bez otwierania |
| Aplikacja Apple | Identyfikator pakietu zaczyna się od `com.apple.` |

Jeśli listy procesów nie da się odczytać, w tym skanowaniu nie jest zgłaszana żadna aplikacja, a komunikat wyjaśnia dlaczego. Aplikacja bez daty ostatniego użycia i bez daty dodania jest zgłaszana jako `(no usage history)` tylko wtedy, gdy żadna z powyższych kontroli nie ma zastosowania.

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
