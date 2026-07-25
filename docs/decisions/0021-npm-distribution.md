# ADR 0021 — Package npm de distribution : façade + postinstall, pas de sous-paquets par plateforme

## Statut

Accepté (2026-07-24).

## Contexte

Étape 5 de la suite séquencée, dépendante de l'étape 4 (GoReleaser, ADR 0020) : `lynxor` doit être installable via `npm install -g lynxor` sans que l'utilisateur ait Go installé. Scope et approche confirmés par l'utilisateur avant de coder : package comme sous-répertoire de ce même repo (`npm/`), publication automatisée sur `npm` déclenchée par tag, via un secret `NPM_TOKEN` configuré côté GitHub.

## Décision : script `postinstall` qui télécharge depuis GitHub Releases, pas des sous-paquets `optionalDependencies` par plateforme

L'alternative "moderne" (esbuild, swc, turbo, sharp) publie un paquet par combinaison OS/arch et un paquet façade qui les liste en `optionalDependencies` — npm n'installe que celui qui correspond, sans script `postinstall` ni téléchargement réseau custom. C'est objectivement plus robuste (fonctionne derrière un miroir npm d'entreprise sans accès à GitHub, pas de code de téléchargement/vérification à maintenir).

Non retenu ici : le plan de l'utilisateur spécifiait explicitement "un script postinstall qui télécharge le bon binaire précompilé" — une décision déjà prise, pas rouverte sans besoin identifié. Le compromis (un point de défaillance réseau supplémentaire vers GitHub à l'installation) est jugé acceptable pour un outil qui, une fois installé, ne fait plus aucun appel réseau non explicitement demandé (`--deps`).

## Décision : aucune dépendance npm — extraction via le `tar` système

`bin/lynxor.js` (le shim exécuté par `npm`) et `scripts/install.js` (le `postinstall`) n'ont ni l'un ni l'autre de `dependencies` dans `package.json`. Le téléchargement utilise `fetch` global (disponible nativement depuis Node 18, d'où `engines.node: ">=18"`). L'extraction shell-out vers le `tar` système plutôt que d'ajouter un paquet JS (`tar` pour le `.tar.gz`, un second comme `yauzl` pour le `.zip` Windows) :

- Linux/macOS : le `.tar.gz` s'extrait avec n'importe quel `tar` (GNU ou BSD).
- Windows : le `.zip` s'extrait avec le `tar.exe` livré en standard depuis Windows 10 1803 (bsdtar, qui gère le zip en plus du tar) — `tar -xf archive.zip` fonctionne sans rien installer.

Zéro dépendance npm pour un paquet dont le seul rôle est de télécharger un binaire — cohérent avec la philosophie "zéro dépendance" du binaire Go lui-même. Risque assumé : un environnement Windows exotique sans `tar.exe` (image container minimaliste, très ancienne version) ferait échouer l'installation avec une erreur claire plutôt que silencieusement.

## Décision : vérification de checksum avant extraction, pas seulement un téléchargement direct

`install.js` télécharge aussi `checksums.txt` (généré par GoReleaser, ADR 0020) et compare le SHA256 de l'archive téléchargée avant de l'extraire ou de l'exécuter. Pour un outil dont toute la raison d'être est la sécurité de la chaîne d'approvisionnement logicielle, télécharger et exécuter un binaire sans vérifier son intégrité aurait été une incohérence de fond, pas un simple oubli — décidé dès la conception, pas ajouté après coup.

## Décision : shim `bin/lynxor.js` fixe qui exec le binaire téléchargé, pas un `bin` pointant directement sur un fichier téléchargé

`package.json.bin` pointe vers `bin/lynxor.js`, committé, toujours présent. Le vrai binaire atterrit dans `npm/.bin/` (gitignoré) après le `postinstall`. Évite tout problème d'ordre entre la création des liens `bin` de npm et l'exécution du `postinstall` — le fichier que `bin` référence existe dès la publication du paquet, qu'il ait ou non déjà téléchargé le binaire réel.

## Décision : synchronisation de version côté CI, pas dans le fichier commité

`npm/package.json` committe un placeholder `"version": "0.0.0"`. Le job `publish-npm` (ajouté à `.github/workflows/release.yml`, `needs: release`) exécute `npm version --no-git-tag-version "${GITHUB_REF_NAME#v}"` juste avant `npm publish` — la version npm est toujours celle du tag Git qui a déclenché le run, sans le préfixe `v` (npm n'accepte pas de préfixe dans un numéro de version semver, contrairement à toutes les autres conventions de ce projet — voir ADR 0013).

## Décision : `publish-npm` dépend (`needs:`) du job `release`, dans le même workflow

Les deux jobs se déclenchent sur le même `push: tags: v*`. Sans `needs: release`, `publish-npm` pourrait publier avant que GoReleaser ait fini d'uploader les assets — un `npm install` juste après une publication échouerait alors de façon transitoire (le `postinstall` chercherait un binaire pas encore là). `needs: release` élimine cette fenêtre de course. Permissions par job, pas au niveau du workflow : `release` a besoin de `contents: write` (créer la release), `publish-npm` n'a besoin que de `contents: read` (checkout) — le défaut du workflow est passé à `read`, chaque job déclare explicitement ce dont il a besoin.

## Validé, pas juste écrit

- `goreleaser build`/`release --snapshot` (étape 4) avait déjà prouvé les binaires eux-mêmes ; ici, un second tag jetable (`v0.0.0-npm-test`, poussé puis supprimé après vérification, même discipline que ADR 0020) a produit une vraie release avec de vrais assets, contre laquelle `node scripts/install.js` (avec `LYNXOR_INSTALL_VERSION` pour cibler ce tag sans modifier `package.json`) a été exécuté réellement : téléchargement, vérification de checksum, extraction via `tar` système, `chmod`, et un run réel de `lynxor --version` puis `lynxor scan .` à travers `bin/lynxor.js` — tous corrects (darwin/arm64, la plateforme de développement).
- Self-scan (`./lynxor scan .`) après ajout de `npm/` et du job `publish-npm` : score inchangé (97/100), aucun nouveau finding.

## Conséquences

- Le nom de paquet npm `lynxor` était disponible (vérifié via `npm view lynxor`, 404 avant publication) — pas de collision de nom à gérer.
- Pas encore testé au moment de l'écriture initiale : un vrai `npm publish` (le job `publish-npm` n'avait pas été exercé pour de vrai, seul `install.js` avait été validé directement contre des assets réels — publier réellement sur le registre npm public depuis un tag jetable aurait pollué le registre de façon non supprimable, contrairement à une release GitHub).

## Mise à jour — `NPM_TOKEN` remplacé par le trusted publishing OIDC, avant le premier `npm publish` réel

Décision initiale (secret `NPM_TOKEN` à configurer côté GitHub) jamais exécutée : avant de le faire, npm a proposé le trusted publishing (OIDC) comme option à la configuration du package sur npmjs.com — pas de token longue durée à générer, stocker, ni faire tourner. Adopté à la place, aucun `NPM_TOKEN` n'existe ni n'existera dans les secrets de ce repo.

Mécaniquement (vérifié contre la doc npm réelle, pas supposé) : le job `publish-npm` déclare `permissions: id-token: write` (scopé à ce job, pas au workflow, même discipline que `contents: write` pour le job `release`) ; `actions/setup-node` passé de `@v4`/Node 20 à `@v7`/Node 22 (le trusted publishing exige npm CLI ≥ 11.5.1 et Node ≥ 22.14.0, Node 20 est sous les deux planchers) ; l'étape `Publish` n'a plus d'`env: NODE_AUTH_TOKEN` — l'authentification passe par le jeton OIDC, pas un secret. L'attestation de provenance est automatique sous trusted publishing, pas besoin du flag `--provenance`.
