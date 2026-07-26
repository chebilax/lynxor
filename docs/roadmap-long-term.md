# 🌍 Long-term roadmap — Lynxor

**A deliberate stop, not just a "to be continued"**: since the GitHub Action and the CI integrations, the project is considered stable — v1.0 + GitHub Action + CI integrations is the reference state, until a real friction point or a real request shows up. That applies to any item on this list (plugin marketplace, VSCode extension, SaaS) or to an idea that isn't even on it yet. This isn't a temporary pause while waiting to work through each item in order "whenever there's time" — it's a deliberate choice not to advance this list by default, and to only pick it back up on a real signal, not on the roadmap's own inertia. Homebrew tap distribution, added after this stop, is exactly that kind of real signal — an explicit request, not the list resuming by default.

This document covers what comes after v1.0 (Phases 1-5, see `vision.md`).

**A difference in nature from vision.md**: Phases 1-5 each had a clear technical scope, a measurable exit criterion, and an order imposed by their dependencies on each other. Nothing here starts with that level of certainty — these are directions, not commitments. Two items (GitHub Action, multi-platform CI) were mature enough to be scoped as real phases, and are now done. The plugin marketplace and the VSCode extension are paused, for lack of a real identified need for either — not just "not yet audited," a deliberate distinction so as not to imply a design audit is simply the next logical step. The pause criterion doesn't depend on a feature's risk level (the marketplace is risky, the VSCode extension isn't): only the existence of a real need counts, otherwise the exact drift the vision.md Non-Goals exist to prevent happens here too, just more slowly. SaaS stays at the "direction + question to settle before starting" level.

---

## ✅ Done — Official GitHub Action

### MVP scope

```yaml
- uses: lynxor/action@v1
  with:
    fail-on-new: true    # uses `lynxor diff` if base-ref/head-ref are detected (PR), otherwise `lynxor scan`
```

- Composite action (no custom runtime): installs the already-existing `lynxor` binary, runs it, exposes the exit code.
- On a pull request: uses `lynxor diff <base> <head>` (Phase 3, already shipped) — the mode built precisely for this use case.
- Outside a PR (push to a branch): falls back to `lynxor scan . --format json`, published as a build artifact.
- No new CLI-side feature — this work is pure packaging around what already exists (`diff`, `--format json`).

### Why this isn't a design audit like Phase 4

Unlike the Plugin System, there's no untrusted third-party code to isolate here — the action runs the official binary, published by the project's maintainer, in the CI environment of a user who explicitly chose it. No new risk surface to settle before coding.

### Exit criterion — validated

- `fail-on-new: true` fails the GitHub check exactly like `lynxor diff` does locally (same exit code, same semantics) — verified by a real CI run, not just locally.
- Tested against this very repo under real CI conditions via `.github/workflows/lynxor-self-check.yml` (all three paths: PR/diff, push/scan, `workflow_dispatch`) — two real bugs found and fixed along the way (ephemeral merge SHA, sum.golang.org freshness). See [docs/decisions/0011-github-action.md](decisions/0011-github-action.md).

---

## ✅ Done — Multi-platform CI integration (GitLab, Jenkins)

Documented snippets (`docs/ci-integrations.md`), not a published artifact (no GitLab CI/CD Component, no Jenkins Shared Library) — decision and reasoning in [docs/decisions/0012-multi-ci-integrations.md](decisions/0012-multi-ci-integrations.md). Unlike the GitHub Action, neither the GitLab snippet nor the Jenkins snippet has run on a real instance — an assumed and stated validation gap, not the same "tested in real CI" guarantee.

---

## ✅ Done — Homebrew tap distribution

**Didn't appear in either roadmap document before this entry** — neither `vision.md` nor this file. Added explicitly here before any code, so the decision wouldn't get lost the way Phase 1's test corpus nearly was.

**Goal**: install `lynxor` without cloning the repo or having Go pre-installed manually — Homebrew handles the build dependency itself. Works on Linux and macOS (Homebrew, not just macOS).

**Scope**: a separate repo, [chebilax/homebrew-lynxor](https://github.com/chebilax/homebrew-lynxor) (Homebrew convention), a single `Formula/lynxor.rb` file. The formula points at a published tag's tarball, built with `go build` (`depends_on "go" => :build`) — no precompiled binaries for this path, even once GoReleaser arrived later (see below): a reconfirmed decision, not just an oversight.

**Prerequisite discovered before coding**: `--version` didn't exist on the binary (verified empirically: `unknown flag: --version`) — added as part of this same work rather than after the fact (ADR 0013).

**Gap discovered along the way, then fixed**: the formula first pointed at `v1.0.0`, which predates `--version` — so `--version` stayed meaningless for anyone installing Lynxor via `go install` or the formula, not just for the formula's own test. Fixed by cutting `v1.0.1` right after this PR merged, repointing the formula at it (`sha256` recomputed, tested locally again), and updating the README (`go install ...@v1.0.1`). The formula's `test do` now actually checks `lynxor --version`, plus the `scan --help` fallback.

**Validated, not just written**: `brew tap` + `brew install --build-from-source` + `brew test` actually run locally before pushing the formula — all three green. Install command documented in the main README, next to `go install` (which also didn't exist as a direct user-facing instruction before this same entry — added along the way).

**Rename RepoAudit → RepoScan (before publication, 2026-07-24)**: the tap moved to `xchebila/homebrew-reposcan`, `Formula/reposcan.rb`, pointing at `v1.0.2` — the first tag cut under the renamed module (`v1.0.0` and `v1.0.1` still declared `github.com/xchebila/repoaudit` in their `go.mod`, so they were incompatible with `go install github.com/xchebila/reposcan@...` no matter which tag was requested, independent of any GitHub redirect — verified empirically: `go install ...@v1.0.1` failed with `module declares its path as: github.com/xchebila/repoaudit but was required as: github.com/xchebila/reposcan`). The same triple local validation was redone on the new formula before pushing it. The old `homebrew-repoaudit` tap still needed deleting — blocked at the time: this environment's `gh` token didn't have the `delete_repo` scope, deleted manually since.

**Rename RepoScan → Lynxor**: same mechanics, same technical constraint (the tap and the module need to be repointed under a new tag). Details: [dedicated entry below](#-done--rename-reposcan--lynxor) and [ADR 0022](decisions/0022-rename-reposcan-to-lynxor.md).

---

## ✅ Done — GoReleaser (precompiled binaries)

**Goal**: publish precompiled binaries (Linux/macOS/Windows, amd64/arm64) as GitHub Release assets on every tag — needed for the future npm distribution package (a postinstall that downloads the right binary based on `process.platform`/`process.arch`, not yet done at the time of this entry), without depending on `go install` or a local build.

**Scope**: `.goreleaser.yaml` + `.github/workflows/release.yml`, triggered on a `v*` tag push. `CGO_ENABLED=0` (no cgo dependency in this project, verified with a real build before writing the config). GitHub Release binaries only — no Docker, no automatic publish to the Homebrew tap, no `.deb`/`.rpm`: the tap stays hand-managed (see previous entry), same reasoning as the GitHub Action (ADR 0011).

**Validated, not just written**: `goreleaser build`/`release --snapshot` locally first, then a real disposable tag (`v0.0.0-goreleaser-test`) pushed to trigger `release.yml` for real on GitHub Actions — the resulting release contained the 6 expected archives (`lynxor_<os>_<arch>.tar.gz`/`.zip`) plus `checksums.txt`, before the test tag and release were deleted.

Details and alternatives considered: [ADR 0020](decisions/0020-goreleaser.md).

---

## ✅ Done — npm distribution package

**Goal**: `npm install -g lynxor` with no Go installed, building on the precompiled binaries from the previous entry.

**Scope**: a `npm/` subdirectory of this repo (`package.json`, `bin/lynxor.js`, `scripts/install.js`). `postinstall` downloads the GitHub Release archive matching `process.platform`/`process.arch`, verifies its SHA256 against `checksums.txt`, extracts it via the system `tar` (zero npm dependencies — bsdtar on Windows since 10 1803 handles zip too). Explicitly rejected: per-platform `optionalDependencies` sub-packages (the esbuild/swc pattern) — more robust, but not what the plan asked for, not reopened without an identified need. Published via a `publish-npm` job (`.github/workflows/release.yml`, `needs: release`) that sets the version from the tag, then `npm publish`.

**Validated, not just written — with an accepted limitation**: `scripts/install.js` actually run against a second disposable tag (`v0.0.0-npm-test`, deleted after verification) — download, checksum verification, extraction, and a real `reposcan scan .` through `bin/reposcan.js` (era-appropriate names, before the Lynxor rename), all correct. **Not tested at the time of this entry**: the `publish-npm` job itself — neither the `NPM_TOKEN` secret (not yet configured on GitHub) nor a real `npm publish` had been exercised.

**Update**: the first real tag (`v1.1.0`, under the `reposcan` name) confirmed the expected failure — `publish-npm` did fail for lack of `NPM_TOKEN`, with no side effects, while the `release`/GoReleaser job succeeded normally (6 real assets published). The first real `npm publish` therefore still hadn't happened yet, neither under `reposcan` nor under `lynxor` — it would happen directly under `lynxor` once `NPM_TOKEN` was configured and a new tag cut under the renamed module (see [ADR 0022](decisions/0022-rename-reposcan-to-lynxor.md)).

**Final update — trusted publishing (OIDC) instead of `NPM_TOKEN`, two real bugs found and fixed, distribution closed out**: `NPM_TOKEN` was in the end never configured, replaced by npm trusted publishing. Two real failures before the first success, each with its own real cause: (1) `actions/setup-node@v7` was writing a broken `_authToken` line into `.npmrc` as soon as `registry-url` was set, preventing any OIDC attempt at all (`401`/`404`) — fixed by no longer setting `registry-url`; (2) the Trusted Publisher config on npmjs.com had been filled in but never saved on npm's side (`ENEEDAUTH`) — not a bug in this repo. Once both were fixed, `v1.1.2` published `lynxor@1.1.2` for real (`published ... by GitHub Actions`, not a human token). Final validation in a clean environment across all three channels: `go install`, `npm install -g lynxor`, `brew tap chebilax/lynxor && brew install lynxor` — all green.

Details and alternatives considered: [ADR 0021](decisions/0021-npm-distribution.md).

---

## ✅ Done — Rename RepoScan → Lynxor

**Context, different from the first rename**: RepoAudit → RepoScan had been deliberately timed *before* any publication. This one happens after: `action.yml` is a usable GitHub Action, the `homebrew-reposcan` tap exists, and `v1.1.0` is this project's first real GitHub Release. Flagged explicitly before coding; started anyway, on confirmation, in parallel with the still-open validation of the first `npm publish`.

**Scope**: same mechanics as the first rename — inventory via `git grep`, replacement in several passes (`RepoScan`→`Lynxor`, `reposcan`→`lynxor`, then `REPOSCAN`→`LYNXOR` after an initially incomplete check), 3 files/directories renamed (`.claude/skills/`, the self-check workflow, `npm/bin/`), plugin protocol field `reposcan_version`→`lynxor_version`, HTML golden file regenerated via `-update`.

**Problem discovered while digging in, fixed before continuing**: the blind global replacement nearly rewrote a historical account in `roadmap-long-term.md` (the paragraph documenting the first rename, quoting a real error message) — turned into a false claim, "RepoAudit → Lynxor." Fixed manually; that paragraph stays frozen on the real names of the time. A second, unrelated, pre-existing stale reference (`repoaudit-self-check.yml`, mentioned in ADR 0020/`release.yml`, already wrong even before this rename) fixed along the way.

**Validated, not just written**: full `make check` green after the Go module rename; a real binary built and run (`--version`, `scan .` — score unchanged at 97/100 —, `--format html`/`--format json`, correct branding).

**Not done in this commit, separate follow-ups**: renaming the GitHub repo itself, a new real tag under `github.com/xchebila/lynxor` (no existing tag resolves under this module — a technical constraint, not a choice), a new `homebrew-lynxor` tap, the first real `npm publish` (will be directly `lynxor`, never `reposcan`).

**Update (2026-07-25)**: all the follow-ups above resolved within the hour after the merge, except the npm publish. GitHub repo renamed (by the user). The old `v1.1.0` (cut before this rename, `reposcan_*` assets already published) deleted — tag and release — and recut on the newly renamed commit, rather than using a different number (explicit user decision: only one public `v1.1.0` would ever have existed, this one). GoReleaser ran on it for real: 6 `lynxor_*` assets published. Homebrew formula updated (`reposcan.rb`→`lynxor.rb`, url/sha256 pointing at the real `v1.1.0`) and validated twice under real conditions (local tap, then after pushing to the real tap). Tap renamed `homebrew-lynxor` via the GitHub API (`repo` scope, no `delete_repo` needed). Re-validated with the README's exact command: `brew tap xchebila/lynxor && brew install lynxor` — green. README fixed for `go install .../lynxor@v1.1.0` (verified: resolves correctly).

Details: [ADR 0022](decisions/0022-rename-reposcan-to-lynxor.md).

---

## ⏸️ Paused — waiting for a real need

### Plugin marketplace

**Status**: paused, not "to do." No design audit started, for lack of an identified need.

**Why a pause rather than an audit**: the isolation protocol (Phase 4) was built so the maintainer (or a contributor) could write their own rules — never with third-party discovery/publication in mind ("plugin discovery/installation" was in fact explicitly ruled out of scope during the Phase 4 audit, ADR 0008). Nothing since has come from a real need ("someone wants to publish a plugin") — this is an anticipated direction, not a request. A marketplace also introduces the most serious risk in this entire roadmap: a trust question about discovered and installed code (signing, review, sandboxing the marketplace itself), which Phase 4's execution protocol doesn't address at all — isolating a plugin protects against a buggy or malicious one *once launched*, not against discovering one that's already compromised. Building this risk surface with no user waiting for it would go against the principle already applied everywhere else in this project: don't build before it's needed (see Non-Goals, vision.md).

**Condition to end the pause**: a real usage signal — someone who wants to publish a plugin, or a concrete use case raised. The day that signal exists, go through the same format as the Phase 4 audit: questions asked explicitly, answers verified empirically where possible, decision recorded in an ADR before the first line of code.

---

### VSCode extension

**Status**: paused, not "to do." Not started, for lack of an identified need.

**Why the pause, even though the technical risk is low here**: unlike the marketplace, there's no third-party code and no trust question — a thin wrapper around the existing CLI binary would be consistent with "minimal core, don't duplicate" and technically low-risk. But the pause criterion isn't about a feature's risk level, only about whether a real need exists — otherwise this reproduces exactly the drift the vision.md Non-Goals section exists to prevent: building a "reasonable" feature in isolation, without ever asking whether anyone asked for it. Nothing so far points to a real need for IDE integration — this is a direction written down in this document, not a request received.

**Question that would still need settling, the day a need appears**: a thin wrapper around the existing CLI binary (like the GitHub Action's packaging), or a real reimplementation of logic on the extension side (TypeScript, the VSCode API — a different skill set from the rest of the project, 100% Go so far)? The first option stays the most consistent with the principle already applied everywhere else in the project.

---

## 🧭 Directions to audit before coding

### Optional SaaS

**Status**: to be re-questioned before even starting to scope anything.

**The real question, not yet asked**: what problem does a SaaS solve that CLI + GitHub Action don't already solve? vision.md itself calls it "not mandatory" — this isn't a commitment, it's an option with no clear justification yet.

**Tension with the project's philosophy**: a SaaS introduces hosting, potentially user accounts, an attack surface, and an operational load unrelated to the "local CLI, zero-config, zero-dependency" ethos that's been at Lynxor's core since the original vision.md. Before any technical scoping, this deserves a real written answer (in an ADR or equivalent) to "why," not just "how."

---

## Recommended order

1. ✅ **GitHub Action** — done
2. ✅ **Multi-platform CI** — done
3. ⏸️ **Plugin marketplace** — paused, waiting for a real identified need (not an ongoing design audit)
4. ⏸️ **VSCode extension** — paused, waiting for a real identified need (technical risk is low, but that's not the criterion)
5. **Optional SaaS** — after answering "why," not before
