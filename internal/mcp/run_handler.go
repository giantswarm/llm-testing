package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/llm-testing/internal/kserve"
	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/runner"
	"github.com/giantswarm/llm-testing/internal/server"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

// modelInput represents a model configuration passed via the MCP "models" JSON parameter.
type modelInput struct {
	Name        string  `json:"name"`
	Temperature float64 `json:"temperature"`
	ModelURI    string  `json:"model_uri,omitempty"`
	GPUCount    int     `json:"gpu_count,omitempty"`
}

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

	deploy, _ := args["deploy"].(bool)

	// Override models from parameters.
	if modelsJSON, ok := args["models"].(string); ok && modelsJSON != "" {
		var models []modelInput
		if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid models JSON: %v", err)), nil
		}
		suiteModels := make([]testsuite.Model, 0, len(models))
		for _, m := range models {
			suiteModels = append(suiteModels, testsuite.Model{
				Name:        m.Name,
				Temperature: m.Temperature,
			})
		}
		suite.Models = suiteModels

		// If deploy is requested, deploy each model that has a model_uri.
		if deploy && sc.KServeManager != nil {
			for _, m := range models {
				if m.ModelURI != "" {
					if err := deployModelForRun(ctx, sc, m); err != nil {
						return mcp.NewToolResultError(fmt.Sprintf("failed to deploy model %q: %v", m.Name, err)), nil
					}
				}
			}
		}
	} else if modelName, ok := args["model"].(string); ok && modelName != "" {
		// Single model override (backward compatible).
		temp := 0.0
		if t, ok := args["temperature"].(float64); ok {
			temp = t
		}
		suite.Models = []testsuite.Model{{Name: modelName, Temperature: temp}}
	}

	// Determine the LLM client to use.
	client := sc.LLMClient
	if endpoint, ok := args["endpoint"].(string); ok && endpoint != "" {
		// Explicit endpoint overrides everything.
		client = llm.NewOpenAIClient(llm.WithBaseURL(endpoint))
	} else if sc.KServeManager != nil && len(suite.Models) > 0 {
		// Try KServe auto-discovery: look up the first model's endpoint.
		status, err := sc.KServeManager.Get(ctx, suite.Models[0].Name)
		if err == nil && status.Ready && status.EndpointURL != "" {
			slog.Info("auto-discovered KServe endpoint for model",
				"model", suite.Models[0].Name,
				"endpoint", status.EndpointURL,
			)
			client = llm.NewOpenAIClient(llm.WithBaseURL(status.EndpointURL))
		}
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

// deployModelForRun deploys a model via KServe before a test run.
func deployModelForRun(ctx context.Context, sc *server.ServerContext, m modelInput) error {
	cfg := kserve.DefaultModelConfig(m.Name, m.ModelURI)
	if m.GPUCount > 0 {
		cfg.GPUCount = m.GPUCount
	}

	slog.Info("deploying model for test run", "model", m.Name, "uri", m.ModelURI)
	_, err := sc.KServeManager.Deploy(ctx, cfg)
	return err
}
