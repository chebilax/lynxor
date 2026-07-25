package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chebilax/lynxor/analyzers"
	"github.com/chebilax/lynxor/analyzers/cicd"
	"github.com/chebilax/lynxor/analyzers/dependencies"
	"github.com/chebilax/lynxor/analyzers/githistory"
	"github.com/chebilax/lynxor/analyzers/plugin"
	"github.com/chebilax/lynxor/core"
	"github.com/chebilax/lynxor/output"
)

func newScanCmd() *cobra.Command {
	var fullHistory bool
	var noHistory bool
	var checkDeps bool
	var pluginPaths []string
	var pluginArgs []string
	var format string

	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan a local repository for exposed secrets, Dockerfile, and CI/CD misconfigurations",
		Long: `Scan a local repository for exposed secrets, Dockerfile, and CI/CD misconfigurations, in the working tree and in git history.

By default, git history scanning is bounded by a short time budget so the
scan stays fast. --full-history removes that bound and can take several
minutes on repos with a large history (observed: ~18 minutes on a repo with
~18k commits) — if you use it in CI, set a generous timeout, or it will
look like a hung job.

Dependency vulnerability checking (go.sum, requirements.txt via OSV.dev)
is off by default too, for a different reason: it's the only check here
that needs the network. --deps enables it explicitly.

--plugin runs an external plugin executable alongside the built-in rules
(see docs/plugin-protocol.md). A plugin that crashes, times out, or
misbehaves is dropped for the rest of the scan with a warning — it never
fails the whole scan. --plugin-arg passes it configuration, addressed by
the plugin's own self-declared name from its handshake, e.g.
--plugin-arg my-plugin:api-key=xyz.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if fullHistory && noHistory {
				return fmt.Errorf("--full-history and --no-history are mutually exclusive")
			}
			if format != "cli" && format != "json" && format != "html" {
				return fmt.Errorf("--format must be \"cli\", \"json\", or \"html\", got %q", format)
			}
			argsByPlugin, err := parsePluginArgs(pluginArgs)
			if err != nil {
				return err
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			if strings.Contains(path, "://") {
				return fmt.Errorf("scanning a remote URL is not yet implemented, clone the repo and pass a local path instead")
			}

			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("cannot access %s: %w", path, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", path)
			}

			analyzerList := analyzers.BuiltinAnalyzers()
			var loadedPlugins []*plugin.Plugin
			for _, p := range pluginPaths {
				loaded, err := plugin.Load(p)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  %v — skipping this plugin\n", err)
					continue
				}
				if args, ok := argsByPlugin[loaded.Name()]; ok {
					if err := loaded.Configure(args); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  plugin %q: %v — skipping this plugin\n", loaded.Name(), err)
						loaded.Close()
						continue
					}
				}
				analyzerList = append(analyzerList, loaded)
				loadedPlugins = append(loadedPlugins, loaded)
			}
			closePlugins := func() {
				for _, p := range loadedPlugins {
					p.Close()
				}
			}
			// Covers every normal return path (including the error
			// returns below). os.Exit further down skips deferred calls
			// entirely, so that path calls closePlugins explicitly too —
			// closing lets a well-behaved plugin see stdin close and exit
			// cleanly, instead of relying on process teardown to do it.
			defer closePlugins()

			scanner := core.NewScanner(path, analyzerList...)
			for _, w := range scanner.Warnings() {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  gitignore: %s\n", w)
			}

			findings, err := scanner.Scan()
			if err != nil {
				return fmt.Errorf("scan failed: %w", err)
			}
			repoAnalyzers := []core.RepoAnalyzer{cicd.NewDependabotAnalyzer()}

			deps := dependencies.Discover(path)
			switch {
			case len(deps) == 0:
				// Nothing to check either way — stay silent, not every
				// repo has a Go or Python manifest.
			case checkDeps:
				result := dependencies.CheckVulnerabilities(deps)
				if result.Warning != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  %s\n", result.Warning)
				}
				findings = append(findings, result.Findings...)
			default:
				noun := "dependencies"
				if len(deps) == 1 {
					noun = "dependency"
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "ℹ️  Found %d %s — run with --deps to check them against known vulnerabilities (requires network).\n", len(deps), noun)
			}

			if !noHistory {
				opts := githistory.Options{FullHistory: fullHistory}
				if fullHistory {
					opts.OnProgress = func(n, total int) {
						if total > 0 {
							fmt.Fprintf(cmd.ErrOrStderr(), "   ... scanned %d/%d commits so far\n", n, total)
						} else {
							fmt.Fprintf(cmd.ErrOrStderr(), "   ... scanned %d commits so far\n", n)
						}
					}
				}
				repoAnalyzers = append(repoAnalyzers, githistory.NewAnalyzer(opts))
			}

			for _, ra := range repoAnalyzers {
				raFindings, err := ra.Run(path)
				switch {
				case errors.Is(err, githistory.ErrNotAGitRepo):
					// Not every scan target is a git checkout; working-tree
					// scanning alone is still fully valid.
				case err != nil:
					fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  %s: %v\n", ra.Name(), err)
				default:
					findings = append(findings, raFindings...)
				}
			}

			score := core.ComputeCategoryScore(findings)
			switch format {
			case "json":
				if err := output.WriteJSONReport(cmd.OutOrStdout(), findings, score); err != nil {
					return fmt.Errorf("writing JSON report: %w", err)
				}
			case "html":
				if err := output.WriteHTMLReport(cmd.OutOrStdout(), findings, score, path); err != nil {
					return fmt.Errorf("writing HTML report: %w", err)
				}
			default:
				output.WriteReport(cmd.OutOrStdout(), findings, score)
			}

			if score.Value < 70 {
				closePlugins()
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&fullHistory, "full-history", false, "scan entire reachable history + dangling commits, no time budget — can take several minutes on large repos, avoid in CI without a generous timeout")
	cmd.Flags().BoolVar(&noHistory, "no-history", false, "skip git history scanning, working tree only")
	cmd.Flags().BoolVar(&checkDeps, "deps", false, "check go.sum/requirements.txt dependencies against known vulnerabilities via OSV.dev — requires network, off by default")
	cmd.Flags().StringArrayVar(&pluginPaths, "plugin", nil, "path to an external plugin executable (see docs/plugin-protocol.md) — repeatable")
	cmd.Flags().StringArrayVar(&pluginArgs, "plugin-arg", nil, `configuration for a loaded plugin, format "<plugin_name>:<key>=<value>" — repeatable, matched against the plugin's own self-declared name from its handshake (see docs/plugin-protocol.md)`)
	cmd.Flags().StringVar(&format, "format", "cli", `output format: "cli" (default, colored terminal), "json" (machine-readable, docs/decisions/0009-json-output-schema.md), or "html" (self-contained dashboard, docs/decisions/0010-html-dashboard.md)`)

	return cmd
}

// parsePluginArgs turns repeated "<plugin_name>:<key>=<value>" flag values
// into a lookup by plugin name. Name-addressed rather than positional
// (matching --plugin occurrences up by order) or uniform-for-all-plugins:
// --plugin is repeatable, and Cobra/pflag give no ordering guarantee that
// would let a positional scheme reliably pair a --plugin-arg with the
// --plugin instance it's meant for. Addressing by the plugin's own
// self-declared name (from its hello_ack) has no such ambiguity, at the
// cost of the user needing to already know that name -- already documented
// in docs/plugin-protocol.md as something a plugin author must declare.
func parsePluginArgs(raw []string) (map[string]map[string]string, error) {
	byPlugin := map[string]map[string]string{}
	for _, entry := range raw {
		name, kv, ok := strings.Cut(entry, ":")
		if !ok {
			return nil, fmt.Errorf(`--plugin-arg %q: expected format "<plugin_name>:<key>=<value>"`, entry)
		}
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf(`--plugin-arg %q: expected format "<plugin_name>:<key>=<value>"`, entry)
		}
		if byPlugin[name] == nil {
			byPlugin[name] = map[string]string{}
		}
		byPlugin[name][key] = value
	}
	return byPlugin, nil
}
