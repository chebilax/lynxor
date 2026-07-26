# ADR 0021 - Package npm de distribution : façade + postinstall, pas de sous-paquets par plateforme

## Statut

Accepté (2026-07-24).

## Contexte

Étape 5 de la suite séquencée, dépendante de l'étape 4 (GoReleaser, ADR 0020) : `lynxor` doit être installable via `npm install -g lynxor` sans que l'utilisateur ait Go installé. Scope et approche confirmés par l'utilisateur avant de coder : package comme sous-répertoire de ce même repo (`npm/`), publication automatisée sur `npm` déclenchée par tag, via un secret `NPM_TOKEN` configuré côté GitHub.

## Décision : script `postinstall` qui télécharge depuis GitHub Releases, pas des sous-paquets `optionalDependencies` par plateforme

L'alternative "moderne" (esbuild, swc, turbo, sharp) publie un paquet par combinaison OS/arch et un paquet façade qui les liste en `optionalDependencies` - npm n'installe que celui qui correspond, sans script `postinstall` ni téléchargement réseau custom. C'est objectivement plus robuste (fonctionne derrière un miroir npm d'entreprise sans accès à GitHub, pas de code de téléchargement/vérification à maintenir).

Non retenu ici : le plan de l'utilisateur spécifiait explicitement "un script postinstall qui télécharge le bon binaire précompilé" - une décision déjà prise, pas rouverte sans besoin identifié. Le compromis (un point de défaillance réseau supplémentaire vers GitHub à l'installation) est jugé acceptable pour un outil qui, une fois installé, ne fait plus aucun appel réseau non explicitement demandé (`--deps`).

## Décision : aucune dépendance npm - extraction via le `tar` système

`bin/lynxor.js` (le shim exécuté par `npm`) et `scripts/install.js` (le `postinstall`) n'ont ni l'un ni l'autre de `dependencies` dans `package.json`. Le téléchargement utilise `fetch` global (disponible nativement depuis Node 18, d'où `engines.node: ">=18"`). L'extraction shell-out vers le `tar` système plutôt que d'ajouter un paquet JS (`tar` pour le `.tar.gz`, un second comme `yauzl` pour le `.zip` Windows) :

- Linux/macOS : le `.tar.gz` s'extrait avec n'importe quel `tar` (GNU ou BSD).
- Windows : le `.zip` s'extrait avec le `tar.exe` livré en standard depuis Windows 10 1803 (bsdtar, qui gère le zip en plus du tar) - `tar -xf archive.zip` fonctionne sans rien installer.

Zéro dépendance npm pour un paquet dont le seul rôle est de télécharger un binaire - cohérent avec la philosophie "zéro dépendance" du binaire Go lui-même. Risque assumé : un environnement Windows exotique sans `tar.exe` (image container minimaliste, très ancienne version) ferait échouer l'installation avec une erreur claire plutôt que silencieusement.

## Décision : vérification de checksum avant extraction, pas seulement un téléchargement direct

`install.js` télécharge aussi `checksums.txt` (généré par GoReleaser, ADR 0020) et compare le SHA256 de l'archive téléchargée avant de l'extraire ou de l'exécuter. Pour un outil dont toute la raison d'être est la sécurité de la chaîne d'approvisionnement logicielle, télécharger et exécuter un binaire sans vérifier son intégrité aurait été une incohérence de fond, pas un simple oubli - décidé dès la conception, pas ajouté après coup.

## Décision : shim `bin/lynxor.js` fixe qui exec le binaire téléchargé, pas un `bin` pointant directement sur un fichier téléchargé

`package.json.bin` pointe vers `bin/lynxor.js`, committé, toujours présent. Le vrai binaire atterrit dans `npm/.bin/` (gitignoré) après le `postinstall`. Évite tout problème d'ordre entre la création des liens `bin` de npm et l'exécution du `postinstall` - le fichier que `bin` référence existe dès la publication du paquet, qu'il ait ou non déjà téléchargé le binaire réel.

## Décision : synchronisation de version côté CI, pas dans le fichier commité

`npm/package.json` committe un placeholder `"version": "0.0.0"`. Le job `publish-npm` (ajouté à `.github/workflows/release.yml`, `needs: release`) exécute `npm version --no-git-tag-version "${GITHUB_REF_NAME#v}"` juste avant `npm publish` - la version npm est toujours celle du tag Git qui a déclenché le run, sans le préfixe `v` (npm n'accepte pas de préfixe dans un numéro de version semver, contrairement à toutes les autres conventions de ce projet - voir ADR 0013).

## Décision : `publish-npm` dépend (`needs:`) du job `release`, dans le même workflow

Les deux jobs se déclenchent sur le même `push: tags: v*`. Sans `needs: release`, `publish-npm` pourrait publier avant que GoReleaser ait fini d'uploader les assets - un `npm install` juste après une publication échouerait alors de façon transitoire (le `postinstall` chercherait un binaire pas encore là). `needs: release` élimine cette fenêtre de course. Permissions par job, pas au niveau du workflow : `release` a besoin de `contents: write` (créer la release), `publish-npm` n'a besoin que de `contents: read` (checkout) - le défaut du workflow est passé à `read`, chaque job déclare explicitement ce dont il a besoin.

## Validé, pas juste écrit

- `goreleaser build`/`release --snapshot` (étape 4) avait déjà prouvé les binaires eux-mêmes ; ici, un second tag jetable (`v0.0.0-npm-test`, poussé puis supprimé après vérification, même discipline que ADR 0020) a produit une vraie release avec de vrais assets, contre laquelle `node scripts/install.js` (avec `LYNXOR_INSTALL_VERSION` pour cibler ce tag sans modifier `package.json`) a été exécuté réellement : téléchargement, vérification de checksum, extraction via `tar` système, `chmod`, et un run réel de `lynxor --version` puis `lynxor scan .` à travers `bin/lynxor.js` - tous corrects (darwin/arm64, la plateforme de développement).
- Self-scan (`./lynxor scan .`) après ajout de `npm/` et du job `publish-npm` : score inchangé (97/100), aucun nouveau finding.

## Conséquences

- Le nom de paquet npm `lynxor` était disponible (vérifié via `npm view lynxor`, 404 avant publication) - pas de collision de nom à gérer.
- Pas encore testé au moment de l'écriture initiale : un vrai `npm publish` (le job `publish-npm` n'avait pas été exercé pour de vrai, seul `install.js` avait été validé directement contre des assets réels - publier réellement sur le registre npm public depuis un tag jetable aurait pollué le registre de façon non supprimable, contrairement à une release GitHub).

## Mise à jour - `NPM_TOKEN` remplacé par le trusted publishing OIDC, avant le premier `npm publish` réel

Décision initiale (secret `NPM_TOKEN` à configurer côté GitHub) jamais exécutée : avant de le faire, npm a proposé le trusted publishing (OIDC) comme option à la configuration du package sur npmjs.com - pas de token longue durée à générer, stocker, ni faire tourner. Adopté à la place, aucun `NPM_TOKEN` n'existe ni n'existera dans les secrets de ce repo.

Mécaniquement (vérifié contre la doc npm réelle, pas supposé) : le job `publish-npm` déclare `permissions: id-token: write` (scopé à ce job, pas au workflow, même discipline que `contents: write` pour le job `release`) ; `actions/setup-node` passé de `@v4`/Node 20 à `@v7`/Node 24 (Active LTS, support jusqu'à avril 2028 ; le trusted publishing exige npm CLI ≥ 11.5.1 et Node ≥ 22.14.0, Node 20 est sous les deux planchers, Node 24 les dépasse largement) ; l'étape `Publish` n'a plus d'`env: NODE_AUTH_TOKEN` - l'authentification passe par le jeton OIDC, pas un secret. L'attestation de provenance est automatique sous trusted publishing, pas besoin du flag `--provenance`.

**Plancher npm CLI, garanti explicitement, pas supposé** : la version d'npm fournie par défaut avec `actions/setup-node`/Node n'est pas documentée par les notes de release Node elles-mêmes (vérifié : pas de mention du npm embarqué dans les notes de la 22.22.0) - pas de raison de lui faire confiance à l'aveugle pour un plancher aussi précis (11.5.1). Étape `npm install -g npm@11` ajoutée explicitement avant `npm version`/`npm publish` : suit la ligne majeure 11.x (comme `node-version`/`go-version` ailleurs dans ce repo - pas de maintenance manuelle pour chaque patch), sans sauter sur `@latest`/npm 12 (sorti ~2 semaines avant cette décision, a changé les défauts de sécurité des scripts d'installation - pas encore assez éprouvé pour ce pipeline).

## Mise à jour - premier run réel de `publish-npm` : 404, root cause trouvée par un vrai run de debug, pas par lecture de doc

Le premier vrai tag (`v1.1.1`) a bien déclenché `publish-npm` pour de vrai - et il a échoué : `npm error 404 Not Found - PUT https://registry.npmjs.org/lynxor`. Investigation, dans l'ordre :

1. **Hypothèse écartée** : `repository.url` de `npm/package.json` pointerait encore vers l'ancienne URL `xchebila` (le paquet `lynxor@1.1.0`, publié manuellement par l'utilisateur comme bootstrap avant que l'OIDC ne soit prêt, affichait bien l'ancienne URL sur le registre) - vérifié directement sur le commit du tag `v1.1.1` (`git show v1.1.1:npm/package.json`) : `repository.url`/`homepage`/`bugs` étaient déjà corrects (`chebilax/lynxor`). La métadonnée périmée sur le registre est un artefact du publish manuel antérieur, pas un bug de ce repo.
2. **Root cause trouvée par un vrai tag jetable de debug** (`v0.0.0-npm-oidc-debug`, avec une étape de diagnostic temporaire imprimant `.npmrc` et `npm whoami`), pas par une théorie non vérifiée : `actions/setup-node@v7`, quand `registry-url` est renseigné, écrit quand même `//registry.npmjs.org/:_authToken=${NODE_AUTH_TOKEN}` dans `.npmrc` - même sans aucun `NODE_AUTH_TOKEN` dans l'environnement du job. npm lit cette config non résolue, croit qu'une authentification par token est configurée, et ne tente jamais la négociation OIDC : `401 Unauthorized` sur `npm whoami`, `404` sur `npm publish` (le message générique "not found or no permission" de npm pour un accès en écriture refusé). `actions/setup-node#1551` documente le même symptôme (fermé comme doublon) ; le changelog de la v7.0.0 prétend avoir supprimé ce comportement ("Remove dummy NODE_AUTH_TOKEN export"), mais le `.npmrc` réel de notre propre run contenait toujours la ligne verbatim - pas de nouvelle version corrigée disponible au moment de cette entrée.
3. **Correctif** : ne plus renseigner `registry-url` du tout sur l'étape `Set up Node`. `registry.npmjs.org` est déjà le registre par défaut compilé dans npm - rien d'autre ne change, seule la ligne `_authToken` cassée disparaît.
4. **Validé par un second tag jetable** (`v0.0.0-npm-oidc-fix-test`) après le correctif : erreur différente et attendue - `You must specify a tag using --tag when publishing a prerelease version` (npm refuse de taguer `latest` une version avec suffixe de prérelease type `0.0.0-npm-oidc-fix-test` ; artefact du nom de tag de test jetable, pas un problème pour un vrai tag `X.Y.Z` sans suffixe comme `v1.1.1`/`v1.1.2`). Passer ce point confirme que l'authentification OIDC elle-même fonctionne désormais.

Les deux tags jetables et leurs releases GitHub générées ont été supprimés après vérification, même discipline que pour ADR 0020.

## Mise à jour - `v1.1.2` : `ENEEDAUTH`, puis résolu - la cause n'était pas dans ce repo

Le tag suivant (`v1.1.2`, coupé pour porter le correctif ci-dessus) a d'abord échoué différemment : `npm error code ENEEDAUTH` / `need auth This command requires you to be logged in` - npm ne tentait même plus l'OIDC du tout, cette fois. En creusant en parallèle, l'utilisateur a réalisé que la configuration du Trusted Publisher sur npmjs.com n'avait pas été **enregistrée** (juste renseignée dans le formulaire, jamais sauvegardée) - donc aucune configuration Trusted Publisher n'existait réellement côté npm au moment du run, quoi que ce repo fasse. Une fois la sauvegarde faite et le job relancé (sans changement de code), `publish-npm` a réussi pour de vrai : `lynxor@1.1.2` publié sur le registre npm public, `published ... by GitHub Actions <npm-oidc-no-reply@github.com>` (pas un token humain), métadonnées correctement `chebilax/lynxor`.

Leçon distincte de celle du `.npmrc`/`_authToken` : toutes les causes d'un échec OIDC ne sont pas dans ce repo - une configuration non sauvegardée côté npmjs.com produit un symptôme différent (`ENEEDAUTH`, aucune tentative d'auth) de celui d'un `.npmrc` cassé (`401`/`404`, tentative d'auth avec un token invalide). Utile pour la prochaine fois que ce pipeline échoue : regarder d'abord *quel type* d'erreur avant de re-creuser le code.

**Validation complète, environnement propre, les trois canaux** : `go install github.com/chebilax/lynxor@v1.1.2` (résout, `--version` affiche `dev` - limite déjà documentée du `go install` sans ldflags, pas un bug) ; `npm install -g lynxor` (télécharge le bon binaire précompilé, `--version` affiche correctement `v1.1.2`) ; `brew tap chebilax/lynxor && brew install lynxor` + `brew test` (formule repointée sur `v1.1.2`, les trois verts). Distribution npm/OIDC considérée close.
