package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of llm-testing",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "llm-testing version %s\n", rootCmd.Version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", buildCommit)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", buildDate)
		},
	}
}
