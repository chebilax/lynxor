# 🌍 Roadmap long terme — Lynxor

**Arrêt délibéré, pas un simple "à suivre"** : depuis le GitHub Action et les intégrations CI, le projet est considéré stable — v1.0 + GitHub Action + intégrations CI est l'état de référence, tant qu'aucun vrai point de friction ou vraie demande ne se présente. Ça vaut pour n'importe quel item de cette liste (marketplace de plugins, extension VSCode, SaaS) ou une idée qui n'y figure même pas encore. Ce n'est pas une pause temporaire en attendant de dérouler chaque item dans l'ordre "quand on aura le temps" — c'est un choix délibéré de ne pas avancer cette liste par défaut, et de ne la reprendre que sur un signal réel, pas sur l'inertie de la roadmap elle-même. La distribution via Homebrew tap, ajoutée après cet arrêt, est exactement ce genre de signal réel — une demande explicite, pas une reprise de la liste par défaut.

Ce document couvre ce qui vient après le v1.0 (Phases 1-5, voir `vision.md`).

**Différence de nature avec vision.md** : les Phases 1-5 avaient chacune un scope technique clair, un critère de sortie mesurable, et un ordre imposé par les dépendances entre elles. Rien ici n'a ce niveau de certitude au départ — ce sont des directions, pas des engagements. Deux items (GitHub Action, CI multi-plateforme) étaient assez mûrs pour être cadrés comme de vraies phases, et sont maintenant faits. Le marketplace de plugins et l'extension VSCode sont en pause, faute de besoin réel identifié pour l'un comme pour l'autre — pas juste "pas encore audité", une distinction volontaire pour ne pas laisser croire qu'un audit de conception est simplement la prochaine étape logique. Le critère de pause ne dépend pas du niveau de risque de la feature (le marketplace est risqué, l'extension VSCode ne l'est pas) : seule l'existence d'un besoin réel compte, sinon la même dérive que les Non-Goals du vision.md existent pour empêcher se reproduit ici, juste plus lentement. Le SaaS reste au niveau "direction + question à trancher avant de commencer".

---

## ✅ Fait — GitHub Action officiel

### Scope MVP

```yaml
- uses: lynxor/action@v1
  with:
    fail-on-new: true    # utilise `lynxor diff` si base-ref/head-ref détectés (PR), sinon `lynxor scan`
```

- Action composite (pas de runtime custom) : installe le binaire `lynxor` déjà existant, l'exécute, expose le code de sortie.
- Sur une pull request : utilise `lynxor diff <base> <head>` (Phase 3, déjà livré) — le mode conçu précisément pour ce cas d'usage.
- Hors PR (push sur une branche) : bascule sur `lynxor scan . --format json`, publié en artifact de build.
- Aucune nouvelle feature côté CLI — ce travail est du packaging pur autour de ce qui existe déjà (`diff`, `--format json`).

### Pourquoi ce n'est pas un audit de conception comme Phase 4

Contrairement au Plugin System, il n'y a pas de code tiers non fiable à isoler ici — l'action exécute le binaire officiel, publié par le mainteneur du projet, dans l'environnement CI de l'utilisateur qui l'a explicitement choisi. Pas de nouvelle surface de risque à trancher avant de coder.

### Critère de sortie — validé

- `fail-on-new: true` fait échouer le check GitHub exactement comme `lynxor diff` en local (même code de sortie, même sémantique) — vérifié par un vrai run CI, pas seulement en local.
- Testé sur ce repo lui-même en conditions CI réelles via `.github/workflows/lynxor-self-check.yml` (les trois chemins : PR/diff, push/scan, `workflow_dispatch`) — deux vrais bugs trouvés et corrigés en cours de route (SHA de merge éphémère, fraîcheur de sum.golang.org). Voir [docs/decisions/0011-github-action.md](decisions/0011-github-action.md).

---

## ✅ Fait — Intégration CI multi-plateforme (GitLab, Jenkins)

Snippets documentés (`docs/ci-integrations.md`), pas un artefact publié (pas de GitLab CI/CD Component, pas de Jenkins Shared Library) — décision et raisons dans [docs/decisions/0012-multi-ci-integrations.md](decisions/0012-multi-ci-integrations.md). Contrairement au GitHub Action, ni le snippet GitLab ni le snippet Jenkins n'ont tourné sur une vraie instance — écart de validation assumé et déclaré, pas cette même garantie de "testé en CI réelle".

---

## ✅ Fait — Distribution via Homebrew tap

**N'apparaissait dans aucun des deux documents de roadmap avant cette entrée** — ni `vision.md` ni ce fichier. Ajouté explicitement ici avant tout code, pour ne pas perdre la décision comme le corpus de test de la Phase 1 a failli l'être.

**Objectif** : installer `lynxor` sans cloner le repo ni avoir Go préinstallé manuellement — Homebrew gère la dépendance de build lui-même. Fonctionne sur Linux et macOS (Homebrew, pas seulement macOS).

**Scope** : repo séparé [xchebila/homebrew-lynxor](https://github.com/xchebila/homebrew-lynxor) (convention Homebrew), un seul fichier `Formula/lynxor.rb`. La formula pointe vers un tarball de tag publié, build avec `go build` (`depends_on "go" => :build`) — pas de binaires précompilés pour ce chemin-là, même quand GoReleaser est arrivé plus tard (voir plus bas) : décision reconfirmée, pas juste un oubli.

**Prérequis découvert avant de coder** : `--version` n'existait pas sur le binaire (vérifié empiriquement : `unknown flag: --version`) — ajouté dans ce même travail plutôt qu'après coup (ADR 0013).

**Écart découvert en cours de route, puis corrigé** : la formula pointait d'abord vers `v1.0.0`, qui précède `--version` — `--version` restait donc vide de sens pour quiconque installait Lynxor via `go install` ou la formula, pas seulement pour le test de la formula elle-même. Corrigé en coupant `v1.0.1` immédiatement après le merge de cette PR, en repointant la formula dessus (`sha256` recalculé, testée en local à nouveau), et en mettant à jour le README (`go install ...@v1.0.1`). Le `test do` de la formula vérifie maintenant réellement `lynxor --version`, plus le repli `scan --help`.

**Validé, pas juste écrit** : `brew tap` + `brew install --build-from-source` + `brew test` exécutés réellement en local avant de pousser la formula — les trois verts. Commande d'installation documentée dans le README principal, à côté de `go install` (qui n'existait pas non plus comme instruction directe utilisateur avant cette même entrée — ajouté au passage).

**Renommage RepoAudit → RepoScan (avant publication, 2026-07-24)** : le tap est passé à `xchebila/homebrew-reposcan`, `Formula/reposcan.rb`, pointant vers `v1.0.2` — le premier tag coupé sous le module renommé (`v1.0.0` et `v1.0.1` déclaraient encore `github.com/xchebila/repoaudit` dans leur `go.mod`, donc incompatibles avec `go install github.com/xchebila/reposcan@...` quel que soit le tag demandé, indépendamment de tout redirect GitHub — vérifié empiriquement : `go install ...@v1.0.1` échouait avec `module declares its path as: github.com/xchebila/repoaudit but was required as: github.com/xchebila/reposcan`). Même triple validation locale refaite sur la nouvelle formula avant de la pousser. L'ancien tap `homebrew-repoaudit` restait à supprimer — bloqué à l'époque : le token `gh` de cet environnement n'avait pas le scope `delete_repo`, suppression faite manuellement depuis.

**Renommage RepoScan → Lynxor** : mêmes mécaniques, même contrainte technique (le tap et le module doivent être re-pointés sous un nouveau tag). Détails : [entrée dédiée plus bas](#-fait--renommage-reposcan--lynxor) et [ADR 0022](decisions/0022-rename-reposcan-to-lynxor.md).

---

## ✅ Fait — GoReleaser (binaires précompilés)

**Objectif** : publier des binaires précompilés (Linux/macOS/Windows, amd64/arm64) comme assets de GitHub Release à chaque tag — nécessaire pour le futur package npm de distribution (postinstall qui télécharge le bon binaire selon `process.platform`/`process.arch`, pas encore fait au moment de cette entrée), sans dépendre d'un `go install` ou d'un build local.

**Scope** : `.goreleaser.yaml` + `.github/workflows/release.yml`, déclenché sur push de tag `v*`. `CGO_ENABLED=0` (aucune dépendance cgo dans ce projet, vérifié par un vrai build avant d'écrire la config). Uniquement des binaires GitHub Release — pas de Docker, pas de publication automatique sur le tap Homebrew, pas de `.deb`/`.rpm` : le tap reste géré à la main (voir entrée précédente), même raisonnement que pour le GitHub Action (ADR 0011).

**Validé, pas juste écrit** : `goreleaser build`/`release --snapshot` en local d'abord, puis un vrai tag jetable (`v0.0.0-goreleaser-test`) poussé pour déclencher `release.yml` pour de vrai sur GitHub Actions — la release générée contenait les 6 archives attendues (`lynxor_<os>_<arch>.tar.gz`/`.zip`) plus `checksums.txt`, avant suppression du tag et de la release de test.

Détails et alternatives écartées : [ADR 0020](decisions/0020-goreleaser.md).

---

## ✅ Fait (partiellement — publication npm non testée en réel) — Package npm de distribution

**Objectif** : `npm install -g lynxor` sans Go installé, en s'appuyant sur les binaires précompilés de l'entrée précédente.

**Scope** : sous-répertoire `npm/` de ce repo (`package.json`, `bin/lynxor.js`, `scripts/install.js`). `postinstall` télécharge l'archive GitHub Release correspondant à `process.platform`/`process.arch`, vérifie son SHA256 contre `checksums.txt`, extrait via le `tar` système (zéro dépendance npm — bsdtar sur Windows depuis 10 1803 gère aussi le zip). Choix explicitement écarté : les sous-paquets `optionalDependencies` par plateforme (pattern esbuild/swc) — plus robuste mais pas ce que le plan demandait, pas rouvert sans besoin identifié. Publication via un job `publish-npm` (`.github/workflows/release.yml`, `needs: release`) qui fixe la version depuis le tag puis `npm publish`.

**Validé, pas juste écrit — avec une limite assumée** : `scripts/install.js` exécuté réellement contre un second tag jetable (`v0.0.0-npm-test`, supprimé après vérification) — téléchargement, vérification de checksum, extraction, et un vrai `reposcan scan .` à travers `bin/reposcan.js` (noms d'époque, avant le renommage Lynxor), tout correct. **Ce qui n'a pas été testé au moment de cette entrée** : le job `publish-npm` lui-même — ni le secret `NPM_TOKEN` (pas encore configuré côté GitHub) ni un vrai `npm publish` n'avaient été exercés.

**Mise à jour** : le premier vrai tag (`v1.1.0`, sous le nom `reposcan`) a confirmé l'échec attendu — `publish-npm` a bien échoué faute de `NPM_TOKEN`, sans effet de bord, pendant que le job `release`/GoReleaser réussissait normalement (6 assets réels en ligne). Le premier `npm publish` réel n'a donc encore jamais eu lieu, ni sous `reposcan` ni sous `lynxor` — il se fera directement sous `lynxor` une fois `NPM_TOKEN` configuré et un nouveau tag coupé sous le module renommé (voir [ADR 0022](decisions/0022-rename-reposcan-to-lynxor.md)).

Détails et alternatives écartées : [ADR 0021](decisions/0021-npm-distribution.md).

---

## ✅ Fait — Renommage RepoScan → Lynxor

**Contexte, différent du premier renommage** : RepoAudit → RepoScan avait été chronométré explicitement *avant* toute publication. Celui-ci arrive après : `action.yml` est un GitHub Action utilisable, le tap `homebrew-reposcan` existe, et `v1.1.0` est la première vraie GitHub Release de ce projet. Signalé explicitement avant de coder ; démarré quand même, sur confirmation, en parallèle de la validation encore ouverte du premier `npm publish`.

**Scope** : mêmes mécaniques que le premier renommage — inventaire par `git grep`, remplacement en plusieurs passes (`RepoScan`→`Lynxor`, `reposcan`→`lynxor`, puis `REPOSCAN`→`LYNXOR` après une vérification initiale incomplète), renommage de 3 fichiers/dossiers (`.claude/skills/`, le self-check workflow, `npm/bin/`), champ de protocole plugin `reposcan_version`→`lynxor_version`, golden file HTML régénéré via `-update`.

**Problème découvert en creusant, corrigé avant de continuer** : le remplacement global aveugle a failli réécrire un récit historique dans `roadmap-long-term.md` (le paragraphe qui documente le premier renommage, avec un vrai message d'erreur cité) — transformé en une fausse affirmation "RepoAudit → Lynxor". Corrigé manuellement ; ce paragraphe reste figé sur les noms réels de l'époque. Un second stale antérieur et indépendant (`repoaudit-self-check.yml` mentionné dans ADR 0020/`release.yml`, déjà faux avant même ce renommage) corrigé au passage.

**Validé, pas juste écrit** : `make check` complet vert après le renommage du module Go ; binaire réel construit et exécuté (`--version`, `scan .` — score inchangé 97/100 —, `--format html`/`--format json`, branding correct).

**Pas fait ici, suivis séparés** : renommage du repo GitHub, nouveau tag réel sous `github.com/xchebila/lynxor` (aucun tag existant ne résout sous ce module — contrainte technique, pas un choix), nouveau tap `homebrew-lynxor`, premier `npm publish` réel (sera directement `lynxor`, jamais `reposcan`).

Détails : [ADR 0022](decisions/0022-rename-reposcan-to-lynxor.md).

---

## ⏸️ En pause — en attente d'un besoin réel

### Marketplace de plugins

**Statut** : en pause, pas "à faire". Pas d'audit de conception lancé, faute de besoin identifié.

**Pourquoi la pause plutôt qu'un audit** : le protocole d'isolation (Phase 4) a été construit pour que le mainteneur (ou un contributeur) écrive ses propres règles — jamais dans un but de découverte/publication par des tiers ("découverte/installation de plugins" a d'ailleurs été explicitement mis hors scope lors de l'audit Phase 4, ADR 0008). Rien depuis n'est venu d'un vrai besoin ("quelqu'un veut publier un plugin") — c'est une direction anticipée, pas une demande. Un marketplace introduit en plus le risque le plus sérieux de toute cette roadmap : une question de confiance sur du code découvert et installé (signature, review, sandboxing du marketplace lui-même), que le protocole d'exécution de Phase 4 ne résout pas du tout — l'isoler protège contre un plugin buggé ou malveillant *une fois lancé*, pas contre la découverte d'un plugin déjà compromis. Construire cette surface de risque sans utilisateur en face serait aller à l'encontre du principe déjà appliqué partout ailleurs dans ce projet : ne pas construire avant que ce soit nécessaire (cf. Non-Goals, vision.md).

**Condition de sortie de pause** : un vrai signal d'usage — quelqu'un qui veut publier un plugin, ou un cas d'usage concret remonté. Le jour où ce signal existe, repasser par le même format que l'audit Phase 4 : questions posées explicitement, réponses vérifiées empiriquement quand possible, décision actée dans un ADR avant la première ligne de code.

---

### Extension VSCode

**Statut** : en pause, pas "à faire". Pas commencé, faute de besoin identifié.

**Pourquoi la pause, alors que le risque technique est faible ici** : contrairement au marketplace, il n'y a pas de code tiers ni de question de confiance — un wrapper léger autour du binaire CLI existant serait cohérent avec "core minimal, ne pas dupliquer" et peu risqué techniquement. Mais le critère de pause ne porte pas sur le niveau de risque d'une feature, seulement sur l'existence d'un vrai besoin — sinon on reproduit exactement la dérive que la section Non-Goals du vision.md existe pour empêcher : construire une feature "raisonnable" prise isolément, sans jamais se demander si quelqu'un l'a demandée. Rien à ce jour n'indique un vrai besoin d'intégration IDE — c'est une direction rédigée dans ce document, pas une demande reçue.

**Question qui resterait à trancher, le jour où un besoin apparaît** : wrapper léger autour du binaire CLI existant (comme le packaging du GitHub Action), ou une vraie réimplémentation de logique côté extension (TypeScript, API VSCode — une compétence différente du reste du projet, 100% Go jusqu'ici) ? La première option reste la plus cohérente avec le principe déjà appliqué partout ailleurs dans le projet.

---

## 🧭 Directions à auditer avant de coder

### SaaS optionnel

**Statut** : à requestionner avant même de commencer à cadrer quoi que ce soit.

**La vraie question, pas encore posée** : quel problème un SaaS résout-il que CLI + GitHub Action ne résolvent pas déjà ? Le vision.md le qualifie lui-même de "non obligatoire" — ce n'est pas un engagement, c'est une option qui n'a pas encore de justification claire.

**Tension avec la philosophie du projet** : un SaaS introduit un hébergement, potentiellement des comptes utilisateurs, une surface d'attaque et une charge opérationnelle sans rapport avec "CLI local, zéro-config, zéro dépendance" qui est au cœur de Lynxor depuis le vision.md original. Avant tout cadrage technique, ce point mérite une vraie réponse écrite (dans un ADR ou équivalent) à la question "pourquoi", pas seulement "comment".

---

## Ordre recommandé

1. ✅ **GitHub Action** — fait
2. ✅ **CI multi-plateforme** — fait
3. ⏸️ **Marketplace de plugins** — en pause, en attente d'un besoin réel identifié (pas un audit de conception en cours)
4. ⏸️ **Extension VSCode** — en pause, en attente d'un besoin réel identifié (le risque technique est faible, mais ce n'est pas le critère)
5. **SaaS optionnel** — après avoir répondu à "pourquoi", pas avant