// Package cli wires Cobra commands to the core scan engine.
package cli

import "github.com/spf13/cobra"

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "lynxor",
		Short: "Security sanity check for Git repositories",
		Long: `Security sanity check for Git repositories: committed secrets, exposed keys,
risky Dockerfile patterns, CI/CD workflow misconfigurations, and
known-vulnerable dependencies. Checks the working tree and recent git
history (a secret committed and later removed is still a real leak).

  lynxor scan .              score this repo now
  lynxor diff main HEAD      what changed between two refs (for CI)

Run "lynxor scan --help" or "lynxor diff --help" for the full flag
reference and examples.`,
		Version: version,
	}
	root.AddCommand(newScanCmd())
	root.AddCommand(newDiffCmd())
	return root
}
