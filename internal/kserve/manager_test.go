package kserve

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newFakeManager(t *testing.T, objects ...runtime.Object) *Manager {
	t.Helper()
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			isvcGVR: "InferenceServiceList",
		},
		objects...,
	)
	return NewManagerWithClient(client, "test-namespace")
}

func makeISVC(name, namespace string, ready bool) *unstructured.Unstructured {
	conditions := []interface{}{}
	if ready {
		conditions = append(conditions, map[string]interface{}{
			"type":   "Ready",
			"status": "True",
		})
	} else {
		conditions = append(conditions, map[string]interface{}{
			"type":    "Ready",
			"status":  "False",
			"reason":  "Pending",
			"message": "waiting for model download",
		})
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": managedBy,
					"app.kubernetes.io/name":       name,
				},
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"status": map[string]interface{}{
				"conditions": conditions,
				"url":        "http://" + name + "." + namespace + ".example.com/v1",
			},
		},
	}
}

func TestManagerList(t *testing.T) {
	isvc1 := makeISVC("model-a", "test-namespace", true)
	isvc2 := makeISVC("model-b", "test-namespace", false)

	m := newFakeManager(t, isvc1, isvc2)

	statuses, err := m.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, statuses, 2)

	// Find model-a (ready).
	var modelA *ModelStatus
	for i := range statuses {
		if statuses[i].Name == "model-a" {
			modelA = &statuses[i]
			break
		}
	}
	require.NotNil(t, modelA, "model-a should be in the list")
	assert.True(t, modelA.Ready)
	assert.Equal(t, "http://model-a.test-namespace.example.com/v1", modelA.EndpointURL)

	// Find model-b (not ready).
	var modelB *ModelStatus
	for i := range statuses {
		if statuses[i].Name == "model-b" {
			modelB = &statuses[i]
			break
		}
	}
	require.NotNil(t, modelB, "model-b should be in the list")
	assert.False(t, modelB.Ready)
	assert.Equal(t, "pending", modelB.Message)
}

func TestManagerListEmpty(t *testing.T) {
	m := newFakeManager(t)

	statuses, err := m.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, statuses)
}

func TestManagerGet(t *testing.T) {
	isvc := makeISVC("my-model", "test-namespace", true)
	m := newFakeManager(t, isvc)

	status, err := m.Get(context.Background(), "my-model")
	require.NoError(t, err)
	assert.Equal(t, "my-model", status.Name)
	assert.True(t, status.Ready)
	assert.Equal(t, "http://my-model.test-namespace.example.com/v1", status.EndpointURL)
}

func TestManagerGetNotFound(t *testing.T) {
	m := newFakeManager(t)

	_, err := m.Get(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get InferenceService")
}

func TestManagerTeardown(t *testing.T) {
	isvc := makeISVC("to-delete", "test-namespace", true)
	m := newFakeManager(t, isvc)

	err := m.Teardown(context.Background(), "to-delete")
	require.NoError(t, err)

	// Verify the delete action was called.
	actions := m.client.(*dynamicfake.FakeDynamicClient).Actions()
	var deleteFound bool
	for _, a := range actions {
		if da, ok := a.(k8stesting.DeleteAction); ok {
			if da.GetName() == "to-delete" {
				deleteFound = true
				break
			}
		}
	}
	assert.True(t, deleteFound, "delete action should have been called for 'to-delete'")
}

func TestManagerTeardownNotFound(t *testing.T) {
	m := newFakeManager(t)

	err := m.Teardown(context.Background(), "nonexistent")
	assert.NoError(t, err)
}

func TestManagerDeploy(t *testing.T) {
	m := newFakeManager(t)

	cfg := ModelConfig{
		Name:         "deploy-test",
		ModelURI:     "hf://org/model",
		Runtime:      "kserve-vllm",
		GPUCount:     1,
		ReadyTimeout: 1 * time.Second,
	}

	// The fake client doesn't support watches with ready transitions, so Deploy
	// will timeout. We verify that the create action succeeds and the object
	// is created correctly.
	_, err := m.Deploy(context.Background(), cfg)
	// Deploy will fail waiting for ready (fake client doesn't send watch events),
	// but the create should have succeeded.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")

	// Verify the create action happened.
	actions := m.client.(*dynamicfake.FakeDynamicClient).Actions()
	var createFound bool
	for _, a := range actions {
		if ca, ok := a.(k8stesting.CreateAction); ok {
			obj := ca.GetObject().(*unstructured.Unstructured)
			if obj.GetName() == "deploy-test" {
				createFound = true
				// Verify the object structure.
				assert.Equal(t, apiVersion, obj.GetAPIVersion())
				assert.Equal(t, kind, obj.GetKind())
				assert.Equal(t, "test-namespace", obj.GetNamespace())
				labels := obj.GetLabels()
				assert.Equal(t, managedBy, labels["app.kubernetes.io/managed-by"])
				break
			}
		}
	}
	assert.True(t, createFound, "create action should have been called")
}

func TestManagerCheckCRDAvailable(t *testing.T) {
	m := newFakeManager(t)

	// With the fake client, the List call should succeed (CRD "exists").
	err := m.CheckCRDAvailable(context.Background())
	assert.NoError(t, err)
}

func TestManagerCheckCRDNotAvailable(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			isvcGVR: "InferenceServiceList",
		},
	)
	// Add a reactor that returns a not-found error, simulating missing CRD.
	client.PrependReactor("list", "inferenceservices", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    "serving.kserve.io",
			Resource: "inferenceservices",
		}, "")
	})
	m := NewManagerWithClient(client, "test-namespace")

	err := m.CheckCRDAvailable(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestIsReady(t *testing.T) {
	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		wantReady bool
	}{
		{
			name:      "ready true",
			obj:       makeISVC("test", "ns", true),
			wantReady: true,
		},
		{
			name:      "ready false",
			obj:       makeISVC("test", "ns", false),
			wantReady: false,
		},
		{
			name: "no conditions",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": apiVersion,
					"kind":       kind,
					"metadata":   map[string]interface{}{"name": "test"},
				},
			},
			wantReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isvc, err := fromUnstructured(tt.obj)
			require.NoError(t, err)
			assert.Equal(t, tt.wantReady, isvc.Status.IsReady())
		})
	}
}
