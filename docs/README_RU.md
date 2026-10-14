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

### Кэши проектов
- **node_modules** — каталоги `node_modules/` в проектах JavaScript/TypeScript (безопасно)
- **.build (Swift)** — каталоги `.build/` в проектах Swift Package Manager (безопасно)
- **target (Rust)** — каталоги `target/` в проектах Cargo/Rust (безопасно)
- **venv (Python)** — каталоги `venv/` и `.venv/` в проектах Python (умеренный риск)
- **build (Java/Kotlin)** — каталоги `build/` в проектах Gradle/Maven (безопасно)

Кэши проектов обнаруживаются путём обхода домашнего каталога и валидации каждого кандидата по файлу манифеста (например, `package.json` для node_modules, `Cargo.toml` для target). Проекты, неактивные более 6 месяцев, отмечаются индикатором неактивности.

### Файлы домашнего каталога
- **Большие файлы (>100 МБ)** — файлы больше 100 МБ в домашнем каталоге (рискованно)
- **Старые файлы (>180 дней)** — файлы старше 180 дней и больше 10 МБ в домашнем каталоге (рискованно)

Сканирование домашнего каталога исключает `~/Library`, `~/Applications`, `~/.Trash`, `~/Downloads` и каталоги с точкой (например, `~/.config`). Результаты отмечаются как "рискованные", так как это личные файлы, а не восстановимые кэши.

> **Примечание:** Файлы-заполнители iCloud Drive могут показывать оригинальный размер, даже если они удалены из локального хранилища.

### Очистка Git-репозиториев
- **Git-репозитории** — каталоги `.git/`, найденные в домашнем каталоге (умеренный риск)

Очистка Git-репозиториев обнаруживает каталоги `.git` в вашем домашнем каталоге, показывает их размер и оценку освобождаемого пространства через `git count-objects`, и предлагает очистку `git gc --aggressive`. Для репозиториев с Git LFS (обнаруженных через `.git/lfs`) автоматически запускается `git lfs prune`, если `git-lfs` установлен. Репозитории с активными операциями (наличие `index.lock`) безопасно пропускаются.

> **Примечание:** Очистка запускает `git gc --aggressive` и `git lfs prune` — файлы репозитория не удаляются. Это безопасно для всех репозиториев.

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

### Кэши Фото и медиа
- **Кэш приложения Фото** — `~/Library/Containers/com.apple.Photos/` (безопасно)
- **Кэш анализа Фото** — `~/Library/Containers/com.apple.photoanalysisd/` данные ML-моделей (безопасно)
- **Кэш синхронизации iCloud Фото** — `~/Library/Caches/com.apple.cloudd/` (умеренный риск)
- **Общие фото из Сообщений** — `~/Library/Messages/Attachments/` синхронизированные медиа (рискованно)

### Системные данные
- **Метаданные CoreSpotlight** — `~/Library/Caches/com.apple.Spotlight/` (безопасно)
- **База данных Mail** — `~/Library/Mail/` индекс и данные (рискованно)
- **Кэш вложений Mail** — `~/Library/Mail Downloads/` (умеренный риск)
- **Вложения Сообщений** — `~/Library/Messages/` медиа и вложения (рискованно)
- **Обновления ПО iOS** — `~/Library/iTunes/iPhone Software Updates/` (безопасно)
- **Локальные снимки Time Machine** — метаданные локальных снимков TM (рискованно)
- **Виртуальные машины Parallels** — `~/Parallels/` образы дисков виртуальных машин (рискованно)
- **Виртуальные машины UTM** — `~/Library/Containers/com.utmapp.UTM/` виртуальные машины (рискованно)
- **Виртуальные машины VMware Fusion** — `~/Virtual Machines.localized/` образы дисков (рискованно)

### Неиспользуемые приложения
- **Неиспользуемые приложения** — приложения в `/Applications` и `~/Applications`, не открывавшиеся более 180 дней, с общим объёмом занимаемого пространства включая данные `~/Library/` (рискованно)

Подробности см. в документации [Обнаружение неиспользуемых приложений](unused-apps_RU.md).

## Безопасность

mac-cleaner разработан для защиты вашей системы:

- **SIP-защищённые пути заблокированы** — `/System`, `/usr`, `/bin`, `/sbin` никогда не затрагиваются (`/usr/local` разрешён)
- **Защита swap/VM** — `/private/var/vm` всегда заблокирован для предотвращения паники ядра
- **Разрешение символических ссылок** — все пути разрешаются перед удалением
- **Три уровня риска** — каждая категория классифицируется как **безопасная**, **умеренная** или **рискованная**
- **Повторная валидация перед удалением** — проверки безопасности выполняются снова во время удаления, а не только при сканировании
- **Режим предварительного просмотра** — просмотр всего перед выполнением с `--dry-run`
- **Интерактивное подтверждение** — требуется явное согласие пользователя перед удалением (если не используется `--force`)

Подробный анализ безопасности см. в документе [Архитектура безопасности](SECURITY_RU.md).

## Установка

### Homebrew

```bash
brew install sp3esu/tap/mac-cleaner
```

### Сборка из исходного кода

**Требования:** Go 1.25+, macOS

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Автодополнение в оболочке

Генерация скриптов автодополнения для подстановки флагов и подкоманд по Tab.

**Bash:**
```bash
# Загрузить в текущей сессии:
source <(mac-cleaner completion bash)

# Установить на постоянной основе:
mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner
```

**Zsh:**
```bash
mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"
# Затем перезапустите оболочку или выполните: compinit
```

**Fish:**
```bash
mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish
```

**PowerShell:**
```powershell
mac-cleaner completion powershell | Out-String | Invoke-Expression
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

**Сканировать кэши сборки проектов (node_modules, .build, target, venv, build):**
```bash
./mac-cleaner --project-caches --dry-run
```

**Сканировать большие и старые файлы в домашнем каталоге:**
```bash
./mac-cleaner --home-files --dry-run
```

**Сканировать Git-репозитории на предмет освобождаемого кэша:**
```bash
./mac-cleaner --git-repos --dry-run
```

**Сканировать только каталоги node_modules:**
```bash
./mac-cleaner scan --node-modules --dry-run
```

**Точечное сканирование — только определённые элементы (через подкоманду `scan`):**
```bash
./mac-cleaner scan --npm --safari --dry-run
```

**Точечное сканирование — целая группа плюс отдельные элементы:**
```bash
./mac-cleaner scan --dev-caches --safari
```

**Точечное сканирование — группа за вычетом определённых элементов:**
```bash
./mac-cleaner scan --dev-caches --skip-docker
```

**Структурированная справка для AI-агентов:**
```bash
./mac-cleaner --help-json
```

## Флаги CLI

### Категории сканирования

| Флаг | Описание |
|------|----------|
| `--all` | Сканировать все категории |
| `--system-caches` | Сканировать кэш приложений, логи и миниатюры QuickLook |
| `--browser-data` | Сканировать кэши Safari, Chrome и Firefox |
| `--dev-caches` | Сканировать кэши Xcode, npm/yarn, Homebrew и Docker |
| `--project-caches` | Сканировать кэши сборки проектов (node_modules, .build, target, venv, build) |
| `--home-files` | Сканировать большие и старые файлы в домашнем каталоге |
| `--git-repos` | Сканировать кэш Git-репозиториев |
| `--app-leftovers` | Сканировать осиротевшие настройки, резервные копии iOS и старые загрузки |
| `--creative-caches` | Сканировать кэши Adobe, Sketch и Figma |
| `--messaging-caches` | Сканировать кэши Slack, Discord, Teams и Zoom |
| `--unused-apps` | Сканировать приложения, не открывавшиеся более 180 дней |
| `--photos` | Сканировать кэши приложения Фото и данные анализа медиа |
| `--system-data` | Сканировать Spotlight, Mail, Сообщения, обновления iOS, Time Machine и виртуальные машины |

### Вывод и поведение

| Флаг | Описание |
|------|----------|
| `--dry-run` | Предварительный просмотр без удаления |
| `--json` | Вывод результатов в формате JSON |
| `--verbose` | Подробный список файлов |
| `--force` | Пропустить запрос подтверждения |
| `--help-json` | Вывод структурированной справки в формате JSON для AI-агентов |

### Флаги пропуска категорий

| Флаг | Описание |
|------|----------|
| `--skip-system-caches` | Пропустить сканирование системных кэшей |
| `--skip-browser-data` | Пропустить сканирование данных браузеров |
| `--skip-dev-caches` | Пропустить сканирование кэшей разработчика |
| `--skip-project-caches` | Пропустить сканирование кэшей проектов |
| `--skip-home-files` | Пропустить сканирование файлов домашнего каталога |
| `--skip-git-repos` | Пропустить сканирование Git-репозиториев |
| `--skip-app-leftovers` | Пропустить сканирование остатков приложений |
| `--skip-creative-caches` | Пропустить сканирование кэшей креативных приложений |
| `--skip-messaging-caches` | Пропустить сканирование кэшей мессенджеров |
| `--skip-unused-apps` | Пропустить сканирование неиспользуемых приложений |
| `--skip-photos` | Пропустить сканирование кэшей Фото |
| `--skip-system-data` | Пропустить сканирование системных данных |

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
| `--skip-node-modules` | Пропустить каталоги node_modules |
| `--skip-swift-build` | Пропустить каталоги .build (Swift) |
| `--skip-rust-target` | Пропустить каталоги target (Rust) |
| `--skip-python-venv` | Пропустить каталоги venv (Python) |
| `--skip-java-build` | Пропустить каталоги build (Java/Kotlin) |
| `--skip-large-files` | Пропустить большие файлы в домашнем каталоге |
| `--skip-old-files` | Пропустить старые файлы в домашнем каталоге |
| `--skip-adobe` | Пропустить кэш Adobe |
| `--skip-adobe-media` | Пропустить медиа-кэш Adobe |
| `--skip-sketch` | Пропустить кэш Sketch |
| `--skip-figma` | Пропустить кэш Figma |
| `--skip-slack` | Пропустить кэш Slack |
| `--skip-discord` | Пропустить кэш Discord |
| `--skip-teams` | Пропустить кэш Microsoft Teams |
| `--skip-zoom` | Пропустить кэш Zoom |
| `--skip-photos-caches` | Пропустить кэш приложения Фото |
| `--skip-photos-analysis` | Пропустить кэш анализа Фото |
| `--skip-photos-icloud-cache` | Пропустить кэш синхронизации iCloud Фото |
| `--skip-photos-syndication` | Пропустить общие фото из Сообщений |
| `--skip-spotlight` | Пропустить метаданные CoreSpotlight |
| `--skip-mail` | Пропустить базу данных Mail |
| `--skip-mail-downloads` | Пропустить кэш вложений Mail |
| `--skip-messages` | Пропустить вложения Сообщений |
| `--skip-ios-updates` | Пропустить обновления ПО iOS |
| `--skip-timemachine` | Пропустить локальные снимки Time Machine |
| `--skip-vm-parallels` | Пропустить виртуальные машины Parallels |
| `--skip-vm-utm` | Пропустить виртуальные машины UTM |
| `--skip-vm-vmware` | Пропустить виртуальные машины VMware Fusion |

### Подкоманда scan

Подкоманда `scan` обеспечивает точечное сканирование на уровне отдельных элементов. В отличие от основной команды (которая по умолчанию запускает интерактивный режим), `scan` требует явного указания флагов и поддерживает выбор конкретных элементов.

```bash
# Сканировать только кэши npm и yarn
mac-cleaner scan --npm --yarn --dry-run

# Сканировать все кэши разработчика плюс Safari
mac-cleaner scan --dev-caches --safari

# Сканировать всё, кроме Docker
mac-cleaner scan --all --skip-docker

# Вывод в JSON для автоматизации
mac-cleaner scan --npm --json
```

Выполните `mac-cleaner scan --help` для полного списка флагов точечного сканирования, сгруппированных по категориям.

## Очистка по расписанию

`mac-cleaner schedule` устанавливает пользовательский агент launchd (`~/Library/LaunchAgents/com.mac-cleaner.plist`), который автоматически очищает кэши разработчика — по умолчанию Xcode DerivedData, каждую ночь в 03:00. Агент работает от имени вашего пользователя, поэтому `sudo` не требуется.

```bash
# Очищать Xcode DerivedData ежедневно в 03:00
mac-cleaner schedule install

# Также очищать кэши симулятора iOS, в 22:30 по понедельникам и пятницам
mac-cleaner schedule install --at 22:30 --weekdays mon,fri --simulator-caches

# Предпросмотр без установки
mac-cleaner schedule install --dry-run

mac-cleaner schedule status      # расписание, следующий запуск, последний код выхода, путь к журналу
mac-cleaner schedule run         # запустить задачу сейчас
mac-cleaner schedule uninstall   # удалить агент
```

| Флаг | Описание |
|------|----------|
| `--at HH:MM` | Время запуска в 24-часовом формате (по умолчанию `03:00`) |
| `--weekdays` | Дни через запятую, например `mon,wed,fri` (по умолчанию каждый день) |
| `--targets` | Элементы для очистки, по именам флагов `scan` (по умолчанию `derived-data`) |
| `--simulator-caches` | Также очищать кэши симулятора iOS |
| `--even-if-running` | Очищать, даже если Xcode запущен (по умолчанию запуск пропускается) |

Примечания:
- Если Mac спит в запланированное время, задача выполнится после пробуждения; если он выключен, этот запуск пропускается.
- Вывод дописывается в `~/Library/Logs/mac-cleaner/scheduled.log`.
- Повторный `install` заменяет существующее расписание. При установке через Homebrew используется стабильный путь `/opt/homebrew/bin/mac-cleaner`, поэтому обновления не нарушают работу.

## Лицензия

MIT

## Создано с помощью

Этот проект был создан с использованием [Claude Code](https://claude.com/product/claude-code) и плагина [Get Shit Done](https://github.com/gsd-build/get-shit-done).
