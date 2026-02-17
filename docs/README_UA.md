# mac-cleaner

Швидкий та безпечний CLI-інструмент для звільнення дискового простору в macOS.

[English](../README.md) | [Polski](README_PL.md) | [Deutsch](README_DE.md) | Українська | [Русский](README_RU.md) | [Français](README_FR.md)

## Можливості

### Системні кеші
- **Кеш додатків** — `~/Library/Caches/` (безпечно)
- **Логи користувача** — `~/Library/Logs/` (безпечно)
- **Мініатюри QuickLook** — кеш QuickLook користувача (безпечно)

### Дані браузерів
- **Кеш Safari** — `~/Library/Caches/com.apple.Safari/` (помірний ризик)
- **Кеш Chrome** — `~/Library/Caches/Google/Chrome/` для всіх профілів (помірний ризик)
- **Кеш Firefox** — `~/Library/Caches/Firefox/` (помірний ризик)

### Кеші розробника
- **Xcode DerivedData** — `~/Library/Developer/Xcode/DerivedData/` (ризиковано)
- **Кеш npm** — `~/.npm/` (помірний ризик)
- **Кеш Yarn** — `~/Library/Caches/yarn/` (помірний ризик)
- **Кеш Homebrew** — `~/Library/Caches/Homebrew/` (помірний ризик)
- **Docker — ресурси для відновлення** — контейнери, образи, кеш збірки, томи (ризиковано)
- **Кеш симулятора iOS** — `~/Library/Developer/CoreSimulator/Caches/` (безпечно)
- **Логи симулятора iOS** — `~/Library/Logs/CoreSimulator/` (безпечно)
- **Xcode Device Support** — `~/Library/Developer/Xcode/iOS DeviceSupport/` (помірний ризик)
- **Xcode Archives** — `~/Library/Developer/Xcode/Archives/` (ризиковано)
- **Сховище pnpm** — `~/Library/pnpm/store/` (помірний ризик)
- **Кеш CocoaPods** — `~/Library/Caches/CocoaPods/` (помірний ризик)
- **Кеш Gradle** — `~/.gradle/caches/` (помірний ризик)
- **Кеш pip** — `~/Library/Caches/pip/` (безпечно)

### Залишки додатків
- **Осиротілі налаштування** — файли `.plist` у `~/Library/Preferences/` для видалених додатків (ризиковано)
- **Резервні копії пристроїв iOS** — `~/Library/Application Support/MobileSync/Backup/` (ризиковано)
- **Старі завантаження** — файли у `~/Downloads/` старші за 90 днів (помірний ризик)

### Кеші креативних додатків
- **Кеш Adobe** — `~/Library/Caches/Adobe/` (безпечно)
- **Медіа-кеш Adobe** — `~/Library/Application Support/Adobe/Common/Media Cache Files/` + `Media Cache/` (помірний ризик)
- **Кеш Sketch** — `~/Library/Caches/com.bohemiancoding.sketch3/` (безпечно)
- **Кеш Figma** — `~/Library/Application Support/Figma/` (безпечно)

### Кеші месенджерів
- **Кеш Slack** — `~/Library/Application Support/Slack/Cache/` + `Service Worker/CacheStorage/` (безпечно)
- **Кеш Discord** — `~/Library/Application Support/discord/Cache/` + `Code Cache/` (безпечно)
- **Кеш Microsoft Teams** — `~/Library/Application Support/Microsoft/Teams/Cache/` + `~/Library/Caches/com.microsoft.teams2/` (безпечно)
- **Кеш Zoom** — `~/Library/Application Support/zoom.us/data/` (безпечно)

### Невикористовувані додатки
- **Невикористовувані додатки** — додатки в `/Applications` та `~/Applications`, які не відкривались понад 180 днів, із загальним обсягом включно з даними `~/Library/` (ризиковано)

Деталі див. у документації [Виявлення невикористовуваних додатків](unused-apps_UA.md).

## Безпека

mac-cleaner створений для захисту вашої системи:

- **SIP-захищені шляхи заблоковані** — `/System`, `/usr`, `/bin`, `/sbin` ніколи не зачіпаються (`/usr/local` дозволено)
- **Захист swap/VM** — `/private/var/vm` завжди заблокований для запобігання паніки ядра
- **Розв'язання символічних посилань** — усі шляхи розв'язуються перед видаленням
- **Три рівні ризику** — кожна категорія класифікується як **безпечна**, **помірна** або **ризикована**
- **Повторна валідація перед видаленням** — перевірки безпеки виконуються знову під час видалення, а не лише під час сканування
- **Режим попереднього перегляду** — перегляд усього перед виконанням з `--dry-run`
- **Інтерактивне підтвердження** — потрібна явна згода користувача перед видаленням (якщо не використовується `--force`)

Детальний аналіз безпеки див. у документі [Архітектура безпеки](SECURITY_UA.md).

## Встановлення

### Передумови

- **Go 1.25+**
- **macOS**

### Збірка з вихідного коду

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Використання

**Інтерактивний режим** (за замовчуванням — проводить через кожну категорію):
```bash
./mac-cleaner
```

**Сканувати все, лише попередній перегляд:**
```bash
./mac-cleaner --all --dry-run
```

**Очистити системні кеші без підтвердження:**
```bash
./mac-cleaner --system-caches --force
```

**Сканувати все, вивід у JSON:**
```bash
./mac-cleaner --all --json
```

**Сканувати все, але пропустити Docker та резервні копії iOS:**
```bash
./mac-cleaner --all --skip-docker --skip-ios-backups
```

## Прапорці CLI

### Категорії сканування

| Прапорець | Опис |
|-----------|------|
| `--all` | Сканувати всі категорії |
| `--system-caches` | Сканувати кеш додатків, логи та мініатюри QuickLook |
| `--browser-data` | Сканувати кеші Safari, Chrome та Firefox |
| `--dev-caches` | Сканувати кеші Xcode, npm/yarn, Homebrew та Docker |
| `--app-leftovers` | Сканувати осиротілі налаштування, резервні копії iOS та старі завантаження |
| `--creative-caches` | Сканувати кеші Adobe, Sketch та Figma |
| `--messaging-caches` | Сканувати кеші Slack, Discord, Teams та Zoom |
| `--unused-apps` | Сканувати додатки, які не відкривались понад 180 днів |

### Вивід та поведінка

| Прапорець | Опис |
|-----------|------|
| `--dry-run` | Попередній перегляд без видалення |
| `--json` | Вивід результатів у форматі JSON |
| `--verbose` | Детальний список файлів |
| `--force` | Пропустити запит на підтвердження |

### Прапорці пропуску категорій

| Прапорець | Опис |
|-----------|------|
| `--skip-system-caches` | Пропустити сканування системних кешів |
| `--skip-browser-data` | Пропустити сканування даних браузерів |
| `--skip-dev-caches` | Пропустити сканування кешів розробника |
| `--skip-app-leftovers` | Пропустити сканування залишків додатків |
| `--skip-creative-caches` | Пропустити сканування кешів креативних додатків |
| `--skip-messaging-caches` | Пропустити сканування кешів месенджерів |
| `--skip-unused-apps` | Пропустити сканування невикористовуваних додатків |

### Прапорці пропуску елементів

| Прапорець | Опис |
|-----------|------|
| `--skip-derived-data` | Пропустити Xcode DerivedData |
| `--skip-npm` | Пропустити кеш npm |
| `--skip-yarn` | Пропустити кеш Yarn |
| `--skip-homebrew` | Пропустити кеш Homebrew |
| `--skip-docker` | Пропустити ресурси Docker для відновлення |
| `--skip-safari` | Пропустити кеш Safari |
| `--skip-chrome` | Пропустити кеш Chrome |
| `--skip-firefox` | Пропустити кеш Firefox |
| `--skip-quicklook` | Пропустити мініатюри QuickLook |
| `--skip-orphaned-prefs` | Пропустити осиротілі налаштування |
| `--skip-ios-backups` | Пропустити резервні копії пристроїв iOS |
| `--skip-old-downloads` | Пропустити старі завантаження |
| `--skip-simulator-caches` | Пропустити кеш симулятора iOS |
| `--skip-simulator-logs` | Пропустити логи симулятора iOS |
| `--skip-xcode-device-support` | Пропустити файли Xcode Device Support |
| `--skip-xcode-archives` | Пропустити Xcode Archives |
| `--skip-pnpm` | Пропустити сховище pnpm |
| `--skip-cocoapods` | Пропустити кеш CocoaPods |
| `--skip-gradle` | Пропустити кеш Gradle |
| `--skip-pip` | Пропустити кеш pip |
| `--skip-adobe` | Пропустити кеш Adobe |
| `--skip-adobe-media` | Пропустити медіа-кеш Adobe |
| `--skip-sketch` | Пропустити кеш Sketch |
| `--skip-figma` | Пропустити кеш Figma |
| `--skip-slack` | Пропустити кеш Slack |
| `--skip-discord` | Пропустити кеш Discord |
| `--skip-teams` | Пропустити кеш Microsoft Teams |
| `--skip-zoom` | Пропустити кеш Zoom |

## Ліцензія

Проєкт наразі не містить файлу ліцензії.

## Створено за допомогою

Цей проєкт було створено за допомогою [Claude Code](https://claude.com/product/claude-code) та плагіна [Get Shit Done](https://github.com/gsd-build/get-shit-done).
