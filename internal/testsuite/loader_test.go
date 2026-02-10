package testsuite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEmbeddedSuite(t *testing.T) {
	suite, err := Load("kubernetes-cka-v2", "")
	require.NoError(t, err)

	assert.Equal(t, "Kubernetes CKA", suite.Name)
	assert.Equal(t, "2", suite.Version)
	assert.Equal(t, "qa", suite.Strategy)
	assert.Equal(t, 100, len(suite.Questions))
}

func TestLoadEmbeddedSuiteQuestions(t *testing.T) {
	suite, err := Load("kubernetes-cka-v2", "")
	require.NoError(t, err)

	// Verify first question.
	q := suite.Questions[0]
	assert.Equal(t, "1", q.ID)
	assert.Equal(t, "Setup & Aliases", q.Section)
	assert.Contains(t, q.QuestionText, "alias")
	assert.Contains(t, q.ExpectedAnswer, "alias k=kubectl")

	// Verify last question.
	last := suite.Questions[len(suite.Questions)-1]
	assert.Equal(t, "100", last.ID)
}

func TestLoadNonexistentSuite(t *testing.T) {
	_, err := Load("nonexistent-suite", "")
	assert.Error(t, err)
}

func TestListEmbeddedSuites(t *testing.T) {
	names, err := List("")
	require.NoError(t, err)
	assert.Contains(t, names, "kubernetes-cka-v2")
}

func TestSuiteDefaults(t *testing.T) {
	suite, err := Load("kubernetes-cka-v2", "")
	require.NoError(t, err)

	// Strategy defaults to "qa".
	assert.Equal(t, "qa", suite.Strategy)

	// Questions file defaults to "questions.csv".
	assert.Equal(t, "questions.csv", suite.QuestionsFile)
}

func TestSuitePromptConfig(t *testing.T) {
	suite, err := Load("kubernetes-cka-v2", "")
	require.NoError(t, err)

	assert.NotEmpty(t, suite.Prompt.SystemMessage)
	assert.Contains(t, suite.Prompt.SystemMessage, "Kubernetes")
}
