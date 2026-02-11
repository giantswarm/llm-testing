package kserve

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	apiVersion = "serving.kserve.io/v1beta1"
	kind       = "InferenceService"
	managedBy  = "llm-testing"
)

// BuildInferenceService creates a typed InferenceService object from a ModelConfig.
func BuildInferenceService(cfg ModelConfig, namespace string) *InferenceService {
	storageURI := cfg.ModelURI

	isvc := &InferenceService{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiVersion,
			Kind:       kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sanitizeName(cfg.Name),
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": managedBy,
				"app.kubernetes.io/name":       cfg.Name,
			},
		},
		Spec: InferenceServiceSpec{
			Predictor: PredictorSpec{
				Model: &ISvcModelSpec{
					ModelFormat: ModelFormat{
						Name: "vLLM",
					},
					StorageURI: &storageURI,
				},
			},
		},
	}

	if cfg.Runtime != "" {
		rt := cfg.Runtime
		isvc.Spec.Predictor.Model.Runtime = &rt
	}

	if cfg.GPUCount > 0 {
		gpuQty := resource.MustParse(strconv.Itoa(cfg.GPUCount))
		isvc.Spec.Predictor.Model.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"nvidia.com/gpu": gpuQty,
			},
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": gpuQty,
			},
		}
	}

	if len(cfg.RuntimeArgs) > 0 {
		isvc.Spec.Predictor.Model.Args = cfg.RuntimeArgs
	}

	return isvc
}

// toUnstructured converts a typed InferenceService to an unstructured object
// for use with the dynamic Kubernetes client.
func toUnstructured(isvc *InferenceService) (*unstructured.Unstructured, error) {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(isvc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert InferenceService to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

// fromUnstructured converts an unstructured object back to a typed InferenceService.
func fromUnstructured(obj *unstructured.Unstructured) (*InferenceService, error) {
	isvc := &InferenceService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, isvc); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to InferenceService: %w", err)
	}
	return isvc, nil
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
