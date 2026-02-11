package cmd

import (
	"os"

	"github.com/giantswarm/llm-testing/internal/llm"
)

// newLLMClientFromFlags creates an LLM client from common CLI flags.
// It checks the endpoint and apiKey flags, falling back to the OPENAI_API_KEY
// environment variable when no explicit key is provided.
func newLLMClientFromFlags(endpoint, apiKey string) llm.Client {
	var opts []llm.Option
	if endpoint != "" {
		opts = append(opts, llm.WithBaseURL(endpoint))
	}
	if apiKey != "" {
		opts = append(opts, llm.WithAPIKey(apiKey))
	} else if envKey := os.Getenv("OPENAI_API_KEY"); envKey != "" {
		opts = append(opts, llm.WithAPIKey(envKey))
	}
	return llm.NewOpenAIClient(opts...)
}
