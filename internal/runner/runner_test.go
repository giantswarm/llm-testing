package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/llm-testing/internal/testutil"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

func TestRunnerExecutesSuite(t *testing.T) {
	tmpDir := t.TempDir()

	client := &testutil.MockLLMClient{
		Responses: map[string]string{
			"What is kubectl?": "kubectl is the Kubernetes CLI",
		},
	}

	strategy, err := GetStrategy("qa")
	require.NoError(t, err)

	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "test-suite",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "You are a test assistant."},
		Questions: []testsuite.Question{
			{ID: "1", Section: "Test", QuestionText: "What is kubectl?", ExpectedAnswer: "CLI tool"},
		},
	}

	models := []testsuite.Model{{Name: "test-model", Temperature: 0.0}}

	run, err := r.Run(context.Background(), suite, models)
	require.NoError(t, err)

	assert.Equal(t, "test-suite", run.Suite)
	assert.Len(t, run.Models, 1)
	assert.Equal(t, "test-model", run.Models[0].ModelName)
	assert.Len(t, run.Models[0].Results, 1)
	assert.Equal(t, "kubectl is the Kubernetes CLI", run.Models[0].Results[0].Answer)
	assert.Equal(t, 1, client.Calls)

	// Verify files were written.
	assert.FileExists(t, run.Models[0].ResultsFile)

	metadataFile := filepath.Join(tmpDir, run.ID, "resultset.json")
	assert.FileExists(t, metadataFile)
}

func TestRunnerMultipleModels(t *testing.T) {
	tmpDir := t.TempDir()

	client := &testutil.MockLLMClient{}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "multi",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q?", ExpectedAnswer: "A"},
		},
	}

	models := []testsuite.Model{
		{Name: "model-a", Temperature: 0.0},
		{Name: "model-b", Temperature: 0.5},
	}

	run, err := r.Run(context.Background(), suite, models)
	require.NoError(t, err)
	assert.Len(t, run.Models, 2)
	assert.Equal(t, 2, client.Calls) // one per model
}

func TestRunnerNoModels(t *testing.T) {
	tmpDir := t.TempDir()

	client := &testutil.MockLLMClient{}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "empty",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q?", ExpectedAnswer: "A"},
		},
	}

	_, err := r.Run(context.Background(), suite, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no models specified")
}

func TestRunnerProgressCallback(t *testing.T) {
	tmpDir := t.TempDir()

	client := &testutil.MockLLMClient{}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	var progressCalls []int
	r.SetProgressFunc(func(model string, idx, total int) {
		progressCalls = append(progressCalls, idx)
	})

	suite := &testsuite.TestSuite{
		Name:     "progress",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q1", ExpectedAnswer: "A1"},
			{ID: "2", Section: "S", QuestionText: "Q2", ExpectedAnswer: "A2"},
		},
	}

	models := []testsuite.Model{{Name: "m", Temperature: 0}}

	_, err := r.Run(context.Background(), suite, models)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, progressCalls)
}

func TestRunnerContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a client that respects context cancellation.
	client := &testutil.MockLLMClient{}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	suite := &testsuite.TestSuite{
		Name:     "cancel",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q", ExpectedAnswer: "A"},
		},
	}

	models := []testsuite.Model{{Name: "m", Temperature: 0}}

	// Should succeed before timeout.
	_, err := r.Run(ctx, suite, models)
	require.NoError(t, err)
}

func TestRunnerDefaultFilename(t *testing.T) {
	tmpDir := t.TempDir()

	client := &testutil.MockLLMClient{}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	suite := &testsuite.TestSuite{
		Name:     "filename-test",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q", ExpectedAnswer: "A"},
		},
	}

	models := []testsuite.Model{{Name: "my-model", Temperature: 0}}

	run, err := r.Run(context.Background(), suite, models)
	require.NoError(t, err)

	// Verify the results file uses <model>.txt naming.
	expectedFile := filepath.Join(tmpDir, run.ID, "my-model.txt")
	assert.Equal(t, expectedFile, run.Models[0].ResultsFile)
	assert.FileExists(t, expectedFile)

	content, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "NO. 1")
}

func TestRunnerAfterModelHook(t *testing.T) {
	tmpDir := t.TempDir()

	client := &testutil.MockLLMClient{}
	strategy, _ := GetStrategy("qa")
	r := NewRunner(client, strategy, tmpDir)

	var teardownCalls []string
	r.SetAfterModelFunc(func(ctx context.Context, model testsuite.Model) error {
		teardownCalls = append(teardownCalls, model.Name)
		return nil
	})

	suite := &testsuite.TestSuite{
		Name:     "hooks",
		Strategy: "qa",
		Prompt:   testsuite.Prompt{SystemMessage: "test"},
		Questions: []testsuite.Question{
			{ID: "1", Section: "S", QuestionText: "Q", ExpectedAnswer: "A"},
		},
	}

	models := []testsuite.Model{
		{Name: "model-a"},
		{Name: "model-b"},
	}

	_, err := r.Run(context.Background(), suite, models)
	require.NoError(t, err)
	assert.Equal(t, []string{"model-a", "model-b"}, teardownCalls)
}
