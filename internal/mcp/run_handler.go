package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/runner"
	"github.com/giantswarm/llm-testing/internal/server"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

func handleRunTestSuite(ctx context.Context, request mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	suiteName, ok := args["test_suite"].(string)
	if !ok || suiteName == "" {
		return mcp.NewToolResultError("test_suite is required"), nil
	}

	suite, err := testsuite.Load(suiteName, sc.SuitesDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load test suite: %v", err)), nil
	}

	// Override model if specified.
	if modelName, ok := args["model"].(string); ok && modelName != "" {
		temp := 0.0
		if t, ok := args["temperature"].(float64); ok {
			temp = t
		}
		suite.Models = []testsuite.Model{{Name: modelName, Temperature: temp}}
	}

	// Override endpoint if specified.
	client := sc.LLMClient
	if endpoint, ok := args["endpoint"].(string); ok && endpoint != "" {
		client = llm.NewOpenAIClient(llm.WithBaseURL(endpoint))
	}

	strategy, err := runner.GetStrategy(suite.Strategy)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("unsupported strategy: %v", err)), nil
	}

	r := runner.NewRunner(client, strategy, sc.OutputDir)
	run, err := r.Run(ctx, suite)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("test run failed: %v", err)), nil
	}

	// Return summary.
	models := make([]map[string]interface{}, 0, len(run.Models))
	for _, m := range run.Models {
		models = append(models, map[string]interface{}{
			"model":        m.ModelName,
			"results_file": m.ResultsFile,
			"duration":     m.Duration.String(),
		})
	}

	summary := map[string]interface{}{
		"run_id":   run.ID,
		"suite":    run.Suite,
		"duration": run.Duration.String(),
		"models":   models,
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal summary: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
