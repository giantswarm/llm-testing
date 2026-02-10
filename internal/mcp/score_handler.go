package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/llm-testing/internal/scorer"
	"github.com/giantswarm/llm-testing/internal/server"
)

func handleScoreResults(ctx context.Context, request mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	if sc.LLMClient == nil {
		return mcp.NewToolResultError("LLM client is not configured"), nil
	}

	args := request.GetArguments()

	resultsFile, ok := args["results_file"].(string)
	if !ok || resultsFile == "" {
		return mcp.NewToolResultError("results_file is required"), nil
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

	output, err := s.ScoreFile(ctx, resultsFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("scoring failed: %v", err)), nil
	}

	// Write scores file.
	scoresFile, err := scorer.WriteScoreFile(output, resultsFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write scores: %v", err)), nil
	}

	// Return result.
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
