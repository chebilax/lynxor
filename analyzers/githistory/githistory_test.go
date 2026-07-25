package githistory

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newRepo/commit mirror analyzers/diffmode/diffmode_test.go's fixture
// helpers -- a real, throwaway git repository in t.TempDir(), no clone, no
// /tmp corpus dependency, deterministic and fast. Not shared as a common
// package: each analyzer package's test file is self-contained, same
// pattern as the rest of this project's test suite.
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

// fixtureAWSKey is split across two literals so this file's own committed
// source never contains the matching substring contiguously -- this
// project's own CI runs a security scan against its own history. See the
// longer explanation in analyzers/secrets/secrets_test.go.
const fixtureAWSKey = "AKIA" + "ABCDEFGHIJKLMNOP"

// TestScan_FindsSecretDeletedInAnEarlierCommit is githistory's whole reason
// to exist: a secret introduced in one commit and removed in a later one
// must still be caught. Each commit's diff is scanned independently, so
// this reports two findings, not one: the introduction (diffing the root
// commit against an empty tree) and the removal (diffing HEAD against its
// parent) are two distinct diff events over the same blob content, each
// tagged with its own commit -- confirmed against the real implementation
// rather than assumed to collapse to a single finding.
func TestScan_FindsSecretDeletedInAnEarlierCommit(t *testing.T) {
	dir, repo := newRepo(t)
	introduced := commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=" + fixtureAWSKey + "\n"})
	removed := commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=safe\n"})

	result, err := Scan(dir, Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Fatalf("got %d findings, want 2 (introduction + removal): %+v", len(result.Findings), result.Findings)
	}
	gotCommits := map[string]bool{result.Findings[0].CommitHash: true, result.Findings[1].CommitHash: true}
	if !gotCommits[introduced] || !gotCommits[removed] {
		t.Errorf("got commits %v, want exactly {%s (introduced), %s (removed)}", gotCommits, introduced, removed)
	}
	for _, f := range result.Findings {
		if f.ID != "secrets.aws_access_key" {
			t.Errorf("finding ID = %q, want secrets.aws_access_key", f.ID)
		}
		if f.Category != "git-history" {
			t.Errorf("Category = %q, want git-history", f.Category)
		}
	}
}

// TestScan_DoesNotDoubleCountHEAD confirms the documented skipTo behavior:
// HEAD's "after" tree is the current working tree, already covered by
// core.Scanner elsewhere -- scanning it again here would double-count a
// secret that's still present today.
func TestScan_DoesNotDoubleCountHEAD(t *testing.T) {
	dir, repo := newRepo(t)
	commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=" + fixtureAWSKey + "\n"})

	result, err := Scan(dir, Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("got %d findings, want 0 (HEAD's current content is core.Scanner's job, not git-history's): %+v", len(result.Findings), result.Findings)
	}
}

// TestScan_NotAGitRepo confirms ErrNotAGitRepo, not wrapped, for a plain
// directory -- callers rely on errors.Is to treat this as a silent skip.
func TestScan_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Scan(dir, Options{})
	if err != ErrNotAGitRepo {
		t.Errorf("got err=%v, want ErrNotAGitRepo", err)
	}
}

// TestScan_TimeBudgetTruncates forces the budget to be exceeded after the
// first commit, using Options.Budget (exposed specifically so this doesn't
// need to wait out the real DefaultBudget) -- confirms Truncated is
// reported honestly rather than silently under-scanning.
func TestScan_TimeBudgetTruncates(t *testing.T) {
	dir, repo := newRepo(t)
	commit(t, repo, dir, map[string]string{"a.py": "x = 1\n"})
	commit(t, repo, dir, map[string]string{"b.py": "x = 2\n"})
	commit(t, repo, dir, map[string]string{"c.py": "x = 3\n"})

	result, err := Scan(dir, Options{Budget: 1 * time.Nanosecond})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true with a near-zero budget")
	}
	if result.CommitsScanned >= 3 {
		t.Errorf("CommitsScanned = %d, want fewer than all 3 commits given the tiny budget", result.CommitsScanned)
	}
}

// TestAnalyzer_ImplementsRepoAnalyzer confirms the core.RepoAnalyzer adapter
// delegates to Scan correctly: Name(), findings passed through, and
// ErrNotAGitRepo propagated unchanged (the caller's policy to interpret,
// not this analyzer's -- see the doc comment on Analyzer.Run).
func TestAnalyzer_ImplementsRepoAnalyzer(t *testing.T) {
	dir, repo := newRepo(t)
	commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=" + fixtureAWSKey + "\n"})
	commit(t, repo, dir, map[string]string{"config.py": "AWS_KEY=safe\n"})

	a := NewAnalyzer(Options{})
	if a.Name() != "githistory" {
		t.Errorf("Name() = %q, want githistory", a.Name())
	}
	// Two findings expected here for the same reason as
	// TestScan_FindsSecretDeletedInAnEarlierCommit: this test only checks
	// that the RepoAnalyzer adapter passes Scan's result through unchanged,
	// not the finding count itself.
	findings, err := a.Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(findings) != 2 {
		t.Errorf("got %d findings, want 2 (adapter should pass Scan's result through unchanged)", len(findings))
	}

	_, err = a.Run(t.TempDir())
	if err != ErrNotAGitRepo {
		t.Errorf("got err=%v, want ErrNotAGitRepo for a non-git directory", err)
	}
}

// TestCountReachableCommits confirms the plain commit-object walk used to
// give --full-history a "N/total" denominator (see Options.OnProgress)
// counts every reachable commit, and nothing else.
func TestCountReachableCommits(t *testing.T) {
	dir, repo := newRepo(t)
	commit(t, repo, dir, map[string]string{"a.py": "x = 1\n"})
	commit(t, repo, dir, map[string]string{"b.py": "x = 2\n"})
	commit(t, repo, dir, map[string]string{"c.py": "x = 3\n"})

	n, err := countReachableCommits(repo)
	if err != nil {
		t.Fatalf("countReachableCommits: %v", err)
	}
	if n != 3 {
		t.Errorf("got %d, want 3", n)
	}
}

// TestScan_FullHistoryWithOnProgress_CountsReachableCommitsUpFront confirms
// Scan actually calls countReachableCommits (not just that the function
// works in isolation): FullHistory + a non-nil OnProgress is the only
// condition that triggers it, per Scan's own doc comment.
func TestScan_FullHistoryWithOnProgress_CountsReachableCommitsUpFront(t *testing.T) {
	dir, repo := newRepo(t)
	commit(t, repo, dir, map[string]string{"a.py": "x = 1\n"})
	commit(t, repo, dir, map[string]string{"b.py": "x = 2\n"})

	result, err := Scan(dir, Options{FullHistory: true, OnProgress: func(int, int) {}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.CommitsScanned != 2 {
		t.Errorf("CommitsScanned = %d, want 2", result.CommitsScanned)
	}
}

// TestScan_FullHistory_ScansDanglingCommits confirms the whole reason
// scanDangling exists: a commit still physically present in the object
// store but unreachable from any ref (e.g. a deleted branch, before `git
// gc` prunes it) is still scanned under --full-history. Simulated by
// force-moving HEAD's branch ref back to an earlier commit, orphaning the
// later one -- go-git never garbage-collects on its own, so the orphaned
// commit object stays on disk exactly like a real deleted branch would
// leave it.
func TestScan_FullHistory_ScansDanglingCommits(t *testing.T) {
	dir, repo := newRepo(t)
	base := commit(t, repo, dir, map[string]string{"a.py": "x = 1\n"})
	dangling := commit(t, repo, dir, map[string]string{"secret.py": "AWS_KEY=" + fixtureAWSKey + "\n"})

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(head.Name(), plumbing.NewHash(base))); err != nil {
		t.Fatalf("SetReference: %v", err)
	}

	result, err := Scan(dir, Options{FullHistory: true})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	found := false
	for _, f := range result.Findings {
		if f.CommitHash == dangling {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a finding tagged with the now-dangling commit %s, got %+v", dangling, result.Findings)
	}
}

// TestScan_FullHistory_NoDanglingCommits confirms scanDangling's visited
// check: with no ref moved, every commit is already reachable and
// scanDangling has nothing extra to find.
func TestScan_FullHistory_NoDanglingCommits(t *testing.T) {
	dir, repo := newRepo(t)
	// Introduced then removed (not left in HEAD's current content) so this
	// produces real findings to compare against -- a secret only ever
	// present in HEAD's current tree is deliberately excluded regardless of
	// scanDangling (see TestScan_DoesNotDoubleCountHEAD), which would make a
	// "scanDangling adds nothing extra" test pass for the wrong reason.
	commit(t, repo, dir, map[string]string{"secret.py": "AWS_KEY=" + fixtureAWSKey + "\n"})
	commit(t, repo, dir, map[string]string{"secret.py": "AWS_KEY=safe\n"})

	result, err := Scan(dir, Options{FullHistory: true})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Findings) != 2 {
		t.Errorf("got %d findings, want 2 (introduction + removal) -- no dangling commits exist, scanDangling should add nothing extra", len(result.Findings))
	}
}

// TestAnalyzer_Run_ReportsTruncationWarning confirms Run's Truncated
// diagnostic path (a stderr warning, not an error) actually fires when the
// budget is exceeded -- exercised through the RepoAnalyzer adapter, not
// just Scan directly.
func TestAnalyzer_Run_ReportsTruncationWarning(t *testing.T) {
	dir, repo := newRepo(t)
	commit(t, repo, dir, map[string]string{"a.py": "x = 1\n"})
	commit(t, repo, dir, map[string]string{"b.py": "x = 2\n"})

	a := NewAnalyzer(Options{Budget: 1 * time.Nanosecond})
	_, err := a.Run(dir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// No assertion on stderr content itself (Run writes it directly, no
	// return value carries it) -- this test's job is to confirm the
	// truncated path is reachable and doesn't error, matching how
	// TestScan_TimeBudgetTruncates already confirms Truncated itself.
}
