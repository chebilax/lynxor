# ADR 0019 - Support npm (`package-lock.json`) via l'interface `Parser`

## Statut

Accepté (2026-07-24).

## Contexte

`--deps` ne couvrait que Go (`go.sum`) et Python (`requirements.txt`) - aucun support npm, l'écosystème le plus fréquent dans les incidents réels de supply chain, déjà anticipé dans `vision.md` ("Node en option") mais jamais implémenté. Étape 3 d'une suite séquencée, après ADR 0017/0018.

## Décision : un registre de `Parser`, introduit une fois dans `dependencies.go`

Le scope demandé - une interface `Parser` (`Matches(filename) bool`, `Parse(path, manifestRel string) []Dependency`), un fichier `npm.go` séparé - supposait implicitement que `dependencies.go` ait déjà un point d'extension. Il n'en avait pas : `Discover()` était un `switch d.Name()` figé sur deux noms de fichiers, sans aucun registre.

**Signalé avant de coder, pas glissé en silence** : impossible d'ajouter un troisième format sans toucher `dependencies.go` au moins une fois, pour introduire le registre lui-même. Fait une seule fois dans cette PR : `Parser` (l'interface), `parsers []Parser` (le registre), `registerParser()`, et `goSumParser`/`requirementsTxtParser` (les deux formats existants, encapsulés dans le même style que `packageLockParser`). `Discover()` boucle maintenant sur `parsers` au lieu du `switch`. **Après cette PR**, `npm.go` s'enregistre lui-même via son propre `init()` - et tout futur format (`yarn.lock`, `pnpm-lock.yaml`) fera de même, sans plus jamais toucher `dependencies.go`. `osv.go` n'a effectivement demandé aucun changement : l'écosystème `"npm"` est déjà un identifiant OSV standard, tout le pipeline (requête batch, dédup d'alias, mapping de sévérité) est déjà agnostique de l'écosystème.

## Décision : lit `"packages"` (lockfileVersion 2/3), pas l'arbre `"dependencies"` legacy

`package-lock.json` a deux représentations selon la version : `lockfileVersion` 1 (pré-npm-7, arbre `"dependencies"` imbriqué) et 2/3 (npm 7+, map plate `"packages"` - la 2 garde `"dependencies"` en plus pour compat npm 6, la 3 le supprime entièrement). Lire `"packages"` couvre 2 et 3 sans avoir à distinguer laquelle a produit le fichier ; la 1 n'est pas supportée dans cette PR (npm 7 date de 2020, un choix de scope raisonnable pour une première version).

Nom du package extrait du dernier segment `node_modules/` de la clé (`node_modules/foo` → `foo`, `node_modules/@scope/name` → `@scope/name`, `node_modules/a/node_modules/b` → `b` pour un conflit de hoisting). L'entrée racine (clé `""`) et les membres de workspace liés (`"link": true`) sont ignorés - ni l'un ni l'autre n'est une dépendance externe à vérifier.

**Dédup sur nom+version** : un conflit de hoisting peut lister le même package+version à plusieurs chemins `node_modules` différents - sans dédup, la même vraie vulnérabilité serait requêtée et rapportée deux fois pour ce qui est conceptuellement une seule dépendance.

## Conséquences

- Vérifié contre la vraie API OSV.dev, pas seulement en local : `lodash@4.17.4` (7 CVE connues, sévérités CRITICAL/HIGH/MEDIUM) donne un résultat identique en forme à ce que Go/Python produisent déjà - confirme qu'`osv.go` n'avait vraiment besoin d'aucun changement.
- `yarn.lock`/`pnpm-lock.yaml` restent hors scope, volontairement - PR séparée une fois celle-ci validée, comme prévu dès le départ.
- `lodash@4.17.4` devient la troisième fixture de référence documentée dans `docs/testing.md`, aux côtés de `x/text@v0.3.0` et `urllib3==1.24.1`.
