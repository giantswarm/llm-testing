package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/giantswarm/llm-testing/internal/kserve"
	"github.com/giantswarm/llm-testing/internal/server"
)

func registerModelTools(s *mcpserver.MCPServer, sc *server.ServerContext) error {
	// deploy_model
	deployTool := mcp.NewTool("deploy_model",
		mcp.WithDescription("Deploy a model via KServe InferenceService (vLLM runtime). Creates a new InferenceService CRD and waits for it to become ready."),
		mcp.WithString("model_name",
			mcp.Required(),
			mcp.Description("Name for the InferenceService resource"),
		),
		mcp.WithString("model_uri",
			mcp.Required(),
			mcp.Description("Model storage URI (e.g. 'hf://mistralai/Mistral-7B-Instruct-v0.3')"),
		),
		mcp.WithNumber("gpu_count",
			mcp.Description("Number of GPUs to request (default: 1)"),
		),
		mcp.WithArray("runtime_args",
			mcp.Description("Optional runtime arguments for the serving runtime (e.g. ['--max-model-len=4096'])"),
			mcp.WithStringItems(),
		),
	)
	s.AddTool(deployTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleDeployModel(ctx, request, sc)
	})

	// teardown_model
	teardownTool := mcp.NewTool("teardown_model",
		mcp.WithDescription("Delete a KServe InferenceService to stop serving a model"),
		mcp.WithString("model_name",
			mcp.Required(),
			mcp.Description("Name of the InferenceService to delete"),
		),
	)
	s.AddTool(teardownTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleTeardownModel(ctx, request, sc)
	})

	// list_models
	listTool := mcp.NewTool("list_models",
		mcp.WithDescription("List InferenceService resources managed by llm-testing"),
	)
	s.AddTool(listTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleListModels(ctx, request, sc)
	})

	return nil
}

func handleDeployModel(ctx context.Context, request mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	if sc.KServeManager == nil {
		return mcp.NewToolResultError("KServe manager is not configured (not running in-cluster or KServe not available)"), nil
	}

	args := request.GetArguments()

	modelName, ok := args["model_name"].(string)
	if !ok || modelName == "" {
		return mcp.NewToolResultError("model_name is required"), nil
	}

	modelURI, ok := args["model_uri"].(string)
	if !ok || modelURI == "" {
		return mcp.NewToolResultError("model_uri is required"), nil
	}

	cfg := kserve.DefaultModelConfig(modelName, modelURI)

	if gpuCount, ok := args["gpu_count"].(float64); ok && gpuCount > 0 {
		cfg.GPUCount = int(gpuCount)
	}
	if rawArgs, ok := args["runtime_args"].([]interface{}); ok && len(rawArgs) > 0 {
		runtimeArgs := make([]string, 0, len(rawArgs))
		for _, arg := range rawArgs {
			argStr, ok := arg.(string)
			if !ok {
				return mcp.NewToolResultError("runtime_args must be an array of strings"), nil
			}
			argStr = strings.TrimSpace(argStr)
			if argStr == "" {
				return mcp.NewToolResultError("runtime_args entries must be non-empty strings"), nil
			}
			runtimeArgs = append(runtimeArgs, argStr)
		}
		cfg.RuntimeArgs = runtimeArgs
	}

	status, err := sc.KServeManager.Deploy(ctx, cfg)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to deploy model: %v", err)), nil
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal status: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func handleTeardownModel(ctx context.Context, request mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	if sc.KServeManager == nil {
		return mcp.NewToolResultError("KServe manager is not configured"), nil
	}

	args := request.GetArguments()

	modelName, ok := args["model_name"].(string)
	if !ok || modelName == "" {
		return mcp.NewToolResultError("model_name is required"), nil
	}

	if err := sc.KServeManager.Teardown(ctx, modelName); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to teardown model: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("InferenceService %q deleted", modelName)), nil
}

func handleListModels(ctx context.Context, _ mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	if sc.KServeManager == nil {
		return mcp.NewToolResultError("KServe manager is not configured"), nil
	}

	statuses, err := sc.KServeManager.List(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list models: %v", err)), nil
	}

	data, err := json.MarshalIndent(statuses, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal statuses: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
