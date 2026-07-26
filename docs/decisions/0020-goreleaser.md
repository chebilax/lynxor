# ADR 0020 - GoReleaser : pipeline de binaires précompilés, déclenché sur tag

## Statut

Accepté (2026-07-24).

## Contexte

Étape 4 d'une suite séquencée : nécessaire pour l'étape 5 (package npm de distribution), qui a besoin de télécharger un binaire précompilé au moment de l'installation plutôt que de compiler depuis les sources. `action.yml` et la formula Homebrew continuent volontairement de builder depuis les sources (ADR 0011) - décidé explicitement de ne pas les retrofitter dans cette même étape, scope confirmé avant de coder.

## Décision : Linux/macOS/Windows × amd64/arm64, `CGO_ENABLED=0`

6 cibles. `go-git` (la seule dépendance native de ce projet) n'a aucune dépendance cgo - confirmé par un vrai `CGO_ENABLED=0 go build ./...` avant d'écrire la config, pas supposé. Ça permet une cross-compilation propre sans toolchain C par plateforme.

## Décision : `{{.Tag}}` pour le ldflags de version, pas `{{.Version}}`

`{{.Version}}` (le tag GoReleaser sans le préfixe `v`) casserait la cohérence avec tous les autres chemins de build de ce projet, qui injectent tous la forme `vX.Y.Z` (le `git describe` du Makefile, le ref d'`action.yml`, `v#{version}` de la formula Homebrew - voir ADR 0013). `{{.Tag}}` garde le `v`. Vérifié en local (`goreleaser build --snapshot`), pas juste lu dans la doc GoReleaser : le binaire produit rapporte bien `lynxor version v1.0.2`.

## Décision : uniquement des binaires GitHub Release - pas de Docker, pas de packaging Homebrew/deb/rpm

Le tap Homebrew (`chebilax/homebrew-lynxor`) reste maintenu à la main et testé en local avant chaque push (`brew install --build-from-source` + `brew test`) - laisser GoReleaser y publier directement court-circuiterait cette discipline déjà établie (et déjà utile : elle a trouvé un vrai écart de version avant ce jour). Docker et les paquets `.deb`/`.rpm` n'ont pas été demandés - ajoutés uniquement si un vrai besoin se présente, même principe que partout ailleurs dans ce projet.

## Décision : permissions scopées, pas `write-all` - et testé contre le propre analyzer `cicd` du projet

`.github/workflows/release.yml` déclare `permissions: contents: write` (le seul droit dont GoReleaser a besoin pour créer une release et y uploader des assets), pas `write-all` - que l'analyzer `cicd` de ce projet flaguerait lui-même. `lynxor-self-check` reste `contents: read` ; c'est le seul workflow de ce repo à avoir besoin d'un accès en écriture. Confirmé par un vrai self-scan après ajout du fichier : score inchangé (97/100), aucun nouveau finding `cicd.*`.

## Décision : validé par un vrai tag poussé, pas seulement par `goreleaser build --snapshot`

Comme pour l'Action GitHub (ADR 0011) et le tap Homebrew : un YAML "qui compile" ne prouve rien. `goreleaser build --snapshot --clean` et `goreleaser release --snapshot --clean` ont d'abord validé la config en local (6 binaires produits, `--version` correct, archives nommées `lynxor_<os>_<arch>.(tar.gz|zip)`, `checksums.txt` généré) - mais le vrai test est un tag réel poussé, qui déclenche `release.yml` pour de vrai sur GitHub Actions, avec un vrai token et une vraie création de release. Un tag de test (`v0.0.0-goreleaser-test`, supprimé après vérification avec sa release générée) a servi à ça sans consommer un vrai numéro de version avant que cette PR ne soit mergée.

## Conséquences

- Le nommage exact des archives (`lynxor_<os>_<arch>.tar.gz`/`.zip`) est un contrat pour l'étape 5 (le script `postinstall` npm construira cette URL depuis `process.platform`/`process.arch`) - le changer plus tard serait un breaking change pour ce package, pas juste cosmétique.
- `dist/` ajouté au `.gitignore` - sortie locale de `goreleaser build`/`release`, jamais commitée.
- Pas de retrofit d'`action.yml`/la formula Homebrew sur les binaires précompilés dans cette étape - décision confirmée avant de coder, à faire séparément si voulu.
