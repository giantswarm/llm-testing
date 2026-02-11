package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/giantswarm/llm-testing/internal/runner"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

func newRunCmd() *cobra.Command {
	var (
		model       string
		endpoint    string
		apiKey      string
		temperature float64
		outputDir   string
		suitesDir   string
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "run <test-suite>",
		Short: "Run a test suite against an LLM endpoint",
		Long: `Execute a test suite by sending questions to an LLM and recording the responses.

Results are written to the output directory as text files with a JSON metadata manifest.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			suiteName := args[0]

			suite, err := testsuite.Load(suiteName, suitesDir)
			if err != nil {
				return fmt.Errorf("failed to load test suite: %w", err)
			}

			// Override model if specified via flag.
			if model != "" {
				suite.Models = []testsuite.Model{{Name: model, Temperature: temperature}}
			}

			// Set up LLM client.
			client := newLLMClientFromFlags(endpoint, apiKey)

			strategy, err := runner.GetStrategy(suite.Strategy)
			if err != nil {
				return err
			}

			r := runner.NewRunner(client, strategy, outputDir)
			r.SetProgressFunc(func(modelName string, idx, total int) {
				fmt.Printf("\r  [%s] Processing question %d/%d...", modelName, idx, total)
			})

			fmt.Printf("Test Suite: %s\n", suite.Name)
			fmt.Printf("Description: %s\n", suite.Description)
			fmt.Printf("Models to test: %d\n", len(suite.Models))
			for i, m := range suite.Models {
				fmt.Printf("  %d. %s (temperature: %.1f)\n", i+1, m.Name, m.Temperature)
			}
			fmt.Println()

			run, err := r.Run(ctx, suite)
			if err != nil {
				return err
			}

			fmt.Printf("\n\nTest suite completed.\n")
			fmt.Printf("Run ID: %s\n", run.ID)
			fmt.Printf("Duration: %s\n", run.Duration)
			fmt.Printf("Results:\n")
			for _, m := range run.Models {
				fmt.Printf("  - %s: %s\n", m.ModelName, m.ResultsFile)
			}

			slog.Info("test run complete", "run_id", run.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Model name (overrides suite config)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "LLM API endpoint URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key (or set OPENAI_API_KEY)")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.0, "Temperature for generation")
	cmd.Flags().StringVar(&outputDir, "output-dir", "results", "Directory for test results")
	cmd.Flags().StringVar(&suitesDir, "suites-dir", "", "External test suites directory")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Overall timeout for the test run (e.g. 30m, 1h). 0 means no timeout")

	return cmd
}
