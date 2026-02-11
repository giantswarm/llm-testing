package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOpenAIClientDefaults(t *testing.T) {
	client := NewOpenAIClient()
	assert.NotNil(t, client.client)
}

func TestNewOpenAIClientWithAllOptions(t *testing.T) {
	client := NewOpenAIClient(
		WithBaseURL("https://api.example.com/v1"),
		WithAPIKey("sk-test"),
	)
	assert.NotNil(t, client.client)
}
