# ADR 0022 — Renommage RepoScan → Lynxor, project-wide

## Statut

Accepté (2026-07-25).

## Contexte

Deuxième renommage de ce projet dans la même série de travail, après RepoAudit → RepoScan (ADR implicite dans `docs/roadmap-long-term.md`, pas d'ADR dédié à l'époque). Contrairement au premier renommage — explicitement chronométré *avant* toute publication ("no GitHub Action published, no npm package, no wide sharing yet") — celui-ci intervient **après publication réelle** : `action.yml` est un GitHub Action utilisable, le tap Homebrew `xchebila/homebrew-reposcan` existe et fonctionne, et `v1.1.0` est la première vraie GitHub Release de ce projet (assets GoReleaser réels, ADR 0020). Cette asymétrie a été signalée explicitement avant de coder.

Séquencement confirmé avec l'utilisateur avant toute exécution :
- Démarrer immédiatement (en parallèle de la validation encore ouverte du premier `npm publish` de l'étape 5), pas après.
- Même discipline que le reste du projet : branche dédiée, ADR, validation réelle avant merge.
- Le premier vrai `npm publish` ne se fera jamais sous le nom `reposcan` — directement sous `lynxor`, une fois ce renommage mergé. L'édition locale de `npm/package.json` (version `1.1.0`, en cours pour préparer manuellement la publication `reposcan`) a été abandonnée et réinitialisée au placeholder `0.0.0`.

## Décision : renommage textuel bloc par bloc — grep d'inventaire d'abord, remplacement ensuite, vérification après

Même mécanique que le premier renommage : `git grep -lI -i "reposcan"` pour inventorier (63 fichiers touchés, 3 renommages de fichier/dossier), remplacement en plusieurs passes (`RepoScan` → `Lynxor`, `reposcan` → `lynxor`, puis une passe `REPOSCAN` → `LYNXOR` découverte nécessaire après coup — une variable d'environnement `REPOSCAN_INSTALL_VERSION` dans `npm/scripts/install.js` et l'ADR 0021 avaient été manqués par la vérification initiale, qui n'avait pas retesté après la création de ces fichiers dans la session), puis re-vérification globale jusqu'à ce que seul le fichier golden (voir plus bas) reste.

Fichiers/dossiers renommés : `.claude/skills/reposcan-finding/` → `lynxor-finding/`, `.github/workflows/reposcan-self-check.yml` → `lynxor-self-check.yml`, `npm/bin/reposcan.js` → `npm/bin/lynxor.js`.

## Décision : le champ `hello_ack` du protocole plugin (`reposcan_version` → `lynxor_version`) est renommé aussi

Même raisonnement que pour le premier renommage : changement cassant en principe pour le protocole de plugins (`docs/plugin-protocol.md`), inoffensif en pratique puisque aucun auteur de plugin externe n'existe à ce jour (seul `docs/examples/reference-plugin.py`, qui ne lit pas ce champ, existe dans ce repo).

## Décision : `output/testdata/report.html.golden` régénéré via `go test ./output/... -update`, jamais édité à la main

Même règle que documentée dans `testing.md` pour ce fichier : le golden file doit refléter un vrai rendu, pas une substitution de texte manuelle qui pourrait diverger silencieusement du template réel (`RepoScan report` → `Lynxor report`, dans le `<title>` et le `<h1>`).

## Découverte en cours de route : le remplacement global a failli corrompre un récit historique

`docs/roadmap-long-term.md` contient un récit daté du premier renommage (RepoAudit → RepoScan), citant un message d'erreur réel (`module declares its path as: github.com/xchebila/repoaudit but was required as: github.com/xchebila/reposcan`) et des noms d'artefacts réels de l'époque (`homebrew-reposcan`, `reposcan.rb`). Le remplacement global aveugle `reposcan` → `lynxor` aurait réécrit ce fait historique en une fausse affirmation ("RepoAudit → Lynxor"), alors que ce renommage-là n'a jamais impliqué le nom Lynxor. Corrigé manuellement : ce paragraphe reste figé sur les noms réels de l'époque (RepoScan, reposcan), et une nouvelle entrée séparée documente ce renommage-ci. Leçon retenue : un remplacement global de texte sur un projet qui documente sa propre histoire au fil de l'eau doit distinguer prose descriptive du produit actuel (renommable sans risque) et récit d'un événement passé nommé explicitement (à laisser tel quel, ou à traiter comme une nouvelle entrée distincte).

Un second stale antérieur, indépendant du renommage Lynxor, a été corrigé au passage : `docs/decisions/0020-goreleaser.md` et `.github/workflows/release.yml` référençaient encore `repoaudit-self-check.yml` — un nom de fichier qui n'existait déjà plus au moment où ADR 0020/PR #30 ont été écrits (le fichier s'appelait déjà `reposcan-self-check.yml` à l'époque). Corrigé pour refléter l'état réel (`lynxor-self-check.yml` maintenant).

## Ce qui n'est PAS fait dans ce commit — suivis séquencés, comme pour le premier renommage

- **Renommage du repo GitHub lui-même** (`xchebila/reposcan` → `xchebila/lynxor`) — action manuelle externe, hors du contrôle de ce commit.
- **Nouveau tag réel sous le module renommé** — `go install github.com/xchebila/lynxor@...` ne peut résoudre aucun tag existant : `v1.0.0`/`v1.0.1`/`v1.0.2`/`v1.1.0` déclarent tous encore `module github.com/xchebila/reposcan` dans leur `go.mod` (contrainte technique identique à celle documentée pour le premier renommage). Les références `go install ...@v1.0.2` restées dans le README ne fonctionneront donc pas tant qu'un nouveau tag n'aura pas été coupé sous le nom Lynxor — décision de version différée à l'utilisateur, comme pour `v1.1.0`.
- **Nouveau tap Homebrew** `homebrew-lynxor` — à créer ; l'ancien `homebrew-reposcan` à traiter séparément (supprimer ou rediriger, à confirmer).
- **Premier `npm publish` réel** — se fera directement sous `lynxor`, jamais sous `reposcan` (décision utilisateur explicite, voir Contexte).
- **Renommage du dépôt npm `reposcan`** — n'existe pas : aucun `npm publish` sous ce nom n'a jamais eu lieu (vérifié : le premier `publish-npm` réel a échoué faute de `NPM_TOKEN`, avant ce renommage). Rien à dépublier.

## Validé, pas juste écrit

- `go build ./...`, `go vet ./...`, `gofmt -l .` (une correction d'alignement nécessaire après le renommage du champ `lynxor_version`, `go fmt`-ée), `go test ./...` : tous verts après le renommage complet du module.
- Binaire réel construit et exécuté : `./lynxor --version`, `./lynxor scan .` (score inchangé, 97/100, même finding Dependabot connu), `./lynxor scan . --format html` et `--format json` (branding "Lynxor" correct dans le titre/`<h1>` HTML, structure JSON inchangée).
- `git grep -lI -i "reposcan"` : plus aucune occurrence en dehors du récit historique volontairement préservé dans `roadmap-long-term.md`.
