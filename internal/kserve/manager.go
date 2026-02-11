package kserve

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var isvcGVR = schema.GroupVersionResource{
	Group:    "serving.kserve.io",
	Version:  "v1beta1",
	Resource: "inferenceservices",
}

// Manager handles KServe InferenceService lifecycle.
type Manager struct {
	client    dynamic.Interface
	namespace string
}

// NewManager creates a new KServe manager.
func NewManager(namespace string, kubeconfig string, inCluster bool) (*Manager, error) {
	var config *rest.Config
	var err error

	if inCluster {
		config, err = rest.InClusterConfig()
	} else {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			loadingRules.ExplicitPath = kubeconfig
		}
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, &clientcmd.ConfigOverrides{},
		).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Manager{
		client:    client,
		namespace: namespace,
	}, nil
}

// NewManagerWithClient creates a Manager with an existing dynamic client (for testing).
func NewManagerWithClient(client dynamic.Interface, namespace string) *Manager {
	return &Manager{
		client:    client,
		namespace: namespace,
	}
}

// CheckCRDAvailable verifies that the InferenceService CRD is installed in the cluster.
// Returns nil if the CRD is available, or an error describing why it is not.
func (m *Manager) CheckCRDAvailable(ctx context.Context) error {
	_, err := m.client.Resource(isvcGVR).Namespace(m.namespace).List(ctx, metav1.ListOptions{
		Limit: 1,
	})
	if err != nil {
		return fmt.Errorf("KServe InferenceService CRD is not available in the cluster: %w", err)
	}
	return nil
}

// Deploy creates an InferenceService and waits for it to become ready.
func (m *Manager) Deploy(ctx context.Context, cfg ModelConfig) (*ModelStatus, error) {
	isvc := BuildInferenceService(cfg, m.namespace)
	name := isvc.Name

	obj, err := toUnstructured(isvc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert InferenceService: %w", err)
	}

	slog.Info("deploying InferenceService",
		"name", name,
		"model_uri", cfg.ModelURI,
		"gpu_count", cfg.GPUCount,
	)

	// Create the InferenceService.
	created, err := m.client.Resource(isvcGVR).Namespace(m.namespace).Create(
		ctx, obj, metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create InferenceService %s: %w", name, err)
	}

	slog.Info("InferenceService created, waiting for ready",
		"name", name,
	)

	// Wait for ready.
	if err := m.waitForReady(ctx, name, cfg.ReadyTimeout); err != nil {
		return nil, fmt.Errorf("InferenceService %s not ready: %w", name, err)
	}

	return &ModelStatus{
		Name:        name,
		Ready:       true,
		EndpointURL: endpointURL(isvc, m.namespace),
		CreatedAt:   created.GetCreationTimestamp().Format(time.RFC3339),
	}, nil
}

// Teardown deletes an InferenceService with graceful shutdown.
func (m *Manager) Teardown(ctx context.Context, name string) error {
	sanitized := sanitizeName(name)
	slog.Info("tearing down InferenceService", "name", sanitized)

	gracePeriod := int64(30)
	propagation := metav1.DeletePropagationForeground

	err := m.client.Resource(isvcGVR).Namespace(m.namespace).Delete(
		ctx, sanitized, metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
			PropagationPolicy:  &propagation,
		},
	)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete InferenceService %s: %w", sanitized, err)
	}

	return nil
}

// List returns all InferenceService resources managed by llm-testing.
func (m *Manager) List(ctx context.Context) ([]ModelStatus, error) {
	list, err := m.client.Resource(isvcGVR).Namespace(m.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=" + managedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list InferenceServices: %w", err)
	}

	statuses := make([]ModelStatus, 0, len(list.Items))
	for _, item := range list.Items {
		isvc, err := fromUnstructured(&item)
		if err != nil {
			slog.Warn("failed to convert InferenceService", "name", item.GetName(), "error", err)
			continue
		}
		statuses = append(statuses, m.statusFromISVC(isvc))
	}

	return statuses, nil
}

// Get returns the status of a specific InferenceService.
func (m *Manager) Get(ctx context.Context, name string) (*ModelStatus, error) {
	sanitized := sanitizeName(name)
	item, err := m.client.Resource(isvcGVR).Namespace(m.namespace).Get(
		ctx, sanitized, metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get InferenceService %s: %w", sanitized, err)
	}

	isvc, err := fromUnstructured(item)
	if err != nil {
		return nil, fmt.Errorf("failed to convert InferenceService %s: %w", sanitized, err)
	}

	status := m.statusFromISVC(isvc)
	return &status, nil
}

// statusFromISVC extracts a ModelStatus from a typed InferenceService.
func (m *Manager) statusFromISVC(isvc *InferenceService) ModelStatus {
	status := ModelStatus{
		Name:      isvc.Name,
		CreatedAt: isvc.CreationTimestamp.Format(time.RFC3339),
	}

	if isvc.Status.IsReady() {
		status.Ready = true
		status.EndpointURL = endpointURL(isvc, m.namespace)
	} else {
		status.Message = "pending"
	}

	return status
}

func (m *Manager) waitForReady(ctx context.Context, name string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	watcher, err := m.client.Resource(isvcGVR).Namespace(m.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + name,
	})
	if err != nil {
		return fmt.Errorf("failed to watch InferenceService: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for InferenceService %s to become ready", name)
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed for InferenceService %s", name)
			}

			if event.Type == watch.Modified || event.Type == watch.Added {
				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				isvc, err := fromUnstructured(obj)
				if err != nil {
					slog.Warn("failed to convert watch event", "error", err)
					continue
				}

				if isvc.Status.IsReady() {
					slog.Info("InferenceService ready", "name", name)
					return nil
				}

				if cond := isvc.Status.GetReadyCondition(); cond != nil && cond.Status == "False" {
					slog.Debug("InferenceService not ready yet",
						"name", name,
						"reason", cond.Reason,
						"message", cond.Message,
					)
				}
			}
		}
	}
}

func endpointURL(isvc *InferenceService, namespace string) string {
	if isvc.Status.URL != "" {
		return isvc.Status.URL
	}
	return EndpointURL(isvc.Name, namespace)
}
