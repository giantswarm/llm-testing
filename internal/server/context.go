package server

import (
	"github.com/giantswarm/llm-testing/internal/kserve"
	"github.com/giantswarm/llm-testing/internal/llm"
)

// ServerContext holds shared dependencies for MCP tool handlers.
type ServerContext struct {
	KServeManager *kserve.Manager
	LLMClient     llm.Client
	LLMAPIKey     string
	Namespace     string
	OutputDir     string
	SuitesDir     string // external test suites directory (optional)
	ScoringModel  string // default model for LLM-as-judge scoring
}
