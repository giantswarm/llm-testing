package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

func TestQAStrategyLoadQuestions(t *testing.T) {
	s := &QAStrategy{}

	suite := &testsuite.TestSuite{
		Questions: []testsuite.Question{
			{ID: "1", Section: "Test", QuestionText: "What?", ExpectedAnswer: "42"},
		},
	}

	questions, err := s.LoadQuestions(suite)
	require.NoError(t, err)
	assert.Len(t, questions, 1)
}

func TestQAStrategyLoadQuestionsEmpty(t *testing.T) {
	s := &QAStrategy{}
	suite := &testsuite.TestSuite{}

	_, err := s.LoadQuestions(suite)
	assert.Error(t, err)
}

func TestQAStrategyFormatResults(t *testing.T) {
	s := &QAStrategy{}

	results := []*testsuite.Result{
		{
			Question: testsuite.Question{
				ID:             "1",
				Section:        "Setup",
				QuestionText:   "What is kubectl?",
				ExpectedAnswer: "CLI tool",
			},
			Answer: "kubectl is the Kubernetes command-line tool",
		},
	}

	output := s.FormatResults(results)
	assert.Contains(t, output, "NO. 1 - Setup")
	assert.Contains(t, output, "QUESTION: What is kubectl?")
	assert.Contains(t, output, "EXPECTED ANSWER: CLI tool")
	assert.Contains(t, output, "ACTUAL ANSWER: kubectl is the Kubernetes command-line tool")
}

// qaTestClient is a mock LLM client for QA strategy tests.
type qaTestClient struct{}

func (c *qaTestClient) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "mock answer for: " + req.UserMessage}, nil
}

func (c *qaTestClient) ChatCompletionStream(_ context.Context, _ llm.ChatRequest) (*llm.StreamReader, error) {
	return nil, assert.AnError
}

func TestQAStrategyExecute(t *testing.T) {
	s := &QAStrategy{}
	client := &qaTestClient{}

	question := testsuite.Question{
		ID:             "42",
		Section:        "Test",
		QuestionText:   "What is a Pod?",
		ExpectedAnswer: "Smallest deployable unit",
	}

	result, err := s.Execute(context.Background(), client, "test-model", question, "You are helpful.", 0.0)
	require.NoError(t, err)
	assert.Equal(t, "42", result.Question.ID)
	assert.Equal(t, "mock answer for: What is a Pod?", result.Answer)
	assert.True(t, result.Duration > 0)
}

func TestQAStrategyExecutePassesSystemPrompt(t *testing.T) {
	s := &QAStrategy{}

	// Client that captures the system message.
	var capturedSystem string
	client := &captureClient{onCompletion: func(req llm.ChatRequest) {
		capturedSystem = req.SystemMessage
	}}

	question := testsuite.Question{
		ID:           "1",
		QuestionText: "test",
	}

	_, err := s.Execute(context.Background(), client, "model", question, "custom system prompt", 0.5)
	require.NoError(t, err)
	assert.Equal(t, "custom system prompt", capturedSystem)
}

// captureClient captures request parameters for verification.
type captureClient struct {
	onCompletion func(llm.ChatRequest)
}

func (c *captureClient) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if c.onCompletion != nil {
		c.onCompletion(req)
	}
	return &llm.ChatResponse{Content: "ok"}, nil
}

func (c *captureClient) ChatCompletionStream(_ context.Context, _ llm.ChatRequest) (*llm.StreamReader, error) {
	return nil, assert.AnError
}
