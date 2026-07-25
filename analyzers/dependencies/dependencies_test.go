package dependencies

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseGoSum_SkipsGoModOnlyLines is the regression test for a real bug
// found while investigating issue #13 (vulnerable transitive deps): a
// go.sum line ending in "/go.mod" records only that version's go.mod file
// hash, read to resolve the dependency graph -- its actual module content
// was never fetched or compiled in. `go mod tidy` does not remove these
// lines; they're legitimate bookkeeping for versions a transitive
// dependency's own go.mod once required, superseded elsewhere by MVS. The
// previous version of parseGoSum treated every such line the same as a
// real content line, reporting versions that were never actually built as
// if they were pinned dependencies -- confirmed in practice:
// golang.org/x/text@v0.3.6 and gopkg.in/yaml.v2@v2.2.2 (the latter not
// even reachable from this project's import graph at all, per `go mod why`)
// both kept showing up as vulnerable long after the real, selected
// versions were upgraded.
func TestParseGoSum_SkipsGoModOnlyLines(t *testing.T) {
	dir := t.TempDir()
	goSum := `golang.org/x/text v0.3.6/go.mod h1:5Zoc/QRtKVWzQhOtBMvqHzDpF6irO9z98xDceosuGiQ=
golang.org/x/text v0.40.0 h1:Ub2Z6/xjgF1WrYQz2nuITOEegKFtiIy+rieRJ5lHZKs=
golang.org/x/text v0.40.0/go.mod h1:hpnzDAfGV753zIKo+wk3u1bVKCGPbrnF7+7LBF/UHVY=
gopkg.in/yaml.v2 v2.2.2/go.mod h1:hI93XBmqTisBFMUTm0b8Fm+jr3Dg1NNxqwp+5A1VGuI=
`
	path := filepath.Join(dir, "go.sum")
	if err := os.WriteFile(path, []byte(goSum), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := parseGoSum(path, "go.sum")

	if len(deps) != 1 {
		t.Fatalf("got %d dependencies, want 1 (only the real content line): %+v", len(deps), deps)
	}
	got := deps[0]
	if got.Name != "golang.org/x/text" || got.Version != "v0.40.0" {
		t.Errorf("got %+v, want golang.org/x/text@v0.40.0 (the only line with real module content, not a go.mod-only line)", got)
	}

	for _, d := range deps {
		if d.Version == "v0.3.6" {
			t.Errorf("x/text@v0.3.6 was reported, but its go.sum entry is go.mod-only -- that version was never actually built")
		}
		if d.Name == "gopkg.in/yaml.v2" {
			t.Errorf("yaml.v2 was reported, but it has no real content line and isn't reachable from this project's import graph")
		}
	}
}

func TestParseGoSum_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.sum")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if deps := parseGoSum(path, "go.sum"); deps != nil {
		t.Errorf("got %v, want nil for an empty go.sum", deps)
	}
}

func TestParseGoSum_MissingFile(t *testing.T) {
	if deps := parseGoSum("/does/not/exist/go.sum", "go.sum"); deps != nil {
		t.Errorf("got %v, want nil for a missing file", deps)
	}
}

func TestGoSumParser_Matches(t *testing.T) {
	p := goSumParser{}
	if !p.Matches("go.sum") {
		t.Error("Matches(\"go.sum\") = false, want true")
	}
	if p.Matches("go.mod") || p.Matches("requirements.txt") {
		t.Error("Matches should only match go.sum")
	}
}

func TestGoSumParser_Parse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.sum")
	if err := os.WriteFile(path, []byte("example.com/mod v1.0.0 h1:abc=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := goSumParser{}.Parse(path, "go.sum")
	if len(deps) != 1 || deps[0].Name != "example.com/mod" {
		t.Errorf("got %+v, want one dependency example.com/mod@v1.0.0 (Parse must delegate to parseGoSum)", deps)
	}
}

func TestRequirementsTxtParser_Matches(t *testing.T) {
	p := requirementsTxtParser{}
	if !p.Matches("requirements.txt") {
		t.Error("Matches(\"requirements.txt\") = false, want true")
	}
	if p.Matches("go.sum") || p.Matches("requirements-dev.txt") {
		t.Error("Matches should only match the exact filename requirements.txt")
	}
}

func TestParseRequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "requirements.txt")
	content := `# a comment, skipped entirely
requests==2.31.0
django[bcrypt]==4.2.1  # inline comment stripped
flask>=2.0  # unpinned, must be skipped -- see Discover's doc comment
numpy==1.26.0; python_version >= "3.9"

-r other-requirements.txt
git+https://example.com/pkg.git
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := parseRequirementsTxt(path, "requirements.txt")

	want := map[string]string{
		"requests": "2.31.0",
		"django":   "4.2.1",
		"numpy":    "1.26.0",
	}
	if len(deps) != len(want) {
		t.Fatalf("got %d dependencies, want %d: %+v", len(deps), len(want), deps)
	}
	for _, d := range deps {
		wantVersion, ok := want[d.Name]
		if !ok {
			t.Errorf("unexpected dependency %+v", d)
			continue
		}
		if d.Version != wantVersion {
			t.Errorf("%s: got version %q, want %q", d.Name, d.Version, wantVersion)
		}
		if d.Ecosystem != "PyPI" {
			t.Errorf("%s: Ecosystem = %q, want PyPI", d.Name, d.Ecosystem)
		}
	}
}

func TestParseRequirementsTxt_MissingFile(t *testing.T) {
	if deps := parseRequirementsTxt("/does/not/exist/requirements.txt", "requirements.txt"); deps != nil {
		t.Errorf("got %v, want nil for a missing file", deps)
	}
}

func TestDiscover_WalksAndDispatchesToRegisteredParsers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.sum", "example.com/mod v1.0.0 h1:abc=\n")
	writeFile(t, dir, "requirements.txt", "requests==2.31.0\n")
	writeFile(t, dir, filepath.Join("sub", "requirements.txt"), "flask==2.0.0\n")

	deps := Discover(dir)

	byName := map[string]Dependency{}
	for _, d := range deps {
		byName[d.Name] = d
	}
	if len(deps) != 3 {
		t.Fatalf("got %d dependencies, want 3 (one per manifest file found): %+v", len(deps), deps)
	}
	if d, ok := byName["example.com/mod"]; !ok || d.Ecosystem != "Go" {
		t.Errorf("expected the top-level go.sum dependency, got %+v", byName)
	}
	if d, ok := byName["flask"]; !ok || d.Manifest != filepath.Join("sub", "requirements.txt") {
		t.Errorf("expected sub/requirements.txt's dependency with the right Manifest path, got %+v", byName)
	}
}

func TestDiscover_SkipsGitAndVendoredDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, filepath.Join(".git", "go.sum"), "should.not/be-seen v1.0.0 h1:abc=\n")
	writeFile(t, dir, filepath.Join("vendor", "go.sum"), "also.not/seen v1.0.0 h1:abc=\n")
	writeFile(t, dir, filepath.Join("frontend", "node_modules", "requirements.txt"), "should-not-appear==1.0.0\n")

	deps := Discover(dir)
	if len(deps) != 0 {
		t.Errorf("got %+v, want no dependencies -- .git and vendored paths must be skipped entirely", deps)
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
