package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

// mockClient is a test double for llm.Client.
type mockClient struct {
	responses map[string]string
	calls     int
}

func (m *mockClient) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.calls++
	resp, ok := m.responses[req.UserMessage]
	if !ok {
		resp = "default mock response"
	}
	return &llm.ChatResponse{Content: resp}, nil
}

func (m *mockClient) ChatCompletionStream(_ context.Context, _ llm.ChatRequest) (*llm.StreamReader, error) {
	return nil, assert.AnError
}

func TestRunnerExecutesSuite(t *testing.T) {
	tmpDir := t.TempDir()

	client := &mockClient{
		responses: map[string]string{
			"What is kubectl?": "kubectl is the Kubernetes CLI",
		},
	}

	strategy, err := GetStrategy("qa")
	require.NoError(t, err)

	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "test-suite",
		Strategy: "qa",
		Models:   []testsuite.Model{{Name: "test-model", Temperature: 0.0}},
		Prompt:   testsuite.Prompt{SystemMessage: "You are a test assistant."},
		Questions: []testsuite.Question{
			{ID: "1", Section: "Test", QuestionText: "What is kubectl?", ExpectedAnswer: "CLI tool"},
		},
	}

	run, err := r.Run(context.Background(), suite)
	require.NoError(t, err)

	assert.Equal(t, "test-suite", run.Suite)
	assert.Len(t, run.Models, 1)
	assert.Equal(t, "test-model", run.Models[0].ModelName)
	assert.Len(t, run.Models[0].Results, 1)
	assert.Equal(t, "kubectl is the Kubernetes CLI", run.Models[0].Results[0].Answer)
	assert.Equal(t, 1, client.calls)

	// Verify files were written.
	assert.FileExists(t, run.Models[0].ResultsFile)

	metadataFile := filepath.Join(tmpDir, run.ID, "resultset.json")
	assert.FileExists(t, metadataFile)
}

func TestRunnerMultipleModels(t *testing.T) {
	tmpDir := t.TempDir()

	client := &mockClient{responses: map[string]string{}}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "multi",
		Strategy: "qa",
		Models: []testsuite.Model{
			{Name: "model-a", Temperature: 0.0},
			{Name: "model-b", Temperature: 0.5},
		},
		Prompt: testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q?", ExpectedAnswer: "A"},
		},
	}

	run, err := r.Run(context.Background(), suite)
	require.NoError(t, err)
	assert.Len(t, run.Models, 2)
	assert.Equal(t, 2, client.calls) // one per model
}

func TestRunnerProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()

	client := &mockClient{responses: map[string]string{}}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	var progressCalls []int
	r.SetProgressFunc(func(model string, idx, total int) {
		progressCalls = append(progressCalls, idx)
	})

	suite := &testsuite.TestSuite{
		Name:     "progress",
		Strategy: "qa",
		Models:   []testsuite.Model{{Name: "m", Temperature: 0}},
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q1", ExpectedAnswer: "A1"},
			{ID: "2", Section: "S", QuestionText: "Q2", ExpectedAnswer: "A2"},
		},
	}

	_, err := r.Run(context.Background(), suite)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, progressCalls)
}

func TestRunnerContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a client that respects context cancellation.
	client := &mockClient{responses: map[string]string{}}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	suite := &testsuite.TestSuite{
		Name:     "cancel",
		Strategy: "qa",
		Models:   []testsuite.Model{{Name: "m", Temperature: 0}},
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q", ExpectedAnswer: "A"},
		},
	}

	// Should succeed before timeout.
	_, err := r.Run(ctx, suite)
	require.NoError(t, err)
}

func TestRunnerDefaultFilename(t *testing.T) {
	tmpDir := t.TempDir()

	client := &mockClient{responses: map[string]string{}}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "filename-test",
		Strategy: "qa",
		Models:   []testsuite.Model{{Name: "my-model", Temperature: 0}},
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q", ExpectedAnswer: "A"},
		},
	}

	run, err := r.Run(context.Background(), suite)
	require.NoError(t, err)

	// Verify the results file uses <model>.txt naming.
	expectedFile := filepath.Join(tmpDir, run.ID, "my-model.txt")
	assert.Equal(t, expectedFile, run.Models[0].ResultsFile)
	assert.FileExists(t, expectedFile)

	content, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "NO. 1")
}
