# 🛡️ Lynxor — Repository Security Auditor

## 📌 One-sentence pitch

> **Lynxor does not analyze code quality. It detects real-world security mistakes that leak data or break production.**

Shorter version, for a README or a GitHub tagline:

> Lynxor is a 10-second security sanity check for Git repositories.

Both formulations do something a "security git status" tagline didn't quite manage: they explicitly rule SonarQube out of the comparison instead of leaving the reader to draw that line themselves.

## 🧭 How Lynxor should feel

- Runs on any repo, no setup
- Results in seconds
- Understandable without reading the docs
- Every finding is immediately actionable

If a feature contradicts any of these four points, it's probably out of scope (see Non-Goals below).

## 🎯 Who is this for?

| Persona | Need | When |
|---|---|---|
| Solo dev / small team | Quickly confirm a repo has no leaks before pushing/open-sourcing it | `pre-commit`, before publishing a repo |
| Tech lead / DevOps | A simple security gate in CI, without configuring an enterprise tool | CI/CD pipeline |
| Open source maintainer | Reassure contributors and users about the repo's health | A score badge in the README |

Without a clear persona, a "developer" tool quickly becomes a tool "for no one." It's worth deciding which of these three you serve first — probably the solo dev, given the MVP.

## 🧠 Philosophy (unchanged, this is the project's core)

1. **Signal > noise** — not 500 warnings, only what's actionable.
2. **Explainable** — every finding says *why* it's dangerous and *how* to fix it.
3. **Extensible** — minimal core, rules as plugins.

## 🔥 Differentiation

| Tool | Goal | Limitation |
|---|---|---|
| Gitleaks | Secrets only | No overall score, no other categories |
| SonarQube | Full static analysis | Heavy to install/configure, built for the enterprise |
| Snyk | Dependency vulnerabilities | Paid at scale, not repo-health-centered |
| **Lynxor** | **Fast, actionable health check** | Still to prove: must avoid becoming "SonarQube v2" |

**The project's main risk**: the roadmap (v0.1 to v0.6) covers roughly everything SonarQube does. The real differentiator isn't the list of checks, it's the UX: speed, readability, zero config. Keep that as the compass for every feature added — if a feature slows the scan down or complicates the config, it works against the promise.

### 🚫 Non-Goals

Without this section, the roadmap naturally drifts toward "mini SonarQube" — each new check category looks like a legitimate feature in isolation, but the sum ends up recreating the very tool this was meant to avoid. These exclusions need to be just as visible as the roadmap itself:

Lynxor will **not** try to:

- Replace SonarQube or SAST tools
- Provide deep static analysis (AST, dataflow)
- Detect every possible vulnerability
- Aim for zero false positives at all costs
- Require complex configuration

**Tie-breaker rule**: if a check is slow, noisy, or ambiguous, it's out of scope — even if it's technically feasible.

## 🏗️ Overall architecture

```
lynxor
│
├── core/            → scanner engine, git reader, report generator, scoring engine
├── analyzers/        → secrets, git-history, dependencies, docker, ci, code-smells
├── plugins/          → external rules
├── cli/              → commands
└── output/           → cli, json, html
```

## ⚙️ Product roadmap

### Phase 1 — MVP: Secrets Scanner (1–2 weeks)
Detection: committed `.env`, AWS keys, GitHub tokens, private keys (`.pem`, `.key`, `id_rsa`), Stripe/Slack/Discord/OpenAI tokens, raw JWTs, secrets in `.yaml`/`.json`/`.env`/`.config`.
Features: file scanning, regex rules, `.gitignore` respected, simple CLI output, basic score.

**MVP exit criterion**: able to scan a real repo in < 5 seconds with zero major false positives across a test set of ~20 known public repos. Without this measurable criterion, "MVP" stays vague.

### Phase 2 — Git History + Docker
- Git History Analyzer: secrets removed but still present in history (commits, deleted branches, associated hash).
- Docker Analyzer: `USER root`, `latest` tags, `ADD .` instead of `COPY`, secrets in `ENV`, no non-root user.

### Phase 3 — Dependencies + CI/CD
- Dependency Scanner via the OSV API and the GitHub Advisory DB (Go, Python, then Node optionally).
- CI/CD Analyzer: overly broad permissions (`write-all`), unpinned actions (`@main`), secrets exposed in workflows, no Dependabot.
- **Security Diff Mode** — the feature that shifts the positioning from "scanner" to "security review tool":

```
lynxor diff main feature-branch
```

```
❌ NEW: GitHub token introduced
⚠️ NEW: Dockerfile now runs as root
✔️ FIXED: .env removed from repo
```

This is especially strong in CI/CD on a pull request: instead of a static score for the entire repo, it shows exactly what *this* PR introduces or fixes. That's the kind of feature that gives a concrete reason to add Lynxor to a pipeline, rather than running it once and forgetting about it.

### Phase 4 — Plugin System
Minimal interface:
```go
type Analyzer interface {
    Name() string
    Run(repo RepoContext) []Finding
}
```
Possible external plugins: Terraform, Kubernetes manifests, Python static analysis, custom enterprise rules.

### Phase 5 — Reporting v1.0
- Outputs: colored CLI, machine-readable JSON, HTML dashboard.
- Content: security score (0–100), per-category breakdown, findings list, severity levels.

## 📊 Scoring system

```
Secrets           10/10
Git History        7/10
Docker              6/10
Dependencies        8/10
CI/CD               9/10
Code Safety         7/10

TOTAL SCORE: 78/100
GRADE: B
```

### ⚠️ Scoring principle

**A single critical problem must dominate the score, not just add up like one minor issue among others.** An exposed secret isn't "10 points off, same as a Docker `latest` tag" — it's an incident, and the score must reflect that immediately.

Indicative model to refine in Phase 5, but laid down now as a principle:

- 🔴 Critical (exposed secret, leak) → -40 to -100
- 🟠 High (exploitable misconfiguration) → -15 to -40
- 🟡 Medium → -5 to -15
- 🟢 Low → -1 to -5

Without this explicit hierarchy, a repo with an exposed AWS secret but few other findings could still show a decent score — which would destroy the tool's credibility the first time it actually caught a real incident.

## 🖥️ CLI design

```
lynxor scan .
lynxor scan https://github.com/user/repo

lynxor report --format html
lynxor report --format json

lynxor plugins list
lynxor plugins install xyz
```

## 📄 Example output

```
❌ HIGH   - GitHub Token detected in commit a83f1c
⚠️ MEDIUM - Docker runs as root
⚠️ MEDIUM - requirements.txt contains vulnerable dependency
✔️ OK     - No secrets in working tree
```

## 🧾 Configuration

```yaml
# .lynxor.yml
score:
  threshold: 70

scan:
  git_history: true
  dependencies: true

ignore:
  - node_modules
  - vendor

plugins:
  - secrets
  - docker
  - ci
```

## 🧱 Tech stack

- **Go** — a good choice for a fast CLI, single binary, easy to distribute
- CLI: Cobra or urfave/cli
- Git: go-git
- YAML (CI/CD Analyzer, Phase 3): gopkg.in/yaml.v3 — no YAML parser in stdlib, the Go ecosystem's reference library
- Parallelization: goroutines
- Output: templating + JSON
- HTTP: OSV / GitHub API

## 🌍 Long-term vision

See [docs/roadmap-long-term.md](docs/roadmap-long-term.md) for post-v1.0 directions.

## 🧠 Mental positioning

Don't think: *"I'm building a security scanner."*
Think: *"I'm building a security health-check tool for developers."*

---

## ✍️ Revision history

**V2 (this version)**
- Pitch rewritten to explicitly rule out the SonarQube comparison rather than leaving it implicit
- **Non-Goals** section added — the most important guardrail against "mini SonarQube" drift
- **Security Diff Mode** added in Phase 3: shifts the positioning from a one-off scanner to a review tool, especially strong in CI/CD on a PR
- Scoring principle framed: a critical issue must dominate the score, not add up as an equal among minor ones
- "How Lynxor should feel" section added at the top, as a UX compass

**V1**
- Explicit personas (solo dev / tech lead / OSS maintainer)
- Positioning risk named (roadmap = SonarQube's scope)
- Measurable MVP exit criterion
- Everything else (philosophy, architecture, stack, CLI design) unchanged — already solid in the original draft
