package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// fixtureAWSKey is split across two literals so this file's own committed
// source never contains the matching substring contiguously -- this
// project's own CI runs a security scan (and `lynxor diff`) against its own
// history. See the longer explanation in analyzers/secrets/secrets_test.go.
const fixtureAWSKey = "AKIA" + "ABCDEFGHIJKLMNOP"

// newRepo/commit mirror analyzers/diffmode/diffmode_test.go's fixture
// helpers -- not shared as a common package, same "each test file is
// self-contained" pattern as the rest of this project's test suite.
func newRepo(t *testing.T) (dir string, repo *git.Repository) {
	t.Helper()
	dir = t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git.PlainInit: %v", err)
	}
	return dir, repo
}

func commit(t *testing.T, repo *git.Repository, dir string, files map[string]string) string {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	if _, err := wt.Add("."); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hash, err := wt.Commit("test commit", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return hash.String()
}

// newDiffCmd's RunE hardcodes "." as diffmode.Diff's repoPath (no
// --repo-style flag), so exercising it means the test process's own
// working directory must actually be the fixture repo -- t.Chdir handles
// restoring the original directory automatically, including on failure.
func chdirToRepo(t *testing.T, dir string) {
	t.Helper()
	t.Chdir(dir)
}

func TestNewDiffCmd_RequiresExactlyTwoArgs(t *testing.T) {
	cases := [][]string{{}, {"only-one"}, {"one", "two", "three"}}
	for _, args := range cases {
		cmd := newDiffCmd()
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Errorf("Execute(%v): got nil error, want an arg-count error (cobra.ExactArgs(2))", args)
		}
	}
}

func TestNewDiffCmd_NotAGitRepo(t *testing.T) {
	chdirToRepo(t, t.TempDir())

	cmd := newDiffCmd()
	cmd.SetArgs([]string{"HEAD~1", "HEAD"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute: got nil error, want a not-a-git-repository error")
	}
	if got := err.Error(); got != "not a git repository: diff mode needs git history to compare, unlike a plain scan" {
		t.Errorf("Execute error = %q, want the not-a-git-repo message", got)
	}
}

// TestNewDiffCmd_NoNewFindings_DoesNotExit confirms the RunE path that
// never reaches os.Exit: a diff with only a Fixed finding (no New one)
// must return nil, not terminate the process. Safe to run in-process --
// unlike the New-finding case below, this path is never supposed to call
// os.Exit at all.
func TestNewDiffCmd_NoNewFindings_DoesNotExit(t *testing.T) {
	dir, repo := newRepo(t)
	before := commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=" + fixtureAWSKey + "\n"})
	after := commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=safe\n"})
	chdirToRepo(t, dir)

	var out bytes.Buffer
	cmd := newDiffCmd()
	cmd.SetArgs([]string{before, after})
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() == 0 {
		t.Error("expected a written diff report, got empty output")
	}
}

// TestNewDiffCmd_ExitsOneWhenNewFindingExists exercises the os.Exit(1)
// path, which cannot be called in-process without killing the whole `go
// test` run. Standard idiom for this (also used by os/exec's own tests):
// re-exec this same test binary as a subprocess, targeted at a helper
// "test" that isn't really a test -- see TestHelperProcess_Diff -- and
// inspect the subprocess's real exit code.
func TestNewDiffCmd_ExitsOneWhenNewFindingExists(t *testing.T) {
	dir, repo := newRepo(t)
	before := commit(t, repo, dir, map[string]string{"config.py": "x = 1\n"})
	after := commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=" + fixtureAWSKey + "\n"})

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess_Diff")
	cmd.Env = append(os.Environ(),
		"LYNXOR_TEST_HELPER_PROCESS=1",
		"LYNXOR_TEST_HELPER_DIR="+dir,
		"LYNXOR_TEST_HELPER_REFA="+before,
		"LYNXOR_TEST_HELPER_REFB="+after,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected an *exec.ExitError from the helper process, got %v (stderr: %s)", err, stderr.String())
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("helper process exit code = %d, want 1 (a New finding must cause exit 1); stderr: %s", exitErr.ExitCode(), stderr.String())
	}
}

// TestHelperProcess_Diff is not a real test -- it's the re-exec target for
// TestNewDiffCmd_ExitsOneWhenNewFindingExists. It only does anything when
// LYNXOR_TEST_HELPER_PROCESS=1 is set, so a normal `go test` run treats it
// as a no-op.
func TestHelperProcess_Diff(t *testing.T) {
	if os.Getenv("LYNXOR_TEST_HELPER_PROCESS") != "1" {
		return
	}
	if dir := os.Getenv("LYNXOR_TEST_HELPER_DIR"); dir != "" {
		if err := os.Chdir(dir); err != nil {
			os.Exit(3)
		}
	}
	cmd := newDiffCmd()
	cmd.SetArgs([]string{os.Getenv("LYNXOR_TEST_HELPER_REFA"), os.Getenv("LYNXOR_TEST_HELPER_REFB")})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	if err := cmd.Execute(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
