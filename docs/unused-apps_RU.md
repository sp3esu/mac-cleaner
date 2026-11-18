# Обнаружение неиспользуемых приложений

## Принцип работы

macOS Spotlight записывает метаданные `kMDItemLastUsedDate` для каждого пакета `.app` при каждом его запуске пользователем. mac-cleaner запрашивает эти метаданные для выявления приложений, которые не открывались в течение настраиваемого периода (по умолчанию: 180 дней).

```bash
mdls -name kMDItemLastUsedDate -raw /Applications/SomeApp.app
# Returns: "2024-05-14 09:23:41 +0000" or "(null)" if never opened
```

Специальные разрешения не требуются. Работает на всех современных версиях macOS (APFS/HFS+).

## Что входит в «общий объём занимаемого пространства»

Указанный размер каждого неиспользуемого приложения включает:

1. **Сам пакет `.app`** — пакет приложения в `/Applications` или `~/Applications`
2. **Связанные директории `~/Library/`** — данные, хранимые приложением в следующих местах:

| Расположение | Совпадение по |
|----------|----------|
| `~/Library/Application Support/<id or name>/` | Bundle ID или имя приложения |
| `~/Library/Caches/<id>/` | Bundle ID |
| `~/Library/Containers/<id>/` | Bundle ID |
| `~/Library/Group Containers/*<id>*/` | Bundle ID (glob) |
| `~/Library/Preferences/<id>.plist` | Bundle ID |
| `~/Library/Preferences/ByHost/<id>.*.plist` | Bundle ID (glob) |
| `~/Library/Saved Application State/<id>.savedState/` | Bundle ID |
| `~/Library/HTTPStorages/<id>/` | Bundle ID |
| `~/Library/WebKit/<id>/` | Bundle ID |
| `~/Library/Logs/<id or name>/` | Bundle ID или имя приложения |
| `~/Library/Cookies/<id>.binarycookies` | Bundle ID |
| `~/Library/LaunchAgents/<id>*.plist` | Bundle ID (glob) |

Пути показываются в том регистре, в котором они записаны на диске (например, `Application Support/discord` для приложения Discord), а директория, найденная и по Bundle ID, и по имени приложения, учитывается один раз.

## Когда приложение не попадает в отчёт

Приложение, дата последнего использования которого старше порога или отсутствует, всё равно исключается, если любая из этих проверок показывает, что оно используется:

| Проверка | Правило |
|----------|---------|
| Запущено | Исполняемый файл внутри пакета есть в списке процессов (`ps -axo comm=`, читается один раз за сканирование) |
| Задание launchd | plist в `~/Library/LaunchAgents`, `/Library/LaunchAgents` или `/Library/LaunchDaemons`, у которого `Program`, `ProgramArguments[0]` или `BundleProgram` указывает внутрь пакета, либо `AssociatedBundleIdentifiers` содержит Bundle ID приложения (читается через `plutil -convert json`) |
| Свежие данные приложения | Что-либо изменённое в пределах порога в `Preferences/<id>.plist`, `Preferences/ByHost/<id>.*.plist`, `Group Containers/<id>` или `Group Containers/<префикс>.<id>`, `Containers/<id>` и `Containers/<id>/Data`, `Application Support/<id or name>`, `Caches/<id>`, `Saved Application State`, `HTTPStorages`, `WebKit` или `Logs`. В директориях ищется самая свежая запись, не глубже 4 уровней и не более 5000 записей в каждой, поэтому учитывается и образ диска, который записывается глубоко внутри старой папки |
| Недавно установлено | `kMDItemLastUsedDate` равно `(null)`, а `kMDItemDateAdded` в пределах порога |
| Хост расширений | Пакет содержит `Contents/PlugIns/*.appex`, `Contents/Library/QuickLook`, `Contents/Library/Spotlight` или `Contents/Library/SystemExtensions`. Такие приложения работают через Finder, Quick Look, Safari или систему без открытия |
| Приложение Apple | Bundle ID начинается с `com.apple.` |

Если список процессов прочитать не удалось, в этом сканировании не сообщается ни одно приложение, а примечание объясняет причину. Приложение без даты последнего использования и без даты добавления помечается как `(no usage history)` только если ни одна из проверок выше не сработала.

## Пороговое значение по умолчанию

Приложения, не открывавшиеся в течение **180 дней** (приблизительно 6 месяцев), помечаются как неиспользуемые. Приложения, открытые позже, не включаются в результаты.

## Сканируемые директории

| Директория | Включена |
|-----------|----------|
| `/Applications` | Да |
| `/Applications/Utilities` | Да |
| `~/Applications` | Да |
| `/System/Applications` | Нет (системные приложения никогда не помечаются) |

## Почему приложения в /Applications требуют удаления вручную

Существующий механизм `safety.IsPathBlocked()` блокирует удаление путей за пределами `~/`. Поскольку `/Applications` находится вне домашней директории пользователя, модуль очистки автоматически пропускает такие записи. Приложения в `~/Applications` находятся внутри `~` и могут быть очищены в обычном режиме.

Чтобы удалить приложение из `/Applications`, перетащите его в Корзину вручную или выполните:
```bash
sudo rm -rf /Applications/SomeApp.app
```

## Использование CLI

```bash
# Scan for unused apps only
mac-cleaner --unused-apps --dry-run

# Include unused apps in a full scan
mac-cleaner --all --dry-run

# Full scan but skip unused apps
mac-cleaner --all --skip-unused-apps

# JSON output
mac-cleaner --unused-apps --json
```

## Уровень риска

Все записи в категории `unused-apps` классифицируются как **рискованные**, поскольку удаление приложений трудно обратимо.
