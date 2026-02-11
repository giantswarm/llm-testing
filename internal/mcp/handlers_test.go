package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/llm-testing/internal/server"
)

func TestHandleListTestSuites(t *testing.T) {
	sc := &server.ServerContext{
		SuitesDir: "",
	}

	result, err := handleListTestSuites(context.Background(), mcp.CallToolRequest{}, sc)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return at least the embedded kubernetes-cka-v2 suite.
	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "Kubernetes CKA")

	// Verify it's valid JSON.
	var suites []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content.Text), &suites))
	assert.GreaterOrEqual(t, len(suites), 1)

	// Verify required fields.
	s := suites[0]
	assert.Contains(t, s, "name")
	assert.Contains(t, s, "description")
	assert.Contains(t, s, "version")
	assert.Contains(t, s, "strategy")
	assert.Contains(t, s, "question_count")
}

func TestHandleRunTestSuiteMissingRequired(t *testing.T) {
	sc := &server.ServerContext{}

	// Missing test_suite parameter.
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "test_suite is required")
}

func TestHandleRunTestSuiteInvalidSuite(t *testing.T) {
	sc := &server.ServerContext{}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "nonexistent-suite",
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "failed to load test suite")
}

func TestHandleScoreResultsMissingRequired(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: nil, // No client configured.
	}

	// Missing results_file AND no LLM client -- client check comes first.
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "LLM client is not configured")
}

func TestHandleScoreResultsNoClient(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: nil,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"results_file": "some-file.txt",
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "LLM client is not configured")
}

func TestHandleGetResultsEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	sc := &server.ServerContext{
		OutputDir: tmpDir,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleGetResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	// Should return empty list, not an error.
	assert.Equal(t, "[]", content.Text)
}

func TestHandleGetResultsNonexistentDir(t *testing.T) {
	sc := &server.ServerContext{
		OutputDir: "/nonexistent/directory",
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleGetResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Equal(t, "[]", content.Text)
}

func TestHandleGetResultsSpecificRun(t *testing.T) {
	tmpDir := t.TempDir()
	runDir := filepath.Join(tmpDir, "test-run")
	require.NoError(t, os.MkdirAll(runDir, 0o755))

	metadata := `{"id": "test-run", "suite": "test"}`
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "resultset.json"), []byte(metadata), 0o644))

	sc := &server.ServerContext{
		OutputDir: tmpDir,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"run_id": "test-run",
	}

	result, err := handleGetResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "test-run")
}

func TestHandleDeployModelNoManager(t *testing.T) {
	sc := &server.ServerContext{
		KServeManager: nil,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"model_name": "test",
		"model_uri":  "hf://org/model",
	}

	result, err := handleDeployModel(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "KServe manager is not configured")
}

func TestHandleTeardownModelNoManager(t *testing.T) {
	sc := &server.ServerContext{
		KServeManager: nil,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"model_name": "test",
	}

	result, err := handleTeardownModel(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "KServe manager is not configured")
}

func TestHandleListModelsNoManager(t *testing.T) {
	sc := &server.ServerContext{
		KServeManager: nil,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleListModels(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "KServe manager is not configured")
}

func TestHandleDeployModelMissingRequired(t *testing.T) {
	sc := &server.ServerContext{
		// A nil KServeManager should be caught first.
		KServeManager: nil,
	}

	// Missing model_name.
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"model_uri": "hf://org/model",
	}

	result, err := handleDeployModel(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "KServe manager is not configured")
}
