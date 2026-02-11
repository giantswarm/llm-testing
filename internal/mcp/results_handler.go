package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/giantswarm/llm-testing/internal/server"
)

func handleGetResults(_ context.Context, request mcp.CallToolRequest, sc *server.ServerContext) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	runID, _ := args["run_id"].(string)

	if runID != "" {
		runPath, err := resolveRunPath(sc.OutputDir, runID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid run_id: %v", err)), nil
		}
		return getSpecificRun(runID, runPath)
	}
	return listRuns(sc.OutputDir)
}

func listRuns(outputDir string) (*mcp.CallToolResult, error) {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return mcp.NewToolResultText("[]"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to read results directory: %v", err)), nil
	}

	runs := make([]map[string]interface{}, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		metadataPath := filepath.Join(outputDir, e.Name(), "resultset.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(data, &metadata); err != nil {
			continue
		}

		// Check for score files.
		files, _ := os.ReadDir(filepath.Join(outputDir, e.Name()))
		var scoreFiles []string
		for _, f := range files {
			if strings.HasSuffix(f.Name(), "_scores.json") {
				scoreFiles = append(scoreFiles, f.Name())
			}
		}
		metadata["score_files"] = scoreFiles
		runs = append(runs, metadata)
	}

	data, err := json.MarshalIndent(runs, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal runs: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func getSpecificRun(runID, runPath string) (*mcp.CallToolResult, error) {
	metadataPath := filepath.Join(runPath, "resultset.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("run %q not found: %v", runID, err)), nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse run metadata: %v", err)), nil
	}

	// Include score data if available.
	files, _ := os.ReadDir(runPath)
	scores := make(map[string]interface{})
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "_scores.json") {
			scoreData, err := os.ReadFile(filepath.Join(runPath, f.Name()))
			if err == nil {
				var scoreObj interface{}
				if json.Unmarshal(scoreData, &scoreObj) == nil {
					scores[f.Name()] = scoreObj
				}
			}
		}
	}
	if len(scores) > 0 {
		metadata["scores"] = scores
	}

	result, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(result)), nil
}
