package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/giantswarm/llm-testing/internal/testsuite"
)

func newListCmd() *cobra.Command {
	var suitesDir string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available test suites",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := testsuite.List(suitesDir)
			if err != nil {
				return fmt.Errorf("failed to list test suites: %w", err)
			}

			if len(names) == 0 {
				fmt.Println("No test suites found.")
				return nil
			}

			fmt.Printf("Available test suites:\n\n")
			for _, name := range names {
				suite, err := testsuite.Load(name, suitesDir)
				if err != nil {
					fmt.Printf("  - %s (error loading: %v)\n", name, err)
					continue
				}
				fmt.Printf("  - %s\n", suite.Name)
				fmt.Printf("    Description: %s\n", suite.Description)
				fmt.Printf("    Version: %s\n", suite.Version)
				fmt.Printf("    Strategy: %s\n", suite.Strategy)
				fmt.Printf("    Questions: %d\n\n", len(suite.Questions))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&suitesDir, "suites-dir", "", "External test suites directory")

	return cmd
}
