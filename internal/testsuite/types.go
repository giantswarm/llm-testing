package testsuite

import "time"

// TestSuite represents a loaded test suite with its configuration and questions.
type TestSuite struct {
	Name          string     `yaml:"name"`
	Description   string     `yaml:"description"`
	Version       string     `yaml:"version"`
	Strategy      string     `yaml:"strategy"` // e.g. "qa" (default)
	QuestionsFile string     `yaml:"questions_file"`
	Models        []Model    `yaml:"models"`
	Prompt        Prompt     `yaml:"prompt"`
	Output        Output     `yaml:"output"`
	Questions     []Question `yaml:"-"` // loaded separately from CSV
}

// Model defines a model to test.
type Model struct {
	Name        string  `yaml:"name"`
	Temperature float64 `yaml:"temperature"`
}

// Prompt defines system prompt configuration for a test suite.
type Prompt struct {
	Role          string `yaml:"role"`
	SystemMessage string `yaml:"system_message"`
}

// Output defines output configuration.
type Output struct {
	FilenamePattern string `yaml:"filename_pattern"`
}

// Question represents a single test question.
type Question struct {
	ID             string
	Section        string
	QuestionText   string
	ExpectedAnswer string
}

// Result represents the result of running a single question against a model.
type Result struct {
	Question Question
	Answer   string
	Duration time.Duration
}

// TestRun represents metadata and results for a complete test execution.
type TestRun struct {
	ID        string        `json:"id"`
	Suite     string        `json:"suite"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Models    []ModelRun    `json:"models"`
}

// ModelRun holds results for a single model within a test run.
type ModelRun struct {
	ModelName   string        `json:"model_name"`
	Duration    time.Duration `json:"duration"`
	ResultsFile string        `json:"results_file"`
	Results     []*Result     `json:"-"`
}
