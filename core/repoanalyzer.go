package core

// RepoAnalyzer is the repo-level counterpart to Analyzer: some checks are
// inherently about the whole repository (a missing file at a fixed path, a
// walk of git history) rather than any single file's content, and don't fit
// Analyzer's per-file Run(FileContext) shape at all.
//
// Deliberately minimal, same reasoning as Analyzer itself: Name() plus one
// Run method. Implementations that need their own configuration (git-history
// scanning has several: FullHistory, a time Budget, an OnProgress callback)
// bind it at construction time via their own constructor, the same pattern
// secrets.New()/docker.New() already use for the per-file Analyzer -- the
// interface itself never grows a parameter for one implementation's needs.
//
// Non-fatal diagnostics (a progress readout during a long scan, a warning
// that history scanning was truncated) are written directly to os.Stderr by
// the implementation itself, not returned through a dedicated channel on
// this interface -- the same convention analyzers/plugin/plugin.go and
// core.RunAnalyzer already established, rather than inventing a new one here.
//
// RepoAnalyzer.Run is NOT wrapped in RunAnalyzer's per-file AnalyzerTimeout:
// that budget exists for per-file work, where a healthy call is expected to
// return in milliseconds. A repo-level scan can legitimately run for minutes
// (--full-history on a large repo) -- applying a 5s guard here would fire on
// every normal use of that mode. git-history already manages its own
// internal time budget appropriately; nothing here needs a second one yet.
type RepoAnalyzer interface {
	Name() string
	Run(repoRoot string) ([]Finding, error)
}
