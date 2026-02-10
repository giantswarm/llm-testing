package kserve

import "time"

// ModelConfig defines a model to be served via KServe InferenceService.
type ModelConfig struct {
	// Name is the identifier for the InferenceService resource.
	Name string

	// ModelURI is the model storage URI (e.g. "hf://mistralai/Mistral-7B-Instruct-v0.3").
	ModelURI string

	// Runtime is the KServe serving runtime (default: "kserve-vllm").
	Runtime string

	// GPUCount is the number of GPUs to request.
	GPUCount int

	// RuntimeArgs are additional arguments passed to the vLLM runtime.
	RuntimeArgs []string

	// ReadyTimeout is how long to wait for the InferenceService to become ready.
	ReadyTimeout time.Duration
}

// ModelStatus represents the observed state of a deployed model.
type ModelStatus struct {
	Name        string `json:"name"`
	Ready       bool   `json:"ready"`
	EndpointURL string `json:"endpoint_url,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Message     string `json:"message,omitempty"`
}

// DefaultModelConfig returns sensible defaults for a model config.
func DefaultModelConfig(name, modelURI string) ModelConfig {
	return ModelConfig{
		Name:         name,
		ModelURI:     modelURI,
		Runtime:      "kserve-vllm",
		GPUCount:     1,
		ReadyTimeout: 10 * time.Minute,
	}
}
