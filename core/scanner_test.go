package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsVendoredPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"vendor/github.com/foo/bar.go", true},
		{"frontend/node_modules/left-pad/index.js", true},
		{"node_modules/foo/index.js", true},
		{"src/vendor.go", false}, // "vendor" must be a whole path segment
		{"src/main.go", false},
		{"a/b/c/vendor/d.go", true},
	}
	for _, c := range cases {
		if got := IsVendoredPath(c.path); got != c.want {
			t.Errorf("IsVendoredPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsBinary(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
		want    bool
	}{
		{"plain text", []byte("package main\n\nfunc main() {}\n"), false},
		{"empty", []byte{}, false},
		{"NUL byte", []byte("hello\x00world"), true},
		{"NUL byte far in", append([]byte(strings.Repeat("a", 100)), 0), true},
	}
	for _, c := range cases {
		if got := IsBinary(c.content); got != c.want {
			t.Errorf("%s: IsBinary() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsBinary_OnlyChecksFirst8000Bytes(t *testing.T) {
	// A NUL byte past the 8000-byte inspection window must not be detected
	// -- matches git's own heuristic, which this project deliberately
	// mirrors (see IsBinary's doc comment) rather than scanning whole
	// multi-megabyte files just to classify them.
	content := append([]byte(strings.Repeat("a", 8000)), 0)
	if IsBinary(content) {
		t.Error("IsBinary() = true for a NUL byte past the 8000-byte window, want false")
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

// recordingAnalyzer records every path it's asked to look at, so tests can
// assert exactly which files Scan actually reached without needing a real
// detection rule.
type recordingAnalyzer struct {
	seen *[]string
}

func (recordingAnalyzer) Name() string { return "recording" }

func (a recordingAnalyzer) Run(file FileContext) []Finding {
	*a.seen = append(*a.seen, file.Path)
	return nil
}

func TestScanner_Scan_SkipsAlwaysSkipDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, filepath.Join(".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, dir, filepath.Join("vendor", "dep.go"), "package dep\n")
	writeFile(t, dir, filepath.Join("frontend", "node_modules", "pkg", "index.js"), "module.exports = {}\n")

	var seen []string
	s := NewScanner(dir, recordingAnalyzer{seen: &seen})
	if _, err := s.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(seen) != 1 || seen[0] != "main.go" {
		t.Errorf("got %v, want only [main.go] -- .git/vendor/node_modules must never be walked into", seen)
	}
}

func TestScanner_Scan_SkipsOversizedAndBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "small.go", "package main\n")
	writeFile(t, dir, "big.bin", strings.Repeat("x", MaxFileSize+1))
	if err := os.WriteFile(filepath.Join(dir, "binary.dat"), []byte("hello\x00world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var seen []string
	s := NewScanner(dir, recordingAnalyzer{seen: &seen})
	if _, err := s.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(seen) != 1 || seen[0] != "small.go" {
		t.Errorf("got %v, want only [small.go] -- oversized and binary files must be skipped", seen)
	}
}

func TestScanner_Scan_RespectsGitignore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.log\nbuild/\n")
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "debug.log", "log line\n")
	writeFile(t, dir, filepath.Join("build", "output.txt"), "built\n")

	var seen []string
	s := NewScanner(dir, recordingAnalyzer{seen: &seen})
	if _, err := s.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// .gitignore itself has no special-case exclusion -- it's a normal repo
	// file like any other, still reachable by analyzers -- only the files
	// its own patterns (*.log, build/) match are excluded.
	want := map[string]bool{".gitignore": true, "main.go": true}
	if len(seen) != len(want) {
		t.Fatalf("got %v, want exactly %v", seen, want)
	}
	for _, p := range seen {
		if !want[p] {
			t.Errorf("unexpected path scanned: %q (debug.log and build/output.txt should be gitignored)", p)
		}
	}
}

func TestScanner_Warnings_SurfacesUnsupportedGitignorePatterns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "!important.log\nbuild/**/output\n*.log\n")

	s := NewScanner(dir)
	warnings := s.Warnings()

	if len(warnings) != 2 {
		t.Fatalf("got %d warnings, want 2 (negation + double-star): %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "negation") {
		t.Errorf("warnings[0] = %q, want it to mention the unsupported negation pattern", warnings[0])
	}
	if !strings.Contains(warnings[1], "double-star") {
		t.Errorf("warnings[1] = %q, want it to mention the unsupported double-star pattern", warnings[1])
	}
}

func TestScanner_Warnings_NoGitignore(t *testing.T) {
	s := NewScanner(t.TempDir())
	if warnings := s.Warnings(); warnings != nil {
		t.Errorf("got %v, want nil when there's no .gitignore at all", warnings)
	}
}

func TestScanner_Scan_RunsEveryRegisteredAnalyzer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\n")

	var seenA, seenB []string
	s := NewScanner(dir, recordingAnalyzer{seen: &seenA}, recordingAnalyzer{seen: &seenB})
	if _, err := s.Scan(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(seenA) != 1 || len(seenB) != 1 {
		t.Errorf("got seenA=%v seenB=%v, want both analyzers to see the one file", seenA, seenB)
	}
}
