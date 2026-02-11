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
		mcp.WithDescription(`Execute a test suite against one or more models. Models are specified at runtime -- they are NOT part of the test suite configuration.

When models have a 'model_uri', they can be automatically deployed via KServe InferenceService before testing and torn down afterwards. Models are tested sequentially to respect GPU memory constraints.

Use 'models' for multi-model configs (JSON array) or 'model' for a single model.`),
		mcp.WithString("test_suite",
			mcp.Required(),
			mcp.Description("Name of the test suite to run (e.g. 'kubernetes-cka-v2')"),
		),
		mcp.WithString("model",
			mcp.Description("Single model name to test. For multiple models, use the 'models' parameter instead."),
		),
		mcp.WithString("models",
			mcp.Description(`JSON array of model configs. Each model can include:
- "name" (required): model identifier
- "temperature": generation temperature (default: 0.0)
- "model_uri": KServe storage URI for auto-deploy (e.g. "hf://org/model")
- "gpu_count": GPUs to request when deploying (default: 1)

Example: [{"name":"mistral-7b","model_uri":"hf://mistralai/Mistral-7B-Instruct-v0.3","gpu_count":1}]`),
		),
		mcp.WithString("endpoint",
			mcp.Description("LLM endpoint URL (overrides KServe auto-discovery). Use when models are served externally."),
		),
		mcp.WithNumber("temperature",
			mcp.Description("Temperature for generation when using single 'model' param (default: 0.0)"),
		),
		mcp.WithBoolean("deploy",
			mcp.Description("Whether to auto-deploy models with model_uri via KServe (default: true)"),
		),
	)
	s.AddTool(runTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleRunTestSuite(ctx, request, sc)
	})

	// score_results
	scoreTool := mcp.NewTool("score_results",
		mcp.WithDescription("Score a completed test run using an LLM as judge. Provide exactly one of 'run_id' (all result files in a run) or 'results_file' (one specific file)."),
		mcp.WithString("run_id",
			mcp.Description("Run ID to score (scores all result files in the run directory)"),
		),
		mcp.WithString("results_file",
			mcp.Description("Path to a specific results file to score"),
		),
		mcp.WithString("scoring_model",
			mcp.Description("Model to use for scoring (default: claude-sonnet-4-5-20250514)"),
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
