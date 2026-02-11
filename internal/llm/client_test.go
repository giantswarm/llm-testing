package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOpenAIClientDefaults(t *testing.T) {
	client := NewOpenAIClient()
	assert.Empty(t, client.model)
	assert.Nil(t, client.temperature)
}

func TestNewOpenAIClientWithModel(t *testing.T) {
	client := NewOpenAIClient(WithModel("gpt-4"))
	assert.Equal(t, "gpt-4", client.model)
}

func TestNewOpenAIClientWithTemperature(t *testing.T) {
	client := NewOpenAIClient(WithTemperature(0.7))
	assert.NotNil(t, client.temperature)
	assert.Equal(t, 0.7, *client.temperature)
}

func TestNewOpenAIClientWithAllOptions(t *testing.T) {
	client := NewOpenAIClient(
		WithBaseURL("https://api.example.com/v1"),
		WithAPIKey("sk-test"),
		WithModel("gpt-4"),
		WithTemperature(0.5),
	)
	assert.Equal(t, "gpt-4", client.model)
	assert.NotNil(t, client.temperature)
	assert.Equal(t, 0.5, *client.temperature)
}

func TestApplyDefaultsUsesClientModel(t *testing.T) {
	client := NewOpenAIClient(WithModel("gpt-4"))

	req := client.applyDefaults(ChatRequest{
		SystemMessage: "test",
		UserMessage:   "hello",
	})
	assert.Equal(t, "gpt-4", req.Model)
}

func TestApplyDefaultsRequestModelTakesPrecedence(t *testing.T) {
	client := NewOpenAIClient(WithModel("gpt-4"))

	req := client.applyDefaults(ChatRequest{
		Model:         "gpt-3.5",
		SystemMessage: "test",
		UserMessage:   "hello",
	})
	assert.Equal(t, "gpt-3.5", req.Model)
}

func TestApplyDefaultsUsesClientTemperature(t *testing.T) {
	client := NewOpenAIClient(WithTemperature(0.8))

	req := client.applyDefaults(ChatRequest{
		Model:       "test",
		UserMessage: "hello",
	})
	assert.Equal(t, 0.8, req.Temperature)
}

func TestApplyDefaultsRequestTemperatureTakesPrecedence(t *testing.T) {
	client := NewOpenAIClient(WithTemperature(0.8))

	req := client.applyDefaults(ChatRequest{
		Model:       "test",
		UserMessage: "hello",
		Temperature: 0.5,
	})
	assert.Equal(t, 0.5, req.Temperature)
}
