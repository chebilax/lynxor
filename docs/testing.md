# Testing — corpus, exit criteria, where to find what

This file centralizes how Lynxor is validated against real repos, so this knowledge doesn't need rebuilding every phase (the full corpus was already lost once, in Phase 2, when an interrupted session wiped `/tmp`).

The design decisions themselves (why a time budget rather than a fixed depth, why severity isn't downgraded based on a path pattern...) live in `docs/decisions/` — this file doesn't touch that, it only points there.

## Corpus of 20 public repos (Phase 1)

Used to validate the MVP exit criterion. Shallow clones (`--depth 1`) are enough to test the working-tree scan — testing the git-history analyzer (Phase 2+) needs full clones (see below).

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

To rebuild it:

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

Repos with legitimate secret findings (test keys in `testdata/`/`fixtures/`, test `.env` files): axios, caddy, flask, gin, prometheus, requests. The rest scan clean. Useful for spotting a regression immediately: if one of the "clean" repos starts surfacing a finding, that's a false positive to investigate before merging, not a new true positive to celebrate.

`caddy`, `gin`, and `prometheus` have `.gitignore` files with negation patterns (`!file`) — useful for validating the `.gitignore` warning concretely instead of leaving it theoretical.

## Full clones (Phase 2+, git-history)

The git-history analyzer needs real history, not a shallow clone. Three representative sizes, already used to calibrate the time budget (see `docs/decisions/0002-git-history-depth.md`):

| Repo | Commits | Tracked files |
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

`prometheus` is the case that has revealed most of the real limitations so far (a massive vendor bump in its history, false positives in vendored code, a real Dockerfile with a `latest` tag and a `Dockerfile.distroless`) — it's the first repo to re-test whenever a change touches `githistory` or `docker`.

## Real Dockerfiles and workflows in the Phase 1 corpus

The 20-repo corpus also validates `docker` and `cicd` against real content, not just synthetic fixtures — 9 of the 20 have at least one real GitHub Actions workflow (axios, caddy, chalk, cobra, flask, gin, ohmyzsh, prometheus, requests; 57 `.yml` files in total). This is what revealed that both `gin/.github/workflows/codeql.yml` and `requests/.github/workflows/codeql-analysis.yml` contain `@main`/`@master` in a context that isn't an action reference (`branches: [main]`, a comment) — the empirical justification for structural YAML parsing rather than regex, see `docs/decisions/0005-cicd-analyzer-scope.md`.

## Built-in rules (secrets/docker/cicd): table-driven tests, since ADR 0014

The real corpus validates speed and the absence of false positives at scale; it doesn't protect against a regression on one specific rule (a tightened regex that breaks an exclusion, multi-stage Dockerfile logic that goes wrong). `analyzers/secrets/secrets_test.go`, `analyzers/docker/docker_test.go`, and `analyzers/cicd/cicd_test.go` now cover every rule and every false-positive exclusion documented in a comment (AWS's `EXAMPLE` suffix, a truncated PEM body, multi-stage `FROM builder`, the distroless `nonroot` tag, `ADD` of a URL/archive, `permissions` as a scoped map vs. `write-all`, a boolean secret check vs. a direct `echo`) — 95-97% coverage across the three packages. `analyzers/cicd/cicd_test.go` also covers `CheckDependabot`, the repo-level function called directly by `cli/scan.go`.

## Dependency Scanner: tested against the real OSV.dev API, no mock

`analyzers/dependencies` makes real network calls (`--deps`) — validated against the real `api.osv.dev` API, never mocked. Three deliberately old/vulnerable dependencies serve as reference fixtures for this code path: `golang.org/x/text@v0.3.0` (Go), `urllib3==1.24.1` (Python), `lodash@4.17.4` (npm, since ADR 0019) — all with several known CVEs and covering the three severity-mapping cases (a direct `database_specific.severity`, the CVSS heuristic, the Medium fallback — see `docs/decisions/0006-dependency-scanner-scope.md`).

`gin` (57 dependencies) and `prometheus` (1075 dependencies across 5 `go.sum` files) from the corpus also serve as a real scale test for this path — it was prometheus that revealed OSV's undocumented batch ceiling (max 1000 requests): without chunking, the dependency check failed entirely and silently on this repo, with a misleading message ("network unavailable").

Since ADR 0014, this exact bug also has an automated regression test: `TestQueryBatch_Chunking` (`analyzers/dependencies/osv_test.go`) replays the chunking logic against a fake `httptest.Server` that rejects any request over 1000 dependencies — `osvBatchURL`/`osvVulnURL` are `var`s (not `const`s) precisely to allow this, the only production code change motivated by testability. The rest of `osv.go`'s pure logic (alias dedup, severity mapping, the CVSS heuristic) is covered in the same file. The real OSV.dev API stays manually tested pre-release, with no mock — it's that real network test that found the batch ceiling in the first place; the HTTP fake only protects the chunking logic already discovered, it doesn't replace it as a method for discovering new bugs.

## Security Diff Mode: Git fixtures generated in Go, plus the corpus for performance

The 5 scenarios below were a manual checklist until ADR 0014 — they're now real automated tests in `analyzers/diffmode/diffmode_test.go`, against Git repos generated on the fly (`t.TempDir()` + `go-git`, no clone, no dependency on `/tmp`, which has already vanished once):
1. A secret added on the branch → `NEW` (`TestDiff_NewSecret`).
2. A secret removed on the branch → `FIXED` (`TestDiff_FixedSecret`).
3. A pre-existing issue on both branches, untouched by the branch → absent from the diff (`TestDiff_PreexistingIssueIsNotReported`).
4. An unchanged secret whose line number shifts → absent from the diff, no false `NEW`+`FIXED` (`TestDiff_LineShiftDoesNotFalselyReport`).
5. Two findings sharing the same key `(File, ID, Category)`, one removed → exactly 1 `FIXED` (`TestDiff_CountAwarePairing`).

`prometheus` (full clone, two distant version tags) stays the reference for the performance test on a real large repo — see `docs/benchmarks.md`. The real corpus and the generated fixtures don't replace each other: one validates performance/false positives against real content, the other validates the pure pairing logic deterministically and fast.

## Plugin System: a reference plugin in Python, not just Go

`docs/examples/reference-plugin.py` — written in Python, deliberately, to honestly validate the "the protocol has nothing Go-specific about it" promise instead of leaving it as an unverified claim in `docs/plugin-protocol.md`. Accepts a `--misbehave=timeout|crash|fatal|error` flag to replay each failure scenario below on demand — not meant for real plugin authors, only for the test suite.

Seven reference scenarios to reproduce if `analyzers/plugin` is touched:
1. Normal operation → the plugin's finding appears in the report, with `id` correctly prefixed by `plugin_name`.
2. Fatal error at handshake (`--misbehave=fatal`) → plugin ignored at load time, scan continues normally.
3. Non-fatal error on a file (`--misbehave=error`) → a warning for the affected file, plugin stays active for subsequent files.
4. Timeout (`--misbehave=timeout`, the script sleeps 30s) → abandoned after exactly 5s, no retry on subsequent files.
5. Crash (`--misbehave=crash`, `sys.exit(1)`) → detected immediately (EOF on stdout), no waiting for the timeout.
6. Two plugins at once, one crashing and one working → isolation confirmed, one crashing doesn't affect the other's findings.
7. `--plugin-arg reference-example:foo=bar` (ADR 0018) → the reference plugin's finding shows `context: configured args: {'foo': 'bar'}`, confirmed by a real run, not just read from the protocol. With no `--plugin-arg` at all, or one addressed to a different plugin name, `context` stays empty — both also verified.

`--plugin` takes an executable path, not a command with arguments (a real plugin is self-contained, it doesn't need CLI flags) — so testing each mode needs a small wrapper per mode rather than passing `--misbehave=X` directly:

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

## Measurable exit criteria (already validated)

- **Speed < 5s** (MVP exit criterion, vision.md): validated on the 20 Phase 1 corpus repos (max observed: ~1.5s, fastapi/svelte) and on the full clones in default mode (max observed: ~3s, prometheus — 1.5s git-history budget + working-tree scan + process overhead). `--full-history` is **not** subject to this criterion: it's an explicitly "no budget" mode, up to 18 minutes observed on prometheus (18k commits) — see `docs/decisions/0002-git-history-depth.md`.
- **Zero major false positives**: validated on the 20 Phase 1 repos after fixes (a `.pem`/`.key` extension confused with a certificate, a private-key regex matching a doc placeholder, an official example AWS key and a Google fixture in vendored code). What remains is a known, documented class (test keys in `testdata`/`fixtures`) — see `docs/decisions/0001-test-fixture-context.md` for why that's not removed, only annotated.
- **Time budget: per-analyzer, not global, but more indirect since ADR 0016.** `DefaultBudget` (1.5s) stays internal to `githistory.Scan()`. The working-tree scanner (`core.Scanner`) and `diffmode.scanTree()` now route every analyzer call through `core.RunAnalyzer`, which gives up waiting (log + skip, never a hang) if an analyzer doesn't respond within `AnalyzerTimeout` (5s, matched to the plugin protocol's budget). Before ADR 0016, this guardrail was entirely indirect ("every analyzer stays lightweight parsing, validated empirically each time one's added") — identified in an external architecture review as the project's most serious and most silent risk: nothing structurally prevented a future analyzer from blocking the whole scan. Verified with a fake "stuck" analyzer sleeping 10s in `core/timeout_test.go`: the full scan comes back in ~60ms with the normal analyzer's findings, not the stuck one's 10s.

## JSON output: validity + stdout/stderr isolation

`--format json` is validated on three points, all checked with real commands rather than by reading code:
1. The produced JSON is syntactically valid (piped through `python3 -m json.tool`).
2. `findings: []` on a clean repo, never `null` — a consumer shouldn't have to handle both cases.
3. Diagnostics (`.gitignore`, dependencies, git-history, plugins) stay on stderr, never mixed into stdout's JSON — verified by explicitly separating the two streams (`2>/dev/null` vs. `2>&1 1>/dev/null`), not assumed.

Reference fixture covering every field at once: a mini-repo with a test key added then removed in `testdata/` — produces a finding with a real `commit_hash` (git-history) *and* a real `context` (test/fixture path) at the same time, to confirm every field survives serialization correctly.

Since ADR 0014, this same multi-field fixture is also an automated golden file: `output/golden_test.go` defines `goldenFindings` (one finding per notable field: `commit_hash` set, `context` set, different severities, different categories), and `output/json_test.go`/`output/html_test.go` compare real output against `output/testdata/report.{json,html}.golden`. JSON syntactic validity is still checked directly (`encoding/json.Unmarshal`), not just via golden comparison. `go test ./output/... -update` regenerates the golden files after a deliberate schema change. Tested for real, not assumed: temporarily breaking a title in the HTML template makes `TestWriteHTMLReport_Golden` fail as expected — confirmed before writing this note.

## HTML dashboard: this project's first automated Go test, plus a structural render check

`core/scoring_test.go` — this project's first `_test.go` file, written specifically for an invariant the usual CLI check doesn't cover well: the guarantee that no finding is counted twice or dropped when `ComputeCategoryBreakdown` partitions by category. Two tests:
1. `TestComputeCategoryBreakdown_PartitionsWithoutDuplicationOrLoss` — uses `ComputeCategoryScore` as an oracle: every category score in the breakdown must match the one obtained by manually filtering that category's findings.
2. `TestComputeCategoryBreakdown_TotalIsNotAnAggregateOfCategoryScores` — a CRITICAL in one category, LOWs elsewhere; the total score must diverge clearly from the categories' naive average (35 vs. 78 on the chosen fixture), proof the total is never derived from the breakdown.

The generated HTML itself is validated in two steps, neither by simply reading code:
- Structurally well-formed: balanced tags. First checked manually with Python's stdlib HTML parser (`html.parser`); since ADR 0014, `assertBalancedHTML` (`output/html_test.go`) does the same check in Go on every `go test` run, with no external dependency.
- Visual rendering inspected via a temporarily published Artifact — this environment has no screenshot tool, so pixel-perfect verification (alignment, spacing) stays on human review; automation covers structure and color/status logic, not layout. The template's `generated <date>` timestamp is normalized before comparing against the golden file (`normalizeHTMLTimestamp`), otherwise every run would produce a diff on the one value that's supposed to change every run.

## GoReleaser: local `--snapshot` first, but validated by a real pushed tag

`.goreleaser.yaml` and `.github/workflows/release.yml` (ADR 0020) weren't judged ready on the sole basis of a local `goreleaser build --snapshot --clean` (6 binaries produced, `--version` correctly returns the `vX.Y.Z` form thanks to `{{.Tag}}`) — same standard as for the GitHub Action and the Homebrew tap: a disposable test tag (`v0.0.0-goreleaser-test`) was pushed for real, triggering a real `release.yml` run on GitHub Actions with a real `GITHUB_TOKEN`. The resulting release contained the 6 expected archives (`lynxor_<os>_<arch>.tar.gz`/`.zip`) plus a `checksums.txt` with the right SHA256s — verified via `gh release view`, not just assumed because the job went green. Test tag and release deleted after verification.

## npm: `install.js` validated against a real release, `npm publish` untested (an accepted limitation)

`npm/scripts/install.js` (ADR 0021) was actually run (not just reviewed) against a second disposable tag (`v0.0.0-npm-test`, pushed then deleted after verification): downloading the real GitHub Release assets, verifying the SHA256 against `checksums.txt`, extracting via the system `tar`, `chmod`, then a real `lynxor --version` and `lynxor scan .` run through `bin/lynxor.js` — all correct on darwin/arm64 (the only platform testable in this dev environment).

Explicitly accepted limitation, not hidden: the `publish-npm` job itself (`.github/workflows/release.yml`) hadn't been exercised for real. Actually publishing to the public npm registry isn't as clean an operation to undo (`npm unpublish` has restrictions, unlike `gh release delete`) — so the next real tag would be the first real test of `publish-npm`.

**Update**: `NPM_TOKEN` was in the end never configured — replaced by npm trusted publishing (OIDC) before the first real attempt (see ADR 0021's update). `id-token: write` scoped to the job, `actions/setup-node` bumped to `@v7`/Node 24, no more `NODE_AUTH_TOKEN`.

**Update 2 — the first real run failed, root cause found via two disposable debug tags, not by theory**: `v1.1.1` triggered `publish-npm` for real → `404`. A disposable tag with a temporary diagnostic step (printing `.npmrc`/`npm whoami`) showed the real cause: `actions/setup-node@v7` writes `//registry.npmjs.org/:_authToken=${NODE_AUTH_TOKEN}` into `.npmrc` as soon as `registry-url` is set, even with no secret — npm thinks it has a token, never attempts OIDC (`401` on `whoami`, `404` on `publish`). Fix: stop setting `registry-url` (npm's default registry is already `registry.npmjs.org`). A second disposable tag after the fix confirmed authentication now succeeds: a different, expected error (a prerelease with no `--tag`, an artifact of the test tag's name, not an issue for a real `X.Y.Z`). Full detail: ADR 0021's update.

**Update 3 — `v1.1.2`: success, after a cause outside this repo**: `ENEEDAUTH` (no auth attempt at all) on the first try — the Trusted Publisher config on npmjs.com had been filled in but never saved. Once saved and the job re-run with no code change, `publish-npm` succeeded: `lynxor@1.1.2` published for real, `published ... by GitHub Actions` (not a human token). Full clean-environment validation, all three channels: `go install .../lynxor@v1.1.2` (resolves, `--version` = `dev`, an already-documented limitation), `npm install -g lynxor` (`--version` = `v1.1.2`), `brew tap chebilax/lynxor && brew install lynxor` + `brew test` (formula repointed at `v1.1.2`, all green). npm/OIDC distribution closed out.

## GitHub account rename xchebila → chebilax (2026-07-26)

No branding decision to trace (unlike RepoAudit→RepoScan and RepoScan→Lynxor) — just a GitHub handle change, so no dedicated ADR, only this technical marker. Inventory (`git grep -lI -i "xchebila"`) done and confirmed before any replacement: 33 files renamed (code, `go.mod`, `action.yml`, `.goreleaser.yaml`, `npm/`, docs describing the current state), 2 files deliberately left untouched — `docs/decisions/0022-rename-reposcan-to-lynxor.md` in full, and 3 lines of `docs/roadmap-long-term.md` — because they narrate events that actually happened under the `xchebila` account (quoted error messages, real commands), not prose about the project's current state.

Validated after the replacement: `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` all green under the new module path `github.com/chebilax/lynxor`; a real binary built and run (`scan .`, score unchanged at 97/100, same known finding). Left to do, tracked separately: a new tag under the renamed module (no existing tag resolves under `chebilax` — same technical constraint as every previous rename), npm republish, Homebrew formula repointed.

## Step 7 — extended test coverage (`plugin`, `githistory`, `dependencies`, `core`)

Explicitly targeted, as planned in the sequenced plan: `plugin` (0% before, no test since step 2), `githistory` (63.4%, step 1), `dependencies` (54.7%, step 3), and `core` (65.7%, `RepoAnalyzer` added in step 1).

- **`analyzers/plugin`: 0% → 92.2%**. The real obstacle was that `Load` shells out to a real executable (`exec.Command`) — no way to exercise the handshake/protocol without a real process on the other end. Solved with `testdata/fakeplugin/`, a small Go program built once (`TestMain`, `go build`) whose behavior is driven by the `LYNXOR_FAKE_PLUGIN_SCENARIO` environment variable (inherited by `exec.Command` — `Load` doesn't set `cmd.Env`): a failed handshake (3 variants + a crash), a valid result, ID prefixing and the default category fallback, every `abandon` mode (invalid JSON, unexpected type, wrong path, fatal error, crash, a real 5s timeout — deliberately slow, no way to shorten it without touching production code), a non-fatal error that does *not* abandon the plugin, and `Configure`. The one deliberately slow test (~5s) waits out the real `requestTimeout` rather than simulating it.
- **`analyzers/githistory`: 63.4% → 83.9%**. `countReachableCommits` and `scanDangling` had no direct test. The latter needed a real "dangling" commit (physically present, unreachable from any ref) — simulated by forcing the branch ref backward via `repo.Storer.SetReference`, without ever calling `git gc`, exactly like a deleted branch would leave one on disk.
- **`analyzers/dependencies`: 54.7% → 69.9%**. `Discover` (the extension point introduced in step 3) had never been tested directly — only the individual parsers were. Added: `Discover` with real files on disk (dispatch, relative manifest paths, `.git`/vendored exclusion), plus the `Matches`/`Parse` methods of both wrappers (`goSumParser`, `requirementsTxtParser`) and `parseRequirementsTxt` itself, previously uncovered. `osv.go`'s network functions stay deliberately under-covered — consistent with `[[project_osv_rate_limiting_watch]]`, not urgent to add more real API calls to the test suite.
- **`core`: 65.7% → 92.6%**. No `scanner_test.go` existed before — `Scanner.Scan`, `Warnings`, `IsBinary`, `IsVendoredPath`, and the homegrown `.gitignore` matcher were only ever exercised indirectly through other packages. Added directly in `core`: `.git`/vendor/node_modules exclusion, oversized/binary files, real respect for a `.gitignore` (simple patterns, warnings on unsupported negation/`**`), and the fact that several registered analyzers all see the same file.

**Not handled here, tracked separately**: `cli` (10.2%) and `output` (33.3%) stay low but weren't explicitly named in step 7's plan — they need a different testing style (Cobra/IO integration rather than unit fixtures), not added in this same effort to avoid mixing the two approaches.

**Update**: `cli/diff.go` (0% on `newDiffCmd`, no dedicated test — only `parsePluginArgs`, a pure function, was) filled in afterward (`cli`: 10.2% → 19.5%). The real obstacle was `os.Exit(1)` called directly in `RunE` (the same limitation `cli/scan.go` has too, but stays untested on this exact point): impossible to exercise in-process without killing the whole `go test` run. Solved with the standard Go idiom for this (used by `os/exec`'s own tests): re-exec the test binary itself as a subprocess, targeted at a `TestHelperProcess_Diff` function that isn't a real test (a no-op unless a dedicated environment variable is set), and inspect the subprocess's real exit code. The other paths (invalid repo, wrong argument count, a diff with no new finding) are tested directly in-process, no such detour needed, since they never go through `os.Exit`.

## Issue #27 — introduce/remove cross-referencing via `Context`, re-validated against a real repo

The fix ([ADR 0023](decisions/0023-githistory-context-crossref.md)) was re-validated against the same real `gin-gonic/gin` clone that originally confirmed the problem (not just against the new synthetic test fixtures): the repo's 3 real introduce+remove pairs now carry the right commit cross-reference in `Finding.Context`, and `testdata/certificate/key.pem` (still present in the current working tree — a different situation, not a clean pair) correctly gets no `Context` added. CLI rendering checked on a small local fixture repo: the cross-reference displays correctly on both sides with no change to `output/cli.go`/`output/html.go`.

## Where the numbers are

`docs/benchmarks.md` — an append-only table, one run per phase/PR. This file says *what* is tested and *why*; benchmarks.md says *what was measured, when*.
