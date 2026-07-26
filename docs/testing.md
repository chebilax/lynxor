# Testing — corpus, critères de sortie, où trouver quoi

Ce fichier centralise comment Lynxor est validé contre des repos réels, pour ne pas avoir à reconstruire cette connaissance à chaque phase (le corpus complet a déjà été perdu une fois, en Phase 2, quand `/tmp` a été vidé par une interruption de session).

Les décisions de design elles-mêmes (pourquoi un budget de temps plutôt qu'une profondeur fixe, pourquoi ne pas dégrader la sévérité sur un pattern de chemin...) vivent dans `docs/decisions/` — ce fichier n'y touche pas, il n'y renvoie que.

## Corpus de 20 repos publics (Phase 1)

Utilisé pour valider le critère de sortie du MVP. Clones shallow (`--depth 1`) suffisants pour tester le scan working-tree — pour tester le git-history analyzer (Phase 2+), il faut des clones complets (voir plus bas).

```
spf13/cobra
gin-gonic/gin
junegunn/fzf
sharkdp/fd
BurntSushi/ripgrep
jesseduffield/lazygit
charmbracelet/glow
prometheus/prometheus
gohugoio/hugo
caddyserver/caddy
pallets/flask
psf/requests
tiangolo/fastapi
expressjs/express
sveltejs/svelte
axios/axios
lodash/lodash
chalk/chalk
ohmyzsh/ohmyzsh
github/gitignore
```

Pour le reconstituer :

```bash
mkdir -p /tmp/corpus && cd /tmp/corpus
while read -r repo; do
  name=$(basename "$repo")
  git clone --depth 1 --quiet "https://github.com/$repo.git" "$name"
done <<'EOF'
spf13/cobra
gin-gonic/gin
junegunn/fzf
sharkdp/fd
BurntSushi/ripgrep
jesseduffield/lazygit
charmbracelet/glow
prometheus/prometheus
gohugoio/hugo
caddyserver/caddy
pallets/flask
psf/requests
tiangolo/fastapi
expressjs/express
sveltejs/svelte
axios/axios
lodash/lodash
chalk/chalk
ohmyzsh/ohmyzsh
github/gitignore
EOF
```

Repos avec des findings secrets légitimes (clés de test dans `testdata/`/`fixtures/`, `.env` de test) : axios, caddy, flask, gin, prometheus, requests. Les autres scannent propres. Utile pour repérer immédiatement une régression : si l'un des repos "propres" se met à remonter un finding, c'est un faux positif à investiguer avant de merger, pas un nouveau vrai positif à célébrer.

`caddy`, `gin` et `prometheus` ont des `.gitignore` avec des patterns de négation (`!fichier`) — utiles pour valider concrètement le warning `.gitignore` plutôt que de le laisser théorique.

## Clones complets (Phase 2+, git-history)

Le git-history analyzer a besoin de vrai historique, pas d'un clone shallow. Trois tailles représentatives, déjà utilisées pour calibrer le budget de temps (voir `docs/decisions/0002-git-history-depth.md`) :

| Repo | Commits | Fichiers trackés |
|---|---|---|
| cobra | ~1.1k | ~66 |
| gin | ~2k | ~130 |
| prometheus | ~18k | ~1.6k |

```bash
mkdir -p /tmp/corpus-full && cd /tmp/corpus-full
git clone --quiet https://github.com/spf13/cobra.git
git clone --quiet https://github.com/gin-gonic/gin.git
git clone --quiet https://github.com/prometheus/prometheus.git
```

`prometheus` est le cas qui a révélé la plupart des limites réelles jusqu'ici (vendor bump massif dans l'historique, faux positifs dans du code vendoré, Dockerfile réel avec un tag `latest` et un `Dockerfile.distroless`) — c'est le premier repo à re-tester dès qu'un changement touche `githistory` ou `docker`.

## Dockerfiles et workflows réels dans le corpus Phase 1

Le corpus de 20 repos sert aussi à valider `docker` et `cicd` contre du contenu réel, pas seulement des fixtures synthétiques — 9 des 20 ont au moins un vrai workflow GitHub Actions (axios, caddy, chalk, cobra, flask, gin, ohmyzsh, prometheus, requests ; 57 fichiers `.yml` au total). C'est ce qui a révélé que `gin/.github/workflows/codeql.yml` et `requests/.github/workflows/codeql-analysis.yml` contiennent tous les deux `@main`/`@master` dans un contexte qui n'est pas une référence d'action (`branches: [main]`, un commentaire) — la justification empirique du parsing YAML structurel plutôt que regex, voir `docs/decisions/0005-cicd-analyzer-scope.md`.

## Règles internes (secrets/docker/cicd) : tests table-driven, depuis l'ADR 0014

Le corpus réel valide la vitesse et l'absence de faux positifs à grande échelle ; il ne protège pas contre une régression sur une règle précise (une regex resserrée qui casse une exclusion, une logique multi-stage Dockerfile qui se dérègle). `analyzers/secrets/secrets_test.go`, `analyzers/docker/docker_test.go` et `analyzers/cicd/cicd_test.go` couvrent maintenant chaque règle et chaque exclusion de faux positif documentée en commentaire (suffixe `EXAMPLE` d'AWS, corps PEM tronqué, `FROM builder` multi-stage, tag `nonroot` distroless, `ADD` d'URL/archive, `permissions` en map scopée vs `write-all`, vérification booléenne d'un secret vs `echo` direct) — couverture 95-97% sur les trois packages. `analyzers/cicd/cicd_test.go` couvre aussi `CheckDependabot`, la fonction niveau-repo appelée directement par `cli/scan.go`.

## Dependency Scanner : test contre l'API OSV.dev réelle, pas de mock

`analyzers/dependencies` fait de vrais appels réseau (`--deps`) — validé contre la vraie API `api.osv.dev`, jamais mockée. Trois dépendances volontairement anciennes/vulnérables servent de fixtures de référence pour ce chemin de code : `golang.org/x/text@v0.3.0` (Go), `urllib3==1.24.1` (Python), `lodash@4.17.4` (npm, depuis ADR 0019) — toutes avec plusieurs CVE connues et couvrant les trois cas de mapping de sévérité (`database_specific.severity` direct, heuristique CVSS, fallback Medium — voir `docs/decisions/0006-dependency-scanner-scope.md`).

`gin` (57 dépendances) et `prometheus` (1075 dépendances sur 5 `go.sum`) du corpus servent aussi de test d'échelle réel pour ce chemin — c'est prometheus qui a révélé le plafond de batch OSV non documenté (1000 requêtes max) : sans le découpage en chunks, le check de dépendances échouait entièrement en silence sur ce repo, avec un message trompeur ("réseau indisponible").

Depuis l'ADR 0014, ce bug précis a aussi un test de non-régression automatisé : `TestQueryBatch_Chunking` (`analyzers/dependencies/osv_test.go`) rejoue le découpage en chunks contre un `httptest.Server` fake qui rejette toute requête de plus de 1000 dépendances — `osvBatchURL`/`osvVulnURL` sont des `var` (pas des `const`) précisément pour permettre ça, seul changement de code de production motivé par la testabilité. Le reste de la logique pure d'`osv.go` (dédup d'alias, mapping de sévérité, heuristique CVSS) est couvert dans le même fichier. La vraie API OSV.dev reste testée manuellement en pré-release, sans mock — c'est ce test-réseau réel qui a trouvé le plafond de batch en premier lieu ; le fake HTTP protège seulement la logique de chunking déjà découverte, il ne la remplace pas comme méthode de découverte de nouveaux bugs.

## Security Diff Mode : fixtures Git générées en Go, plus le corpus pour la perf

Les 5 scénarios ci-dessous étaient une checklist manuelle jusqu'à l'ADR 0014 — ce sont maintenant de vrais tests automatisés dans `analyzers/diffmode/diffmode_test.go`, contre des dépôts Git générés à la volée (`t.TempDir()` + `go-git`, pas de clone, pas de dépendance à `/tmp` qui a déjà disparu une fois) :
1. Un secret ajouté sur la branche → `NEW` (`TestDiff_NewSecret`).
2. Un secret supprimé sur la branche → `FIXED` (`TestDiff_FixedSecret`).
3. Un problème préexistant sur les deux branches, non touché par la branche → absent du diff (`TestDiff_PreexistingIssueIsNotReported`).
4. Un secret inchangé mais dont le numéro de ligne se décale → absent du diff, pas de faux `NEW`+`FIXED` (`TestDiff_LineShiftDoesNotFalselyReport`).
5. Deux findings de même clé `(File, ID, Category)`, un supprimé → exactement 1 `FIXED` (`TestDiff_CountAwarePairing`).

`prometheus` (clone complet, deux tags de version distants) reste la référence pour le test de perf sur un vrai gros repo — voir `docs/benchmarks.md`. Le corpus réel et les fixtures générées ne se remplacent pas : l'un valide la performance/les faux positifs sur du contenu réel, l'autre valide la logique pure d'appariement de façon déterministe et rapide.

## Plugin System : plugin de référence en Python, pas seulement du Go

`docs/examples/reference-plugin.py` — écrit en Python, délibérément, pour valider honnêtement la promesse "le protocole n'a rien de spécifique à Go" plutôt que de la laisser comme une affirmation non vérifiée dans `docs/plugin-protocol.md`. Accepte un flag `--misbehave=timeout|crash|fatal|error` pour rejouer chacun des scénarios de défaillance ci-dessous à la demande — pas destiné aux vrais auteurs de plugins, uniquement à la suite de test.

Sept scénarios de référence à reproduire si on touche `analyzers/plugin` :
1. Fonctionnement normal → le finding du plugin apparaît dans le rapport, avec l'`id` correctement préfixé par `plugin_name`.
2. Erreur fatale au handshake (`--misbehave=fatal`) → plugin ignoré au chargement, scan continue normalement.
3. Erreur non-fatale sur un fichier (`--misbehave=error`) → warning par fichier concerné, plugin reste actif pour les fichiers suivants.
4. Timeout (`--misbehave=timeout`, le script dort 30s) → abandon après exactement 5s, pas de nouvelle tentative sur les fichiers suivants.
5. Crash (`--misbehave=crash`, `sys.exit(1)`) → détecté immédiatement (EOF sur stdout), pas d'attente du timeout.
6. Deux plugins en même temps, un qui crashe et un qui fonctionne → isolation confirmée, le crash de l'un n'affecte pas les findings de l'autre.
7. `--plugin-arg reference-example:foo=bar` (ADR 0018) → le finding du plugin de référence affiche `context: configured args: {'foo': 'bar'}`, confirmé par un vrai run, pas seulement lu dans le protocole. Sans aucun `--plugin-arg`, ou avec un `--plugin-arg` adressé à un autre nom de plugin, le `context` reste vide — les deux vérifiés aussi.

`--plugin` prend un chemin d'exécutable, pas une commande avec arguments (un vrai plugin est autonome, il n'a pas besoin de flags CLI) — pour tester chaque mode il faut donc un petit wrapper par mode plutôt que de passer `--misbehave=X` directement :

```bash
for mode in fatal crash timeout error; do
  cat > /tmp/reference-$mode.sh <<EOF
#!/bin/sh
exec python3 "$(pwd)/docs/examples/reference-plugin.py" --misbehave=$mode
EOF
  chmod +x /tmp/reference-$mode.sh
done
./lynxor scan . --plugin /tmp/reference-crash.sh
```

## Critères de sortie mesurables (déjà validés)

- **Vitesse < 5s** (critère de sortie du MVP, vision.md) : validé sur les 20 repos du corpus Phase 1 (max observé : ~1.5s, fastapi/svelte) et sur les clones complets en mode par défaut (max observé : ~3s, prometheus — budget git-history de 1.5s + scan working-tree + overhead process). `--full-history` n'est **pas** soumis à ce critère : c'est un mode explicitement "sans budget", jusqu'à 18 minutes observées sur prometheus (18k commits) — voir `docs/decisions/0002-git-history-depth.md`.
- **Zéro faux positif majeur** : validé sur les 20 repos Phase 1 après correctifs (extension `.pem`/`.key` confondue avec certificat, regex de clé privée matchant un placeholder de doc, clé AWS d'exemple officielle et fixture Google dans du code vendoré). Ce qui reste est une classe connue et documentée (clés de test dans `testdata/`/`fixtures/`) — voir `docs/decisions/0001-test-fixture-context.md` pour pourquoi ce n'est pas supprimé, seulement annoté.
- **Budget de temps : per-analyzer, pas global, mais plus indirect depuis l'ADR 0016.** `DefaultBudget` (1.5s) reste interne à `githistory.Scan()`. Le scanner working-tree (`core.Scanner`) et `diffmode.scanTree()` passent maintenant chaque appel d'analyzer par `core.RunAnalyzer`, qui abandonne l'attente (log + skip, jamais de hang) si un analyzer ne répond pas dans `AnalyzerTimeout` (5s, aligné sur le budget du protocole de plugin). Avant l'ADR 0016, ce garde-fou était entièrement indirect ("chaque analyzer reste un parsing léger, validé empiriquement à chaque ajout") — identifié dans une revue d'architecture externe comme le risque le plus sérieux et le plus silencieux du projet : rien n'empêchait structurellement un futur analyzer de bloquer tout le scan. Vérifié avec un analyzer factice "bloqué" 10s dans `core/timeout_test.go` : le scan complet revient en ~60ms avec les findings de l'analyzer normal, pas les 10s du bloqué.

## JSON output : validité + isolation stdout/stderr

`--format json` se valide sur trois points, tous vérifiés avec de vraies commandes plutôt que par lecture de code :
1. Le JSON produit est syntaxiquement valide (`python3 -m json.tool` en pipe).
2. `findings: []` sur un repo propre, jamais `null` — un consommateur ne doit pas avoir à gérer les deux cas.
3. Les diagnostics (`.gitignore`, dépendances, git-history, plugins) restent sur stderr, jamais mêlés au JSON de stdout — vérifié en séparant explicitement les deux flux (`2>/dev/null` vs `2>&1 1>/dev/null`), pas supposé.

Fixture de référence pour couvrir tous les champs simultanément : un mini-repo avec une clé de test ajoutée puis supprimée dans `testdata/` — produit un finding avec `commit_hash` réel (git-history) *et* `context` réel (chemin test/fixture) en même temps, pour confirmer que chaque champ traverse la sérialisation correctement.

Depuis l'ADR 0014, cette même fixture multi-champs est aussi un golden file automatisé : `output/golden_test.go` définit `goldenFindings` (un finding par champ notable : `commit_hash` rempli, `context` rempli, sévérités différentes, catégories différentes), et `output/json_test.go`/`output/html_test.go` comparent la sortie réelle à `output/testdata/report.{json,html}.golden`. Validité syntaxique du JSON toujours vérifiée directement (`encoding/json.Unmarshal`), pas seulement par comparaison au golden. `go test ./output/... -update` régénère les golden files après un changement de schéma volontaire. Testé pour de vrai, pas supposé : casser temporairement un titre dans le template HTML fait échouer `TestWriteHTMLReport_Golden` comme attendu, confirmé avant d'écrire cette note.

## Dashboard HTML : premier test Go automatisé du projet, plus une vérification structurelle du rendu

`core/scoring_test.go` — premier fichier `_test.go` de ce projet, écrit spécifiquement pour un invariant que la vérification CLI habituelle ne couvre pas bien : la garantie qu'aucun finding n'est compté deux fois ni oublié quand `ComputeCategoryBreakdown` partitionne par catégorie. Deux tests :
1. `TestComputeCategoryBreakdown_PartitionsWithoutDuplicationOrLoss` — utilise `ComputeCategoryScore` comme oracle : chaque score de catégorie dans le breakdown doit être identique à celui obtenu en filtrant manuellement les findings de cette catégorie.
2. `TestComputeCategoryBreakdown_TotalIsNotAnAggregateOfCategoryScores` — un CRITICAL dans une catégorie, des LOW ailleurs ; le score total doit diverger nettement de la moyenne naïve des catégories (35 contre 78 sur la fixture retenue), preuve que le total n'est jamais dérivé du breakdown.

Le HTML généré lui-même se valide en deux temps, aucun des deux par simple lecture de code :
- Bien-formé structurellement : tags équilibrés. Vérifié une première fois manuellement avec le parseur HTML de la stdlib Python (`html.parser`) ; depuis l'ADR 0014, `assertBalancedHTML` (`output/html_test.go`) fait la même vérification en Go à chaque run de `go test`, sans dépendance externe.
- Rendu visuel inspecté via un Artifact publié temporairement — cet environnement n'a pas d'outil de capture d'écran, donc la vérification pixel-perfect (alignement, espacement) reste à la charge de la review humaine ; l'automatisé couvre la structure et la logique couleur/statut, pas la mise en page. Le timestamp `generated <date>` du template est normalisé avant comparaison au golden file (`normalizeHTMLTimestamp`), sinon chaque run produirait un diff sur la seule valeur qui doit changer à chaque run.

## GoReleaser : local `--snapshot` d'abord, mais validé par un vrai tag poussé

`.goreleaser.yaml` et `.github/workflows/release.yml` (ADR 0020) n'ont pas été jugés prêts sur la seule base d'un `goreleaser build --snapshot --clean` local (6 binaires produits, `--version` retourne bien la forme `vX.Y.Z` grâce à `{{.Tag}}`) — même standard que pour l'Action GitHub et le tap Homebrew : un tag de test jetable (`v0.0.0-goreleaser-test`) a été poussé pour de vrai, déclenchant un vrai run de `release.yml` sur GitHub Actions avec un vrai `GITHUB_TOKEN`. La release générée contenait les 6 archives attendues (`lynxor_<os>_<arch>.tar.gz`/`.zip`) plus `checksums.txt` avec les bons SHA256 — vérifié via `gh release view`, pas juste supposé parce que le job est passé au vert. Tag et release de test supprimés après vérification.

## npm : `install.js` validé contre une vraie release, `npm publish` non testé (limite assumée)

`npm/scripts/install.js` (ADR 0021) a été exécuté réellement (pas juste relu) contre un second tag jetable (`v0.0.0-npm-test`, poussé puis supprimé après vérification) : téléchargement des vrais assets GitHub Release, vérification du SHA256 contre `checksums.txt`, extraction via `tar` système, `chmod`, puis un vrai `lynxor --version` et `lynxor scan .` exécutés à travers `bin/lynxor.js` — tout correct sur darwin/arm64 (seule plateforme testable dans cet environnement de dev).

Limite explicitement assumée, pas cachée : le job `publish-npm` lui-même (`.github/workflows/release.yml`) n'a pas été exercé pour de vrai. Publier pour de vrai sur le registre npm public n'est pas une opération aussi propre à annuler (`npm unpublish` a des restrictions, contrairement à `gh release delete`) — le prochain tag réel sera donc le premier vrai test de `publish-npm`.

**Mise à jour** : `NPM_TOKEN` n'a finalement jamais été configuré — remplacé par le trusted publishing npm (OIDC) avant le premier essai réel (voir la mise à jour d'ADR 0021). `id-token: write` scopé au job, `actions/setup-node` passé à `@v7`/Node 24, plus de `NODE_AUTH_TOKEN`.

**Mise à jour 2 — le premier vrai run a échoué, root cause trouvée par deux tags jetables de debug, pas par théorie** : `v1.1.1` a déclenché `publish-npm` pour de vrai → `404`. Un tag jetable avec étape de diagnostic temporaire (impression de `.npmrc`/`npm whoami`) a montré la vraie cause : `actions/setup-node@v7` écrit `//registry.npmjs.org/:_authToken=${NODE_AUTH_TOKEN}` dans `.npmrc` dès que `registry-url` est renseigné, même sans secret — npm croit avoir un token, n'essaie jamais l'OIDC (`401` sur `whoami`, `404` sur `publish`). Correctif : ne plus renseigner `registry-url` (le registre par défaut d'npm est déjà `registry.npmjs.org`). Un second tag jetable après correctif a confirmé que l'authentification passe désormais : erreur différente et attendue (prérelease sans `--tag`, artefact du nom du tag de test, pas un problème pour un vrai `X.Y.Z`). Détail complet : mise à jour d'ADR 0021.

**Mise à jour 3 — `v1.1.2` : succès, après une cause hors de ce repo** : `ENEEDAUTH` (aucune tentative d'auth du tout) au premier essai — la config Trusted Publisher côté npmjs.com avait été remplie mais jamais sauvegardée. Une fois sauvegardée et le job relancé sans changement de code, `publish-npm` a réussi : `lynxor@1.1.2` publié pour de vrai, `published ... by GitHub Actions` (pas un token humain). Validation complète en environnement propre, les trois canaux : `go install .../lynxor@v1.1.2` (résout, `--version` = `dev`, limite déjà documentée), `npm install -g lynxor` (`--version` = `v1.1.2`), `brew tap chebilax/lynxor && brew install lynxor` + `brew test` (formule repointée sur `v1.1.2`, tout vert). Distribution npm/OIDC close.

## Renommage de compte GitHub xchebila → chebilax (2026-07-26)

Pas de raisonnement de marque à tracer (contrairement à RepoAudit→RepoScan et RepoScan→Lynxor) — juste un changement de pseudo GitHub, donc pas d'ADR dédié, seulement ce repère technique. Inventaire (`git grep -lI -i "xchebila"`) fait et confirmé avant tout remplacement : 33 fichiers renommés (code, `go.mod`, `action.yml`, `.goreleaser.yaml`, `npm/`, docs tournées vers l'état courant), 2 fichiers volontairement laissés intacts — `docs/decisions/0022-rename-reposcan-to-lynxor.md` en entier, et 3 lignes de `docs/roadmap-long-term.md` — parce qu'ils narrent des événements réellement arrivés sous le compte `xchebila` (citations de messages d'erreur, commandes réelles), pas de la prose sur l'état actuel du projet.

Validé après le remplacement : `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` tous verts sous le nouveau chemin de module `github.com/chebilax/lynxor` ; binaire réel construit et exécuté (`scan .`, score inchangé 97/100, même finding connu). Reste à faire, suivi séparé : nouveau tag sous le module renommé (aucun tag existant ne résout sous `chebilax` — même contrainte technique que pour chaque renommage précédent), republication npm, tap Homebrew repointé.

## Étape 7 — extension de couverture de tests (`plugin`, `githistory`, `dependencies`, `core`)

Ciblé explicitement, comme prévu dans le plan séquencé : `plugin` (0% avant, aucun test depuis l'étape 2), `githistory` (63.4%, étape 1), `dependencies` (54.7%, étape 3), et `core` (65.7%, `RepoAnalyzer` ajouté à l'étape 1).

- **`analyzers/plugin` : 0% → 92.2%**. Le vrai obstacle était que `Load` shell-out vers un vrai exécutable (`exec.Command`) — impossible d'exercer le handshake/protocole sans un vrai process en face. Résolu avec `testdata/fakeplugin/`, un petit programme Go compilé une fois (`TestMain`, `go build`) dont le comportement se pilote via la variable d'environnement `LYNXOR_FAKE_PLUGIN_SCENARIO` (héritée par `exec.Command` — `Load` ne fixe pas `cmd.Env`) : handshake raté (3 variantes + crash), résultat valide, préfixage d'ID et catégorie par défaut, chaque mode d'`abandon` (JSON invalide, type inattendu, mauvais chemin, erreur fatale, crash, timeout réel de 5s — délibérément lent, pas de valeur de test raccourcie possible sans toucher au code de prod), erreur non fatale qui ne fait *pas* abandonner le plugin, et `Configure`. Le seul test volontairement lent (~5s) attend le vrai `requestTimeout` plutôt que de le simuler.
- **`analyzers/githistory` : 63.4% → 83.9%**. `countReachableCommits` et `scanDangling` n'avaient aucun test direct. Le second nécessitait un vrai commit "dangling" (physiquement présent, inatteignable depuis aucune ref) — simulé en forçant la ref de la branche à revenir en arrière via `repo.Storer.SetReference`, sans jamais appeler `git gc`, exactement comme une branche supprimée en laisserait un sur disque.
- **`analyzers/dependencies` : 54.7% → 69.9%**. `Discover` (le point d'extension introduit à l'étape 3) n'avait jamais été testé directement — seuls les parsers individuels l'étaient. Ajouté : `Discover` avec de vrais fichiers sur disque (dispatch, chemins de manifest relatifs, exclusion `.git`/vendored), plus les méthodes `Matches`/`Parse` des deux wrappers (`goSumParser`, `requirementsTxtParser`) et `parseRequirementsTxt` lui-même, jusque-là non couvert. Les fonctions réseau d'`osv.go` restent volontairement peu couvertes — cohérent avec `[[project_osv_rate_limiting_watch]]`, pas urgent d'ajouter plus d'appels réels à l'API dans la suite de tests.
- **`core` : 65.7% → 92.6%**. Aucun `scanner_test.go` n'existait avant — `Scanner.Scan`, `Warnings`, `IsBinary`, `IsVendoredPath`, et le matcher `.gitignore` maison n'étaient exercés qu'indirectement via d'autres packages. Ajouté directement dans `core` : exclusion `.git`/vendor/node_modules, fichiers surdimensionnés/binaires, respect réel d'un `.gitignore` (patterns simples, avertissements sur négation/`**` non supportés), et le fait que plusieurs analyzers enregistrés voient tous le même fichier.

**Non traité ici, suivi séparé** : `cli` (10.2%) et `output` (33.3%) restent bas mais n'étaient pas explicitement nommés dans le plan de l'étape 7 — ils demandent un style de test différent (intégration Cobra/capture d'IO plutôt que des fixtures unitaires), pas ajouté dans ce même effort pour ne pas mélanger les deux approches.

**Mise à jour** : `cli/diff.go` (0% sur `newDiffCmd`, aucun test dédié — seul `parsePluginArgs`, une fonction pure, l'était) comblé après coup (`cli` : 10.2% → 19.5%). Le vrai obstacle était `os.Exit(1)` appelé directement dans `RunE` (même limite que `cli/scan.go`, qui l'a aussi mais reste non testé sur ce point précis) : impossible à exercer en process sans tuer le run `go test` entier. Résolu avec l'idiome standard Go pour ce cas (utilisé par les propres tests d'`os/exec`) : ré-exécuter le binaire de test lui-même en sous-processus, ciblé sur une fonction `TestHelperProcess_Diff` qui n'est pas un vrai test (no-op sauf variable d'environnement dédiée), et inspecter le vrai code de sortie du sous-processus. Les autres chemins (repo invalide, mauvais nombre d'arguments, diff sans nouveau finding) sont testés directement en process, sans ce détour, puisqu'ils ne passent jamais par `os.Exit`.

## Issue #27 — cross-référencement introduction/suppression via `Context`, revalidé contre un vrai repo

Le fix ([ADR 0023](decisions/0023-githistory-context-crossref.md)) a été revalidé contre le même vrai clone de `gin-gonic/gin` qui avait initialement confirmé le problème (pas juste contre les fixtures synthétiques des nouveaux tests) : les 3 paires introduction+suppression réelles du repo portent maintenant le bon cross-référencement de commit dans `Finding.Context`, et `testdata/certificate/key.pem` (toujours présent dans l'arbre de travail actuel — une situation différente, pas une paire propre) ne reçoit correctement aucun `Context` ajouté. Rendu CLI vérifié sur un petit repo de fixture local : le cross-référencement s'affiche correctement des deux côtés sans aucune modification à `output/cli.go`/`output/html.go`.

## Où sont les chiffres

`docs/benchmarks.md` — table append-only, un run par phase/PR. Ce fichier-ci dit *quoi* tester et *pourquoi* ; benchmarks.md dit *ce qui a été mesuré, quand*.
