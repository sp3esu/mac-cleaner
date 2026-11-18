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

Les chemins sont affichés avec la casse qu'ils ont sur le disque (par exemple `Application Support/discord` pour une application nommée Discord), et un répertoire trouvé à la fois par le bundle ID et par le nom de l'application n'est compté qu'une fois.

## Quand une application n'est pas signalée

Une application dont la date de dernière utilisation est plus ancienne que le seuil, ou absente, est tout de même écartée si l'une de ces vérifications montre qu'elle est utilisée :

| Vérification | Règle |
|--------------|-------|
| En cours d'exécution | Un exécutable situé dans le paquet figure dans la liste des processus (`ps -axo comm=`, lue une fois par analyse) |
| Tâche launchd | Un plist dans `~/Library/LaunchAgents`, `/Library/LaunchAgents` ou `/Library/LaunchDaemons` dont `Program`, `ProgramArguments[0]` ou `BundleProgram` pointe dans le paquet, ou dont `AssociatedBundleIdentifiers` contient le bundle ID de l'application (lu avec `plutil -convert json`) |
| Données récentes | Un élément modifié pendant la période du seuil dans `Preferences/<id>.plist`, `Preferences/ByHost/<id>.*.plist`, `Group Containers/<id>` ou `Group Containers/<préfixe>.<id>`, `Containers/<id>` et `Containers/<id>/Data`, `Application Support/<id or name>`, `Caches/<id>`, `Saved Application State`, `HTTPStorages`, `WebKit` ou `Logs`. Les répertoires sont parcourus pour trouver leur entrée la plus récente, sur 4 niveaux au plus et 5 000 entrées chacun, de sorte qu'une image disque écrite au fond d'un ancien dossier compte |
| Installée récemment | `kMDItemLastUsedDate` vaut `(null)` et `kMDItemDateAdded` est dans la période du seuil |
| Hôte d'extensions | Le paquet contient `Contents/PlugIns/*.appex`, `Contents/Library/QuickLook`, `Contents/Library/Spotlight` ou `Contents/Library/SystemExtensions`. Ces applications fonctionnent via le Finder, Coup d'œil, Safari ou le système sans être ouvertes |
| Application Apple | Le bundle ID commence par `com.apple.` |

Si la liste des processus ne peut pas être lue, aucune application n'est signalée lors de cette analyse et une note en explique la raison. Une application sans date de dernière utilisation ni date d'ajout n'est signalée comme `(no usage history)` que si aucune des vérifications ci-dessus ne s'applique.

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
