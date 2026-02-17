# mac-cleaner

Un outil CLI rapide et sûr pour récupérer de l'espace disque sous macOS.

[English](../README.md) | [Polski](README_PL.md) | [Deutsch](README_DE.md) | [Українська](README_UA.md) | [Русский](README_RU.md) | Français

## Fonctionnalités

### Caches système
- **Caches des applications** — `~/Library/Caches/` (sûr)
- **Logs utilisateur** — `~/Library/Logs/` (sûr)
- **Miniatures QuickLook** — cache QuickLook de l'utilisateur (sûr)

### Données des navigateurs
- **Cache Safari** — `~/Library/Caches/com.apple.Safari/` (modéré)
- **Cache Chrome** — `~/Library/Caches/Google/Chrome/` pour tous les profils (modéré)
- **Cache Firefox** — `~/Library/Caches/Firefox/` (modéré)

### Caches développeur
- **Xcode DerivedData** — `~/Library/Developer/Xcode/DerivedData/` (risqué)
- **Cache npm** — `~/.npm/` (modéré)
- **Cache Yarn** — `~/Library/Caches/yarn/` (modéré)
- **Cache Homebrew** — `~/Library/Caches/Homebrew/` (modéré)
- **Docker — espace récupérable** — conteneurs, images, cache de build, volumes (risqué)
- **Caches du simulateur iOS** — `~/Library/Developer/CoreSimulator/Caches/` (sûr)
- **Logs du simulateur iOS** — `~/Library/Logs/CoreSimulator/` (sûr)
- **Xcode Device Support** — `~/Library/Developer/Xcode/iOS DeviceSupport/` (modéré)
- **Xcode Archives** — `~/Library/Developer/Xcode/Archives/` (risqué)
- **Store pnpm** — `~/Library/pnpm/store/` (modéré)
- **Cache CocoaPods** — `~/Library/Caches/CocoaPods/` (modéré)
- **Cache Gradle** — `~/.gradle/caches/` (modéré)
- **Cache pip** — `~/Library/Caches/pip/` (sûr)

### Restes d'applications
- **Préférences orphelines** — fichiers `.plist` dans `~/Library/Preferences/` pour les applications désinstallées (risqué)
- **Sauvegardes d'appareils iOS** — `~/Library/Application Support/MobileSync/Backup/` (risqué)
- **Anciens téléchargements** — fichiers dans `~/Downloads/` de plus de 90 jours (modéré)

### Caches des applications créatives
- **Caches Adobe** — `~/Library/Caches/Adobe/` (sûr)
- **Cache média Adobe** — `~/Library/Application Support/Adobe/Common/Media Cache Files/` + `Media Cache/` (modéré)
- **Cache Sketch** — `~/Library/Caches/com.bohemiancoding.sketch3/` (sûr)
- **Cache Figma** — `~/Library/Application Support/Figma/` (sûr)

### Caches des applications de messagerie
- **Cache Slack** — `~/Library/Application Support/Slack/Cache/` + `Service Worker/CacheStorage/` (sûr)
- **Cache Discord** — `~/Library/Application Support/discord/Cache/` + `Code Cache/` (sûr)
- **Cache Microsoft Teams** — `~/Library/Application Support/Microsoft/Teams/Cache/` + `~/Library/Caches/com.microsoft.teams2/` (sûr)
- **Cache Zoom** — `~/Library/Application Support/zoom.us/data/` (sûr)

### Caches Photos et médias
- **Caches de l'application Photos** — caches dans `~/Library/Containers/com.apple.Photos/` (sûr)
- **Caches d'analyse Photos** — données de modèles ML dans `~/Library/Containers/com.apple.photoanalysisd/` (sûr)
- **Cache de synchronisation iCloud Photos** — `~/Library/Caches/com.apple.cloudd/` (modéré)
- **Photos partagées depuis Messages** — médias synchronisés dans `~/Library/Messages/Attachments/` (risqué)

### Données système
- **Métadonnées CoreSpotlight** — `~/Library/Caches/com.apple.Spotlight/` (sûr)
- **Base de données Mail** — index des enveloppes et données dans `~/Library/Mail/` (risqué)
- **Cache des pièces jointes Mail** — `~/Library/Mail Downloads/` (modéré)
- **Pièces jointes Messages** — médias et pièces jointes dans `~/Library/Messages/` (risqué)
- **Mises à jour logicielles iOS** — `~/Library/iTunes/iPhone Software Updates/` (sûr)
- **Instantanés locaux Time Machine** — métadonnées des instantanés TM locaux (risqué)
- **VMs Parallels** — images disque des machines virtuelles dans `~/Parallels/` (risqué)
- **VMs UTM** — machines virtuelles dans `~/Library/Containers/com.utmapp.UTM/` (risqué)
- **VMs VMware Fusion** — images disque dans `~/Virtual Machines.localized/` (risqué)

### Applications inutilisées
- **Applications inutilisées** — applications dans `/Applications` et `~/Applications` non ouvertes depuis plus de 180 jours, avec l'empreinte disque totale incluant les données `~/Library/` (risqué)

Pour plus de détails, voir [Détection des applications inutilisées](unused-apps_FR.md).

## Sécurité

mac-cleaner est conçu pour protéger votre système :

- **Les chemins protégés par SIP sont bloqués** — `/System`, `/usr`, `/bin`, `/sbin` ne sont jamais touchés (`/usr/local` est autorisé)
- **Protection swap/VM** — `/private/var/vm` est toujours bloqué pour éviter les paniques du noyau
- **Résolution des liens symboliques** — tous les chemins sont résolus avant la suppression
- **Trois niveaux de risque** — chaque catégorie est classée comme **sûre**, **modérée** ou **risquée**
- **Revalidation avant suppression** — les vérifications de sécurité sont effectuées à nouveau lors de la suppression, pas seulement lors de l'analyse
- **Mode aperçu** — prévisualiser tout avant d'agir avec `--dry-run`
- **Confirmation interactive** — approbation explicite de l'utilisateur requise avant toute suppression (sauf si `--force` est utilisé)

Pour une analyse de sécurité détaillée, voir [Architecture de sécurité](SECURITY_FR.md).

## Installation

### Homebrew

```bash
brew install sp3esu/tap/mac-cleaner
```

### Compilation depuis les sources

**Prérequis :** Go 1.25+, macOS

```bash
git clone https://github.com/sp3esu/mac-cleaner.git
cd mac-cleaner
go build -o mac-cleaner .
./mac-cleaner --help
```

## Complétion shell

Générez des scripts de complétion shell pour l'auto-complétion des drapeaux et sous-commandes.

**Bash :**
```bash
# Charger dans la session actuelle :
source <(mac-cleaner completion bash)

# Installer de façon permanente :
mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner
```

**Zsh :**
```bash
mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"
# Puis redémarrez votre shell ou exécutez : compinit
```

**Fish :**
```bash
mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish
```

**PowerShell :**
```powershell
mac-cleaner completion powershell | Out-String | Invoke-Expression
```

## Utilisation

**Mode interactif** (par défaut — guide à travers chaque catégorie) :
```bash
./mac-cleaner
```

**Tout analyser, aperçu uniquement :**
```bash
./mac-cleaner --all --dry-run
```

**Nettoyer les caches système sans confirmation :**
```bash
./mac-cleaner --system-caches --force
```

**Tout analyser, sortie JSON :**
```bash
./mac-cleaner --all --json
```

**Tout analyser, mais ignorer Docker et les sauvegardes iOS :**
```bash
./mac-cleaner --all --skip-docker --skip-ios-backups
```

**Analyse ciblée — éléments spécifiques uniquement (via la sous-commande `scan`) :**
```bash
./mac-cleaner scan --npm --safari --dry-run
```

**Analyse ciblée — groupe complet avec éléments individuels :**
```bash
./mac-cleaner scan --dev-caches --safari
```

**Analyse ciblée — groupe sans certains éléments :**
```bash
./mac-cleaner scan --dev-caches --skip-docker
```

**Aide structurée pour les agents IA :**
```bash
./mac-cleaner --help-json
```

## Drapeaux CLI

### Catégories d'analyse

| Drapeau | Description |
|---------|-------------|
| `--all` | Analyser toutes les catégories |
| `--system-caches` | Analyser les caches des applications, les logs et les miniatures QuickLook |
| `--browser-data` | Analyser les caches Safari, Chrome et Firefox |
| `--dev-caches` | Analyser les caches Xcode, npm/yarn, Homebrew et Docker |
| `--app-leftovers` | Analyser les préférences orphelines, les sauvegardes iOS et les anciens téléchargements |
| `--creative-caches` | Analyser les caches Adobe, Sketch et Figma |
| `--messaging-caches` | Analyser les caches Slack, Discord, Teams et Zoom |
| `--unused-apps` | Analyser les applications non ouvertes depuis plus de 180 jours |
| `--photos` | Analyser les caches de l'application Photos et les données d'analyse des médias |
| `--system-data` | Analyser Spotlight, Mail, Messages, les mises à jour iOS, Time Machine et les VMs |

### Sortie et comportement

| Drapeau | Description |
|---------|-------------|
| `--dry-run` | Aperçu des fichiers à supprimer sans suppression |
| `--json` | Sortie des résultats en JSON |
| `--verbose` | Liste détaillée des fichiers |
| `--force` | Ignorer la demande de confirmation |
| `--help-json` | Sortie de l'aide structurée en JSON pour les agents IA |

### Drapeaux d'exclusion de catégories

| Drapeau | Description |
|---------|-------------|
| `--skip-system-caches` | Ignorer l'analyse des caches système |
| `--skip-browser-data` | Ignorer l'analyse des données des navigateurs |
| `--skip-dev-caches` | Ignorer l'analyse des caches développeur |
| `--skip-app-leftovers` | Ignorer l'analyse des restes d'applications |
| `--skip-creative-caches` | Ignorer l'analyse des caches des applications créatives |
| `--skip-messaging-caches` | Ignorer l'analyse des caches des applications de messagerie |
| `--skip-unused-apps` | Ignorer l'analyse des applications inutilisées |
| `--skip-photos` | Ignorer l'analyse des caches Photos |
| `--skip-system-data` | Ignorer l'analyse des données système |

### Drapeaux d'exclusion d'éléments

| Drapeau | Description |
|---------|-------------|
| `--skip-derived-data` | Ignorer Xcode DerivedData |
| `--skip-npm` | Ignorer le cache npm |
| `--skip-yarn` | Ignorer le cache Yarn |
| `--skip-homebrew` | Ignorer le cache Homebrew |
| `--skip-docker` | Ignorer l'espace récupérable Docker |
| `--skip-safari` | Ignorer le cache Safari |
| `--skip-chrome` | Ignorer le cache Chrome |
| `--skip-firefox` | Ignorer le cache Firefox |
| `--skip-quicklook` | Ignorer les miniatures QuickLook |
| `--skip-orphaned-prefs` | Ignorer les préférences orphelines |
| `--skip-ios-backups` | Ignorer les sauvegardes d'appareils iOS |
| `--skip-old-downloads` | Ignorer les anciens téléchargements |
| `--skip-simulator-caches` | Ignorer les caches du simulateur iOS |
| `--skip-simulator-logs` | Ignorer les logs du simulateur iOS |
| `--skip-xcode-device-support` | Ignorer les fichiers Xcode Device Support |
| `--skip-xcode-archives` | Ignorer les Xcode Archives |
| `--skip-pnpm` | Ignorer le store pnpm |
| `--skip-cocoapods` | Ignorer le cache CocoaPods |
| `--skip-gradle` | Ignorer le cache Gradle |
| `--skip-pip` | Ignorer le cache pip |
| `--skip-adobe` | Ignorer les caches Adobe |
| `--skip-adobe-media` | Ignorer le cache média Adobe |
| `--skip-sketch` | Ignorer le cache Sketch |
| `--skip-figma` | Ignorer le cache Figma |
| `--skip-slack` | Ignorer le cache Slack |
| `--skip-discord` | Ignorer le cache Discord |
| `--skip-teams` | Ignorer le cache Microsoft Teams |
| `--skip-zoom` | Ignorer le cache Zoom |
| `--skip-photos-caches` | Ignorer les caches de l'application Photos |
| `--skip-photos-analysis` | Ignorer les caches d'analyse Photos |
| `--skip-photos-icloud-cache` | Ignorer le cache de synchronisation iCloud Photos |
| `--skip-photos-syndication` | Ignorer les photos partagées depuis Messages |
| `--skip-spotlight` | Ignorer les métadonnées CoreSpotlight |
| `--skip-mail` | Ignorer la base de données Mail |
| `--skip-mail-downloads` | Ignorer le cache des pièces jointes Mail |
| `--skip-messages` | Ignorer les pièces jointes Messages |
| `--skip-ios-updates` | Ignorer les mises à jour logicielles iOS |
| `--skip-timemachine` | Ignorer les instantanés locaux Time Machine |
| `--skip-vm-parallels` | Ignorer les VMs Parallels |
| `--skip-vm-utm` | Ignorer les VMs UTM |
| `--skip-vm-vmware` | Ignorer les VMs VMware Fusion |

### Sous-commande scan

La sous-commande `scan` permet une analyse ciblée au niveau des éléments. Contrairement à la commande principale (qui entre en mode interactif par défaut), `scan` nécessite des drapeaux explicites et permet de cibler des éléments individuels.

```bash
# Analyser uniquement les caches npm et yarn
mac-cleaner scan --npm --yarn --dry-run

# Analyser tous les caches développeur plus Safari
mac-cleaner scan --dev-caches --safari

# Tout analyser sauf Docker
mac-cleaner scan --all --skip-docker

# Sortie en JSON pour l'automatisation
mac-cleaner scan --npm --json
```

Exécutez `mac-cleaner scan --help` pour la liste complète des drapeaux ciblés regroupés par catégorie.

## Licence

MIT

## Créé avec

Ce projet a été créé avec [Claude Code](https://claude.com/product/claude-code) et le plugin [Get Shit Done](https://github.com/gsd-build/get-shit-done).
