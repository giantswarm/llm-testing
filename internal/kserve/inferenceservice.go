package kserve

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	apiVersion = "serving.kserve.io/v1beta1"
	kind       = "InferenceService"
	managedBy  = "llm-testing"
)

// BuildInferenceService creates an unstructured InferenceService object from a ModelConfig.
func BuildInferenceService(cfg ModelConfig, namespace string) *unstructured.Unstructured {
	isvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      sanitizeName(cfg.Name),
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": managedBy,
					"app.kubernetes.io/name":       cfg.Name,
				},
			},
			"spec": map[string]interface{}{
				"predictor": buildPredictor(cfg),
			},
		},
	}

	return isvc
}

func buildPredictor(cfg ModelConfig) map[string]interface{} {
	model := map[string]interface{}{
		"modelFormat": map[string]interface{}{
			"name": "vLLM",
		},
		"storageUri": cfg.ModelURI,
	}

	if cfg.Runtime != "" {
		model["runtime"] = cfg.Runtime
	}

	if cfg.GPUCount > 0 {
		model["resources"] = map[string]interface{}{
			"requests": map[string]interface{}{
				"nvidia.com/gpu": strconv.Itoa(cfg.GPUCount),
			},
			"limits": map[string]interface{}{
				"nvidia.com/gpu": strconv.Itoa(cfg.GPUCount),
			},
		}
	}

	if len(cfg.RuntimeArgs) > 0 {
		args := make([]interface{}, len(cfg.RuntimeArgs))
		for i, a := range cfg.RuntimeArgs {
			args[i] = a
		}
		model["args"] = args
	}

	return map[string]interface{}{
		"model": model,
	}
}

// sanitizeName converts a model name to a valid Kubernetes resource name.
func sanitizeName(name string) string {
	result := make([]byte, 0, len(name))
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			result = append(result, byte(c))
		case c >= 'A' && c <= 'Z':
			result = append(result, byte(c-'A'+'a'))
		case c == '_', c == '.', c == '/', c == '@':
			result = append(result, '-')
		default:
			// Skip invalid characters.
		}
	}

	// Ensure it starts with a letter.
	if len(result) > 0 && (result[0] < 'a' || result[0] > 'z') {
		result = append([]byte("m-"), result...)
	}

	// Truncate to 63 characters (Kubernetes name limit).
	if len(result) > 63 {
		result = result[:63]
	}

	// Trim trailing dashes (invalid in DNS labels).
	return strings.TrimRight(string(result), "-")
}

// EndpointURL returns the in-cluster URL for an InferenceService.
func EndpointURL(name, namespace string) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local/v1", sanitizeName(name), namespace)
}
