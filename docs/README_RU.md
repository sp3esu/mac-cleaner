# mac-cleaner

Быстрый и безопасный CLI-инструмент для освобождения дискового пространства в macOS.

[English](../README.md) | [Polski](README_PL.md) | [Deutsch](README_DE.md) | [Українська](README_UA.md) | Русский | [Français](README_FR.md)

## Возможности

### Системные кэши
- **Кэш приложений** — `~/Library/Caches/` (безопасно)
- **Логи пользователя** — `~/Library/Logs/` (безопасно)
- **Миниатюры QuickLook** — кэш QuickLook пользователя (безопасно)

### Данные браузеров
- **Кэш Safari** — `~/Library/Caches/com.apple.Safari/` (умеренный риск)
- **Кэш Chrome** — `~/Library/Caches/Google/Chrome/` для всех профилей (умеренный риск)
- **Кэш Firefox** — `~/Library/Caches/Firefox/` (умеренный риск)

### Кэши разработчика
- **Xcode DerivedData** — `~/Library/Developer/Xcode/DerivedData/` (рискованно)
- **Кэш npm** — `~/.npm/` (умеренный риск)
- **Кэш Yarn** — `~/Library/Caches/yarn/` (умеренный риск)
- **Кэш Homebrew** — `~/Library/Caches/Homebrew/` (умеренный риск)
- **Docker — освобождаемые ресурсы** — контейнеры, образы, кэш сборки, тома (рискованно)
- **Кэш симулятора iOS** — `~/Library/Developer/CoreSimulator/Caches/` (безопасно)
- **Логи симулятора iOS** — `~/Library/Logs/CoreSimulator/` (безопасно)
- **Xcode Device Support** — `~/Library/Developer/Xcode/iOS DeviceSupport/` (умеренный риск)
- **Xcode Archives** — `~/Library/Developer/Xcode/Archives/` (рискованно)
- **Хранилище pnpm** — `~/Library/pnpm/store/` (умеренный риск)
- **Кэш CocoaPods** — `~/Library/Caches/CocoaPods/` (умеренный риск)
- **Кэш Gradle** — `~/.gradle/caches/` (умеренный риск)
- **Кэш pip** — `~/Library/Caches/pip/` (безопасно)

### Остатки приложений
- **Осиротевшие настройки** — файлы `.plist` в `~/Library/Preferences/` для удалённых приложений (рискованно)
- **Резервные копии устройств iOS** — `~/Library/Application Support/MobileSync/Backup/` (рискованно)
- **Старые загрузки** — файлы в `~/Downloads/` старше 90 дней (умеренный риск)

### Кэши креативных приложений
- **Кэш Adobe** — `~/Library/Caches/Adobe/` (безопасно)
- **Медиа-кэш Adobe** — `~/Library/Application Support/Adobe/Common/Media Cache Files/` + `Media Cache/` (умеренный риск)
- **Кэш Sketch** — `~/Library/Caches/com.bohemiancoding.sketch3/` (безопасно)
- **Кэш Figma** — `~/Library/Application Support/Figma/` (безопасно)

### Кэши мессенджеров
- **Кэш Slack** — `~/Library/Application Support/Slack/Cache/` + `Service Worker/CacheStorage/` (безопасно)
- **Кэш Discord** — `~/Library/Application Support/discord/Cache/` + `Code Cache/` (безопасно)
- **Кэш Microsoft Teams** — `~/Library/Application Support/Microsoft/Teams/Cache/` + `~/Library/Caches/com.microsoft.teams2/` (безопасно)
- **Кэш Zoom** — `~/Library/Application Support/zoom.us/data/` (безопасно)

## Безопасность

mac-cleaner разработан для защиты вашей системы:

- **SIP-защищённые пути заблокированы** — `/System`, `/usr`, `/bin`, `/sbin` никогда не затрагиваются (`/usr/local` разрешён)
- **Защита swap/VM** — `/private/var/vm` всегда заблокирован для предотвращения паники ядра
- **Разрешение символических ссылок** — все пути разрешаются перед удалением
- **Три уровня риска** — каждая категория классифицируется как **безопасная**, **умеренная** или **рискованная**
- **Повторная валидация перед удалением** — проверки безопасности выполняются снова во время удаления, а не только при сканировании
- **Режим предварительного просмотра** — просмотр всего перед выполнением с `--dry-run`
- **Интерактивное подтверждение** — требуется явное согласие пользователя перед удалением (если не используется `--force`)

## Установка

### Требования

- **Go 1.25+**
- **macOS**

### Сборка из исходного кода

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Использование

**Интерактивный режим** (по умолчанию — проводит по каждой категории):
```bash
./mac-cleaner
```

**Сканировать всё, только предварительный просмотр:**
```bash
./mac-cleaner --all --dry-run
```

**Очистить системные кэши без подтверждения:**
```bash
./mac-cleaner --system-caches --force
```

**Сканировать всё, вывод в JSON:**
```bash
./mac-cleaner --all --json
```

**Сканировать всё, но пропустить Docker и резервные копии iOS:**
```bash
./mac-cleaner --all --skip-docker --skip-ios-backups
```

## Флаги CLI

### Категории сканирования

| Флаг | Описание |
|------|----------|
| `--all` | Сканировать все категории |
| `--system-caches` | Сканировать кэш приложений, логи и миниатюры QuickLook |
| `--browser-data` | Сканировать кэши Safari, Chrome и Firefox |
| `--dev-caches` | Сканировать кэши Xcode, npm/yarn, Homebrew и Docker |
| `--app-leftovers` | Сканировать осиротевшие настройки, резервные копии iOS и старые загрузки |
| `--creative-caches` | Сканировать кэши Adobe, Sketch и Figma |
| `--messaging-caches` | Сканировать кэши Slack, Discord, Teams и Zoom |

### Вывод и поведение

| Флаг | Описание |
|------|----------|
| `--dry-run` | Предварительный просмотр без удаления |
| `--json` | Вывод результатов в формате JSON |
| `--verbose` | Подробный список файлов |
| `--force` | Пропустить запрос подтверждения |

### Флаги пропуска категорий

| Флаг | Описание |
|------|----------|
| `--skip-system-caches` | Пропустить сканирование системных кэшей |
| `--skip-browser-data` | Пропустить сканирование данных браузеров |
| `--skip-dev-caches` | Пропустить сканирование кэшей разработчика |
| `--skip-app-leftovers` | Пропустить сканирование остатков приложений |
| `--skip-creative-caches` | Пропустить сканирование кэшей креативных приложений |
| `--skip-messaging-caches` | Пропустить сканирование кэшей мессенджеров |

### Флаги пропуска элементов

| Флаг | Описание |
|------|----------|
| `--skip-derived-data` | Пропустить Xcode DerivedData |
| `--skip-npm` | Пропустить кэш npm |
| `--skip-yarn` | Пропустить кэш Yarn |
| `--skip-homebrew` | Пропустить кэш Homebrew |
| `--skip-docker` | Пропустить освобождаемые ресурсы Docker |
| `--skip-safari` | Пропустить кэш Safari |
| `--skip-chrome` | Пропустить кэш Chrome |
| `--skip-firefox` | Пропустить кэш Firefox |
| `--skip-quicklook` | Пропустить миниатюры QuickLook |
| `--skip-orphaned-prefs` | Пропустить осиротевшие настройки |
| `--skip-ios-backups` | Пропустить резервные копии устройств iOS |
| `--skip-old-downloads` | Пропустить старые загрузки |
| `--skip-simulator-caches` | Пропустить кэш симулятора iOS |
| `--skip-simulator-logs` | Пропустить логи симулятора iOS |
| `--skip-xcode-device-support` | Пропустить файлы Xcode Device Support |
| `--skip-xcode-archives` | Пропустить Xcode Archives |
| `--skip-pnpm` | Пропустить хранилище pnpm |
| `--skip-cocoapods` | Пропустить кэш CocoaPods |
| `--skip-gradle` | Пропустить кэш Gradle |
| `--skip-pip` | Пропустить кэш pip |
| `--skip-adobe` | Пропустить кэш Adobe |
| `--skip-adobe-media` | Пропустить медиа-кэш Adobe |
| `--skip-sketch` | Пропустить кэш Sketch |
| `--skip-figma` | Пропустить кэш Figma |
| `--skip-slack` | Пропустить кэш Slack |
| `--skip-discord` | Пропустить кэш Discord |
| `--skip-teams` | Пропустить кэш Microsoft Teams |
| `--skip-zoom` | Пропустить кэш Zoom |

## Лицензия

Проект в настоящее время не содержит файла лицензии.

## Создано с помощью

Этот проект был создан с использованием [Claude Code](https://claude.ai/claude-code) и плагина [Get Shit Done](https://github.com/gsd-build/get-shit-done).
