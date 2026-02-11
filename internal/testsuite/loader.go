package testsuite

import (
	"embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed all:testdata
var embeddedSuites embed.FS

// Load loads a test suite by name, searching first in the external directory
// (if provided), then in the embedded test suites.
func Load(name string, externalDir string) (*TestSuite, error) {
	// Try external directory first.
	if externalDir != "" {
		path := filepath.Join(externalDir, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return loadFromFS(os.DirFS(path), name)
		}
	}

	// Fall back to embedded test suites.
	// Use path.Join (not filepath.Join) because embed.FS always uses forward slashes.
	subFS, err := fs.Sub(embeddedSuites, path.Join("testdata", name))
	if err != nil {
		return nil, fmt.Errorf("test suite %q not found: %w", name, err)
	}
	return loadFromFS(subFS, name)
}

// List returns the names of all available test suites.
func List(externalDir string) ([]string, error) {
	seen := make(map[string]bool)
	var names []string

	// List embedded suites.
	entries, err := fs.ReadDir(embeddedSuites, "testdata")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = true
				names = append(names, e.Name())
			}
		}
	}

	// List external suites.
	if externalDir != "" {
		entries, err := os.ReadDir(externalDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() && !seen[e.Name()] {
					names = append(names, e.Name())
				}
			}
		}
	}

	return names, nil
}

func loadFromFS(fsys fs.FS, name string) (*TestSuite, error) {
	// Load config.yaml.
	configData, err := fs.ReadFile(fsys, "config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read config.yaml for suite %q: %w", name, err)
	}

	var suite TestSuite
	if err := yaml.Unmarshal(configData, &suite); err != nil {
		return nil, fmt.Errorf("failed to parse config.yaml for suite %q: %w", name, err)
	}

	if suite.Strategy == "" {
		suite.Strategy = "qa"
	}
	if suite.QuestionsFile == "" {
		suite.QuestionsFile = "questions.csv"
	}

	// Load questions CSV.
	questions, err := loadQuestionsFromFS(fsys, suite.QuestionsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load questions for suite %q: %w", name, err)
	}
	suite.Questions = questions

	return &suite, nil
}

func loadQuestionsFromFS(fsys fs.FS, filename string) ([]Question, error) {
	f, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable field counts.

	// Read header.
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[strings.TrimSpace(col)] = i
	}

	// Validate required columns.
	for _, required := range []string{"ID", "Section", "Question", "ExpectedAnswer"} {
		if _, ok := colIndex[required]; !ok {
			return nil, fmt.Errorf("missing required CSV column: %s", required)
		}
	}

	// Determine the minimum number of columns required by checking the max column index.
	minCols := 0
	for _, idx := range colIndex {
		if idx >= minCols {
			minCols = idx + 1
		}
	}

	var questions []Question
	for lineNum := 2; ; lineNum++ { // lineNum starts at 2 (1-indexed, after header).
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row %d: %w", lineNum, err)
		}
		if len(record) < minCols {
			return nil, fmt.Errorf("CSV row %d has %d columns, expected at least %d", lineNum, len(record), minCols)
		}

		questions = append(questions, Question{
			ID:             record[colIndex["ID"]],
			Section:        record[colIndex["Section"]],
			QuestionText:   record[colIndex["Question"]],
			ExpectedAnswer: record[colIndex["ExpectedAnswer"]],
		})
	}

	return questions, nil
}
