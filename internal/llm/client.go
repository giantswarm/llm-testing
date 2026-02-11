package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Client abstracts an OpenAI-compatible LLM API.
type Client interface {
	// ChatCompletion sends a chat completion request and returns the response.
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatCompletionStream sends a streaming chat completion request.
	ChatCompletionStream(ctx context.Context, req ChatRequest) (*StreamReader, error)
}

// ChatRequest is a simplified chat request.
type ChatRequest struct {
	Model         string
	SystemMessage string
	UserMessage   string
	Temperature   float64
}

// ChatResponse holds the result of a chat completion.
type ChatResponse struct {
	Content string
}

// StreamReader wraps a streaming response.
type StreamReader struct {
	stream *openai.ChatCompletionStream
}

// Recv reads the next chunk from the stream.
func (s *StreamReader) Recv() (string, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return "", err
	}
	if len(resp.Choices) > 0 {
		return resp.Choices[0].Delta.Content, nil
	}
	return "", nil
}

// Close closes the stream.
func (s *StreamReader) Close() {
	s.stream.Close()
}

// OpenAIClient implements Client using the OpenAI-compatible API.
type OpenAIClient struct {
	client      *openai.Client
	model       string
	temperature *float64
}

// NewOpenAIClient creates a new OpenAI-compatible client.
func NewOpenAIClient(opts ...Option) *OpenAIClient {
	cfg := &clientConfig{
		baseURL: "http://localhost:8000/v1",
		apiKey:  "not-needed",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	config := openai.DefaultConfig(cfg.apiKey)
	config.BaseURL = cfg.baseURL

	return &OpenAIClient{
		client:      openai.NewClientWithConfig(config),
		model:       cfg.model,
		temperature: cfg.temperature,
	}
}

// ChatCompletion sends a non-streaming chat completion request.
func (c *OpenAIClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req = c.applyDefaults(req)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: req.SystemMessage},
		{Role: openai.ChatMessageRoleUser, Content: req.UserMessage},
	}

	temp := float32(req.Temperature)
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: temp,
	})
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	return &ChatResponse{
		Content: resp.Choices[0].Message.Content,
	}, nil
}

// ChatCompletionStream sends a streaming chat completion request.
func (c *OpenAIClient) ChatCompletionStream(ctx context.Context, req ChatRequest) (*StreamReader, error) {
	req = c.applyDefaults(req)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: req.SystemMessage},
		{Role: openai.ChatMessageRoleUser, Content: req.UserMessage},
	}

	temp := float32(req.Temperature)
	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: temp,
	})
	if err != nil {
		return nil, fmt.Errorf("chat completion stream failed: %w", err)
	}

	return &StreamReader{stream: stream}, nil
}

// applyDefaults applies client-level defaults to a request where
// the request does not specify its own values.
func (c *OpenAIClient) applyDefaults(req ChatRequest) ChatRequest {
	if req.Model == "" && c.model != "" {
		req.Model = c.model
	}
	if req.Temperature == 0 && c.temperature != nil {
		req.Temperature = *c.temperature
	}
	return req
}

// CollectStream reads all chunks from a StreamReader and returns the full content.
func CollectStream(sr *StreamReader) (string, error) {
	defer sr.Close()
	var b strings.Builder
	for {
		chunk, err := sr.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return b.String(), err
		}
		b.WriteString(chunk)
	}
	return b.String(), nil
}
