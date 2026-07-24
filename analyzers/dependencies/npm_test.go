package dependencies

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageLockParser_Matches(t *testing.T) {
	p := packageLockParser{}
	if !p.Matches("package-lock.json") {
		t.Error("Matches(\"package-lock.json\") = false, want true")
	}
	if p.Matches("yarn.lock") || p.Matches("pnpm-lock.yaml") || p.Matches("package.json") {
		t.Error("Matches should only match package-lock.json, not other npm-ecosystem files")
	}
}

func TestPackageLockParser_Parse(t *testing.T) {
	// A realistic lockfileVersion 3 shape: root entry (key ""), a plain
	// dependency, a scoped dependency, a workspace member linked in (no
	// real version to check), and a hoisting conflict -- the same
	// package+version reachable at two different node_modules paths, which
	// must collapse to one Dependency, plus a genuinely different version
	// of another package nested under a dependency, which must not.
	lockJSON := `{
  "name": "example",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "example",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21"
    },
    "node_modules/@scope/pkg": {
      "version": "2.0.0"
    },
    "packages/some-workspace-member": {
      "link": true
    },
    "node_modules/left-pad": {
      "version": "1.3.0"
    },
    "node_modules/some-dep/node_modules/left-pad": {
      "version": "1.3.0"
    },
    "node_modules/some-dep/node_modules/right-pad": {
      "version": "2.0.0"
    }
  }
}`
	dir := t.TempDir()
	path := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(path, []byte(lockJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := packageLockParser{}.Parse(path, "package-lock.json")

	got := map[string]string{} // name -> version, for exact-count assertions below
	for _, d := range deps {
		if d.Ecosystem != "npm" {
			t.Errorf("dependency %s: Ecosystem = %q, want npm", d.Name, d.Ecosystem)
		}
		if d.Manifest != "package-lock.json" {
			t.Errorf("dependency %s: Manifest = %q, want package-lock.json", d.Name, d.Manifest)
		}
		got[d.Name] = d.Version
	}

	wantNames := map[string]string{
		"lodash":     "4.17.21",
		"@scope/pkg": "2.0.0",
		"left-pad":   "1.3.0",
		"right-pad":  "2.0.0",
	}
	if len(got) != len(wantNames) {
		t.Fatalf("got %d dependencies %v, want %d: %v", len(got), got, len(wantNames), wantNames)
	}
	for name, version := range wantNames {
		if got[name] != version {
			t.Errorf("dependency %q: got version %q, want %q", name, got[name], version)
		}
	}

	// The root project entry ("") and the linked workspace member must not
	// appear at all -- neither is a real external dependency.
	if _, ok := got["example"]; ok {
		t.Error("root project entry leaked into the dependency list")
	}
}

func TestPackageLockParser_HoistingConflictDeduped(t *testing.T) {
	// left-pad@1.3.0 appears at two different node_modules paths above --
	// confirms it's reported exactly once, not twice, for the same real
	// dependency instance.
	lockJSON := `{
  "packages": {
    "": {},
    "node_modules/left-pad": {"version": "1.3.0"},
    "node_modules/some-dep/node_modules/left-pad": {"version": "1.3.0"}
  }
}`
	dir := t.TempDir()
	path := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(path, []byte(lockJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := packageLockParser{}.Parse(path, "package-lock.json")
	count := 0
	for _, d := range deps {
		if d.Name == "left-pad" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("got %d left-pad entries, want exactly 1 (deduped hoisting conflict)", count)
	}
}

func TestPackageLockParser_MissingOrMalformedFile(t *testing.T) {
	p := packageLockParser{}
	if deps := p.Parse("/does/not/exist/package-lock.json", "package-lock.json"); deps != nil {
		t.Errorf("got %v, want nil for a missing file", deps)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if deps := p.Parse(path, "package-lock.json"); deps != nil {
		t.Errorf("got %v, want nil for malformed JSON", deps)
	}
}

func TestPackageNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"node_modules/lodash", "lodash"},
		{"node_modules/@scope/pkg", "@scope/pkg"},
		{"node_modules/some-dep/node_modules/left-pad", "left-pad"},
		{"packages/some-workspace-member", ""}, // no node_modules segment: not an external dep
		{"", ""},
	}
	for _, tt := range tests {
		if got := packageNameFromPath(tt.path); got != tt.want {
			t.Errorf("packageNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
