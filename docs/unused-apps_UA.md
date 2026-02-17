# Виявлення невикористовуваних додатків

## Принцип роботи

macOS Spotlight записує `kMDItemLastUsedDate` до кожного `.app`-пакету щоразу, коли користувач відкриває його. mac-cleaner запитує ці метадані для виявлення додатків, що не відкривались протягом налаштованого проміжку часу (за замовчуванням: 180 днів).

```bash
mdls -name kMDItemLastUsedDate -raw /Applications/SomeApp.app
# Повертає: "2024-05-14 09:23:41 +0000" або "(null)", якщо додаток ніколи не відкривався
```

Спеціальні дозволи не потрібні. Працює на всіх сучасних версіях macOS (APFS/HFS+).

## Що входить до "загального обсягу"

Повідомлений розмір кожного невикористовуваного додатку включає:

1. **Сам `.app`-пакет** — пакет програми у `/Applications` або `~/Applications`
2. **Пов'язані директорії `~/Library/`** — дані, що зберігаються додатком у таких розташуваннях:

| Розташування | Відповідність за |
|--------------|-----------------|
| `~/Library/Application Support/<id or name>/` | Bundle ID або назва додатку |
| `~/Library/Caches/<id>/` | Bundle ID |
| `~/Library/Containers/<id>/` | Bundle ID |
| `~/Library/Group Containers/*<id>*/` | Bundle ID (glob) |
| `~/Library/Preferences/<id>.plist` | Bundle ID |
| `~/Library/Preferences/ByHost/<id>.*.plist` | Bundle ID (glob) |
| `~/Library/Saved Application State/<id>.savedState/` | Bundle ID |
| `~/Library/HTTPStorages/<id>/` | Bundle ID |
| `~/Library/WebKit/<id>/` | Bundle ID |
| `~/Library/Logs/<id or name>/` | Bundle ID або назва додатку |
| `~/Library/Cookies/<id>.binarycookies` | Bundle ID |
| `~/Library/LaunchAgents/<id>*.plist` | Bundle ID (glob) |

## Поріг за замовчуванням

Додатки, що не відкривались протягом **180 днів** (приблизно 6 місяців), позначаються як невикористовувані. Додатки, відкриті нещодавно, виключаються з результатів.

## Каталоги, що скануються

| Каталог | Включено |
|---------|----------|
| `/Applications` | Так |
| `/Applications/Utilities` | Так |
| `~/Applications` | Так |
| `/System/Applications` | Ні (системні додатки ніколи не позначаються) |

## Чому додатки з /Applications потребують видалення вручну

Наявний механізм `safety.IsPathBlocked()` блокує видалення шляхів поза межами `~/`. Оскільки `/Applications` знаходиться за межами домашнього каталогу користувача, модуль очищення природно пропускатиме ці записи. Додатки у `~/Applications` розміщені під `~` і можуть бути очищені в звичайному режимі.

Щоб видалити додаток із `/Applications`, перетягніть його до Кошика вручну або скористайтесь командою:
```bash
sudo rm -rf /Applications/SomeApp.app
```

## Використання CLI

```bash
# Сканувати лише невикористовувані додатки
mac-cleaner --unused-apps --dry-run

# Включити невикористовувані додатки до повного сканування
mac-cleaner --all --dry-run

# Повне сканування без невикористовуваних додатків
mac-cleaner --all --skip-unused-apps

# Вивід у форматі JSON
mac-cleaner --unused-apps --json
```

## Рівень ризику

Усі записи в категорії `unused-apps` класифікуються як **ризиковані**, оскільки видалення додатків не є легко оборотною операцією.
