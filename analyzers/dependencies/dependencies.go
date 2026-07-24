// Package dependencies checks a repo's Go and Python manifests for known
// vulnerabilities via OSV.dev. Unlike every other analyzer, this one needs
// the network — see docs/decisions/0004-dependency-scanner-network.md for
// why that makes it opt-in (--deps) rather than on by default.
//
// Discovery (finding and parsing manifests) is local and always safe to
// run; only CheckVulnerabilities makes network calls. Splitting these lets
// the default scan mention "N dependencies found" without ever touching
// the network itself.
package dependencies

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xchebila/reposcan/core"
)

// Dependency is one package+version pinned by a manifest file.
type Dependency struct {
	Name      string
	Version   string
	Ecosystem string // OSV ecosystem name: "Go", "PyPI", or "npm"
	Manifest  string // repo-relative path to the manifest that pinned it
}

// Parser extracts pinned dependencies from one lockfile format. Discover
// matches each file it walks against every registered Parser by filename
// rather than a hardcoded per-format switch, so a new lockfile format is a
// new file that calls registerParser() in its own init() (see
// analyzers/dependencies/npm.go) -- never a further edit here. This is the
// one time this file changes to introduce that extension point itself;
// every format added after this one shouldn't need to.
type Parser interface {
	Matches(filename string) bool
	Parse(path, manifestRel string) []Dependency
}

var parsers []Parser

func registerParser(p Parser) {
	parsers = append(parsers, p)
}

func init() {
	registerParser(goSumParser{})
	registerParser(requirementsTxtParser{})
}

// Discover walks repoRoot for known manifest/lockfile formats (see parsers)
// and parses out exact-pinned dependencies. Unpinned requirements (e.g. a
// Python "package>=1.0" with no exact version) are skipped rather than
// queried without a version — OSV would return every vulnerability ever
// reported for that package regardless of whether the installed version is
// actually affected, which is exactly the kind of noisy, ambiguous result
// vision.md's Non-Goals rule out.
func Discover(repoRoot string) []Dependency {
	var deps []Dependency

	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}

		if d.IsDir() {
			if d.Name() == ".git" || core.IsVendoredPath(rel) {
				return filepath.SkipDir
			}
			return nil
		}

		for _, p := range parsers {
			if p.Matches(d.Name()) {
				deps = append(deps, p.Parse(path, rel)...)
				break
			}
		}
		return nil
	})

	return deps
}

type goSumParser struct{}

func (goSumParser) Matches(filename string) bool { return filename == "go.sum" }

func (goSumParser) Parse(path, manifestRel string) []Dependency {
	return parseGoSum(path, manifestRel)
}

func parseGoSum(path, manifestRel string) []Dependency {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// go.sum has two kinds of line per module version: a real content line
	// ("v1.2.3 h1:...", the module's actual zip, fetched and compiled into
	// the build) and a go.mod-only line ("v1.2.3/go.mod h1:...", just that
	// version's go.mod file, read to resolve the dependency graph but
	// never fetched or built). A version can appear as go.mod-only forever
	// with no real content line at all -- e.g. an old version a transitive
	// dependency's own go.mod once required, superseded everywhere by a
	// newer one MVS actually selected. `go mod tidy` does not remove these
	// lines; they're legitimate graph-verification bookkeeping, not stale
	// cruft, confirmed empirically.
	//
	// Treating every distinct version mentioned anywhere in go.sum as an
	// in-use dependency (the previous version of this function) reports
	// versions that were never actually built: real repo, real finding —
	// golang.org/x/text@v0.3.6 and gopkg.in/yaml.v2@v2.2.2 kept showing up
	// as vulnerable in this project's own --deps scan after bumping to
	// x/text@v0.40.0, because their go.mod-only lines never disappear. One
	// of the two (yaml.v2) isn't even reachable from this project's import
	// graph at all (`go mod why -m gopkg.in/yaml.v2` confirms) -- it's
	// entirely a phantom finding. Skipping go.mod-only lines is the fix:
	// only a real content line means a version was actually selected and
	// compiled in.
	var deps []Dependency
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.HasSuffix(fields[1], "/go.mod") {
			continue
		}
		deps = append(deps, Dependency{Name: fields[0], Version: fields[1], Ecosystem: "Go", Manifest: manifestRel})
	}
	return deps
}

type requirementsTxtParser struct{}

func (requirementsTxtParser) Matches(filename string) bool { return filename == "requirements.txt" }

func (requirementsTxtParser) Parse(path, manifestRel string) []Dependency {
	return parseRequirementsTxt(path, manifestRel)
}

// requirementLine matches "package==1.2.3" and "package[extra]==1.2.3",
// ignoring the extra. Anything else (unpinned ">=", a git/URL install, a
// -r include) is deliberately not matched — see Discover's doc comment.
var requirementLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.\-]*)\s*(\[[^\]]*\])?\s*==\s*([A-Za-z0-9_.\-]+)`)

func parseRequirementsTxt(path, manifestRel string) []Dependency {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var deps []Dependency
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, " #"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if idx := strings.Index(line, ";"); idx != -1 {
			line = strings.TrimSpace(line[:idx]) // strip environment markers
		}
		m := requirementLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		deps = append(deps, Dependency{Name: m[1], Version: m[3], Ecosystem: "PyPI", Manifest: manifestRel})
	}
	return deps
}
