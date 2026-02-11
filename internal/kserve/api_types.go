package kserve

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceService represents a KServe InferenceService resource.
// This is a minimal typed representation matching the serving.kserve.io/v1beta1 CRD schema,
// covering only the fields this project needs. It avoids importing the full KServe SDK
// which has heavy transitive dependencies and k8s version compatibility constraints.
type InferenceService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferenceServiceSpec   `json:"spec,omitempty"`
	Status InferenceServiceStatus `json:"status,omitempty"`
}

// InferenceServiceSpec is the desired state of an InferenceService.
type InferenceServiceSpec struct {
	Predictor PredictorSpec `json:"predictor"`
}

// PredictorSpec defines the model serving configuration.
type PredictorSpec struct {
	Model *ISvcModelSpec `json:"model,omitempty"`
}

// ISvcModelSpec defines the model format, storage, runtime, and resource requirements
// for serving a model via KServe.
type ISvcModelSpec struct {
	// ModelFormat specifies the model format (e.g. "vLLM").
	ModelFormat ModelFormat `json:"modelFormat"`

	// Runtime is the KServe ServingRuntime name (e.g. "kserve-vllm").
	Runtime *string `json:"runtime,omitempty"`

	// StorageURI points to the model location (e.g. "hf://org/model").
	StorageURI *string `json:"storageUri,omitempty"`

	// Resources defines compute resource requirements for the model container.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Args are additional arguments passed to the serving runtime.
	Args []string `json:"args,omitempty"`
}

// ModelFormat identifies the model format by name and optional version.
type ModelFormat struct {
	Name    string  `json:"name"`
	Version *string `json:"version,omitempty"`
}

// InferenceServiceStatus represents the observed state of an InferenceService.
type InferenceServiceStatus struct {
	// Conditions holds the status conditions (Ready, PredictorReady, etc.)
	Conditions []StatusCondition `json:"conditions,omitempty"`

	// URL is the endpoint URL assigned by the InferenceService controller.
	URL string `json:"url,omitempty"`
}

// StatusCondition represents a single condition on an InferenceService,
// following the Knative condition schema.
type StatusCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// IsReady returns true if the InferenceService has a Ready=True condition.
func (s *InferenceServiceStatus) IsReady() bool {
	for _, c := range s.Conditions {
		if c.Type == "Ready" && c.Status == "True" {
			return true
		}
	}
	return false
}

// GetReadyCondition returns the Ready condition if present, or nil.
func (s *InferenceServiceStatus) GetReadyCondition() *StatusCondition {
	for i := range s.Conditions {
		if s.Conditions[i].Type == "Ready" {
			return &s.Conditions[i]
		}
	}
	return nil
}
