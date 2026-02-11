package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

// QAStrategy implements EvaluationStrategy for question-and-answer tests.
type QAStrategy struct{}

func (s *QAStrategy) Name() string {
	return "qa"
}

func (s *QAStrategy) LoadQuestions(suite *testsuite.TestSuite) ([]testsuite.Question, error) {
	if len(suite.Questions) == 0 {
		return nil, fmt.Errorf("test suite has no questions")
	}
	return suite.Questions, nil
}

func (s *QAStrategy) Execute(ctx context.Context, client llm.Client, model string, question testsuite.Question, systemPrompt string, temperature float64) (*testsuite.Result, error) {
	start := time.Now()

	resp, err := client.ChatCompletion(ctx, llm.ChatRequest{
		Model:         model,
		SystemMessage: systemPrompt,
		UserMessage:   question.QuestionText,
		Temperature:   llm.Float64Ptr(temperature),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get completion for question %s: %w", question.ID, err)
	}

	return &testsuite.Result{
		Question: question,
		Answer:   resp.Content,
		Duration: time.Since(start),
	}, nil
}

func (s *QAStrategy) FormatResults(results []*testsuite.Result) string {
	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "---\n")
		fmt.Fprintf(&b, "NO. %s - %s\n", r.Question.ID, r.Question.Section)
		fmt.Fprintf(&b, "QUESTION: %s\n", r.Question.QuestionText)
		fmt.Fprintf(&b, "EXPECTED ANSWER: %s\n", r.Question.ExpectedAnswer)
		fmt.Fprintf(&b, "ACTUAL ANSWER: %s\n", r.Answer)
	}
	return b.String()
}
