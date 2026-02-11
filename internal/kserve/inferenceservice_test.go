package kserve

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInferenceService(t *testing.T) {
	cfg := ModelConfig{
		Name:     "mistral-7b",
		ModelURI: "hf://mistralai/Mistral-7B-Instruct-v0.3",
		Runtime:  "kserve-vllm",
		GPUCount: 1,
	}

	isvc := BuildInferenceService(cfg, "llm-testing")

	assert.Equal(t, apiVersion, isvc.APIVersion)
	assert.Equal(t, kind, isvc.Kind)
	assert.Equal(t, "mistral-7b", isvc.Name)
	assert.Equal(t, "llm-testing", isvc.Namespace)

	labels := isvc.Labels
	assert.Equal(t, managedBy, labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "mistral-7b", labels["app.kubernetes.io/name"])

	// Verify predictor spec.
	require.NotNil(t, isvc.Spec.Predictor.Model)
	assert.Equal(t, "vLLM", isvc.Spec.Predictor.Model.ModelFormat.Name)

	require.NotNil(t, isvc.Spec.Predictor.Model.StorageURI)
	assert.Equal(t, "hf://mistralai/Mistral-7B-Instruct-v0.3", *isvc.Spec.Predictor.Model.StorageURI)

	require.NotNil(t, isvc.Spec.Predictor.Model.Runtime)
	assert.Equal(t, "kserve-vllm", *isvc.Spec.Predictor.Model.Runtime)

	// Verify GPU resources.
	gpuReq := isvc.Spec.Predictor.Model.Resources.Requests["nvidia.com/gpu"]
	assert.Equal(t, "1", gpuReq.String())

	gpuLim := isvc.Spec.Predictor.Model.Resources.Limits["nvidia.com/gpu"]
	assert.Equal(t, "1", gpuLim.String())
}

func TestBuildInferenceServiceWithArgs(t *testing.T) {
	cfg := ModelConfig{
		Name:        "llama-70b",
		ModelURI:    "hf://meta-llama/Llama-3-70B-Instruct",
		Runtime:     "kserve-vllm",
		GPUCount:    4,
		RuntimeArgs: []string{"--max-model-len=4096", "--tensor-parallel-size=4"},
	}

	isvc := BuildInferenceService(cfg, "default")

	require.NotNil(t, isvc.Spec.Predictor.Model)
	assert.Len(t, isvc.Spec.Predictor.Model.Args, 2)
	assert.Equal(t, "--max-model-len=4096", isvc.Spec.Predictor.Model.Args[0])
	assert.Equal(t, "--tensor-parallel-size=4", isvc.Spec.Predictor.Model.Args[1])

	// Verify GPU count.
	gpuReq := isvc.Spec.Predictor.Model.Resources.Requests["nvidia.com/gpu"]
	assert.Equal(t, "4", gpuReq.String())
}

func TestBuildInferenceServiceNoRuntime(t *testing.T) {
	cfg := ModelConfig{
		Name:     "test-model",
		ModelURI: "hf://org/model",
		GPUCount: 1,
	}

	isvc := BuildInferenceService(cfg, "default")

	require.NotNil(t, isvc.Spec.Predictor.Model)
	assert.Nil(t, isvc.Spec.Predictor.Model.Runtime)
}

func TestBuildInferenceServiceNoGPU(t *testing.T) {
	cfg := ModelConfig{
		Name:     "cpu-model",
		ModelURI: "hf://org/model",
	}

	isvc := BuildInferenceService(cfg, "default")

	require.NotNil(t, isvc.Spec.Predictor.Model)
	assert.Empty(t, isvc.Spec.Predictor.Model.Resources.Requests)
	assert.Empty(t, isvc.Spec.Predictor.Model.Resources.Limits)
}

func TestToFromUnstructured(t *testing.T) {
	cfg := ModelConfig{
		Name:        "roundtrip-test",
		ModelURI:    "hf://org/model",
		Runtime:     "kserve-vllm",
		GPUCount:    2,
		RuntimeArgs: []string{"--arg1", "--arg2"},
	}

	original := BuildInferenceService(cfg, "test-ns")

	// Convert to unstructured and back.
	obj, err := toUnstructured(original)
	require.NoError(t, err)
	require.NotNil(t, obj)

	restored, err := fromUnstructured(obj)
	require.NoError(t, err)
	require.NotNil(t, restored)

	// Verify the roundtrip preserved key fields.
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.Namespace, restored.Namespace)
	assert.Equal(t, original.Labels, restored.Labels)
	assert.Equal(t, original.Spec.Predictor.Model.ModelFormat.Name, restored.Spec.Predictor.Model.ModelFormat.Name)
	assert.Equal(t, *original.Spec.Predictor.Model.StorageURI, *restored.Spec.Predictor.Model.StorageURI)
	assert.Equal(t, *original.Spec.Predictor.Model.Runtime, *restored.Spec.Predictor.Model.Runtime)
	assert.Equal(t, original.Spec.Predictor.Model.Args, restored.Spec.Predictor.Model.Args)
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mistral-7b", "mistral-7b"},
		{"Mistral-7B", "mistral-7b"},
		{"model/name@version", "model-name-version"},
		{"_starts_with_underscore", "m--starts-with-underscore"},
		{"simple", "simple"},
		{"trailing-dash-after-truncation-" + strings.Repeat("abcdefghij", 6), strings.TrimRight(("trailing-dash-after-truncation-" + strings.Repeat("abcdefghij", 6))[:63], "-")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeName(tt.input))
		})
	}
}

func TestEndpointURL(t *testing.T) {
	url := EndpointURL("mistral-7b", "llm-testing")
	assert.Equal(t, "http://mistral-7b.llm-testing.svc.cluster.local/v1", url)
}

func TestDefaultModelConfig(t *testing.T) {
	cfg := DefaultModelConfig("test-model", "hf://org/model")
	assert.Equal(t, "test-model", cfg.Name)
	assert.Equal(t, "hf://org/model", cfg.ModelURI)
	assert.Equal(t, "kserve-vllm", cfg.Runtime)
	assert.Equal(t, 1, cfg.GPUCount)
}
