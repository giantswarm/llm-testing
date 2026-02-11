package scorer

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/giantswarm/llm-testing/internal/testutil"
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

func TestScorerScore(t *testing.T) {
	client := &testutil.MockLLMClient{
		DefaultResponse: "After evaluation, 72 out of 100 answers are correct.",
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
	s := NewScorer(&testutil.MockLLMClient{DefaultResponse: "50 out of 100"}, Config{})
	assert.Equal(t, 3, s.config.Repetitions)
}

func TestScoreFile(t *testing.T) {
	tmpDir := t.TempDir()
	resultsFile := tmpDir + "/results.txt"
	content := `---
NO. 1 - Setup
QUESTION: What is kubectl?
EXPECTED ANSWER: CLI tool
ACTUAL ANSWER: kubectl is the Kubernetes CLI tool
`
	require.NoError(t, os.WriteFile(resultsFile, []byte(content), 0o644))

	client := &testutil.MockLLMClient{
		DefaultResponse: "85 out of 100 answers are correct.",
	}
	s := NewScorer(client, Config{Model: "scorer", Repetitions: 2})

	output, err := s.ScoreFile(context.Background(), resultsFile)
	require.NoError(t, err)

	assert.Equal(t, resultsFile, output.Metadata.ResultsFile)
	assert.Len(t, output.Runs, 2)
	for _, run := range output.Runs {
		require.NotNil(t, run.Correct)
		assert.Equal(t, 85, *run.Correct)
	}
}

func TestScoreFileNotFound(t *testing.T) {
	client := &testutil.MockLLMClient{}
	s := NewScorer(client, Config{Model: "m", Repetitions: 1})

	_, err := s.ScoreFile(context.Background(), "/nonexistent/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read results file")
}

func TestWriteScoreFile(t *testing.T) {
	tmpDir := t.TempDir()
	resultsFile := tmpDir + "/model.txt"
	// Create an empty results file so the path exists.
	require.NoError(t, os.WriteFile(resultsFile, []byte("test"), 0o644))

	c1, t1 := 80, 100
	p1 := 80.0
	meanC, meanP := 80.0, 80.0
	minC, maxC := 80, 80
	variance := 0.0

	output := &ScoreOutput{
		Metadata: ScoreMetadata{
			Timestamp:    "2024-01-01T00:00:00Z",
			ResultsFile:  resultsFile,
			ScoringModel: "scorer",
			Repetitions:  1,
		},
		Runs: []RunScore{
			{Correct: &c1, Total: &t1, Percent: &p1, RawOutput: "80 out of 100"},
		},
		Summary: Summary{
			MeanCorrect:   &meanC,
			MeanPercent:   &meanP,
			MinCorrect:    &minC,
			MaxCorrect:    &maxC,
			Variance:      &variance,
			AllRunsParsed: true,
		},
	}

	scoresFile, err := WriteScoreFile(output, resultsFile)
	require.NoError(t, err)

	// Verify file was created.
	assert.FileExists(t, scoresFile)
	expectedPath := tmpDir + "/model_scores.json"
	assert.Equal(t, expectedPath, scoresFile)

	// Verify JSON content is valid.
	data, err := os.ReadFile(scoresFile)
	require.NoError(t, err)

	var parsed ScoreOutput
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "scorer", parsed.Metadata.ScoringModel)
	assert.Len(t, parsed.Runs, 1)
	require.NotNil(t, parsed.Summary.MeanCorrect)
	assert.InDelta(t, 80.0, *parsed.Summary.MeanCorrect, 0.01)
}

func TestScorerHandlesParseFailure(t *testing.T) {
	client := &testutil.MockLLMClient{
		DefaultResponse: "The candidate performed adequately.", // no parseable score
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
