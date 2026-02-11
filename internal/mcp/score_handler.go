package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/llm-testing/internal/scorer"
	"github.com/giantswarm/llm-testing/internal/server"
)

func handleScoreResults(ctx context.Context, request mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	if sc.LLMClient == nil {
		return mcp.NewToolResultError("LLM client is not configured"), nil
	}

	args := request.GetArguments()

	resultsFile, _ := args["results_file"].(string)
	runID, _ := args["run_id"].(string)

	if resultsFile == "" && runID == "" {
		return mcp.NewToolResultError("either 'run_id' or 'results_file' is required"), nil
	}

	cfg := scorer.Config{
		Repetitions: 3,
	}

	if model, ok := args["scoring_model"].(string); ok && model != "" {
		cfg.Model = model
	}
	if reps, ok := args["repetitions"].(float64); ok && reps > 0 {
		cfg.Repetitions = int(reps)
	}

	s := scorer.NewScorer(sc.LLMClient, cfg)

	// If run_id is specified, resolve to the results files in the run directory.
	if runID != "" {
		return scoreByRunID(ctx, s, sc.OutputDir, runID)
	}

	return scoreSingleFile(ctx, s, resultsFile)
}

// scoreSingleFile scores a single results file.
func scoreSingleFile(ctx context.Context, s *scorer.Scorer, resultsFile string) (*mcp.CallToolResult, error) {
	output, err := s.ScoreFile(ctx, resultsFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scoring failed: %v", err)), nil
	}

	scoresFile, err := scorer.WriteScoreFile(output, resultsFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write scores: %v", err)), nil
	}

	result := map[string]interface{}{
		"scores_file": scoresFile,
		"summary":     output.Summary,
		"runs":        len(output.Runs),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// scoreByRunID finds all .txt result files in a run directory and scores each one.
func scoreByRunID(ctx context.Context, s *scorer.Scorer, outputDir, runID string) (*mcp.CallToolResult, error) {
	runPath := filepath.Join(outputDir, runID)

	entries, err := os.ReadDir(runPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run %q not found: %v", runID, err)), nil
	}

	// Find result files (*.txt, excluding score files).
	var resultFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".txt") && !strings.HasSuffix(name, "_scores.txt") {
			resultFiles = append(resultFiles, filepath.Join(runPath, name))
		}
	}

	if len(resultFiles) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("no result files found in run %q", runID)), nil
	}

	// Score each result file.
	type fileScore struct {
		ResultsFile string      `json:"results_file"`
		ScoresFile  string      `json:"scores_file"`
		Summary     interface{} `json:"summary"`
		Runs        int         `json:"runs"`
	}

	var scored []fileScore
	for _, rf := range resultFiles {
		output, err := s.ScoreFile(ctx, rf)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scoring failed for %s: %v", rf, err)), nil
		}

		scoresFile, err := scorer.WriteScoreFile(output, rf)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write scores for %s: %v", rf, err)), nil
		}

		scored = append(scored, fileScore{
			ResultsFile: rf,
			ScoresFile:  scoresFile,
			Summary:     output.Summary,
			Runs:        len(output.Runs),
		})
	}

	result := map[string]interface{}{
		"run_id": runID,
		"scored": scored,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
