package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/chebilax/lynxor/analyzers/diffmode"
	"github.com/chebilax/lynxor/output"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <ref-a> <ref-b>",
		Short: "Show security findings introduced or fixed between two git refs",
		Long: `Show security findings introduced or fixed between two git refs (branches,
tags, or commits), instead of a full repo score. Built for CI on a pull
request: it answers "what does THIS change introduce or fix", not "what's
the whole repo's score" - the same secrets, Docker, and CI/CD rules as
"lynxor scan", just diffed between two points instead of scored at one.

<ref-a> is the base (before), <ref-b> is the head (after): a finding
present only in <ref-b> is NEW (this change introduced it), a finding
present only in <ref-a> is FIXED (this change removed it). In a GitHub
Actions pull_request event, that's the PR's base and head SHAs, in that
order - see action.yml.

  lynxor diff main HEAD                  compare the local main branch to HEAD
  lynxor diff origin/main feature-xyz    compare against a remote branch

Dependency vulnerability checking (--deps in "scan") has no equivalent
here yet - it's opt-in and network-dependent there for the same reason it
would be here.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			refA, refB := args[0], args[1]

			findings, err := diffmode.Diff(".", refA, refB)
			switch {
			case errors.Is(err, diffmode.ErrNotAGitRepo):
				return fmt.Errorf("not a git repository: diff mode needs git history to compare, unlike a plain scan")
			case err != nil:
				return fmt.Errorf("diff failed: %w", err)
			}

			output.WriteDiffReport(cmd.OutOrStdout(), findings)

			for _, f := range findings {
				if f.Status == diffmode.New {
					os.Exit(1)
				}
			}
			return nil
		},
	}

	return cmd
}
