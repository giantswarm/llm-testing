package kserve

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildInferenceService(t *testing.T) {
	cfg := ModelConfig{
		Name:     "mistral-7b",
		ModelURI: "hf://mistralai/Mistral-7B-Instruct-v0.3",
		Runtime:  "kserve-vllm",
		GPUCount: 1,
	}

	isvc := BuildInferenceService(cfg, "llm-testing")

	assert.Equal(t, apiVersion, isvc.GetAPIVersion())
	assert.Equal(t, kind, isvc.GetKind())
	assert.Equal(t, "mistral-7b", isvc.GetName())
	assert.Equal(t, "llm-testing", isvc.GetNamespace())

	labels := isvc.GetLabels()
	assert.Equal(t, managedBy, labels["app.kubernetes.io/managed-by"])
	assert.Equal(t, "mistral-7b", labels["app.kubernetes.io/name"])

	// Verify predictor spec.
	predictor, found, err := unstructured.NestedMap(isvc.Object, "spec", "predictor")
	require.NoError(t, err)
	require.True(t, found)

	model, found, err := unstructured.NestedMap(predictor, "model")
	require.NoError(t, err)
	require.True(t, found)

	storageURI, found, err := unstructured.NestedString(model, "storageUri")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "hf://mistralai/Mistral-7B-Instruct-v0.3", storageURI)

	runtime, found, err := unstructured.NestedString(model, "runtime")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "kserve-vllm", runtime)

	// Verify GPU resources.
	gpuReq, found, err := unstructured.NestedString(model, "resources", "requests", "nvidia.com/gpu")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "1", gpuReq)
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

	model, _, _ := unstructured.NestedMap(isvc.Object, "spec", "predictor", "model")
	args, found, _ := unstructured.NestedSlice(model, "args")
	require.True(t, found)
	assert.Len(t, args, 2)
	assert.Equal(t, "--max-model-len=4096", args[0])
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
