# Détection des applications inutilisées

## Fonctionnement

macOS Spotlight enregistre `kMDItemLastUsedDate` sur chaque paquet `.app` chaque fois qu'un utilisateur l'ouvre. mac-cleaner interroge ces métadonnées pour identifier les applications qui n'ont pas été ouvertes depuis une période configurable (par défaut : 180 jours).

```bash
mdls -name kMDItemLastUsedDate -raw /Applications/SomeApp.app
# Returns: "2024-05-14 09:23:41 +0000" or "(null)" if never opened
```

Aucune permission spéciale n'est nécessaire. Cela fonctionne sur toutes les versions modernes de macOS (APFS/HFS+).

## Ce qu'inclut l'« empreinte totale »

La taille rapportée pour chaque application inutilisée comprend :

1. **Le paquet `.app` lui-même** — le paquet d'application dans `/Applications` ou `~/Applications`
2. **Les répertoires `~/Library/` associés** — les données stockées par l'application dans ces emplacements :

| Emplacement | Correspondance par |
|-------------|-------------------|
| `~/Library/Application Support/<id or name>/` | Bundle ID ou nom de l'application |
| `~/Library/Caches/<id>/` | Bundle ID |
| `~/Library/Containers/<id>/` | Bundle ID |
| `~/Library/Group Containers/*<id>*/` | Bundle ID (glob) |
| `~/Library/Preferences/<id>.plist` | Bundle ID |
| `~/Library/Preferences/ByHost/<id>.*.plist` | Bundle ID (glob) |
| `~/Library/Saved Application State/<id>.savedState/` | Bundle ID |
| `~/Library/HTTPStorages/<id>/` | Bundle ID |
| `~/Library/WebKit/<id>/` | Bundle ID |
| `~/Library/Logs/<id or name>/` | Bundle ID ou nom de l'application |
| `~/Library/Cookies/<id>.binarycookies` | Bundle ID |
| `~/Library/LaunchAgents/<id>*.plist` | Bundle ID (glob) |

## Seuil par défaut

Les applications non ouvertes depuis **180 jours** (environ 6 mois) sont signalées comme inutilisées. Les applications ouvertes plus récemment sont exclues des résultats.

## Répertoires analysés

| Répertoire | Inclus |
|------------|--------|
| `/Applications` | Oui |
| `/Applications/Utilities` | Oui |
| `~/Applications` | Oui |
| `/System/Applications` | Non (les applications système ne sont jamais signalées) |

## Pourquoi les applications dans /Applications nécessitent une suppression manuelle

Le mécanisme existant `safety.IsPathBlocked()` bloque la suppression des chemins en dehors de `~/`. Comme `/Applications` se trouve en dehors du répertoire personnel de l'utilisateur, le module de nettoyage ignorera naturellement ces entrées. Les applications dans `~/Applications` se trouvent sous `~` et peuvent être nettoyées normalement.

Pour supprimer une application de `/Applications`, faites-la glisser vers la Corbeille manuellement ou utilisez :
```bash
sudo rm -rf /Applications/SomeApp.app
```

## Utilisation en ligne de commande

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

## Niveau de risque

Toutes les entrées de la catégorie `unused-apps` sont classifiées comme **risquées** car la suppression d'applications n'est pas facilement réversible.
