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

### Кеші проєктів
- **node_modules** — каталоги `node_modules/` у проєктах JavaScript/TypeScript (безпечно)
- **.build (Swift)** — каталоги `.build/` у проєктах Swift Package Manager (безпечно)
- **target (Rust)** — каталоги `target/` у проєктах Cargo/Rust (безпечно)
- **venv (Python)** — каталоги `venv/` та `.venv/` у проєктах Python (помірний ризик)
- **build (Java/Kotlin)** — каталоги `build/` у проєктах Gradle/Maven (безпечно)

Кеші проєктів виявляються шляхом обходу домашнього каталогу та валідації кожного кандидата за файлом маніфесту (наприклад, `package.json` для node_modules, `Cargo.toml` для target). Проєкти, неактивні понад 6 місяців, позначаються індикатором неактивності.

### Файли домашнього каталогу
- **Великі файли (>100 МБ)** — файли більші за 100 МБ у домашньому каталозі (ризиковано)
- **Старі файли (>180 днів)** — файли старші за 180 днів та більші за 10 МБ у домашньому каталозі (ризиковано)

Сканування домашнього каталогу виключає `~/Library`, `~/Applications`, `~/.Trash`, `~/Downloads` та каталоги з крапкою (наприклад, `~/.config`). Результати позначаються як "ризиковані", оскільки це особисті файли, а не кеші, які можна відновити.

> **Примітка:** Файли-заповнювачі iCloud Drive можуть показувати оригінальний розмір, навіть якщо вони видалені з локального сховища.

### Очищення Git-репозиторіїв
- **Git-репозиторії** — каталоги `.git/`, знайдені у домашньому каталозі (помірний ризик)

Очищення Git-репозиторіїв виявляє каталоги `.git` у вашому домашньому каталозі, показує їх розмір та оцінку простору, який можна повернути, через `git count-objects`, та пропонує очищення `git gc --aggressive`. Для репозиторіїв з Git LFS (виявлених через `.git/lfs`) автоматично запускається `git lfs prune`, якщо `git-lfs` встановлено. Репозиторії з активними операціями (наявність `index.lock`) безпечно пропускаються.

> **Примітка:** Очищення запускає `git gc --aggressive` та `git lfs prune` — файли репозиторію не видаляються. Це безпечно для всіх репозиторіїв.

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

### Кеші Фото та медіа
- **Кеш додатку Фото** — `~/Library/Containers/com.apple.Photos/` кеші (безпечно)
- **Кеш аналізу Фото** — `~/Library/Containers/com.apple.photoanalysisd/` дані ML-моделей (безпечно)
- **Кеш синхронізації iCloud Фото** — `~/Library/Caches/com.apple.cloudd/` (помірний ризик)
- **Спільні фото з Повідомлень** — `~/Library/Messages/Attachments/` синхронізовані медіа (ризиковано)

### Системні дані
- **Метадані CoreSpotlight** — `~/Library/Caches/com.apple.Spotlight/` (безпечно)
- **База даних Mail** — `~/Library/Mail/` індекс та дані (ризиковано)
- **Кеш вкладень Mail** — `~/Library/Mail Downloads/` (помірний ризик)
- **Вкладення Повідомлень** — `~/Library/Messages/` медіа та вкладення (ризиковано)
- **Оновлення ПЗ iOS** — `~/Library/iTunes/iPhone Software Updates/` (безпечно)
- **Локальні знімки Time Machine** — метадані локальних знімків TM (ризиковано)
- **Віртуальні машини Parallels** — `~/Parallels/` образи дисків ВМ (ризиковано)
- **Віртуальні машини UTM** — `~/Library/Containers/com.utmapp.UTM/` віртуальні машини (ризиковано)
- **Віртуальні машини VMware Fusion** — `~/Virtual Machines.localized/` образи дисків (ризиковано)

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

### Homebrew

```bash
brew install sp3esu/tap/mac-cleaner
```

### Збірка з вихідного коду

**Передумови:** Go 1.25+, macOS

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Автодоповнення оболонки

Генерація скриптів автодоповнення для табуляції прапорців та підкоманд.

**Bash:**
```bash
# Завантажити в поточну сесію:
source <(mac-cleaner completion bash)

# Встановити назавжди:
mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner
```

**Zsh:**
```bash
mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"
# Потім перезапустіть оболонку або виконайте: compinit
```

**Fish:**
```bash
mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish
```

**PowerShell:**
```powershell
mac-cleaner completion powershell | Out-String | Invoke-Expression
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

**Сканувати кеші збірки проєктів (node_modules, .build, target, venv, build):**
```bash
./mac-cleaner --project-caches --dry-run
```

**Сканувати великі та старі файли у домашньому каталозі:**
```bash
./mac-cleaner --home-files --dry-run
```

**Сканувати Git-репозиторії на предмет кешу, який можна звільнити:**
```bash
./mac-cleaner --git-repos --dry-run
```

**Сканувати лише каталоги node_modules:**
```bash
./mac-cleaner scan --node-modules --dry-run
```

**Прицільне сканування — лише конкретні елементи (через підкоманду `scan`):**
```bash
./mac-cleaner scan --npm --safari --dry-run
```

**Прицільне сканування — повна група та окремі елементи:**
```bash
./mac-cleaner scan --dev-caches --safari
```

**Прицільне сканування — група за винятком конкретних елементів:**
```bash
./mac-cleaner scan --dev-caches --skip-docker
```

**Структурована довідка для AI-агентів:**
```bash
./mac-cleaner --help-json
```

## Прапорці CLI

### Категорії сканування

| Прапорець | Опис |
|-----------|------|
| `--all` | Сканувати всі категорії |
| `--system-caches` | Сканувати кеш додатків, логи та мініатюри QuickLook |
| `--browser-data` | Сканувати кеші Safari, Chrome та Firefox |
| `--dev-caches` | Сканувати кеші Xcode, npm/yarn, Homebrew та Docker |
| `--project-caches` | Сканувати кеші збірки проєктів (node_modules, .build, target, venv, build) |
| `--home-files` | Сканувати великі та старі файли у домашньому каталозі |
| `--git-repos` | Сканувати кеш Git-репозиторіїв |
| `--app-leftovers` | Сканувати осиротілі налаштування, резервні копії iOS та старі завантаження |
| `--creative-caches` | Сканувати кеші Adobe, Sketch та Figma |
| `--messaging-caches` | Сканувати кеші Slack, Discord, Teams та Zoom |
| `--unused-apps` | Сканувати додатки, які не відкривались понад 180 днів |
| `--photos` | Сканувати кеші додатку Фото та дані аналізу медіа |
| `--system-data` | Сканувати Spotlight, Mail, Повідомлення, оновлення iOS, Time Machine та ВМ |

### Вивід та поведінка

| Прапорець | Опис |
|-----------|------|
| `--dry-run` | Попередній перегляд без видалення |
| `--json` | Вивід результатів у форматі JSON |
| `--verbose` | Детальний список файлів |
| `--force` | Пропустити запит на підтвердження |
| `--help-json` | Вивід структурованої довідки у форматі JSON для AI-агентів |

### Прапорці пропуску категорій

| Прапорець | Опис |
|-----------|------|
| `--skip-system-caches` | Пропустити сканування системних кешів |
| `--skip-browser-data` | Пропустити сканування даних браузерів |
| `--skip-dev-caches` | Пропустити сканування кешів розробника |
| `--skip-project-caches` | Пропустити сканування кешів проєктів |
| `--skip-home-files` | Пропустити сканування файлів домашнього каталогу |
| `--skip-git-repos` | Пропустити сканування Git-репозиторіїв |
| `--skip-app-leftovers` | Пропустити сканування залишків додатків |
| `--skip-creative-caches` | Пропустити сканування кешів креативних додатків |
| `--skip-messaging-caches` | Пропустити сканування кешів месенджерів |
| `--skip-unused-apps` | Пропустити сканування невикористовуваних додатків |
| `--skip-photos` | Пропустити сканування кешів Фото |
| `--skip-system-data` | Пропустити сканування системних даних |

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
| `--skip-node-modules` | Пропустити каталоги node_modules |
| `--skip-swift-build` | Пропустити каталоги .build (Swift) |
| `--skip-rust-target` | Пропустити каталоги target (Rust) |
| `--skip-python-venv` | Пропустити каталоги venv (Python) |
| `--skip-java-build` | Пропустити каталоги build (Java/Kotlin) |
| `--skip-large-files` | Пропустити великі файли у домашньому каталозі |
| `--skip-old-files` | Пропустити старі файли у домашньому каталозі |
| `--skip-adobe` | Пропустити кеш Adobe |
| `--skip-adobe-media` | Пропустити медіа-кеш Adobe |
| `--skip-sketch` | Пропустити кеш Sketch |
| `--skip-figma` | Пропустити кеш Figma |
| `--skip-slack` | Пропустити кеш Slack |
| `--skip-discord` | Пропустити кеш Discord |
| `--skip-teams` | Пропустити кеш Microsoft Teams |
| `--skip-zoom` | Пропустити кеш Zoom |
| `--skip-photos-caches` | Пропустити кеш додатку Фото |
| `--skip-photos-analysis` | Пропустити кеш аналізу Фото |
| `--skip-photos-icloud-cache` | Пропустити кеш синхронізації iCloud Фото |
| `--skip-photos-syndication` | Пропустити спільні фото з Повідомлень |
| `--skip-spotlight` | Пропустити метадані CoreSpotlight |
| `--skip-mail` | Пропустити базу даних Mail |
| `--skip-mail-downloads` | Пропустити кеш вкладень Mail |
| `--skip-messages` | Пропустити вкладення Повідомлень |
| `--skip-ios-updates` | Пропустити оновлення ПЗ iOS |
| `--skip-timemachine` | Пропустити локальні знімки Time Machine |
| `--skip-vm-parallels` | Пропустити віртуальні машини Parallels |
| `--skip-vm-utm` | Пропустити віртуальні машини UTM |
| `--skip-vm-vmware` | Пропустити віртуальні машини VMware Fusion |

### Підкоманда scan

Підкоманда `scan` забезпечує прицільне сканування на рівні окремих елементів. На відміну від кореневої команди (яка за замовчуванням запускає інтерактивний режим), `scan` вимагає явного зазначення прапорців та підтримує вибір окремих елементів.

```bash
# Сканувати лише кеші npm та yarn
mac-cleaner scan --npm --yarn --dry-run

# Сканувати всі кеші розробника плюс Safari
mac-cleaner scan --dev-caches --safari

# Сканувати все, крім Docker
mac-cleaner scan --all --skip-docker

# Вивід у JSON для автоматизації
mac-cleaner scan --npm --json
```

Виконайте `mac-cleaner scan --help`, щоб переглянути повний перелік прапорців, згрупованих за категоріями.

## Ліцензія

MIT

## Створено за допомогою

Цей проєкт було створено за допомогою [Claude Code](https://claude.com/product/claude-code) та плагіна [Get Shit Done](https://github.com/gsd-build/get-shit-done).
