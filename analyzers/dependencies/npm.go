package dependencies

import (
	"encoding/json"
	"os"
	"strings"
)

func init() {
	registerParser(packageLockParser{})
}

// packageLockParser reads npm's package-lock.json, lockfileVersion 2 or 3
// only (npm 7+). lockfileVersion 1 (pre-npm-7, a nested "dependencies" tree
// instead of a flat "packages" map) is not supported here -- yarn.lock and
// pnpm-lock.yaml are separate, deliberately deferred to their own PR once
// this one is validated (see docs/roadmap-long-term.md).
type packageLockParser struct{}

func (packageLockParser) Matches(filename string) bool { return filename == "package-lock.json" }

// npmLockFile only models the one field this parser actually reads.
// lockfileVersion 2 carries both "packages" (the flat map, npm 7+'s primary
// representation) and a legacy nested "dependencies" tree kept only for
// npm 6 backward compatibility; lockfileVersion 3 (npm 9+) drops
// "dependencies" entirely. Reading "packages" works for both without
// needing to know which version produced the file.
type npmLockFile struct {
	Packages map[string]npmLockPackage `json:"packages"`
}

type npmLockPackage struct {
	Version string `json:"version"`
	// Link marks a symlinked local workspace member (a monorepo package),
	// not a real registry dependency -- nothing to check against OSV.
	Link bool `json:"link"`
}

func (packageLockParser) Parse(path, manifestRel string) []Dependency {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lock npmLockFile
	if err := json.Unmarshal(content, &lock); err != nil {
		return nil
	}

	// A hoisted dependency tree can list the same real package+version at
	// several different node_modules paths (top-level vs nested under a
	// dependency that needed its own conflicting version) -- deduping on
	// name+version avoids querying and, if vulnerable, reporting the exact
	// same real issue more than once for what's conceptually one
	// dependency instance.
	seen := map[string]bool{}
	var deps []Dependency
	for pkgPath, pkg := range lock.Packages {
		if pkgPath == "" || pkg.Link || pkg.Version == "" {
			continue
		}
		name := packageNameFromPath(pkgPath)
		if name == "" {
			continue
		}
		key := name + "@" + pkg.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		deps = append(deps, Dependency{Name: name, Version: pkg.Version, Ecosystem: "npm", Manifest: manifestRel})
	}
	return deps
}

// packageNameFromPath extracts a package name from a package-lock.json
// "packages" key, e.g. "node_modules/lodash" -> "lodash",
// "node_modules/@scope/name" -> "@scope/name" (npm's own scoped-package
// convention, and exactly what OSV expects for a scoped package's name),
// "node_modules/foo/node_modules/bar" -> "bar" (nested/hoisting conflict:
// only the deepest, actually-resolved segment is the real package name).
// A monorepo workspace member's own key (e.g. "packages/some-app", no
// "node_modules/" segment at all) returns "" and is skipped -- not an
// external dependency to check against OSV.
func packageNameFromPath(pkgPath string) string {
	idx := strings.LastIndex(pkgPath, "node_modules/")
	if idx == -1 {
		return ""
	}
	return pkgPath[idx+len("node_modules/"):]
}
