# Architecture de securite

Ce document decrit l'architecture de securite de mac-cleaner, un outil CLI qui analyse et supprime des fichiers sous macOS. Etant donne la nature destructrice de la suppression de fichiers, l'outil implemente plusieurs couches de defense pour prevenir toute perte accidentelle de donnees ou tout dommage au systeme.

## Modele de menaces

**Ce que fait l'outil :** Analyse les emplacements connus de fichiers cache, de logs et de fichiers temporaires sous macOS et les supprime optionnellement pour recuperer de l'espace disque.

**Ce qui pourrait mal tourner :**
- Suppression de fichiers systeme, rendant macOS non demarrable
- Suppression de donnees utilisateur en dehors des repertoires de cache prevus
- Attaques par liens symboliques redirigeant la suppression vers des cibles non prevues
- Traversee de chemin echappant aux limites de repertoires prevues

**Hypotheses sur l'adversaire :** L'outil s'execute en tant qu'utilisateur courant sans privileges eleves. Le risque principal provient de bugs dans la construction ou la validation des chemins, et non d'attaquants externes. Toutefois, les attaques basees sur les liens symboliques provenant d'autres processus sur le meme systeme sont prises en compte.

## Architecture de securite

mac-cleaner utilise une strategie de defense multicouche. Chaque couche est independante -- une defaillance dans une couche est interceptee par la suivante.

### Couche 1 : Construction de chemins en dur

Toutes les cibles d'analyse sont codees en dur dans les implementations des scanners (`pkg/*/scanner.go`). Les chemins sont construits a l'aide de `filepath.Join()` a partir du repertoire personnel de l'utilisateur -- jamais a partir de saisies utilisateur, d'arguments CLI ou de variables d'environnement (sauf `$TMPDIR` pour QuickLook, qui est valide).

### Couche 2 : Validation des chemins (`internal/safety/`)

Chaque chemin est valide par `safety.IsPathBlocked()` avant toute operation. Cette fonction :

1. **Normalise** le chemin avec `filepath.Clean()` pour supprimer les composants `..`
2. **Resout les liens symboliques** avec `filepath.EvalSymlinks()` pour obtenir le chemin reel du systeme de fichiers
3. **Verifie les chemins critiques** -- les correspondances exactes avec `/`, `/Users`, `/Library`, `/Applications`, `/private`, `/var`, `/etc`, `/Volumes`, `/opt`, `/cores` sont toujours bloquees
4. **Verifie les chemins swap/VM** -- `/private/var/vm` et ses sous-repertoires sont toujours bloques pour prevenir les paniques du noyau
5. **Verifie les chemins proteges par SIP** -- `/System`, `/usr`, `/bin`, `/sbin` sont bloques (avec `/usr/local` comme exception)
6. **Impose le confinement au repertoire personnel** -- tous les chemins supprimables doivent se trouver sous le repertoire personnel de l'utilisateur (`~/`) ou sous `/private/var/folders/` (pour les caches QuickLook). Tout le reste est bloque

### Couche 3 : Revalidation au moment de la suppression

`cleanup.Execute()` reverifie `safety.IsPathBlocked()` immediatement avant d'appeler `os.RemoveAll()` sur chaque chemin. Cela intercepte tout probleme qui pourrait survenir entre le moment de l'analyse et le moment de la suppression.

### Couche 4 : Confirmation de l'utilisateur

Avant toute suppression, l'utilisateur doit confirmer explicitement l'operation. Cela peut se faire par :
- **Mode interactif** (par defaut) -- guide a travers chaque categorie pour approbation
- **Invite de confirmation** -- oui/non explicite avant la suppression en masse
- **Mode apercu** (`--dry-run`) -- previsualise ce qui serait supprime sans reellement supprimer
- **Mode force** (`--force`) -- contourne la confirmation (activation explicite)

### Couche 5 : Classification des risques

Chaque categorie d'analyse se voit attribuer un niveau de risque (`safe`, `moderate` ou `risky`) affiche a l'utilisateur avant confirmation. Cela aide les utilisateurs a prendre des decisions eclairees sur ce qu'il convient de supprimer.

## Details de la validation des chemins

### Gestion des liens symboliques

- **L'analyse** utilise `os.Lstat()` et `filepath.WalkDir()`, qui ne suivent PAS les liens symboliques. Les fichiers lies par des liens symboliques ne sont pas comptabilises dans les calculs de taille.
- **Les verifications de securite** utilisent `filepath.EvalSymlinks()` pour resoudre le chemin reel avant de le comparer aux listes de blocage. Un lien symbolique pointant de `~/Library/Caches/safe-dir` vers `/System/Library` serait detecte et bloque.
- Si la resolution d'un lien symbolique echoue pour un chemin existant (pas `IsNotExist`), le chemin est bloque par mesure de securite.

### Securite des limites de chemin

`pathHasPrefix()` verifie qu'un chemin est egal a un prefixe ou est un enfant propre de ce prefixe (separe par `/`). Cela empeche les faux positifs comme `/SystemVolume` correspondant a `/System`.

### Validation de TMPDIR

Le scanner QuickLook derive un repertoire de cache a partir de `$TMPDIR`. Avant d'utiliser ce chemin :
1. Valide que `$TMPDIR` contient `/var/folders/` (convention macOS)
2. Verifie le repertoire de cache derive aupres de `safety.IsPathBlocked()`
3. Les entrees individuelles au sein du repertoire de cache sont egalement verifiees

## Commandes externes

L'outil execute deux commandes externes :
- `docker system df` -- pour interroger l'utilisation disque de Docker
- `/usr/libexec/PlistBuddy` -- pour lire les identifiants de bundle a partir de fichiers `.plist`

Les deux utilisent `exec.CommandContext()` avec des arguments passes en tant que parametres separes (pas via un shell). Il n'y a aucun risque d'injection shell. Les binaires des commandes sont valides avec `exec.LookPath()` avant execution.

## Ce que nous ne faisons pas

- **Aucun acces reseau** -- l'outil n'effectue jamais de requetes reseau
- **Aucune elevation de privileges** -- pas de `sudo`, pas de setuid, pas d'entitlements
- **Aucune ecriture de fichier** -- l'outil ne fait que lire (analyse) et supprimer (nettoyage)
- **Aucune modification du systeme** -- pas de changement de preferences, pas de gestion de daemons
- **Aucune saisie utilisateur dans les chemins** -- tous les chemins sont derives de bases codees en dur et de l'enumeration du systeme de fichiers

## Outils de securite CI

Le projet utilise ces outils de securite en CI :
- **gosec** -- analyse de securite statique pour Go (detecte les traversees de chemin, les erreurs non verifiees, les problemes de permissions de fichiers)
- **govulncheck** -- analyse de vulnerabilites des dependances avec analyse d'accessibilite
- **Race detector** -- `go test -race` detecte les courses de donnees dans les chemins de code concurrents
- **Tests de fuzzing** -- `FuzzIsPathBlocked` decouvre les cas limites dans la validation des chemins

## Signaler des vulnerabilites

Si vous decouvrez une vulnerabilite de securite, veuillez la signaler de maniere responsable :

1. **N'ouvrez PAS d'issue publique**
2. Envoyez un e-mail au mainteneur ou utilisez la fonctionnalite de signalement prive de vulnerabilites de GitHub
3. Incluez une description de la vulnerabilite, les etapes de reproduction et l'impact potentiel
4. Accordez un delai raisonnable pour un correctif avant toute divulgation publique
