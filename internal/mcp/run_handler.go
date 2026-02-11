package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/llm-testing/internal/kserve"
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

	// Parse models from parameters (required).
	models, err := parseModels(args)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(models) == 0 {
		return mcp.NewToolResultError("at least one model is required: use 'models' (JSON array) or 'model' (single name)"), nil
	}
	if sc.LLMClient == nil {
		return mcp.NewToolResultError("LLM client is not configured"), nil
	}

	deployEnabled := true
	if deploy, ok := args["deploy"].(bool); ok {
		deployEnabled = deploy
	}

	strategy, err := runner.GetStrategy(suite.Strategy)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("unsupported strategy: %v", err)), nil
	}

	r := runner.NewRunner(sc.LLMClient, strategy, sc.OutputDir)

	// When KServe is available and models have model_uri, set up the
	// deploy -> test -> teardown lifecycle. Models are processed sequentially
	// to respect GPU memory constraints.
	if sc.KServeManager != nil {
		r.SetClientForModelFunc(func(ctx context.Context, model testsuite.Model) (llm.Client, error) {
			return clientForModel(ctx, sc, model, args, deployEnabled)
		})
		r.SetAfterModelFunc(func(ctx context.Context, model testsuite.Model) error {
			return teardownModel(ctx, sc, model, deployEnabled)
		})
	} else {
		// No KServe: use explicit endpoint if provided, otherwise default client.
		if endpoint, ok := args["endpoint"].(string); ok && endpoint != "" {
			r = runner.NewRunner(newEndpointClient(endpoint, sc.LLMAPIKey), strategy, sc.OutputDir)
		}
	}

	progressEvents := make([]map[string]interface{}, 0)
	r.SetProgressFunc(func(model string, questionIndex, totalQuestions int) {
		if questionIndex == 1 || questionIndex == totalQuestions || questionIndex%10 == 0 {
			progressEvents = append(progressEvents, map[string]interface{}{
				"model":               model,
				"completed_questions": questionIndex,
				"total_questions":     totalQuestions,
			})
		}
	})

	run, err := r.Run(ctx, suite, models)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("test run failed: %v", err)), nil
	}

	// Return summary.
	modelResults := make([]map[string]interface{}, 0, len(run.Models))
	for _, m := range run.Models {
		modelResults = append(modelResults, map[string]interface{}{
			"model":        m.ModelName,
			"results_file": m.ResultsFile,
			"duration":     m.Duration.String(),
		})
	}

	summary := map[string]interface{}{
		"run_id":           run.ID,
		"suite":            run.Suite,
		"duration":         run.Duration.String(),
		"models":           modelResults,
		"deploy_enabled":   deployEnabled,
		"progress_updates": progressEvents,
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal summary: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// parseModels extracts the model list from MCP tool arguments.
func parseModels(args map[string]interface{}) ([]testsuite.Model, error) {
	// Multi-model JSON array.
	if modelsJSON, ok := args["models"].(string); ok && modelsJSON != "" {
		var models []testsuite.Model
		if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
			return nil, fmt.Errorf("invalid models JSON: %v", err)
		}
		if err := validateModels(models); err != nil {
			return nil, err
		}
		return models, nil
	}

	// Single model shorthand.
	if modelName, ok := args["model"].(string); ok && modelName != "" {
		temp := 0.0
		if t, ok := args["temperature"].(float64); ok {
			temp = t
		}
		models := []testsuite.Model{{Name: modelName, Temperature: temp}}
		if err := validateModels(models); err != nil {
			return nil, err
		}
		return models, nil
	}

	return nil, nil
}

func validateModels(models []testsuite.Model) error {
	for _, model := range models {
		if strings.TrimSpace(model.Name) == "" {
			return fmt.Errorf("model name cannot be empty")
		}
	}
	return nil
}

// clientForModel handles the per-model lifecycle: deploy via KServe if needed,
// then return a client pointing to the model's endpoint.
func clientForModel(ctx context.Context, sc *server.ServerContext, model testsuite.Model, args map[string]interface{}, deployEnabled bool) (llm.Client, error) {
	// Explicit endpoint overrides everything.
	if endpoint, ok := args["endpoint"].(string); ok && endpoint != "" {
		return newEndpointClient(endpoint, sc.LLMAPIKey), nil
	}

	// Deploy via KServe if model_uri is provided.
	if deployEnabled && model.ModelURI != "" && sc.KServeManager != nil {
		cfg := kserve.DefaultModelConfig(model.Name, model.ModelURI)
		if model.GPUCount > 0 {
			cfg.GPUCount = model.GPUCount
		}

		slog.Info("deploying model for test run", "model", model.Name, "uri", model.ModelURI)
		status, err := sc.KServeManager.Deploy(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy model %q: %w", model.Name, err)
		}

		slog.Info("model deployed, using endpoint", "model", model.Name, "endpoint", status.EndpointURL)
		return llm.NewOpenAIClient(llm.WithBaseURL(status.EndpointURL)), nil
	}

	// Try auto-discovery from existing KServe InferenceService.
	if sc.KServeManager != nil {
		status, err := sc.KServeManager.Get(ctx, model.Name)
		if err == nil && status.Ready && status.EndpointURL != "" {
			slog.Info("auto-discovered KServe endpoint", "model", model.Name, "endpoint", status.EndpointURL)
			return llm.NewOpenAIClient(llm.WithBaseURL(status.EndpointURL)), nil
		}
	}

	// Fall back to default client.
	return sc.LLMClient, nil
}

// teardownModel cleans up a model's KServe InferenceService after testing.
// Only tears down models that were deployed by us (i.e. have a model_uri).
func teardownModel(ctx context.Context, sc *server.ServerContext, model testsuite.Model, deployEnabled bool) error {
	if !deployEnabled || model.ModelURI == "" || sc.KServeManager == nil {
		return nil // Not deployed by us, nothing to teardown.
	}

	slog.Info("tearing down model after test", "model", model.Name)
	if err := sc.KServeManager.Teardown(ctx, model.Name); err != nil {
		return fmt.Errorf("failed to teardown model %q: %w", model.Name, err)
	}
	return nil
}

func newEndpointClient(endpoint, apiKey string) llm.Client {
	opts := []llm.Option{llm.WithBaseURL(endpoint)}
	if apiKey != "" {
		opts = append(opts, llm.WithAPIKey(apiKey))
	}
	return llm.NewOpenAIClient(opts...)
}
