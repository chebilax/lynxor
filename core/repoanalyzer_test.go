package core

// fakeRepoAnalyzer is a minimal core.RepoAnalyzer used to confirm the
// interface shape is actually usable generically -- this project's own
// implementations (cicd.DependabotAnalyzer, githistory.Analyzer) are tested
// in their own packages, since testing them here would need core to import
// packages that already import core (see analyzers/builtin.go for the same
// cycle constraint).
type fakeRepoAnalyzer struct {
	name     string
	findings []Finding
	err      error
}

func (f fakeRepoAnalyzer) Name() string { return f.name }

func (f fakeRepoAnalyzer) Run(repoRoot string) ([]Finding, error) {
	return f.findings, f.err
}

// TestRepoAnalyzer_InterfaceShape is a compile-time-adjacent check: if
// RepoAnalyzer's signature ever changes in a way that breaks a minimal,
// well-behaved implementation, this fails to compile rather than silently
// changing behavior everywhere it's implemented.
var _ RepoAnalyzer = fakeRepoAnalyzer{}
