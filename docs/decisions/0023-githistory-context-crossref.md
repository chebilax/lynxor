# ADR 0023 — githistory : cross-référencer introduction/suppression via Context, pas de fusion

## Statut

Accepté (2026-07-26).

## Contexte

Issue #27, ouverte pendant l'étape 1 (ADR 0017) : un secret introduit dans un commit puis retiré dans un commit ultérieur produit deux `Finding` distincts (chaque commit est diffé indépendamment contre son parent). Confirmé réel et visible via un vrai scan `--full-history` sur `gin-gonic/gin` (pas seulement en théorie) : 3 fichiers avec un cycle introduction+suppression complet, chacun produisant deux blocs `CRITICAL` identiques dans le rapport, ne différant que par le hash de commit — un vrai bruit visuel, sans pour autant fausser le score (la pondération à rendement décroissant de `ComputeCategoryScore` sature déjà avant que le doublement ne change grand-chose).

Trois pistes envisagées dans l'issue, plus une quatrième trouvée en relisant le code avant de trancher.

## Décision : cross-référencer via `Finding.Context`, ne pas fusionner en un seul `Finding`

**Fusionner en un seul `Finding`** (une piste listée dans l'issue) résoudrait le doublement de compte mais exige de reformer `core.Finding` (qui ne porte qu'un seul `CommitHash`) pour porter deux commits — un changement du schéma JSON (ADR 0009), donc potentiellement cassant pour tout consommateur externe (l'artifact `lynxor-report.json` du GitHub Action, le dashboard HTML).

**Grouper visuellement seulement dans la sortie** (l'autre piste listée) évite ce changement de schéma, mais la couche de sortie (`output/cli.go`, `output/html.go`) n'a aucune information sur l'ordre des commits (quel `Finding` est l'introduction, lequel est la suppression) — ce que `core.Finding` ne porte pas non plus aujourd'hui. Cette piste ne serait donc pas gratuite : il faudrait quand même faire circuler une donnée supplémentaire jusqu'à la couche de sortie.

**Retenu** : garder les deux `Finding` séparés (exacts : deux vrais événements de diff), mais renseigner `Finding.Context` sur chacun pour référencer l'autre — "also removed in commit X" / "originally introduced in commit Y". `Context` existe déjà précisément pour ce rôle ("non-authoritative hint for triage", ADR 0001) — aucun changement de schéma JSON, aucune couche de sortie à toucher, rétrocompatible avec tout consommateur existant.

## Décision : appariement par chemin + hash de blob, pas par ID de règle

`scanCommit` compare déjà les hashs de blob pour sa propre logique de dédoublonnage (rename/mode change). Réutiliser ce même signal pour l'appariement : grouper les occurrences par (chemin, hash de blob) exact, pas seulement par (chemin, ID de règle). Une correspondance par ID de règle seul aurait pu apparier à tort deux secrets non liés du même type dans le même fichier à des moments différents (ex. une clé AWS retirée puis, des mois plus tard, une clé complètement différente ajoutée) — un mauvais cross-référencement serait pire qu'aucun, en particulier pour un usage de type réponse à incident.

## Décision : conservateur — un groupe ambigu n'est jamais deviné

Seul un groupe de **exactement deux** occurrences pour le même (chemin, blob), une introduction et une suppression, est apparié. Tout le reste — un secret réintroduit plus tard (3+ occurrences du même blob), deux secrets distincts retirés dans le même commit (4 occurrences), etc. — reste sans `Context` ajouté, plutôt que de deviner un appariement qui pourrait induire en erreur. Vérifié par un test dédié (`TestScan_DoesNotPairWhenAmbiguous`, un secret introduit/retiré/réintroduit/retiré à nouveau — aucun des 4 findings ne reçoit de `Context`).

## Implémentation

`scanCommit`/`scanDangling` retournent désormais `[]occurrence` (type interne, non exporté) au lieu de `[]core.Finding` directement — chaque `occurrence` porte un pointeur vers son `Finding`, le hash de blob, et un booléen `introduction` (côté "to" du diff = contenu apparaît, vs côté "from" = contenu disparaît). `Scan` accumule toutes les occurrences de l'historique atteignable et des commits dangling, appelle `pairOccurrences` (qui mute `Finding.Context` en place via les pointeurs) une seule fois à la fin, puis construit `Result.Findings` par copie. La signature publique de `Scan` (et donc de `core.RepoAnalyzer`) ne change pas.

## Validé, pas juste écrit

- Trois nouveaux tests : le cas nominal (introduction+suppression, `Context` référence bien l'autre commit des deux côtés), le cas ambigu (4 occurrences du même blob, aucun `Context` ajouté), plus les tests existants (double-finding, dangling, etc.) tous verts sans modification de leurs attentes sur le compte de findings.
- Revalidé contre un vrai clone de `gin-gonic/gin` (le même repo utilisé pour confirmer le problème initialement) : les 3 paires introduction+suppression réelles portent maintenant le bon cross-référencement ; `testdata/certificate/key.pem` (toujours présent dans l'arbre de travail actuel, une situation différente, pas une paire propre dans l'historique seul) ne reçoit correctement aucun `Context` ajouté.
- Rendu CLI vérifié sur un petit repo de fixture local : le `context:` cross-référencé s'affiche correctement des deux côtés, sans changement nécessaire à `output/cli.go`/`output/html.go`.
- `make check` vert.

## Conséquences

- `Finding.Context` peut désormais porter deux hints concaténés (le hint existant de `secrets` sur les chemins test/fixture, plus ce nouveau cross-référencement) — séparés par " — ", même style que le texte déjà utilisé à l'intérieur d'un seul hint.
- Issue #27 : à fermer, ce fix la résout directement.
