package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
