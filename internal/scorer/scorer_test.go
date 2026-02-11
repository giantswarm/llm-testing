package scorer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/llm-testing/internal/llm"
)

func TestParseScore(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		correct int
		total   int
		hasErr  bool
	}{
		{
			name:    "standard format",
			input:   "58 out of 100 answers are correct.",
			correct: 58,
			total:   100,
		},
		{
			name:    "different numbers",
			input:   "75 out of 100 are correct",
			correct: 75,
			total:   100,
		},
		{
			name:    "with surrounding text",
			input:   "After careful evaluation, I found that 42 out of 100 answers are correct. The candidate showed...",
			correct: 42,
			total:   100,
		},
		{
			name:   "unparseable",
			input:  "The candidate did well overall.",
			hasErr: true,
		},
		{
			name:   "empty",
			input:  "",
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseScore(tt.input)
			if tt.hasErr {
				assert.NotEmpty(t, result.ParseErr)
				assert.Nil(t, result.Correct)
			} else {
				assert.Empty(t, result.ParseErr)
				require.NotNil(t, result.Correct)
				assert.Equal(t, tt.correct, *result.Correct)
				assert.Equal(t, tt.total, *result.Total)
			}
		})
	}
}

func TestCalculateStatistics(t *testing.T) {
	c1, c2, c3 := 58, 60, 59
	t1, t2, t3 := 100, 100, 100
	p1, p2, p3 := 58.0, 60.0, 59.0

	runs := []RunScore{
		{Correct: &c1, Total: &t1, Percent: &p1},
		{Correct: &c2, Total: &t2, Percent: &p2},
		{Correct: &c3, Total: &t3, Percent: &p3},
	}

	stats := calculateStatistics(runs)

	require.NotNil(t, stats.MeanCorrect)
	assert.InDelta(t, 59.0, *stats.MeanCorrect, 0.1)
	assert.Equal(t, 58, *stats.MinCorrect)
	assert.Equal(t, 60, *stats.MaxCorrect)
	require.NotNil(t, stats.Variance)
	// variance of [58,60,59] with mean 59: ((1+1+0)/3) = 0.67
	assert.InDelta(t, 0.67, *stats.Variance, 0.1)
	assert.True(t, stats.AllRunsParsed)
}

func TestCalculateStatisticsWithParseFailures(t *testing.T) {
	c1 := 58
	t1 := 100
	p1 := 58.0

	runs := []RunScore{
		{Correct: &c1, Total: &t1, Percent: &p1},
		{ParseErr: "failed"},
	}

	stats := calculateStatistics(runs)

	require.NotNil(t, stats.MeanCorrect)
	assert.InDelta(t, 58.0, *stats.MeanCorrect, 0.1)
	assert.False(t, stats.AllRunsParsed)
}

func TestCalculateStatisticsAllFailed(t *testing.T) {
	runs := []RunScore{
		{ParseErr: "failed"},
		{ParseErr: "failed again"},
	}

	stats := calculateStatistics(runs)
	assert.Nil(t, stats.MeanCorrect)
	assert.False(t, stats.AllRunsParsed)
}

func TestCalculateStatisticsVariance(t *testing.T) {
	// Variance of [50, 60, 70] with mean 60: ((100+0+100)/3) = 66.67
	c1, c2, c3 := 50, 60, 70
	t1, t2, t3 := 100, 100, 100
	p1, p2, p3 := 50.0, 60.0, 70.0

	runs := []RunScore{
		{Correct: &c1, Total: &t1, Percent: &p1},
		{Correct: &c2, Total: &t2, Percent: &p2},
		{Correct: &c3, Total: &t3, Percent: &p3},
	}

	stats := calculateStatistics(runs)
	require.NotNil(t, stats.Variance)
	assert.InDelta(t, 66.67, *stats.Variance, 0.1)
}

// mockScorerClient returns a consistent scoring response.
type mockScorerClient struct {
	response string
}

func (m *mockScorerClient) ChatCompletion(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: m.response}, nil
}

func (m *mockScorerClient) ChatCompletionStream(_ context.Context, _ llm.ChatRequest) (*llm.StreamReader, error) {
	return nil, assert.AnError
}

func TestScorerScore(t *testing.T) {
	client := &mockScorerClient{
		response: "After evaluation, 72 out of 100 answers are correct.",
	}

	s := NewScorer(client, Config{
		Model:       "scoring-model",
		Repetitions: 3,
	})

	output, err := s.Score(context.Background(), "test content", "test.txt")
	require.NoError(t, err)

	assert.Len(t, output.Runs, 3)
	assert.Equal(t, "scoring-model", output.Metadata.ScoringModel)
	assert.Equal(t, 3, output.Metadata.Repetitions)

	// All runs should parse successfully.
	for _, run := range output.Runs {
		require.NotNil(t, run.Correct)
		assert.Equal(t, 72, *run.Correct)
		assert.Equal(t, 100, *run.Total)
		assert.Empty(t, run.ParseErr)
	}

	// Summary should be consistent.
	require.NotNil(t, output.Summary.MeanCorrect)
	assert.InDelta(t, 72.0, *output.Summary.MeanCorrect, 0.1)
	assert.True(t, output.Summary.AllRunsParsed)

	// Variance should be 0 since all runs returned the same score.
	require.NotNil(t, output.Summary.Variance)
	assert.InDelta(t, 0.0, *output.Summary.Variance, 0.01)
}

func TestScorerDefaultRepetitions(t *testing.T) {
	s := NewScorer(&mockScorerClient{response: "50 out of 100"}, Config{})
	assert.Equal(t, 3, s.config.Repetitions)
}

func TestScorerHandlesParseFailure(t *testing.T) {
	client := &mockScorerClient{
		response: "The candidate performed adequately.", // no parseable score
	}

	s := NewScorer(client, Config{Repetitions: 2})
	output, err := s.Score(context.Background(), "content", "file.txt")
	require.NoError(t, err)

	assert.Len(t, output.Runs, 2)
	for _, run := range output.Runs {
		assert.Nil(t, run.Correct)
		assert.NotEmpty(t, run.ParseErr)
	}

	assert.Nil(t, output.Summary.MeanCorrect)
	assert.False(t, output.Summary.AllRunsParsed)
}
