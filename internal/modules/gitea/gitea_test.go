package gitea

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestGiteaModule_Name(t *testing.T) {
	module := &GiteaModule{}
	if module.Name() != "gitea" {
		t.Errorf("Name() = %s, want gitea", module.Name())
	}
}

func TestGiteaModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		secrets   map[string]string
		wantErr   bool
	}{
		{
			name:      "valid configuration",
			namespace: "infra",
			secrets: map[string]string{
				"gitea_db_password": "secret123",
			},
			wantErr: false,
		},
		{
			name:      "missing gitea_db_password",
			namespace: "infra",
			secrets:   map[string]string{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &GiteaModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "gitea",
					Namespace: tt.namespace,
					Secrets:   tt.secrets,
				},
			}

			secret, pvc, service, deployment, err := module.prepare()

			if tt.wantErr {
				if err == nil {
					t.Error("prepare() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("prepare() unexpected error: %v", err)
				return
			}

			// Verify all objects are not nil
			if secret == nil {
				t.Error("prepare() returned nil Secret")
			}
			if pvc == nil {
				t.Error("prepare() returned nil PVC")
			}
			if service == nil {
				t.Error("prepare() returned nil Service")
			}
			if deployment == nil {
				t.Error("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly
			if secret.Namespace != tt.namespace {
				t.Errorf("Secret namespace = %s, want %s", secret.Namespace, tt.namespace)
			}
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

func TestGiteaModule_PrepareSecret(t *testing.T) {
	dbPassword := "secret123"
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"gitea_db_password": dbPassword,
			},
		},
	}

	secret, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Secret name
	if secret.Name != "gitea-secrets" {
		t.Errorf("Secret name = %s, want gitea-secrets", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret labels
	if secret.Labels["app"] != "gitea" {
		t.Errorf("Secret label app = %s, want gitea", secret.Labels["app"])
	}
	if secret.Labels["managed-by"] != "personal-server" {
		t.Errorf("Secret label managed-by = %s, want personal-server", secret.Labels["managed-by"])
	}

	// Test Secret data
	if string(secret.Data["GITEA__database__PASSWD"]) != dbPassword {
		t.Errorf("Secret data[GITEA__database__PASSWD] = %s, want %s", string(secret.Data["GITEA__database__PASSWD"]), dbPassword)
	}
}

func TestGiteaModule_PreparePVC(t *testing.T) {
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"gitea_db_password": "secret123",
			},
		},
	}

	_, pvc, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test PVC name
	if pvc.Name != "gitea-data-pvc" {
		t.Errorf("PVC name = %s, want gitea-data-pvc", pvc.Name)
	}

	// Test PVC labels
	if pvc.Labels["app"] != "gitea" {
		t.Errorf("PVC label app = %s, want gitea", pvc.Labels["app"])
	}
	if pvc.Labels["managed-by"] != "personal-server" {
		t.Errorf("PVC label managed-by = %s, want personal-server", pvc.Labels["managed-by"])
	}

	// Test access modes
	if len(pvc.Spec.AccessModes) != 1 {
		t.Errorf("PVC access modes count = %d, want 1", len(pvc.Spec.AccessModes))
	}
	if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("PVC access mode = %s, want ReadWriteOnce", pvc.Spec.AccessModes[0])
	}

	// Test storage request
	expectedStorage := resource.MustParse("10Gi")
	actualStorage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if actualStorage.Cmp(expectedStorage) != 0 {
		t.Errorf("PVC storage request = %s, want %s", actualStorage.String(), expectedStorage.String())
	}

	// Test storage class
	if pvc.Spec.StorageClassName == nil {
		t.Error("PVC storage class is nil")
	} else if *pvc.Spec.StorageClassName != "microk8s-hostpath" {
		t.Errorf("PVC storage class = %s, want microk8s-hostpath", *pvc.Spec.StorageClassName)
	}
}

func TestGiteaModule_PrepareService(t *testing.T) {
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"gitea_db_password": "secret123",
			},
		},
	}

	_, _, service, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Service name
	if service.Name != "gitea" {
		t.Errorf("Service name = %s, want gitea", service.Name)
	}

	// Test Service type
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("Service type = %s, want ClusterIP", service.Spec.Type)
	}

	// Test Service labels
	if service.Labels["app"] != "gitea" {
		t.Errorf("Service label app = %s, want gitea", service.Labels["app"])
	}

	// Test selector
	if service.Spec.Selector["app"] != "gitea" {
		t.Errorf("Service selector app = %s, want gitea", service.Spec.Selector["app"])
	}

	// Test Service ports
	if len(service.Spec.Ports) != 2 {
		t.Errorf("Service ports count = %d, want 2", len(service.Spec.Ports))
	}

	// Verify HTTP port
	httpPortFound := false
	sshPortFound := false
	for _, port := range service.Spec.Ports {
		if port.Name == "http" {
			httpPortFound = true
			if port.Port != 3000 {
				t.Errorf("HTTP port = %d, want 3000", port.Port)
			}
		}
		if port.Name == "ssh" {
			sshPortFound = true
			if port.Port != 22 {
				t.Errorf("SSH port = %d, want 22", port.Port)
			}
		}
	}
	if !httpPortFound {
		t.Error("Service missing HTTP port")
	}
	if !sshPortFound {
		t.Error("Service missing SSH port")
	}
}

func TestGiteaModule_PrepareDeployment(t *testing.T) {
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"gitea_db_password": "secret123",
			},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Deployment name
	if deployment.Name != "gitea" {
		t.Errorf("Deployment name = %s, want gitea", deployment.Name)
	}

	// Test labels
	if deployment.Labels["app"] != "gitea" {
		t.Errorf("Deployment label app = %s, want gitea", deployment.Labels["app"])
	}
	if deployment.Labels["managed-by"] != "personal-server" {
		t.Errorf("Deployment label managed-by = %s, want personal-server", deployment.Labels["managed-by"])
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "gitea" {
		t.Errorf("Deployment selector app = %s, want gitea", deployment.Spec.Selector.MatchLabels["app"])
	}
}

func TestGiteaModule_PrepareDeploymentContainer(t *testing.T) {
	domain := "example.com"
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: domain,
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"gitea_db_password": "secret123",
			},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "gitea" {
		t.Errorf("Container name = %s, want gitea", container.Name)
	}

	// Test container image
	if container.Image != "gitea/gitea:1.25" {
		t.Errorf("Container image = %s, want gitea/gitea:1.25", container.Image)
	}

	// Test container ports
	if len(container.Ports) != 2 {
		t.Errorf("Container ports count = %d, want 2", len(container.Ports))
	}

	httpPortFound := false
	sshPortFound := false
	for _, port := range container.Ports {
		if port.Name == "http" && port.ContainerPort == 3000 {
			httpPortFound = true
		}
		if port.Name == "ssh" && port.ContainerPort == 22 {
			sshPortFound = true
		}
	}
	if !httpPortFound {
		t.Error("Container missing HTTP port 3000")
	}
	if !sshPortFound {
		t.Error("Container missing SSH port 22")
	}

	// Test probes
	if container.LivenessProbe == nil {
		t.Error("Container LivenessProbe is nil")
	}
	if container.ReadinessProbe == nil {
		t.Error("Container ReadinessProbe is nil")
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 2 {
		t.Errorf("Container volume mounts count = %d, want 2", len(container.VolumeMounts))
	}

	dataVolumeFound := false
	configVolumeFound := false
	for _, vm := range container.VolumeMounts {
		if vm.Name == "gitea-data" && vm.MountPath == "/data" {
			dataVolumeFound = true
		}
		if vm.Name == "gitea-config" && vm.MountPath == "/etc/gitea" {
			configVolumeFound = true
		}
	}
	if !dataVolumeFound {
		t.Error("Container missing gitea-data volume mount")
	}
	if !configVolumeFound {
		t.Error("Container missing gitea-config volume mount")
	}

	// Check ROOT_URL env var uses domain
	rootURLFound := false
	for _, env := range container.Env {
		if env.Name == "GITEA__server__ROOT_URL" {
			rootURLFound = true
			expectedURL := "https://gitea." + domain
			if env.Value != expectedURL {
				t.Errorf("GITEA__server__ROOT_URL = %s, want %s", env.Value, expectedURL)
			}
		}
	}
	if !rootURLFound {
		t.Error("Container missing GITEA__server__ROOT_URL env var")
	}
}

func TestGiteaModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"gitea_db_password": "secret123",
			},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("Volumes count = %d, want 2", len(deployment.Spec.Template.Spec.Volumes))
	}

	dataVolumeFound := false
	configVolumeFound := false
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == "gitea-data" {
			dataVolumeFound = true
			if volume.PersistentVolumeClaim == nil {
				t.Error("gitea-data volume PVC is nil")
			} else if volume.PersistentVolumeClaim.ClaimName != "gitea-data-pvc" {
				t.Errorf("gitea-data PVC claim name = %s, want gitea-data-pvc", volume.PersistentVolumeClaim.ClaimName)
			}
		}
		if volume.Name == "gitea-config" {
			configVolumeFound = true
			if volume.EmptyDir == nil {
				t.Error("gitea-config volume should be EmptyDir")
			}
		}
	}
	if !dataVolumeFound {
		t.Error("Deployment missing gitea-data volume")
	}
	if !configVolumeFound {
		t.Error("Deployment missing gitea-config volume")
	}
}

func TestGiteaModule_PrepareMissingDBPassword(t *testing.T) {
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, _, err := module.prepare()
	if err == nil {
		t.Error("prepare() expected error for missing gitea_db_password, got nil")
	}

	expectedErr := "gitea_db_password not found in configuration"
	if err.Error() != expectedErr {
		t.Errorf("prepare() error = %s, want %s", err.Error(), expectedErr)
	}
}

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
	module := &GiteaModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "gitea",
			Namespace: "infra",
			Secrets: map[string]string{
				"gitea_db_password": "testpass123",
			},
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify all expected files exist
	expectedFiles := []string{
		"configs/gitea/secret.yaml",
		"configs/gitea/pvc.yaml",
		"configs/gitea/service.yaml",
		"configs/gitea/deployment.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file %s was not generated", file)
		}
	}

	// Verify deployment contains expected content
	deploymentPath := filepath.Join(tempDir, "configs/gitea/deployment.yaml")
	deploymentContent, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("failed to read deployment.yaml: %v", err)
	}
	deploymentStr := string(deploymentContent)

	expectedStrings := []string{
		"gitea",
		"infra",
		"managed-by: personal-server",
		"gitea/gitea",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(deploymentStr, expected) {
			t.Errorf("deployment.yaml missing expected content: %s", expected)
		}
	}

	// Verify secret contains expected content
	secretPath := filepath.Join(tempDir, "configs/gitea/secret.yaml")
	secretContent, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("failed to read secret.yaml: %v", err)
	}
	secretStr := string(secretContent)

	if !strings.Contains(secretStr, "gitea-secrets") {
		t.Errorf("secret.yaml missing expected name: gitea-secrets")
	}
}
