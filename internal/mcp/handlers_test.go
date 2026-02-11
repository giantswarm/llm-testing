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
	"github.com/giantswarm/llm-testing/internal/testutil"
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

func TestHandleRunTestSuiteNoClient(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: nil,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "kubernetes-cka-v2",
		"model":      "test-model",
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "LLM client is not configured")
}

func TestHandleRunTestSuiteNoModels(t *testing.T) {
	sc := &server.ServerContext{}

	// Test suite specified but no model.
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "kubernetes-cka-v2",
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "at least one model is required")
}

func TestHandleRunTestSuiteInvalidSuite(t *testing.T) {
	sc := &server.ServerContext{}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "nonexistent-suite",
		"model":      "test-model",
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "failed to load test suite")
}

func TestHandleRunTestSuiteInvalidModelsJSON(t *testing.T) {
	sc := &server.ServerContext{}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "kubernetes-cka-v2",
		"models":     "not valid json",
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "invalid models JSON")
}

func TestHandleRunTestSuiteEmptyModelName(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "kubernetes-cka-v2",
		"models":     `[{"name":"   "}]`,
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "model name cannot be empty")
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

func TestHandleScoreResultsNeitherRunIDNorFile(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "either 'run_id' or 'results_file' is required")
}

func TestHandleScoreResultsBothRunIDAndFile(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"run_id":       "run-1",
		"results_file": "run-1/model.txt",
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "provide only one of 'run_id' or 'results_file'")
}

func TestHandleScoreResultsRunIDPathTraversal(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
		OutputDir: t.TempDir(),
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"run_id": "../outside",
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "invalid run_id")
}

func TestHandleScoreResultsFilePathTraversal(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
		OutputDir: t.TempDir(),
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"results_file": "../outside.txt",
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "invalid results_file")
}

func TestHandleScoreResultsByRunIDNotFound(t *testing.T) {
	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
		OutputDir: "/nonexistent/path",
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"run_id": "nonexistent-run",
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "run \"nonexistent-run\" not found")
}

func TestHandleScoreResultsByRunIDNoResultFiles(t *testing.T) {
	tmpDir := t.TempDir()
	runDir := filepath.Join(tmpDir, "test-run")
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	// Create a resultset.json but no .txt files.
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "resultset.json"), []byte(`{}`), 0o644))

	sc := &server.ServerContext{
		LLMClient: &testutil.MockLLMClient{},
		OutputDir: tmpDir,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"run_id": "test-run",
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "no result files found")
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

func TestHandleGetResultsRunIDPathTraversal(t *testing.T) {
	sc := &server.ServerContext{
		OutputDir: t.TempDir(),
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"run_id": "../outside",
	}

	result, err := handleGetResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "invalid run_id")
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

func TestHandleDeployModelNoManagerTakesPrecedence(t *testing.T) {
	sc := &server.ServerContext{
		// A nil KServeManager should be caught before parameter validation.
		KServeManager: nil,
	}

	// Even with missing model_name, the nil-manager guard fires first.
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"model_uri": "hf://org/model",
	}

	result, err := handleDeployModel(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "KServe manager is not configured")
}

func TestHandleRunTestSuiteSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	client := &testutil.MockLLMClient{
		DefaultResponse: "The answer is kubectl.",
	}

	sc := &server.ServerContext{
		LLMClient: client,
		OutputDir: tmpDir,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"test_suite": "kubernetes-cka-v2",
		"model":      "test-model",
	}

	result, err := handleRunTestSuite(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)

	// Should return valid JSON summary.
	var summary map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content.Text), &summary))
	assert.Contains(t, summary, "run_id")
	assert.Contains(t, summary, "suite")
	assert.Contains(t, summary, "duration")
	assert.Contains(t, summary, "models")
	assert.Contains(t, summary, "deploy_enabled")
	assert.Contains(t, summary, "progress_updates")

	// The LLM client should have been called (100 questions for CKA).
	assert.Equal(t, 100, client.Calls)
}

func TestHandleScoreResultsFileSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a fake results file.
	resultsContent := `---
NO. 1 - Setup
QUESTION: What is kubectl?
EXPECTED ANSWER: CLI tool
ACTUAL ANSWER: kubectl is the Kubernetes CLI
`
	resultsFile := filepath.Join(tmpDir, "test-model.txt")
	require.NoError(t, os.WriteFile(resultsFile, []byte(resultsContent), 0o644))

	client := &testutil.MockLLMClient{
		DefaultResponse: "72 out of 100 answers are correct.",
	}

	sc := &server.ServerContext{
		LLMClient: client,
		OutputDir: tmpDir,
	}

	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{
		"results_file":  resultsFile,
		"scoring_model": "scoring-model",
		"repetitions":   float64(2),
	}

	result, err := handleScoreResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	var scoreResult map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content.Text), &scoreResult))
	assert.Contains(t, scoreResult, "scores_file")
	assert.Contains(t, scoreResult, "summary")
	assert.Equal(t, float64(2), scoreResult["runs"])
}

func TestHandleGetResultsWithRun(t *testing.T) {
	tmpDir := t.TempDir()
	runDir := filepath.Join(tmpDir, "test-run")
	require.NoError(t, os.MkdirAll(runDir, 0o755))

	metadata := `{"id": "test-run", "suite": "kubernetes-cka-v2", "timestamp": "2024-01-01T00:00:00Z"}`
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "resultset.json"), []byte(metadata), 0o644))

	sc := &server.ServerContext{
		OutputDir: tmpDir,
	}

	// List all runs.
	request := mcp.CallToolRequest{}
	request.Params.Arguments = map[string]interface{}{}

	result, err := handleGetResults(context.Background(), request, sc)
	require.NoError(t, err)

	content := result.Content[0].(mcp.TextContent)
	var runs []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content.Text), &runs))
	assert.Len(t, runs, 1)
	assert.Equal(t, "test-run", runs[0]["id"])
}
