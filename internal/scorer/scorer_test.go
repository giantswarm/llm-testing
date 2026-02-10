package scorer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
