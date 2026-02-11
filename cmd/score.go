package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/scorer"
)

func newScoreCmd() *cobra.Command {
	var (
		scoringModel    string
		scoringEndpoint string
		scoringAPIKey   string
		repetitions     int
	)

	cmd := &cobra.Command{
		Use:   "score <results-file>",
		Short: "Score a results file using an LLM as judge",
		Long: `Evaluate test results by sending them to a scoring LLM that assesses
correctness. Runs multiple evaluation passes for confidence and outputs structured
JSON scores.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resultsFile := args[0]

			if _, err := os.Stat(resultsFile); os.IsNotExist(err) {
				return fmt.Errorf("results file not found: %s", resultsFile)
			}

			var opts []llm.Option
			if scoringEndpoint != "" {
				opts = append(opts, llm.WithBaseURL(scoringEndpoint))
			}
			if scoringAPIKey != "" {
				opts = append(opts, llm.WithAPIKey(scoringAPIKey))
			} else if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" {
				opts = append(opts, llm.WithAPIKey(envKey))
			}
			client := llm.NewOpenAIClient(opts...)

			s := scorer.NewScorer(client, scorer.Config{
				Model:       scoringModel,
				Repetitions: repetitions,
				Endpoint:    scoringEndpoint,
			})

			fmt.Printf("Scoring: %s\n", resultsFile)
			fmt.Printf("Model: %s\n", scoringModel)
			fmt.Printf("Repetitions: %d\n", repetitions)
			fmt.Println()

			output, err := s.ScoreFile(cmd.Context(), resultsFile)
			if err != nil {
				return err
			}

			scoresFile, err := scorer.WriteScoreFile(output, resultsFile)
			if err != nil {
				return err
			}

			fmt.Printf("\nScores written to: %s\n", scoresFile)

			if output.Summary.MeanCorrect != nil {
				fmt.Printf("\nSummary:\n")
				fmt.Printf("  Mean Score: %.2f/%d (%.2f%%)\n",
					*output.Summary.MeanCorrect,
					*output.Runs[0].Total,
					*output.Summary.MeanPercent)
				fmt.Printf("  Range: %d-%d correct\n",
					*output.Summary.MinCorrect,
					*output.Summary.MaxCorrect)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&scoringModel, "scoring-model", "", "Scoring model name")
	cmd.Flags().StringVar(&scoringEndpoint, "scoring-endpoint", "", "Scoring LLM endpoint URL")
	cmd.Flags().StringVar(&scoringAPIKey, "api-key", "", "Scoring API key (or set OPENAI_API_KEY)")
	cmd.Flags().IntVar(&repetitions, "repetitions", 3, "Number of scoring repetitions")

	return cmd
}
