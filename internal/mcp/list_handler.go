package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/llm-testing/internal/server"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

func registerTestSuiteTools(s *mcpserver.MCPServer, sc *server.ServerContext) error {
	// list_test_suites
	listTool := mcp.NewTool("list_test_suites",
		mcp.WithDescription("List available LLM evaluation test suites with metadata"),
	)
	s.AddTool(listTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListTestSuites(ctx, request, sc)
	})

	// run_test_suite
	runTool := mcp.NewTool("run_test_suite",
		mcp.WithDescription("Execute a test suite against specified models. If models are deployed via KServe, the test connects to their endpoints automatically. Use 'models' for multi-model configs or 'model' for a single model override."),
		mcp.WithString("test_suite",
			mcp.Required(),
			mcp.Description("Name of the test suite to run (e.g. 'kubernetes-cka-v2')"),
		),
		mcp.WithString("model",
			mcp.Description("Single model name to test (overrides suite config). For multiple models, use the 'models' parameter instead."),
		),
		mcp.WithString("models",
			mcp.Description("JSON array of model configs, e.g. [{\"name\":\"gpt-4\",\"temperature\":0.0,\"model_uri\":\"hf://org/model\",\"gpu_count\":1}]. When 'deploy' is true, models with 'model_uri' are auto-deployed via KServe."),
		),
		mcp.WithString("endpoint",
			mcp.Description("LLM endpoint URL (overrides auto-discovery from KServe)"),
		),
		mcp.WithNumber("temperature",
			mcp.Description("Temperature for generation when using single 'model' param (default: from suite config)"),
		),
		mcp.WithBoolean("deploy",
			mcp.Description("Auto-deploy models via KServe InferenceService before running tests (requires 'models' with 'model_uri')"),
		),
	)
	s.AddTool(runTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleRunTestSuite(ctx, request, sc)
	})

	// score_results
	scoreTool := mcp.NewTool("score_results",
		mcp.WithDescription("Score a completed test run using an LLM as judge. Provide either 'run_id' to score all results in a run, or 'results_file' to score a specific file."),
		mcp.WithString("run_id",
			mcp.Description("Run ID to score (scores all result files in the run directory)"),
		),
		mcp.WithString("results_file",
			mcp.Description("Path to a specific results file to score"),
		),
		mcp.WithString("scoring_model",
			mcp.Description("Model to use for scoring (default: from config)"),
		),
		mcp.WithNumber("repetitions",
			mcp.Description("Number of scoring repetitions for confidence (default: 3)"),
		),
	)
	s.AddTool(scoreTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleScoreResults(ctx, request, sc)
	})

	// get_results
	getResultsTool := mcp.NewTool("get_results",
		mcp.WithDescription("Retrieve results and scores for past test runs"),
		mcp.WithString("run_id",
			mcp.Description("Specific run ID to retrieve (optional, lists all if omitted)"),
		),
	)
	s.AddTool(getResultsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleGetResults(ctx, request, sc)
	})

	return nil
}

func handleListTestSuites(_ context.Context, _ mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	names, err := testsuite.List(sc.SuitesDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list test suites: %v", err)), nil
	}

	type suiteInfo struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		Version       string `json:"version"`
		Strategy      string `json:"strategy"`
		QuestionCount int    `json:"question_count"`
	}

	var suites []suiteInfo
	for _, name := range names {
		suite, err := testsuite.Load(name, sc.SuitesDir)
		if err != nil {
			continue
		}
		suites = append(suites, suiteInfo{
			Name:          suite.Name,
			Description:   suite.Description,
			Version:       suite.Version,
			Strategy:      suite.Strategy,
			QuestionCount: len(suite.Questions),
		})
	}

	data, err := json.MarshalIndent(suites, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal test suites: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
