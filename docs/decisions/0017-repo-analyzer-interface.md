# ADR 0017 - `core.RepoAnalyzer` : unifie `CheckDependabot` et `githistory.Scan`

## Statut

Accepté (2026-07-24).

## Contexte

`cicd.CheckDependabot` et `githistory.Scan` sont des checks niveau-repo (pas niveau-fichier), appelés directement depuis `cli/scan.go` comme deux cas spéciaux hors de `core.Analyzer` - exception déjà documentée (voir le commentaire historique dans `cicd.go`) mais jamais unifiée. Premier item d'une suite de travaux (support npm, extension des tests) qui doivent se construire sur ce câblage, pas sur l'ad hoc actuel.

## Décision : interface minimale, pas un mécanisme de warnings dédié

```go
type RepoAnalyzer interface {
    Name() string
    Run(repoRoot string) ([]Finding, error)
}
```

`CheckDependabot` (aucune config) et `githistory.Scan` (`Options` riches : `FullHistory`, `Budget`, `OnProgress`) ont des signatures très différentes - l'interface reste minimale, chaque implémentation lie sa propre config à la construction (`githistory.NewAnalyzer(opts)`, même convention que `secrets.New()`/`docker.New()`), plutôt que de faire grossir l'interface pour un seul besoin particulier.

Les diagnostics non-fatals (warning de troncature d'historique, progress pendant `--full-history`) sont écrits directement sur `os.Stderr` par l'implémentation elle-même - même convention déjà établie par `plugin.abandon()` et `core.RunAnalyzer` (ADR 0016), pas un nouveau canal de warnings retournés sur cette interface. Conséquence assumée : le warning de troncature passe de `cmd.ErrOrStderr()` (comme avant, dans `cli/scan.go`) à `os.Stderr` directement - invisible en usage réel (`cmd.ErrOrStderr()` résout vers `os.Stderr` par défaut), mais pertinent si un futur test d'intégration CLI capture la sortie via `cmd.SetErr()` - à garder en tête pour l'étape 7 (extension des tests).

`RepoAnalyzer.Run` ne passe **pas** par le timeout de 5s de `core.RunAnalyzer` (ADR 0016) - ce budget est pensé pour du travail par-fichier, où un appel sain revient en millisecondes. Un scan `--full-history` légitime peut prendre plusieurs minutes ; y appliquer ce garde-fou le déclencherait sur chaque usage normal de ce mode. `githistory` gère déjà son propre budget de temps interne, adapté à son cas - rien ici ne justifie un second mécanisme de timeout.

## Décision : la politique "not a git repo = silencieux" reste côté appelant

`githistory.ErrNotAGitRepo` continue de remonter tel quel via `Run()`, sans être avalé par le wrapper - `cli/scan.go` garde sa logique `errors.Is(err, githistory.ErrNotAGitRepo)` pour décider si c'est un skip silencieux ou un vrai warning. C'est une politique CLI, pas une responsabilité de l'analyzer.

## Conséquences

- `cicd.DependabotAnalyzer` et `githistory.Analyzer` implémentent `RepoAnalyzer` ; `cli/scan.go` les traite via une seule boucle générique (`[]core.RepoAnalyzer`) au lieu de deux blocs de code séparés.
- Vérifié empiriquement, pas seulement compilé : fixture repo avec workflow GitHub Actions sans Dependabot + secret introduit puis retiré dans l'historique - les deux findings apparaissent identiquement à avant le refactor (`--no-history`, `--full-history` testés aussi).
- Premiers tests automatisés de `githistory` (0% de couverture avant cette PR, explicitement identifié comme un manque dans l'ADR 0014) : fixtures Git générées en Go (même pattern que `diffmode_test.go`), couvrant la découverte tardive d'un secret retiré, la non-duplication du HEAD courant, `ErrNotAGitRepo`, et la troncature par budget de temps (`Options.Budget` réduit pour le test, pas d'attente réelle). 63.4% de couverture sur ce package.
- **Découverte en écrivant les tests** : un secret introduit puis retiré produit **deux** findings (un par commit diffé - introduction et suppression), pas un seul comme je le supposais initialement en écrivant le premier jet du test. Comportement pré-existant de `scanCommit`/`scanBlob`, non modifié par ce refactor - corrigé dans le test, pas dans le code.
- `plugin` reste à 0% de couverture - hors scope de cette PR, candidat pour l'étape 7.
