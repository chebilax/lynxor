# Contributing to Lynxor

This describes the process this project actually follows, not an
aspirational one - if you're opening a PR, this is what to expect.

## Before writing code

For anything beyond a small, obvious fix, open an issue first - see
[docs/vision.md](docs/vision.md) for what this project is (and
deliberately isn't) trying to be. A well-executed PR that's out of scope
is harder to handle than a scope question asked up front.

## Workflow

1. **Branch per feature/fix**, off an up-to-date `main`. One logical
   change per branch/PR.
2. **Write an ADR for any non-trivial decision** - not every change needs
   one, but a real choice between alternatives (a library, a data format,
   a trade-off with a real downside) gets written down in
   `docs/decisions/NNNN-short-name.md` alongside the code, not
   reconstructed afterward. See [`docs/decisions/`](docs/decisions/) for
   the expected depth: the decision, the alternatives considered and why
   they were rejected, and how it was validated.
3. **Validate empirically before claiming something works.** "It
   compiles" and "the test passes" aren't enough for anything touching
   CI/release infrastructure, external APIs, or subprocess/protocol code.
   This project's own history has real bugs caught only by running the
   real thing - a real CI run, a real `brew install --build-from-source`,
   a real `go install` against a real tag, a real query against
   OSV.dev - not assumed from reading the config or mocking the exact
   boundary under test.
4. **`make check` before every PR** (`go build`, `go vet`, `gofmt -l`,
   `go test`) - the same checklist CI runs.
5. **Update docs alongside the code, not after.**
   [`docs/testing.md`](docs/testing.md) records what's tested and how,
   including known gaps stated plainly rather than hidden;
   [`docs/roadmap-long-term.md`](docs/roadmap-long-term.md) records what's
   done and why. See [`docs/README.md`](docs/README.md) for the full doc
   structure.
6. Open the PR with a description that explains *why*, not just *what* -
   the diff already shows what changed.

## Code style

Standard `gofmt`, no additional linter config. Comments explain the *why*
- a hidden constraint, a subtle invariant, a workaround for a specific
bug - never the *what* a well-named identifier already says. No
dependency added without a real need (see vision.md's stdlib-first
philosophy).

## Tests

Table-driven where the shape fits. Real fixtures over mocks wherever
practical: a real throwaway git repo in `t.TempDir()`, a real subprocess,
a real HTTP call to a public API when the integration itself is what's
under test. See [`docs/testing.md`](docs/testing.md) for the reasoning
behind each package's testing approach.

## Questions

Open an issue. This is a solo-maintained project (see
[SECURITY.md](SECURITY.md)) - no SLA, but a real answer.
