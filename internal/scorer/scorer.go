package scorer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/giantswarm/llm-testing/internal/llm"
)

// DefaultScoringModel is the default model used for LLM-as-judge scoring.
const DefaultScoringModel = "claude-sonnet-4-5-20250514"

// Config holds scoring configuration.
type Config struct {
	Model       string
	Repetitions int
}

// RunScore represents the parsed result of a single scoring run.
type RunScore struct {
	Correct   *int     `json:"correct"`
	Total     *int     `json:"total"`
	Percent   *float64 `json:"percentage"`
	RawOutput string   `json:"raw_output"`
	ParseErr  string   `json:"parse_error,omitempty"`
}

// ScoreOutput is the full structured scoring output.
type ScoreOutput struct {
	Metadata ScoreMetadata `json:"metadata"`
	Runs     []RunScore    `json:"runs"`
	Summary  Summary       `json:"summary"`
}

// ScoreMetadata holds information about the scoring run.
type ScoreMetadata struct {
	Timestamp    string `json:"timestamp"`
	ResultsFile  string `json:"results_file"`
	ScoringModel string `json:"scoring_model"`
	Repetitions  int    `json:"repetitions"`
}

// Summary holds aggregate statistics from multiple scoring runs.
type Summary struct {
	MeanCorrect   *float64 `json:"mean_correct"`
	MeanPercent   *float64 `json:"mean_percentage"`
	MinCorrect    *int     `json:"min_correct"`
	MaxCorrect    *int     `json:"max_correct"`
	Variance      *float64 `json:"variance"`
	AllRunsParsed bool     `json:"all_runs_parsed"`
}

// Scorer evaluates test results using an LLM as judge.
type Scorer struct {
	client llm.Client
	config Config
}

// NewScorer creates a new Scorer.
func NewScorer(client llm.Client, config Config) *Scorer {
	if config.Repetitions <= 0 {
		config.Repetitions = 3
	}
	if config.Model == "" {
		config.Model = DefaultScoringModel
	}
	return &Scorer{client: client, config: config}
}

// ScoreFile reads a results file and scores it.
func (s *Scorer) ScoreFile(ctx context.Context, resultsFile string) (*ScoreOutput, error) {
	content, err := os.ReadFile(resultsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read results file: %w", err)
	}

	return s.Score(ctx, string(content), resultsFile)
}

// Score evaluates the given results content.
func (s *Scorer) Score(ctx context.Context, content string, resultsFile string) (*ScoreOutput, error) {
	output := &ScoreOutput{
		Metadata: ScoreMetadata{
			Timestamp:    time.Now().Format(time.RFC3339),
			ResultsFile:  resultsFile,
			ScoringModel: s.config.Model,
			Repetitions:  s.config.Repetitions,
		},
		Runs: make([]RunScore, 0, s.config.Repetitions),
	}

	for i := 0; i < s.config.Repetitions; i++ {
		slog.Info("scoring run",
			"run", i+1,
			"total", s.config.Repetitions,
		)

		resultText, err := s.evaluate(ctx, content)
		if err != nil {
			slog.Error("scoring run failed", "run", i+1, "error", err)
			output.Runs = append(output.Runs, RunScore{
				RawOutput: "",
				ParseErr:  err.Error(),
			})
			continue
		}

		parsed := parseScore(resultText)
		output.Runs = append(output.Runs, parsed)

		if parsed.Correct != nil {
			slog.Info("score parsed",
				"run", i+1,
				"correct", *parsed.Correct,
				"total", *parsed.Total,
				"percentage", *parsed.Percent,
			)
		}
	}

	output.Summary = calculateStatistics(output.Runs)

	return output, nil
}

// WriteScoreFile writes the score output as JSON next to the results file.
func WriteScoreFile(output *ScoreOutput, resultsFile string) (string, error) {
	scoresFile := strings.TrimSuffix(resultsFile, ".txt") + "_scores.json"

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal scores: %w", err)
	}

	if err := os.WriteFile(scoresFile, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write scores file: %w", err)
	}

	return scoresFile, nil
}

func (s *Scorer) evaluate(ctx context.Context, content string) (string, error) {
	// Try streaming first.
	stream, err := s.client.ChatCompletionStream(ctx, llm.ChatRequest{
		Model:         s.config.Model,
		SystemMessage: EvaluationPrompt,
		UserMessage:   content,
		Temperature:   llm.Float64Ptr(0),
	})
	if err == nil {
		result, streamErr := llm.CollectStream(stream)
		if streamErr == nil {
			return result, nil
		}
		slog.Warn("streaming evaluation failed, falling back to non-streaming", "error", streamErr)
	} else {
		slog.Debug("streaming not available, using non-streaming", "error", err)
	}

	// Fallback to non-streaming.
	resp, err := s.client.ChatCompletion(ctx, llm.ChatRequest{
		Model:         s.config.Model,
		SystemMessage: EvaluationPrompt,
		UserMessage:   content,
		Temperature:   llm.Float64Ptr(0),
	})
	if err != nil {
		return "", fmt.Errorf("evaluation failed: %w", err)
	}

	return resp.Content, nil
}

var scorePattern = regexp.MustCompile(`(\d+)\s+out\s+of\s+(\d+)`)

func parseScore(text string) RunScore {
	matches := scorePattern.FindStringSubmatch(text)
	if matches == nil {
		return RunScore{
			RawOutput: text,
			ParseErr:  "Could not parse score from output",
		}
	}

	correct, _ := strconv.Atoi(matches[1])
	total, _ := strconv.Atoi(matches[2])
	pct := 0.0
	if total > 0 {
		pct = math.Round(float64(correct)/float64(total)*10000) / 100
	}

	return RunScore{
		Correct:   &correct,
		Total:     &total,
		Percent:   &pct,
		RawOutput: text,
	}
}

func calculateStatistics(runs []RunScore) Summary {
	var correctValues []int
	var percentValues []float64

	for _, r := range runs {
		if r.Correct != nil {
			correctValues = append(correctValues, *r.Correct)
			percentValues = append(percentValues, *r.Percent)
		}
	}

	if len(correctValues) == 0 {
		return Summary{AllRunsParsed: false}
	}

	meanCorrect := meanInt(correctValues)
	meanPercent := meanFloat(percentValues)
	minC := slices.Min(correctValues)
	maxC := slices.Max(correctValues)
	variance := varianceFloat(correctValues, meanCorrect)

	return Summary{
		MeanCorrect:   &meanCorrect,
		MeanPercent:   &meanPercent,
		MinCorrect:    &minC,
		MaxCorrect:    &maxC,
		Variance:      &variance,
		AllRunsParsed: len(correctValues) == len(runs),
	}
}

func meanInt(vals []int) float64 {
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return math.Round(float64(sum)/float64(len(vals))*100) / 100
}

func meanFloat(vals []float64) float64 {
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return math.Round(sum/float64(len(vals))*100) / 100
}

// varianceFloat calculates the population variance of integer values given a precomputed mean.
func varianceFloat(vals []int, mean float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sumSquaredDiff := 0.0
	for _, v := range vals {
		diff := float64(v) - mean
		sumSquaredDiff += diff * diff
	}
	return math.Round(sumSquaredDiff/float64(len(vals))*100) / 100
}
