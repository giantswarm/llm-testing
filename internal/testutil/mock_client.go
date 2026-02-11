// Package testutil provides shared test helpers.
package testutil

import (
	"context"
	"fmt"

	"github.com/giantswarm/llm-testing/internal/llm"
)

// MockLLMClient is a configurable mock for llm.Client used across test packages.
type MockLLMClient struct {
	// Responses maps user messages to canned responses.
	Responses map[string]string

	// DefaultResponse is returned when no matching key is found in Responses.
	DefaultResponse string

	// Calls tracks the number of ChatCompletion invocations.
	Calls int

	// LastRequest stores the most recent ChatRequest for inspection.
	LastRequest llm.ChatRequest
}

func (m *MockLLMClient) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.Calls++
	m.LastRequest = req

	if resp, ok := m.Responses[req.UserMessage]; ok {
		return &llm.ChatResponse{Content: resp}, nil
	}

	if m.DefaultResponse != "" {
		return &llm.ChatResponse{Content: m.DefaultResponse}, nil
	}

	return &llm.ChatResponse{Content: "mock response"}, nil
}

func (m *MockLLMClient) ChatCompletionStream(_ context.Context, _ llm.ChatRequest) (*llm.StreamReader, error) {
	return nil, fmt.Errorf("streaming not supported in mock")
}
