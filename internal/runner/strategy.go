package runner

import (
	"context"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

// EvaluationStrategy defines how a test suite is evaluated.
// Different strategies handle different types of evaluations (Q&A, tool-use, etc.).
type EvaluationStrategy interface {
	// Name returns the strategy identifier (e.g. "qa").
	Name() string

	// LoadQuestions prepares the evaluation items from a test suite.
	LoadQuestions(suite *testsuite.TestSuite) ([]testsuite.Question, error)

	// Execute runs a single question against the LLM and returns the result.
	Execute(ctx context.Context, client llm.Client, model string, question testsuite.Question, systemPrompt string, temperature float64) (*testsuite.Result, error)

	// FormatResults converts results into the output text format.
	FormatResults(results []*testsuite.Result) string
}

// GetStrategy returns an EvaluationStrategy for the given strategy name.
func GetStrategy(name string) (EvaluationStrategy, error) {
	switch name {
	case "qa", "":
		return &QAStrategy{}, nil
	default:
		return nil, &UnsupportedStrategyError{Name: name}
	}
}

// UnsupportedStrategyError is returned when an unknown strategy is requested.
type UnsupportedStrategyError struct {
	Name string
}

func (e *UnsupportedStrategyError) Error() string {
	return "unsupported evaluation strategy: " + e.Name
}
