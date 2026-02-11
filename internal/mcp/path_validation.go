package mcp

import (
	"fmt"
	"path/filepath"
	"strings"
)

func resolveRunPath(outputDir, runID string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", fmt.Errorf("run_id is required")
	}
	if strings.Contains(runID, string(filepath.Separator)) || strings.Contains(runID, "/") {
		return "", fmt.Errorf("path separators are not allowed")
	}
	if runID == "." || runID == ".." {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	return resolvePathWithinBase(outputDir, runID)
}

func resolveResultFilePath(outputDir, resultsFile string) (string, error) {
	if strings.TrimSpace(resultsFile) == "" {
		return "", fmt.Errorf("results_file is required")
	}
	return resolvePathWithinBase(outputDir, resultsFile)
}

func resolvePathWithinBase(baseDir, pathValue string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}
	target := pathValue
	if !filepath.IsAbs(target) {
		target = filepath.Join(baseAbs, target)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be within output directory")
	}
	return targetAbs, nil
}

func joinRunFile(runPath, name string) string {
	return filepath.Join(runPath, name)
}
