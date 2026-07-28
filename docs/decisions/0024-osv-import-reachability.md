# ADR 0024 - Dependency Scanner : atténuer les faux positifs par chemin d'import OSV, Go uniquement

## Statut

Accepté (2026-07-28). Décision prise avant l'implémentation, comme les ADR 0004/0008 - audit explicite d'abord, parce que l'hypothèse de départ (`go list -deps` sur le repo scanné) s'est révélée introduire une régression de fiabilité qu'il valait mieux trancher avant d'écrire du code.

## Contexte

Un vrai scan `--deps` sur ce repo lui-même a remonté `GO-2026-5932` (MEDIUM) : `golang.org/x/crypto/openpgp` est marqué "unsafe by design, unmaintained". Vérification de l'avis OSV réel (`curl https://api.osv.dev/v1/vulns/GO-2026-5932`) : pas de version corrigée du tout (`"introduced": "0"`, aucun `"fixed"`) - l'avis dit "n'utilisez jamais ce sous-package", pas "mettez à jour". Et vérification de `go list -deps .` sur ce repo : le binaire réel n'importe **pas** `golang.org/x/crypto/openpgp` - le seul code openpgp compilé vient de `github.com/ProtonMail/go-crypto/openpgp` (le fork maintenu que l'avis recommande lui-même). `golang.org/x/crypto` n'est présent dans `go.sum` que pour des sous-packages réellement utilisés et sans rapport (`ssh`, `chacha20poly1305`, etc., nécessaires au transport SSH de go-git).

Donc : ni upgrade possible, ni dépendance réellement supprimable (les sous-packages utilisés sont légitimes) - **faux positif structurel**, causé par le fait que `analyzers/dependencies/osv.go` matche au niveau module+version présent dans `go.sum`, jamais au niveau chemin d'import. `parseGoSum` fait déjà un premier filtrage de ce type (ADR 0015 : ignorer les lignes go.mod-only, qui ne correspondent à aucun code réellement compilé) - ce chantier est la suite logique, un cran plus fin : un module peut avoir une vraie ligne de contenu (donc réellement compilé) sans que CHAQUE sous-package de ce module le soit.

Avant de coder, quatre questions posées explicitement, à vérifier empiriquement plutôt qu'en théorie.

## Question 1 - OSV fournit-il des chemins d'import fiables, pour quels écosystèmes ?

Vérifié contre de vrais avis via l'API OSV.dev réelle (`/v1/vulns/<id>`), pas en théorie.

**Go : oui, et même au niveau symbole** (pas juste chemin d'import) pour les avis `"review_status": "REVIEWED"` :
- `GO-2026-5932`, `GO-2023-1571`, `GO-2021-0113`, `GO-2020-0001`, `GO-2024-2687` : tous ont `ecosystem_specific.imports[].path`, certains avec `.symbols` en plus (ex. `GO-2023-1571` liste précisément `Client.Do`, `Client.Get`, etc. sur `net/http`).
- **Mais pas garanti à 100%** : `GO-2025-3703` (`review_status: "UNREVIEWED"`, un CVE ingéré automatiquement) n'a **aucune** donnée `ecosystem_specific.imports`. La richesse des données Go corrèle avec `review_status`, pas universelle - toute implémentation doit dégrader proprement quand le champ est absent, jamais supposer sa présence.

**PyPI : non, jamais vu de donnée équivalente.** Trois avis réels vérifiés (`PYSEC-2026-1994` - urllib3, `PYSEC-2023-98` - langchain, `GHSA-9wx4-h78v-vm56` - requests) : `ecosystem_specific` est `None` sur les trois, natif PySEC comme GHSA-sourced.

**npm : non, même constat.** Trois avis réels vérifiés (`GHSA-29mw-wpgm-hmr9` et `GHSA-p6mc-m468-83gw`, lodash - ReDoS et prototype pollution ; multi-packages dans les deux cas) : `ecosystem_specific` est `None` sur chaque paquet affecté.

**Conclusion : ce raffinement ne peut viser que Go aujourd'hui**, pas par limite technique de notre côté mais par absence de donnée côté OSV pour PyPI/npm. À revérifier périodiquement (OSV pourrait enrichir ces écosystèmes plus tard) - watch-item, pas un blocage définitif.

## Question 2 - `go list -deps` sur le repo scanné : quel impact sur la fiabilité de `--deps` ?

Impact réel et significatif, et **rédhibitoire tel que formulé initialement**.

Aujourd'hui, `--deps` ne dépend que de la présence de `go.sum` sur disque - aucun réseau, aucune résolution de module, aucune compilation. Ça marche même sur un repo cassé, un WIP, un sous-répertoire qui n'est pas un module Go complet. C'est précisément la garantie que l'ADR 0004 pose pour le scan par défaut ("100% local et déterministe") et que `--deps` ne relâche que d'un cran, volontairement (le seul appel réseau autorisé va vers OSV.dev, rien d'autre).

`go list -deps` sur le repo scanné casserait cette garantie sur plusieurs axes à la fois :
- **Réseau supplémentaire, hors du scope posé par l'ADR 0004** : si le module graph n'est pas déjà entièrement présent dans le cache local, `go list` a besoin de télécharger les modules manquants - un deuxième chemin réseau, différent d'OSV.dev, avec ses propres échecs possibles (proxy Go bloqué, module privé nécessitant `GOPRIVATE`/auth non configurée côté Lynxor).
- **Exige un module graph résoluble**, pas juste un `go.sum` lisible : un repo avec des versions non résolvables, un `go.work` multi-module, un `replace` vers du code local absent, ou simplement un repo cloné partiellement (cas déjà géré aujourd'hui par la lecture pure de `go.sum`) ferait échouer `go list` alors que le scan actuel réussit.
- **Sensible à `GOOS`/`GOARCH`/build tags** : `go list -deps` ne voit que le graphe d'import du système sur lequel Lynxor tourne. Un import réel mais restreint à `//go:build windows` resterait invisible sur un scan lancé depuis macOS/Linux CI - risque de marquer à tort "non atteignable" une dépendance qui l'est bel et bien sur une autre plateforme. C'est le sens d'erreur dangereux (faux négatif de risque), à l'inverse d'un faux positif juste bruyant.

**Décision : ne pas exécuter `go list -deps` (ni aucune commande `go`) sur le repo scanné.** À la place : parser statiquement les fichiers `.go` du repo scanné via `go/parser` (le paquet standard, déjà présent dans le binaire Lynxor - aucun processus externe, aucun réseau, aucune résolution de module) pour extraire les chemins d'import littéraux déclarés, et vérifier l'appartenance du chemin flaggé par OSV à cet ensemble.

Cette approche évite chaque régression listée : `go/parser` n'a besoin que d'un fichier syntaxiquement valide, jamais d'un module graph résolu - un repo avec des dépendances non téléchargées, un `go.work` cassé, ou une version irrésoluble reste parsable normalement. Elle est aussi plus permissive que restrictive sur les build tags : elle ne les évalue pas du tout, donc un fichier `_windows.go` compte comme "importé" même hors de sa plateforme cible - sur-inclusif, jamais sous-inclusif. C'est la direction d'erreur sûre : dans le pire cas, on ne confirme pas l'inatteignabilité d'un import qui l'est réellement ailleurs (pas d'annotation ajoutée), on ne masque jamais un import réellement atteignable.

## Question 3 - Un équivalent existe-t-il pour Python/npm sans exécuter de code arbitraire ?

Oui, techniquement : un parsing statique des imports (`ast.parse` en Python sans jamais appeler `compile()`/`exec()` sur le module lui-même, un parsing regex/AST léger des `import`/`require` en JS) est tout à fait faisable sans exécuter la moindre ligne du repo scanné - même philosophie que `go/parser`.

**Mais ça ne sert à rien tant que la Question 1 tient** : OSV ne publie aucune donnée `ecosystem_specific.imports` pour PyPI ou npm, donc il n'y a rien contre quoi comparer les imports qu'on aurait extraits. Construire cette machinerie maintenant serait du code mort en attendant qu'OSV enrichisse ces écosystèmes.

Note additionnelle, indépendante de la disponibilité des données OSV : le problème que ce chantier résout pour Go (un module entier tiré transitivement, dont un seul sous-package est flaggé, alors que ce sous-package précis n'est jamais importé) a une forme beaucoup plus rare côté Python/npm. `requirements.txt`/`package-lock.json` listent des paquets que l'utilisateur (ou un outil qu'il contrôle directement) a choisi d'installer explicitement - contrairement à `go.sum`, qui enregistre tout le graphe MVS transitif y compris des versions jamais réellement sélectionnées (déjà traité par l'ADR 0015). La granularité du faux positif qu'on cherche à éliminer est structurellement différente.

**Décision : rester Go-only pour l'instant.** Revisiter uniquement si OSV publie un jour une donnée équivalente pour PyPI/npm - watch-item à noter, pas un chantier à ouvrir maintenant.

## Question 4 - Ne jamais supprimer silencieusement un finding non confirmé

**Retenu tel que proposé**, et précisé en trois cas distincts (le nouveau check ne peut produire que ces trois issues, jamais une suppression) :

1. **Chemin d'import confirmé absent du repo scanné** (le cas `GO-2026-5932` ci-dessus) : le `Finding` reste, sévérité inchangée, `Context` reçoit une note du type *"présent dans go.sum mais le chemin d'import flaggé n'a pas été trouvé dans le code source de ce repo - probablement non atteignable, conservé pour visibilité plutôt que supprimé silencieusement"*.
2. **Chemin d'import confirmé présent** (le cas normal, la dépendance est réellement utilisée) : aucun changement - pas de `Context` ajouté, pour ne pas noyer le cas courant sous une confirmation qui n'apporte rien de nouveau à l'utilisateur.
3. **Avis OSV sans donnée `ecosystem_specific.imports` du tout** (avis `UNREVIEWED` comme `GO-2025-3703`, ou n'importe quel avis PyPI/npm) : aucun changement non plus - comportement identique à aujourd'hui, le check n'a simplement pas assez d'information pour se prononcer.

Ce découpage suit exactement le même pattern que les deux précédents déjà en place dans `osv.go` : le hint `secrets`-style sur les chemins test/fixture (ADR 0001), et `mapSeverity`'s estimation de sévérité via CVSS avec `Context` explicite quand la donnée officielle manque (`buildFinding`/`mapSeverity`, ligne ~383). Toujours annoter l'incertitude, jamais l'utiliser pour effacer un signal.

**Sévérité jamais modifiée par ce check** : confirmer ou non l'atteignabilité ne change pas la sévérité du `Finding` - seule la présence/absence de la note `Context` change. Une dépendance non atteignable reste un vrai `Finding` visible dans le rapport (cohérent avec le principe déjà posé par l'ADR 0023 : conservateur, ne jamais deviner/supprimer sur la base d'un signal incomplet).

## Décision : portée de l'implémentation (prochaine étape, pas ce ticket)

Cette ADR documente l'audit et la direction ; l'implémentation reste un travail séparé, comme pour l'ADR 0004/0008. Grandes lignes actées pour ce travail à venir :

- Nouveau helper (probablement `analyzers/dependencies/reachability.go`) : parcourt le repo scanné une seule fois via `go/parser` pour construire l'ensemble des chemins d'import littéraux déclarés (même philosophie de parcours que `Discover` - ignorer `.git`, respecter `core.IsVendoredPath`), coût borné et local.
- Ce check ne tourne que pour les findings `Ecosystem == "Go"` dont l'avis OSV a une donnée `ecosystem_specific.imports` non vide - jamais pour PyPI/npm (Question 3), jamais quand la donnée manque (cas 3 ci-dessus).
- `buildFinding` (aujourd'hui dans `osv.go`) a besoin d'accès à cet ensemble de chemins pour décider s'il ajoute le `Context` du cas 1 - implique de faire circuler ce parsing depuis `Discover`/`CheckVulnerabilities` jusqu'à `buildFinding`, sans changer sa signature publique externe si possible (à confirmer pendant l'implémentation).
- Nécessite de récupérer `ecosystem_specific.imports` depuis la réponse `/v1/vulns/<id>` - `osvVulnDetail` ne le décode pas aujourd'hui, à ajouter.

## Conséquences

- `--deps` reste 100% dans le principe posé par l'ADR 0004 (le seul réseau autorisé va vers OSV.dev) - ce chantier n'y ajoute rien de nouveau côté réseau, contrairement à l'hypothèse initiale `go list -deps`.
- Le faux positif réel trouvé sur ce repo (`GO-2026-5932`) ne disparaît pas du rapport une fois ce travail implémenté - il reste visible, juste annoté. Rien à "fixer" dans le sens d'un upgrade ou d'une suppression de dépendance, ce qui n'était de toute façon pas possible (Contexte ci-dessus).
- PyPI/npm gardent leur comportement actuel sans changement - watch-item si OSV enrichit un jour ces écosystèmes.
- Prochaine étape : implémentation suivant les grandes lignes ci-dessus, dans une PR séparée.
