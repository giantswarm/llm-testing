package cmd

import (
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
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return serveCmd.RunE(serveCmd, args)
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
