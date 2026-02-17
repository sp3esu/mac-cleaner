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
