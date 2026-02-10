package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

// ProgressFunc is called to report progress during test execution.
type ProgressFunc func(model string, questionIndex, totalQuestions int)

// Runner orchestrates the execution of test suites.
type Runner struct {
	client    llm.Client
	strategy  EvaluationStrategy
	outputDir string
	progress  ProgressFunc
}

// NewRunner creates a new test runner.
func NewRunner(client llm.Client, strategy EvaluationStrategy, outputDir string) *Runner {
	return &Runner{
		client:   client,
		strategy: strategy,
		outputDir: outputDir,
	}
}

// SetProgressFunc sets the progress callback.
func (r *Runner) SetProgressFunc(fn ProgressFunc) {
	r.progress = fn
}

// Run executes a test suite for all configured models and writes results.
func (r *Runner) Run(ctx context.Context, suite *testsuite.TestSuite) (*testsuite.TestRun, error) {
	questions, err := r.strategy.LoadQuestions(suite)
	if err != nil {
		return nil, fmt.Errorf("failed to load questions: %w", err)
	}

	timestamp := time.Now()
	runID := fmt.Sprintf("%s_%s", suite.Name, timestamp.Format("20060102-150405"))

	// Create output directory.
	outputPath := filepath.Join(r.outputDir, runID)
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	run := &testsuite.TestRun{
		ID:        runID,
		Suite:     suite.Name,
		Timestamp: timestamp,
		Models:    make([]testsuite.ModelRun, 0, len(suite.Models)),
	}

	systemPrompt := suite.Prompt.SystemMessage

	for _, model := range suite.Models {
		slog.Info("running test suite",
			"model", model.Name,
			"questions", len(questions),
			"temperature", model.Temperature,
		)

		modelStart := time.Now()
		var results []*testsuite.Result

		for i, q := range questions {
			if r.progress != nil {
				r.progress(model.Name, i+1, len(questions))
			}

			result, err := r.strategy.Execute(ctx, r.client, model.Name, q, systemPrompt, model.Temperature)
			if err != nil {
				slog.Error("question execution failed",
					"question_id", q.ID,
					"error", err,
				)
				// Continue with next question on error.
				continue
			}
			results = append(results, result)
		}

		// Write results file.
		output := r.strategy.FormatResults(results)
		resultsFile := filepath.Join(outputPath, fmt.Sprintf("%s.txt", model.Name))
		if err := os.WriteFile(resultsFile, []byte(output), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write results for model %s: %w", model.Name, err)
		}

		modelRun := testsuite.ModelRun{
			ModelName:   model.Name,
			Duration:    time.Since(modelStart),
			ResultsFile: resultsFile,
			Results:     results,
		}
		run.Models = append(run.Models, modelRun)

		slog.Info("model evaluation complete",
			"model", model.Name,
			"questions_answered", len(results),
			"duration", modelRun.Duration,
		)
	}

	run.Duration = time.Since(timestamp)

	// Write metadata.
	if err := writeRunMetadata(outputPath, run); err != nil {
		return nil, fmt.Errorf("failed to write run metadata: %w", err)
	}

	return run, nil
}

func writeRunMetadata(outputPath string, run *testsuite.TestRun) error {
	models := make([]map[string]interface{}, 0, len(run.Models))
	for _, m := range run.Models {
		models = append(models, map[string]interface{}{
			"model_name":   m.ModelName,
			"duration":     m.Duration.Seconds(),
			"results_file": m.ResultsFile,
		})
	}

	metadata := map[string]interface{}{
		"id":            run.ID,
		"suite":         run.Suite,
		"timestamp":     run.Timestamp,
		"full_duration": run.Duration.Seconds(),
		"models":        models,
	}

	data, err := json.MarshalIndent(metadata, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(outputPath, "resultset.json"), data, 0o644)
}
