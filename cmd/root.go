package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "llm-testing",
	Short: "LLM evaluation testing framework with MCP server",
	Long: `llm-testing is a framework for evaluating LLM performance on domain-specific
test suites. It manages model serving via KServe InferenceService CRDs (vLLM runtime),
runs Q&A evaluations, scores results using LLM-as-judge, and exposes all functionality
via an MCP server with OAuth 2.1 authentication.

When run without subcommands, it starts the MCP server (equivalent to 'llm-testing serve').`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, _ []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")
		if verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})))
		}
	},
}

// serveCmd is stored so the root command can delegate to it by default.
var serveCmd *cobra.Command

var (
	buildCommit = "unknown"
	buildDate   = "unknown"
)

// SetVersion sets the version for the root command.
func SetVersion(v string) {
	rootCmd.Version = v
}

// SetBuildInfo sets the commit and build date for the version command.
func SetBuildInfo(commit, date string) {
	buildCommit = commit
	buildDate = date
}

// Execute is the main entry point for the CLI application.
func Execute() {
	rootCmd.SetVersionTemplate(`{{printf "llm-testing version %s\n" .Version}}`)

	// Default to the serve command when invoked without arguments.
	// We use Run (not RunE) to print the help text directing the user to use
	// an explicit subcommand, since the root command cannot parse serve-specific
	// flags (like --transport, --http-addr).
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "No subcommand specified. Defaulting to 'serve' (stdio transport).")
		fmt.Fprintln(os.Stderr, "For HTTP transport or OAuth, use: llm-testing serve --transport streamable-http")
		fmt.Fprintln(os.Stderr)
		if err := serveCmd.RunE(serveCmd, args); err != nil {
			slog.Error("serve failed", "error", err)
			os.Exit(1)
		}
	}

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	serveCmd = newServeCmd()
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newScoreCmd())
	rootCmd.AddCommand(newListCmd())

	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().String("kubeconfig", "", "Path to kubeconfig file")
	rootCmd.PersistentFlags().StringP("namespace", "n", "llm-testing", "Kubernetes namespace for InferenceService resources")
}
