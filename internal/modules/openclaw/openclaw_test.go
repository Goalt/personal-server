package openclaw

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestOpenClawModule_Name(t *testing.T) {
	module := &OpenClawModule{}
	if module.Name() != "openclaw" {
		t.Errorf("Name() = %s, want openclaw", module.Name())
	}
}

func TestOpenClawModule_Prepare(t *testing.T) {
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
			namespace: "openclaw-ns",
		},
		{
			name:      "hobby namespace",
			namespace: "hobby",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &OpenClawModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "openclaw",
					Namespace: tt.namespace,
				},
			}

			configPVC, dataPVC, service, deployment := module.prepare()

			// Verify all objects are not nil
			if configPVC == nil {
				t.Fatal("prepare() returned nil Config PVC")
			}
			if dataPVC == nil {
				t.Fatal("prepare() returned nil Data PVC")
			}
			if service == nil {
				t.Fatal("prepare() returned nil Service")
			}
			if deployment == nil {
				t.Fatal("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly on all objects
			if configPVC.Namespace != tt.namespace {
				t.Errorf("Config PVC namespace = %s, want %s", configPVC.Namespace, tt.namespace)
			}
			if dataPVC.Namespace != tt.namespace {
				t.Errorf("Data PVC namespace = %s, want %s", dataPVC.Namespace, tt.namespace)
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

func TestOpenClawModule_PrepareConfigPVC(t *testing.T) {
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "test-namespace",
		},
	}

	configPVC, _, _, _ := module.prepare()

	// Test PVC name
	if configPVC.Name != "openclaw-config-pvc" {
		t.Errorf("Config PVC name = %s, want openclaw-config-pvc", configPVC.Name)
	}

	// Test PVC labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "openclaw",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := configPVC.Labels[key]; !ok {
			t.Errorf("Config PVC missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Config PVC label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test access modes
	if len(configPVC.Spec.AccessModes) != 1 {
		t.Errorf("Config PVC access modes count = %d, want 1", len(configPVC.Spec.AccessModes))
	}
	if configPVC.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("Config PVC access mode = %s, want ReadWriteOnce", configPVC.Spec.AccessModes[0])
	}

	// Test storage request
	expectedStorage := resource.MustParse("100Mi")
	actualStorage := configPVC.Spec.Resources.Requests[corev1.ResourceStorage]
	if actualStorage.Cmp(expectedStorage) != 0 {
		t.Errorf("Config PVC storage request = %s, want %s", actualStorage.String(), expectedStorage.String())
	}
}

func TestOpenClawModule_PrepareDataPVC(t *testing.T) {
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "test-namespace",
		},
	}

	_, dataPVC, _, _ := module.prepare()

	// Test PVC name
	if dataPVC.Name != "openclaw-data-pvc" {
		t.Errorf("Data PVC name = %s, want openclaw-data-pvc", dataPVC.Name)
	}

	// Test PVC labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "openclaw",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := dataPVC.Labels[key]; !ok {
			t.Errorf("Data PVC missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Data PVC label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test access modes
	if len(dataPVC.Spec.AccessModes) != 1 {
		t.Errorf("Data PVC access modes count = %d, want 1", len(dataPVC.Spec.AccessModes))
	}
	if dataPVC.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("Data PVC access mode = %s, want ReadWriteOnce", dataPVC.Spec.AccessModes[0])
	}

	// Test storage request
	expectedStorage := resource.MustParse("1Gi")
	actualStorage := dataPVC.Spec.Resources.Requests[corev1.ResourceStorage]
	if actualStorage.Cmp(expectedStorage) != 0 {
		t.Errorf("Data PVC storage request = %s, want %s", actualStorage.String(), expectedStorage.String())
	}
}

func TestOpenClawModule_PrepareService(t *testing.T) {
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "test-namespace",
		},
	}

	_, _, service, _ := module.prepare()

	// Test Service name
	if service.Name != "openclaw" {
		t.Errorf("Service name = %s, want openclaw", service.Name)
	}

	// Test Service labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "openclaw",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := service.Labels[key]; !ok {
			t.Errorf("Service missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Service label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test Service ports
	if len(service.Spec.Ports) != 1 {
		t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}

	// Verify port 5000
	port5000Found := false
	for _, port := range service.Spec.Ports {
		if port.Port == 5000 {
			port5000Found = true
			if port.Name != "http" {
				t.Errorf("Port 5000 name = %s, want http", port.Name)
			}
			if port.TargetPort.IntVal != 5000 {
				t.Errorf("Port 5000 targetPort = %d, want 5000", port.TargetPort.IntVal)
			}
		}
	}
	if !port5000Found {
		t.Error("Service missing port 5000")
	}

	// Test selector
	if service.Spec.Selector["app"] != "openclaw" {
		t.Errorf("Service selector app = %s, want openclaw", service.Spec.Selector["app"])
	}
}

func TestOpenClawModule_PrepareDeployment(t *testing.T) {
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "test-namespace",
		},
	}

	_, _, _, deployment := module.prepare()

	// Test Deployment name
	if deployment.Name != "openclaw" {
		t.Errorf("Deployment name = %s, want openclaw", deployment.Name)
	}

	// Test Deployment labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "openclaw",
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
	if deployment.Spec.Selector.MatchLabels["app"] != "openclaw" {
		t.Errorf("Deployment selector app = %s, want openclaw", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test pod template labels
	if deployment.Spec.Template.Labels["app"] != "openclaw" {
		t.Errorf("Pod template label app = %s, want openclaw", deployment.Spec.Template.Labels["app"])
	}

	// Test restart policy
	if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("Pod restart policy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
	}
}

func TestOpenClawModule_PrepareDeploymentContainer(t *testing.T) {
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "test-namespace",
		},
	}

	_, _, _, deployment := module.prepare()

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "openclaw" {
		t.Errorf("Container name = %s, want openclaw", container.Name)
	}

	// Test container image
	expectedImage := "openclaw/openclaw:latest"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Errorf("Container ports count = %d, want 1", len(container.Ports))
	}

	port5000Found := false
	for _, port := range container.Ports {
		if port.ContainerPort == 5000 {
			port5000Found = true
		}
	}
	if !port5000Found {
		t.Error("Container missing port 5000")
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 2 {
		t.Errorf("Container volume mounts count = %d, want 2", len(container.VolumeMounts))
	}

	configMountFound := false
	dataMountFound := false
	for _, vm := range container.VolumeMounts {
		if vm.Name == "openclaw-config" && vm.MountPath == "/config" {
			configMountFound = true
		}
		if vm.Name == "openclaw-data" && vm.MountPath == "/data" {
			dataMountFound = true
		}
	}
	if !configMountFound {
		t.Error("Container missing config volume mount at /config")
	}
	if !dataMountFound {
		t.Error("Container missing data volume mount at /data")
	}
}

func TestOpenClawModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "test-namespace",
		},
	}

	_, _, _, deployment := module.prepare()

	// Test volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("Volumes count = %d, want 2", len(deployment.Spec.Template.Spec.Volumes))
	}

	configVolumeFound := false
	dataVolumeFound := false
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "openclaw-config" {
			configVolumeFound = true
			if vol.PersistentVolumeClaim == nil {
				t.Fatal("Config volume PersistentVolumeClaim is nil")
			}
			if vol.PersistentVolumeClaim.ClaimName != "openclaw-config-pvc" {
				t.Errorf("Config volume PVC claim name = %s, want openclaw-config-pvc", vol.PersistentVolumeClaim.ClaimName)
			}
		}
		if vol.Name == "openclaw-data" {
			dataVolumeFound = true
			if vol.PersistentVolumeClaim == nil {
				t.Fatal("Data volume PersistentVolumeClaim is nil")
			}
			if vol.PersistentVolumeClaim.ClaimName != "openclaw-data-pvc" {
				t.Errorf("Data volume PVC claim name = %s, want openclaw-data-pvc", vol.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	if !configVolumeFound {
		t.Error("Missing openclaw-config volume")
	}
	if !dataVolumeFound {
		t.Error("Missing openclaw-data volume")
	}
}

//go:embed testdata/config-pvc.yaml
var expectedConfigPvcYAML string

//go:embed testdata/data-pvc.yaml
var expectedDataPvcYAML string

//go:embed testdata/service.yaml
var expectedServiceYAML string

//go:embed testdata/deployment.yaml
var expectedDeploymentYAML string

func TestGenerate(t *testing.T) {
	// Create a temporary directory for output
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Change to temp directory so Generate creates files there
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Create module with test configuration
	module := &OpenClawModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openclaw",
			Namespace: "infra",
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify generated files exist and match expected content
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{"config-pvc", "configs/openclaw/config-pvc.yaml", expectedConfigPvcYAML},
		{"data-pvc", "configs/openclaw/data-pvc.yaml", expectedDataPvcYAML},
		{"service", "configs/openclaw/service.yaml", expectedServiceYAML},
		{"deployment", "configs/openclaw/deployment.yaml", expectedDeploymentYAML},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read generated file
			generatedPath := filepath.Join(tempDir, tc.filename)
			generatedContent, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("failed to read generated file %s: %v", tc.filename, err)
			}

			// Compare with expected
			if string(generatedContent) != tc.expected {
				t.Errorf("Generated YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s", string(generatedContent), tc.expected)
			}
		})
	}
}
