package bitwarden

import (
	"testing"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestBitwardenModule_Name(t *testing.T) {
	module := &BitwardenModule{}
	if module.Name() != "bitwarden" {
		t.Errorf("Name() = %s, want bitwarden", module.Name())
	}
}

func TestBitwardenModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "default namespace",
			namespace: "infra",
		},
		{
			name:      "custom namespace",
			namespace: "bitwarden-ns",
		},
		{
			name:      "hobby namespace",
			namespace: "hobby",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &BitwardenModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "bitwarden",
					Namespace: tt.namespace,
				},
			}

			pvc, service, deployment := module.prepare()

			// Verify all objects are not nil
			if pvc == nil {
				t.Fatal("prepare() returned nil PVC")
			}
			if service == nil {
				t.Fatal("prepare() returned nil Service")
			}
			if deployment == nil {
				t.Fatal("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly on all objects
			if pvc.Namespace != tt.namespace {
				t.Errorf("PVC namespace = %s, want %s", pvc.Namespace, tt.namespace)
			}
			if service.Namespace != tt.namespace {
				t.Errorf("Service namespace = %s, want %s", service.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
		})
	}
}

func TestBitwardenModule_PreparePVC(t *testing.T) {
	module := &BitwardenModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "bitwarden",
			Namespace: "test-namespace",
		},
	}

	pvc, _, _ := module.prepare()

	// Test PVC name
	if pvc.Name != "bitwarden-claim0" {
		t.Errorf("PVC name = %s, want bitwarden-claim0", pvc.Name)
	}

	// Test PVC labels
	expectedLabels := map[string]string{
		"managed-by":         "personal-server",
		"io.kompose.service": "bitwarden-claim0",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := pvc.Labels[key]; !ok {
			t.Errorf("PVC missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("PVC label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test access modes
	if len(pvc.Spec.AccessModes) != 1 {
		t.Errorf("PVC access modes count = %d, want 1", len(pvc.Spec.AccessModes))
	}
	if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("PVC access mode = %s, want ReadWriteOnce", pvc.Spec.AccessModes[0])
	}

	// Test storage request
	expectedStorage := resource.MustParse("100Mi")
	actualStorage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if actualStorage.Cmp(expectedStorage) != 0 {
		t.Errorf("PVC storage request = %s, want %s", actualStorage.String(), expectedStorage.String())
	}
}

func TestBitwardenModule_PrepareService(t *testing.T) {
	module := &BitwardenModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "bitwarden",
			Namespace: "test-namespace",
		},
	}

	_, service, _ := module.prepare()

	// Test Service name
	if service.Name != "bitwarden" {
		t.Errorf("Service name = %s, want bitwarden", service.Name)
	}

	// Test Service labels
	expectedLabels := map[string]string{
		"managed-by":         "personal-server",
		"io.kompose.service": "bitwarden",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := service.Labels[key]; !ok {
			t.Errorf("Service missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Service label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test Service annotations
	expectedAnnotations := map[string]string{
		"kompose.cmd":     "kompose --file docker-comopose.yaml convert",
		"kompose.version": "1.26.1 (HEAD)",
	}
	for key, expectedValue := range expectedAnnotations {
		if actualValue, ok := service.Annotations[key]; !ok {
			t.Errorf("Service missing annotation: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Service annotation %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test Service ports
	if len(service.Spec.Ports) != 2 {
		t.Errorf("Service ports count = %d, want 2", len(service.Spec.Ports))
	}

	// Verify port 80
	port80Found := false
	for _, port := range service.Spec.Ports {
		if port.Port == 80 {
			port80Found = true
			if port.Name != "80" {
				t.Errorf("Port 80 name = %s, want 80", port.Name)
			}
			if port.TargetPort.IntVal != 80 {
				t.Errorf("Port 80 targetPort = %d, want 80", port.TargetPort.IntVal)
			}
		}
	}
	if !port80Found {
		t.Error("Service missing port 80")
	}

	// Verify port 3012
	port3012Found := false
	for _, port := range service.Spec.Ports {
		if port.Port == 3012 {
			port3012Found = true
			if port.Name != "3012" {
				t.Errorf("Port 3012 name = %s, want 3012", port.Name)
			}
			if port.TargetPort.IntVal != 3012 {
				t.Errorf("Port 3012 targetPort = %d, want 3012", port.TargetPort.IntVal)
			}
		}
	}
	if !port3012Found {
		t.Error("Service missing port 3012")
	}

	// Test selector
	if service.Spec.Selector["app"] != "bitwarden" {
		t.Errorf("Service selector app = %s, want bitwarden", service.Spec.Selector["app"])
	}
}

func TestBitwardenModule_PrepareDeployment(t *testing.T) {
	module := &BitwardenModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "bitwarden",
			Namespace: "test-namespace",
		},
	}

	_, _, deployment := module.prepare()

	// Test Deployment name
	if deployment.Name != "bitwarden" {
		t.Errorf("Deployment name = %s, want bitwarden", deployment.Name)
	}

	// Test Deployment labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "bitwarden",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := deployment.Labels[key]; !ok {
			t.Errorf("Deployment missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Deployment label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test revision history limit
	if deployment.Spec.RevisionHistoryLimit == nil {
		t.Fatal("Deployment revisionHistoryLimit is nil")
	}
	if *deployment.Spec.RevisionHistoryLimit != 1 {
		t.Errorf("Deployment revisionHistoryLimit = %d, want 1", *deployment.Spec.RevisionHistoryLimit)
	}

	// Test selector
	if deployment.Spec.Selector == nil {
		t.Fatal("Deployment selector is nil")
	}
	if deployment.Spec.Selector.MatchLabels["app"] != "bitwarden" {
		t.Errorf("Deployment selector app = %s, want bitwarden", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test pod template labels
	if deployment.Spec.Template.Labels["app"] != "bitwarden" {
		t.Errorf("Pod template label app = %s, want bitwarden", deployment.Spec.Template.Labels["app"])
	}

	// Test restart policy
	if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("Pod restart policy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
	}
}

func TestBitwardenModule_PrepareDeploymentContainer(t *testing.T) {
	module := &BitwardenModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "bitwarden",
			Namespace: "test-namespace",
		},
	}

	_, _, deployment := module.prepare()

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "bitwarden" {
		t.Errorf("Container name = %s, want bitwarden", container.Name)
	}

	// Test container image
	expectedImage := "vaultwarden/server:1.32.0"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test environment variables
	if len(container.Env) != 1 {
		t.Errorf("Container env count = %d, want 1", len(container.Env))
	}
	websocketEnvFound := false
	for _, env := range container.Env {
		if env.Name == "WEBSOCKET_ENABLED" {
			websocketEnvFound = true
			if env.Value != "true" {
				t.Errorf("WEBSOCKET_ENABLED = %s, want true", env.Value)
			}
		}
	}
	if !websocketEnvFound {
		t.Error("Container missing WEBSOCKET_ENABLED env var")
	}

	// Test container ports
	if len(container.Ports) != 2 {
		t.Errorf("Container ports count = %d, want 2", len(container.Ports))
	}

	port80Found := false
	port3012Found := false
	for _, port := range container.Ports {
		if port.ContainerPort == 80 {
			port80Found = true
		}
		if port.ContainerPort == 3012 {
			port3012Found = true
		}
	}
	if !port80Found {
		t.Error("Container missing port 80")
	}
	if !port3012Found {
		t.Error("Container missing port 3012")
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 1 {
		t.Errorf("Container volume mounts count = %d, want 1", len(container.VolumeMounts))
	}

	volumeMount := container.VolumeMounts[0]
	if volumeMount.Name != "bitwarden-claim0" {
		t.Errorf("VolumeMount name = %s, want bitwarden-claim0", volumeMount.Name)
	}
	if volumeMount.MountPath != "/data" {
		t.Errorf("VolumeMount mountPath = %s, want /data", volumeMount.MountPath)
	}
}

func TestBitwardenModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &BitwardenModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "bitwarden",
			Namespace: "test-namespace",
		},
	}

	_, _, deployment := module.prepare()

	// Test volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("Volumes count = %d, want 1", len(deployment.Spec.Template.Spec.Volumes))
	}

	volume := deployment.Spec.Template.Spec.Volumes[0]
	if volume.Name != "bitwarden-claim0" {
		t.Errorf("Volume name = %s, want bitwarden-claim0", volume.Name)
	}

	if volume.PersistentVolumeClaim == nil {
		t.Fatal("Volume PersistentVolumeClaim is nil")
	}

	if volume.PersistentVolumeClaim.ClaimName != "bitwarden-claim0" {
		t.Errorf("Volume PVC claim name = %s, want bitwarden-claim0", volume.PersistentVolumeClaim.ClaimName)
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		hours    float64
		expected string
	}{
		{
			name:     "seconds",
			hours:    0.001, // ~3.6 seconds
			expected: "3s",
		},
		{
			name:     "one minute",
			hours:    1.0 / 60, // 1 minute
			expected: "1m",
		},
		{
			name:     "multiple minutes",
			hours:    0.5, // 30 minutes
			expected: "30m",
		},
		{
			name:     "one hour",
			hours:    1,
			expected: "1h",
		},
		{
			name:     "multiple hours",
			hours:    12,
			expected: "12h",
		},
		{
			name:     "one day",
			hours:    24,
			expected: "1d",
		},
		{
			name:     "multiple days",
			hours:    72,
			expected: "3d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use time.Duration for consistency
			d := time.Duration(tt.hours * float64(time.Hour))
			result := k8s.FormatAge(d)
			if result != tt.expected {
				t.Errorf("k8s.FormatAge(%v hours) = %s, want %s", tt.hours, result, tt.expected)
			}
		})
	}
}
