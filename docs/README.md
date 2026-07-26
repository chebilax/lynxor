# Documentation

**To use Lynxor, the [main README](../README.md) is enough** — install, commands, example output.

What follows documents the *why* behind each architecture decision, as the project was built. Useful if you're curious about a specific choice or contributing to the project — not required just to use it.

## Overview

- [`vision.md`](vision.md) — pitch, philosophy, non-goals, Phases 1-5 roadmap (closed, v1.0)
- [`roadmap-long-term.md`](roadmap-long-term.md) — post-v1.0 directions (GitHub Action, multi-platform CI, Homebrew: done; plugin marketplace, VSCode extension: paused, no real need identified)

## How the project is tested

- [`testing.md`](testing.md) — test corpus, measurable exit criteria
- [`benchmarks.md`](benchmarks.md) — performance measurement history, phase by phase

## Technical contracts

- [`plugin-protocol.md`](plugin-protocol.md) — the plugin system's JSON protocol (separate process, any language)
- [`ci-integrations.md`](ci-integrations.md) — GitLab CI / Jenkins snippets (not tested against a real instance — see the warning at the top of the file)
- [`examples/reference-plugin.py`](examples/reference-plugin.py) — reference plugin in Python

## Architecture decisions (ADR)

Every non-trivial decision in this project is documented at the moment it's made, not reconstructed after the fact. See [`decisions/`](decisions/) for the full list, in chronological order — written in French (not yet translated, unlike the rest of this documentation).

## Contributing

- [`../CONTRIBUTING.md`](../CONTRIBUTING.md) — the real process (branch per feature, ADRs, empirical validation, `make check`)
- [`../CHANGELOG.md`](../CHANGELOG.md) — version history
