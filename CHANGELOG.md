# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project
does not yet follow strict semver pre-1.0-style guarantees, but version
bumps since v1.0.0 do follow semver (MAJOR.MINOR.PATCH) based on
user-visible impact, not just code churn.

## [Unreleased]

## [1.2.0] - 2026-07-26

### Fixed
- `githistory`: a secret introduced then later removed still reports two
  findings (two real, independent diff events — not merged), but each now
  cross-references the other's commit via `Finding.Context` ("also
  removed in commit X" / "originally introduced in commit Y") instead of
  reading as unrelated noise. See [ADR 0023](docs/decisions/0023-githistory-context-crossref.md), issue #27.

### Added
- `CONTRIBUTING.md` and this changelog.
- Test coverage extended on `analyzers/plugin` (0%→92.2%),
  `analyzers/githistory` (63.4%→83.9%), `analyzers/dependencies`
  (54.7%→69.9%), `core` (65.7%→92.6%), and `cli/diff.go` (part of
  `cli`'s 10.2%→19.5%).

### Changed
- Clarified `--help` output on the root command and `lynxor diff`: which
  ref is the base vs. the head, and a concrete example invocation for
  each.

## [1.1.2] - 2026-07-26

### Fixed
- npm trusted publishing (OIDC): `actions/setup-node`'s `registry-url`
  input was writing a stale, unresolved `_authToken` line into `.npmrc`,
  silently preventing the OIDC handshake from ever being attempted (`401`
  on `npm whoami`, `404` on `npm publish`). Fixed by not setting
  `registry-url` at all — `registry.npmjs.org` is npm's own default.

## [1.1.1] - 2026-07-25

### Added
- npm trusted publishing (OIDC) replaces the originally-planned
  `NPM_TOKEN` secret for `npm publish` — no long-lived token stored or
  rotated.

### Changed
- GitHub account/repo renamed `xchebila` → `chebilax`
  (`github.com/chebilax/lynxor`), including the Go module path.

## [1.1.0] - 2026-07-25

### Added
- `core.RepoAnalyzer` interface, unifying Dependabot-config checking and
  git-history scanning under one generic analyzer list in `lynxor scan`.
- `--plugin-arg key=value`: pass configuration directly to a loaded
  `--plugin`, addressed by the plugin's own self-declared name (see
  [docs/plugin-protocol.md](docs/plugin-protocol.md)).
- Dependency scanning now understands npm's `package-lock.json`
  (lockfile versions 2/3), in addition to `go.sum` and
  `requirements.txt`.
- GoReleaser pipeline: every tagged release publishes precompiled
  Linux/macOS/Windows (amd64/arm64) binaries as GitHub Release assets.
- npm distribution package (`npm install -g lynxor`): a postinstall
  script downloads the matching precompiled binary.

### Changed
- Project renamed RepoScan → Lynxor, including the CLI binary name,
  GitHub repo, Homebrew tap, and Go module path.

## [1.0.2] - 2026-07-24

### Added
- `SECURITY.md`.
- Per-analyzer soft timeout: an analyzer that hangs or runs long on one
  file is skipped for that file (with a warning) instead of stalling the
  whole scan.

### Fixed
- `--deps`: `go.sum` lines that only pin a `go.mod` hash (not real module
  content actually fetched and compiled) were reported as vulnerable
  dependencies even when the real, selected version had already been
  upgraded — a real false-positive found investigating this project's own
  `--deps` output.

### Changed
- Project renamed RepoAudit → RepoScan (before any external publication).

## [1.0.1] - 2026-07-23

### Added
- Official GitHub Action (`action.yml`): runs `lynxor diff` on pull
  requests, `lynxor scan` otherwise.
- Homebrew tap (`brew install lynxor`).
- `--version` flag.
- Documented GitLab CI and Jenkins snippets (copy-paste, not published
  artifacts) alongside the GitHub Action.

## [1.0.0] - 2026-07-23

Initial release. The full Phase 1-5 roadmap from
[vision.md](docs/vision.md): committed secrets and exposed keys, git
history scanning, Dockerfile analysis, CI/CD workflow misconfigurations,
dependency vulnerability checking against OSV.dev, a diff mode for pull
requests, an external plugin system, and CLI/JSON/HTML reporting.
