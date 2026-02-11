package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giantswarm/llm-testing/internal/llm"
	"github.com/giantswarm/llm-testing/internal/testsuite"
)

// ProgressFunc is called to report progress during test execution.
type ProgressFunc func(model string, questionIndex, totalQuestions int)

// ClientForModelFunc returns an LLM client configured for the given model.
// This is called before each model's evaluation. The returned client is used
// for all questions in that model's run. This enables the deploy -> test -> teardown
// lifecycle: the function can deploy the model via KServe and return a client
// pointing to its endpoint.
type ClientForModelFunc func(ctx context.Context, model testsuite.Model) (llm.Client, error)

// AfterModelFunc is called after a model's evaluation completes (or fails).
// Use this to tear down resources like KServe InferenceServices.
type AfterModelFunc func(ctx context.Context, model testsuite.Model) error

// Runner orchestrates the execution of test suites.
type Runner struct {
	client         llm.Client         // default client (used when clientForModel is nil)
	clientForModel ClientForModelFunc // optional: per-model client factory (deploy + endpoint discovery)
	afterModel     AfterModelFunc     // optional: called after each model (teardown)
	strategy       EvaluationStrategy
	outputDir      string
	progress       ProgressFunc
}

// NewRunner creates a new test runner with a default LLM client.
func NewRunner(client llm.Client, strategy EvaluationStrategy, outputDir string) *Runner {
	return &Runner{
		client:    client,
		strategy:  strategy,
		outputDir: outputDir,
	}
}

// SetProgressFunc sets the progress callback.
func (r *Runner) SetProgressFunc(fn ProgressFunc) {
	r.progress = fn
}

// SetClientForModelFunc sets the per-model client factory.
// When set, this is called before each model's evaluation to obtain
// a client configured for that model's endpoint.
func (r *Runner) SetClientForModelFunc(fn ClientForModelFunc) {
	r.clientForModel = fn
}

// SetAfterModelFunc sets the post-model callback.
// This is called after each model's evaluation completes,
// typically used to teardown KServe InferenceServices.
func (r *Runner) SetAfterModelFunc(fn AfterModelFunc) {
	r.afterModel = fn
}

// Run executes a test suite for the given models and writes results.
// Models are processed sequentially -- important for GPU memory constraints
// when models are deployed/torn down via KServe between evaluations.
func (r *Runner) Run(ctx context.Context, suite *testsuite.TestSuite, models []testsuite.Model) (*testsuite.TestRun, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("no models specified for test run")
	}

	questions, err := r.strategy.LoadQuestions(suite)
	if err != nil {
		return nil, fmt.Errorf("failed to load questions: %w", err)
	}

	timestamp := time.Now()
	sanitizedName := strings.ReplaceAll(suite.Name, " ", "_")
	runID := fmt.Sprintf("%s_%s", sanitizedName, timestamp.Format("20060102-150405"))

	// Create output directory.
	outputPath := filepath.Join(r.outputDir, runID)
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	run := &testsuite.TestRun{
		ID:        runID,
		Suite:     suite.Name,
		Timestamp: timestamp,
		Models:    make([]testsuite.ModelRun, 0, len(models)),
	}

	systemPrompt := suite.Prompt.SystemMessage

	for _, model := range models {
		// Check for context cancellation between models.
		if err := ctx.Err(); err != nil {
			slog.Warn("test run cancelled before model evaluation", "model", model.Name)
			break
		}

		// Determine the LLM client for this model.
		client := r.client
		if r.clientForModel != nil {
			var err error
			client, err = r.clientForModel(ctx, model)
			if err != nil {
				slog.Error("failed to get client for model", "model", model.Name, "error", err)
				// If we have an afterModel hook, call it to clean up.
				if r.afterModel != nil {
					_ = r.afterModel(ctx, model)
				}
				return nil, fmt.Errorf("failed to prepare model %s: %w", model.Name, err)
			}
		}

		slog.Info("running test suite",
			"model", model.Name,
			"questions", len(questions),
			"temperature", model.Temperature,
		)

		modelStart := time.Now()
		var results []*testsuite.Result

		for i, q := range questions {
			// Check for context cancellation between questions.
			if err := ctx.Err(); err != nil {
				slog.Warn("test run cancelled", "model", model.Name, "completed", i, "total", len(questions))
				break
			}

			if r.progress != nil {
				r.progress(model.Name, i+1, len(questions))
			}

			result, err := r.strategy.Execute(ctx, client, model.Name, q, systemPrompt, model.Temperature)
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
		safeModelName := sanitizeFilename(model.Name)
		resultsFile := filepath.Join(outputPath, fmt.Sprintf("%s.txt", safeModelName))
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

		// Call afterModel hook (e.g. teardown KServe InferenceService).
		if r.afterModel != nil {
			if err := r.afterModel(ctx, model); err != nil {
				slog.Error("after-model hook failed", "model", model.Name, "error", err)
				// Continue with next model; don't fail the entire run.
			}
		}
	}

	run.Duration = time.Since(timestamp)

	// Write metadata.
	if err := writeRunMetadata(outputPath, run); err != nil {
		return nil, fmt.Errorf("failed to write run metadata: %w", err)
	}

	return run, nil
}

// sanitizeFilename replaces characters unsafe for filenames with underscores.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
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
