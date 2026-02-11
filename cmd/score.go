package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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

			client := newLLMClientFromFlags(scoringEndpoint, scoringAPIKey)

			s := scorer.NewScorer(client, scorer.Config{
				Model:       scoringModel,
				Repetitions: repetitions,
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

			if output.Summary.MeanCorrect != nil && output.Summary.MeanPercent != nil {
				fmt.Printf("\nSummary:\n")
				// Find the total from the first run that was successfully parsed.
				var total int
				for _, r := range output.Runs {
					if r.Total != nil {
						total = *r.Total
						break
					}
				}
				fmt.Printf("  Mean Score: %.2f/%d (%.2f%%)\n",
					*output.Summary.MeanCorrect,
					total,
					*output.Summary.MeanPercent)
				if output.Summary.MinCorrect != nil && output.Summary.MaxCorrect != nil {
					fmt.Printf("  Range: %d-%d correct\n",
						*output.Summary.MinCorrect,
						*output.Summary.MaxCorrect)
				}
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
