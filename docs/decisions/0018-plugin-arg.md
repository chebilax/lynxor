# ADR 0018 — `--plugin-arg` : adressé par `plugin_name`, message `configure` dédié

## Statut

Accepté (2026-07-24).

## Contexte

Un plugin ne pouvait recevoir de configuration qu'via l'environnement ou un fichier qu'il découvre lui-même — aucun mécanisme direct côté RepoScan. Étape 2 d'une suite séquencée (après ADR 0017), avant le support npm.

## Décision : adressé par `plugin_name`, pas par position

`--plugin` est répétable ; plusieurs plugins peuvent être chargés dans un même scan. Trois designs envisagés :
- **Positionnel** (associer chaque `--plugin-arg` au `--plugin` le plus proche dans l'ordre des flags) : Cobra/pflag ne garantit aucun ordre d'entrelacement fiable entre deux flags répétables différents — fragile par construction, pas juste en théorie.
- **Uniforme** (les mêmes args pour tous les plugins chargés) : simple, mais aucun moyen de donner une config distincte à deux plugins chargés simultanément.
- **Adressé par nom** (`--plugin-arg <plugin_name>:<key>=<value>`, `plugin_name` étant celui auto-déclaré par le plugin à son `hello_ack`) : aucune ambiguïté d'ordre, scale à n'importe quel nombre de plugins. Coût : l'utilisateur doit déjà connaître le `plugin_name` du plugin — déjà documenté comme quelque chose qu'un auteur de plugin doit annoncer (`docs/plugin-protocol.md`).

Choisi : adressé par nom.

## Décision : un message `configure` dédié, pas un champ dans `hello`

Le host ne connaît pas encore le `plugin_name` du plugin au moment d'envoyer `hello` (le plugin ne le révèle que dans sa réponse `hello_ack`) — impossible de cibler des args par nom à ce stade. Un nouveau type de message `configure` (host → plugin), envoyé une fois juste après le handshake, avant tout message `file` :

```json
{"type": "configure", "args": {"api-key": "xyz", "mode": "strict"}}
```

Envoyé **seulement si** l'utilisateur a passé au moins un `--plugin-arg` adressé à ce `plugin_name` — sinon, ce type de message n'existe simplement pas pour ce plugin, rétrocompatible avec tout plugin existant qui l'ignorerait. Fire-and-forget : aucune réponse attendue ni lue — un plugin qui n'a aucun usage pour la configuration est libre d'ignorer ce message entièrement, lire `configure` est optionnel, pas une nouvelle étape obligatoire du handshake.

**Pas un ajout de champ à `hello`** (`--plugin`, comme au-dessus, ne le permettrait pas de toute façon) **ni dans `file`** (redondant sur chaque message pour une information qui n'a besoin d'être dite qu'une fois).

## Décision : `Plugin.Configure()` suit exactement le même funnel d'échec que `Run()`

Un échec d'écriture du message `configure` appelle `abandon()` — même chemin unifié que tout autre échec de plugin (ADR du protocole initial) : warning loggé, process tué si vivant, plugin ignoré pour le reste du scan. `cli/scan.go` traite un échec de `Configure()` exactement comme un échec de `Load()` : warning, `Close()`, le plugin n'est ajouté ni à la liste d'analyzers ni à la liste à fermer en fin de scan.

## Conséquences

- Pas de bump de `protocol_version` (encore "1.0") : ajout strictement additif, un plugin qui ne reçoit jamais ce message se comporte exactement comme avant.
- `docs/plugin-protocol.md` mis à jour (lifecycle + nouveau message), `docs/examples/reference-plugin.py` étendu pour lire `configure` et exposer les args reçus dans le `context` de ses findings — pas juste documenté, vérifié par un vrai run : `--plugin-arg reference-example:foo=bar` produit `context: configured args: {'foo': 'bar'}`, l'absence de `--plugin-arg` et un `--plugin-arg` adressé à un autre nom laissent tous deux le `context` vide.
- Premier test du package `cli` (`parsePluginArgs`, pure, aucune raison de ne pas le couvrir immédiatement). `Plugin.Configure()` lui-même et le reste du package `plugin` (0% de couverture) restent hors scope de cette PR — étape 7, comme prévu, où la question "comment tester un vrai subprocess sans dépendre de python3 dans la suite Go" mérite sa propre décision plutôt qu'une réponse rapide ici.
