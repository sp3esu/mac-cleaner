# Architektura bezpieczenstwa

Ten dokument opisuje architekture bezpieczenstwa mac-cleaner, narzedzia CLI do skanowania i usuwania plikow w macOS. Ze wzgledu na destrukcyjny charakter usuwania plikow, narzedzie implementuje wiele warstw obrony, aby zapobiec przypadkowej utracie danych lub uszkodzeniu systemu.

## Model zagrozen

**Co robi narzedzie:** Skanuje znane lokalizacje plikow cache, logow i plikow tymczasowych w macOS i opcjonalnie je usuwa, aby odzyskac miejsce na dysku.

**Co moze pojsc nie tak:**
- Usuniecie plikow systemowych, powodujace brak mozliwosci uruchomienia macOS
- Usuniecie danych uzytkownika poza przewidzianymi katalogami cache
- Ataki symlinkami przekierowujace usuwanie na niezamierzone cele
- Traversal sciezek wykraczajacy poza zamierzone granice katalogow

**Zalozenia dotyczace przeciwnika:** Narzedzie dziala jako biezacy uzytkownik bez podwyzsonych uprawnien. Glowne ryzyko stanowia bledy w konstrukcji lub walidacji sciezek, a nie zewnetrzni atakujacy. Niemniej ataki oparte na symlinkach ze strony innych procesow w tym samym systemie sa brane pod uwage.

## Architektura bezpieczenstwa

mac-cleaner stosuje wielowarstwowa strategie obrony. Kazda warstwa jest niezalezna -- awaria jednej warstwy jest wychwytywana przez nastepna.

### Warstwa 1: Zakodowane na stale sciezki

Wszystkie cele skanowania sa zakodowane na stale w implementacjach skanerow (`pkg/*/scanner.go`). Sciezki sa konstruowane za pomoca `filepath.Join()` z katalogu domowego uzytkownika -- nigdy z danych wejsciowych uzytkownika, argumentow CLI ani zmiennych srodowiskowych (z wyjatkiem `$TMPDIR` dla QuickLook, ktory jest walidowany).

### Warstwa 2: Walidacja sciezek (`internal/safety/`)

Kazda sciezka jest walidowana przez `safety.IsPathBlocked()` przed jakakolwiek operacja. Ta funkcja:

1. **Normalizuje** sciezke za pomoca `filepath.Clean()`, usuwajac komponenty `..`
2. **Rozwiazuje symlinki** za pomoca `filepath.EvalSymlinks()`, aby uzyskac rzeczywista sciezke systemu plikow
3. **Sprawdza sciezki krytyczne** -- dokladne dopasowania do `/`, `/Users`, `/Library`, `/Applications`, `/private`, `/var`, `/etc`, `/Volumes`, `/opt`, `/cores` sa zawsze blokowane
4. **Sprawdza sciezki swap/VM** -- `/private/var/vm` i podsciezki sa zawsze blokowane, aby zapobiec panikom jadra
5. **Sprawdza sciezki chronione przez SIP** -- `/System`, `/usr`, `/bin`, `/sbin` sa blokowane (z `/usr/local` jako wyjatkiem)
6. **Wymusza ograniczenie do katalogu domowego** -- wszystkie usuwalne sciezki musza znajdowac sie w katalogu domowym uzytkownika (`~/`) lub w `/private/var/folders/` (dla cache'y QuickLook). Wszystko inne jest blokowane

### Warstwa 3: Ponowna walidacja w momencie usuwania

`cleanup.Execute()` ponownie sprawdza `safety.IsPathBlocked()` bezposrednio przed wywolaniem `os.RemoveAll()` na kazdej sciezce. Wychwytuje to wszelkie problemy, ktore moga powstac miedzy momentem skanowania a momentem usuwania.

### Warstwa 4: Potwierdzenie uzytkownika

Przed jakimkolwiek usunieciem uzytkownik musi jawnie potwierdzic operacje. Mozna to zrobic poprzez:
- **Tryb interaktywny** (domyslny) -- prowadzi przez kazda kategorie do zatwierdzenia
- **Monit o potwierdzenie** -- jawne tak/nie przed masowym usunieciem
- **Tryb podgladu** (`--dry-run`) -- pokazuje podglad tego, co zostaloby usuniete, bez faktycznego usuwania
- **Tryb wymuszony** (`--force`) -- pomija potwierdzenie (jawna zgoda)

### Warstwa 5: Klasyfikacja ryzyka

Kazdej kategorii skanowania przypisywany jest poziom ryzyka (`safe`, `moderate` lub `risky`) wyswietlany uzytkownikowi przed potwierdzeniem. Pomaga to uzytkownikom podejmowac swiadome decyzje o tym, co usunac.

## Szczegoly walidacji sciezek

### Obsluga symlinkow

- **Skanowanie** uzywa `os.Lstat()` i `filepath.WalkDir()`, ktore NIE podazaja za symlinkami. Pliki wskazywane przez symlinki nie sa wliczane do obliczen rozmiaru.
- **Kontrole bezpieczenstwa** uzywaja `filepath.EvalSymlinks()` do rozwiazania rzeczywistej sciezki przed sprawdzeniem jej na listach blokad. Symlink wskazujacy z `~/Library/Caches/safe-dir` na `/System/Library` zostalby wykryty i zablokowany.
- Jesli rozwiazywanie symlinka nie powiedzie sie dla istniejacego pliku (nie `IsNotExist`), sciezka jest blokowana dla bezpieczenstwa.

### Bezpieczenstwo granic sciezek

`pathHasPrefix()` sprawdza, czy sciezka jest rowna prefiksowi lub jest jego wlasciwym dzieckiem (oddzielonym przez `/`). Zapobiega to falszywym dopasowaniom, takim jak `/SystemVolume` pasujacy do `/System`.

### Walidacja TMPDIR

Skaner QuickLook wyprowadza katalog cache z `$TMPDIR`. Przed uzyciem tej sciezki:
1. Sprawdza, czy `$TMPDIR` zawiera `/var/folders/` (konwencja macOS)
2. Weryfikuje wyprowadzony katalog cache wzgledem `safety.IsPathBlocked()`
3. Poszczegolne wpisy w katalogu cache sa rowniez sprawdzane pod katem bezpieczenstwa

## Polecenia zewnetrzne

Narzedzie wykonuje dwa polecenia zewnetrzne:
- `docker system df` -- do zapytania o uzycie dysku przez Docker
- `/usr/libexec/PlistBuddy` -- do odczytu identyfikatorow bundle z plikow `.plist`

Oba uzywaja `exec.CommandContext()` z argumentami przekazywanymi jako oddzielne parametry (nie przez powloke). Nie ma ryzyka wstrzykniecia polewen powloki. Pliki binarne polecen sa walidowane za pomoca `exec.LookPath()` przed wykonaniem.

## Czego nie robimy

- **Brak dostepu do sieci** -- narzedzie nigdy nie wykonuje zapytan sieciowych
- **Brak eskalacji uprawnien** -- brak `sudo`, brak setuid, brak entitlements
- **Brak zapisu plikow** -- narzedzie tylko czyta (skanowanie) i usuwa (czyszczenie)
- **Brak modyfikacji systemu** -- brak zmian preferencji, brak zarzadzania demonami
- **Brak danych wejsciowych uzytkownika w sciezkach** -- wszystkie sciezki sa wyprowadzane z zakodowanych na stale baz i enumeracji systemu plikow

## Narzedzia bezpieczenstwa CI

Projekt uruchamia nastepujace narzedzia bezpieczenstwa w CI:
- **gosec** -- statyczna analiza bezpieczenstwa dla Go (wykrywa traversal sciezek, niesprawdzone bledy, problemy z uprawnieniami plikow)
- **govulncheck** -- skanowanie podatnosci zaleznosci z analiza osiagalnosci
- **Race detector** -- `go test -race` wykrywa wysciigi danych w rownoleglych sciezkach kodu
- **Testy fuzzingowe** -- `FuzzIsPathBlocked` odkrywa przypadki brzegowe w walidacji sciezek

## Zglaszanie podatnosci

Jesli odkryjesz podatnosc bezpieczenstwa, prosimy o odpowiedzialne zgloszenie:

1. **NIE otwieraj publicznego issue**
2. Wyslij e-mail do maintainera lub skorzystaj z funkcji prywatnego zglaszania podatnosci na GitHub
3. Dolacz opis podatnosci, kroki do reprodukcji oraz potencjalny wplyw
4. Daj rozsadna ilosc czasu na poprawke przed publicznym ujawnieniem
