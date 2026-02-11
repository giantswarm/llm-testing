package testsuite

import (
	"os"
	"path/filepath"
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

func TestLoadFromExternalDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	suiteName := "external-suite"
	suiteDir := filepath.Join(tmpDir, suiteName)
	require.NoError(t, os.MkdirAll(suiteDir, 0o755))

	// Write config.yaml.
	config := `name: External Test Suite
description: A test suite loaded from an external directory
version: "1"
strategy: qa
questions_file: questions.csv
prompt:
  system_message: "You are a test assistant."
`
	require.NoError(t, os.WriteFile(filepath.Join(suiteDir, "config.yaml"), []byte(config), 0o644))

	// Write questions.csv.
	csv := `ID,Section,Question,ExpectedAnswer
1,Basics,What is Go?,A programming language
2,Basics,What is gRPC?,A remote procedure call framework
`
	require.NoError(t, os.WriteFile(filepath.Join(suiteDir, "questions.csv"), []byte(csv), 0o644))

	// Load from external dir.
	suite, err := Load(suiteName, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, "External Test Suite", suite.Name)
	assert.Equal(t, "1", suite.Version)
	assert.Equal(t, "qa", suite.Strategy)
	assert.Len(t, suite.Questions, 2)
	assert.Equal(t, "What is Go?", suite.Questions[0].QuestionText)
	assert.Equal(t, "A programming language", suite.Questions[0].ExpectedAnswer)
}

func TestListIncludesExternalSuites(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "custom-suite"), 0o755))

	names, err := List(tmpDir)
	require.NoError(t, err)

	// Should include both embedded and external.
	assert.Contains(t, names, "kubernetes-cka-v2")
	assert.Contains(t, names, "custom-suite")
}

func TestExternalDirOverridesEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	suiteName := "kubernetes-cka-v2" // same name as embedded
	suiteDir := filepath.Join(tmpDir, suiteName)
	require.NoError(t, os.MkdirAll(suiteDir, 0o755))

	// Write a different config.
	config := `name: Custom CKA Override
description: Overrides the embedded suite
version: "99"
strategy: qa
questions_file: questions.csv
prompt:
  system_message: "Custom."
`
	require.NoError(t, os.WriteFile(filepath.Join(suiteDir, "config.yaml"), []byte(config), 0o644))

	csv := `ID,Section,Question,ExpectedAnswer
1,Test,Custom Q?,Custom A
`
	require.NoError(t, os.WriteFile(filepath.Join(suiteDir, "questions.csv"), []byte(csv), 0o644))

	// External directory takes precedence.
	suite, err := Load(suiteName, tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "Custom CKA Override", suite.Name)
	assert.Equal(t, "99", suite.Version)
	assert.Len(t, suite.Questions, 1)
}
