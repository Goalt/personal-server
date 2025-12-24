package postgres

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

func TestPostgresModule_Name(t *testing.T) {
	module := &PostgresModule{}
	if module.Name() != "postgres" {
		t.Errorf("Name() = %s, want postgres", module.Name())
	}
}

func TestPostgresModule_Prepare(t *testing.T) {
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
				"admin_postgres_user":     "postgres",
				"admin_postgres_password": "secret123",
			},
			wantErr: false,
		},
		{
			name:      "missing admin_postgres_user",
			namespace: "infra",
			secrets: map[string]string{
				"admin_postgres_password": "secret123",
			},
			wantErr: true,
		},
		{
			name:      "missing admin_postgres_password",
			namespace: "infra",
			secrets: map[string]string{
				"admin_postgres_user": "postgres",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &PostgresModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "postgres",
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

func TestPostgresModule_PrepareSecret(t *testing.T) {
	user := "postgres"
	password := "secret123"
	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"admin_postgres_user":     user,
				"admin_postgres_password": password,
			},
		},
	}

	secret, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Secret name
	if secret.Name != "postgres-secrets" {
		t.Errorf("Secret name = %s, want postgres-secrets", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret data
	if string(secret.Data["admin_postgres_user"]) != user {
		t.Errorf("Secret data[admin_postgres_user] = %s, want %s", string(secret.Data["admin_postgres_user"]), user)
	}
	if string(secret.Data["admin_postgres_password"]) != password {
		t.Errorf("Secret data[admin_postgres_password] = %s, want %s", string(secret.Data["admin_postgres_password"]), password)
	}
}

func TestPostgresModule_PreparePVC(t *testing.T) {
	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"admin_postgres_user":     "postgres",
				"admin_postgres_password": "secret123",
			},
		},
	}

	_, pvc, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test PVC name
	if pvc.Name != "postgres-data-pvc" {
		t.Errorf("PVC name = %s, want postgres-data-pvc", pvc.Name)
	}

	// Test PVC labels
	if pvc.Labels["app"] != "postgres" {
		t.Errorf("PVC label app = %s, want postgres", pvc.Labels["app"])
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
}

func TestPostgresModule_PrepareService(t *testing.T) {
	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"admin_postgres_user":     "postgres",
				"admin_postgres_password": "secret123",
			},
		},
	}

	_, _, service, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Service name
	if service.Name != "postgres" {
		t.Errorf("Service name = %s, want postgres", service.Name)
	}

	// Test Service type
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("Service type = %s, want ClusterIP", service.Spec.Type)
	}

	// Test Service labels
	if service.Labels["app"] != "postgres" {
		t.Errorf("Service label app = %s, want postgres", service.Labels["app"])
	}

	// Test selector
	if service.Spec.Selector["app"] != "postgres" {
		t.Errorf("Service selector app = %s, want postgres", service.Spec.Selector["app"])
	}

	// Test Service ports
	if len(service.Spec.Ports) != 1 {
		t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}

	port := service.Spec.Ports[0]
	if port.Name != "postgres" {
		t.Errorf("Service port name = %s, want postgres", port.Name)
	}
	if port.Port != 5432 {
		t.Errorf("Service port = %d, want 5432", port.Port)
	}
	if port.TargetPort.IntVal != 5432 {
		t.Errorf("Service targetPort = %d, want 5432", port.TargetPort.IntVal)
	}
}

func TestPostgresModule_PrepareDeployment(t *testing.T) {
	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"admin_postgres_user":     "postgres",
				"admin_postgres_password": "secret123",
			},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Deployment name
	if deployment.Name != "postgres" {
		t.Errorf("Deployment name = %s, want postgres", deployment.Name)
	}

	// Test labels
	if deployment.Labels["app"] != "postgres" {
		t.Errorf("Deployment label app = %s, want postgres", deployment.Labels["app"])
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "postgres" {
		t.Errorf("Deployment selector app = %s, want postgres", deployment.Spec.Selector.MatchLabels["app"])
	}
}

func TestPostgresModule_PrepareDeploymentContainer(t *testing.T) {
	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"admin_postgres_user":     "postgres",
				"admin_postgres_password": "secret123",
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
	if container.Name != "postgres" {
		t.Errorf("Container name = %s, want postgres", container.Name)
	}

	// Test container image
	if container.Image != "postgres:16" {
		t.Errorf("Container image = %s, want postgres:16", container.Image)
	}

	// Test image pull policy
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("Container ImagePullPolicy = %s, want IfNotPresent", container.ImagePullPolicy)
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Errorf("Container ports count = %d, want 1", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 5432 {
		t.Errorf("Container port = %d, want 5432", container.Ports[0].ContainerPort)
	}

	// Test environment variables - verify secret references
	envNames := make(map[string]bool)
	for _, env := range container.Env {
		envNames[env.Name] = true
		if env.Name == "POSTGRES_USER" || env.Name == "POSTGRES_PASSWORD" {
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Errorf("%s should use SecretKeyRef", env.Name)
			} else if env.ValueFrom.SecretKeyRef.Name != "postgres-secrets" {
				t.Errorf("%s secret name = %s, want postgres-secrets", env.Name, env.ValueFrom.SecretKeyRef.Name)
			}
		}
		if env.Name == "PGDATA" && env.Value != "/var/lib/postgresql/data/pgdata" {
			t.Errorf("PGDATA = %s, want /var/lib/postgresql/data/pgdata", env.Value)
		}
	}

	// Verify required env vars exist
	requiredEnvs := []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "PGDATA"}
	for _, envName := range requiredEnvs {
		if !envNames[envName] {
			t.Errorf("Container missing env var: %s", envName)
		}
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 1 {
		t.Errorf("Container volume mounts count = %d, want 1", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != "data" {
		t.Errorf("VolumeMount name = %s, want data", container.VolumeMounts[0].Name)
	}
	if container.VolumeMounts[0].MountPath != "/var/lib/postgresql/data" {
		t.Errorf("VolumeMount mountPath = %s, want /var/lib/postgresql/data", container.VolumeMounts[0].MountPath)
	}

	// Test probes
	if container.LivenessProbe == nil {
		t.Error("Container LivenessProbe is nil")
	}
	if container.ReadinessProbe == nil {
		t.Error("Container ReadinessProbe is nil")
	}
}

func TestPostgresModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"admin_postgres_user":     "postgres",
				"admin_postgres_password": "secret123",
			},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("Volumes count = %d, want 1", len(deployment.Spec.Template.Spec.Volumes))
	}

	volume := deployment.Spec.Template.Spec.Volumes[0]
	if volume.Name != "data" {
		t.Errorf("Volume name = %s, want data", volume.Name)
	}

	if volume.PersistentVolumeClaim == nil {
		t.Fatal("Volume PersistentVolumeClaim is nil")
	}

	if volume.PersistentVolumeClaim.ClaimName != "postgres-data-pvc" {
		t.Errorf("Volume PVC claim name = %s, want postgres-data-pvc", volume.PersistentVolumeClaim.ClaimName)
	}
}

//go:embed testdata/deployment.yaml
var expectedDeploymentYAML string

//go:embed testdata/pvc.yaml
var expectedPvcYAML string

//go:embed testdata/secret.yaml
var expectedSecretYAML string

//go:embed testdata/service.yaml
var expectedServiceYAML string

func TestGenerate(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalWd)

	module := &PostgresModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "postgres",
			Namespace: "infra",
			Secrets: map[string]string{
				"admin_postgres_user":     "admin",
				"admin_postgres_password": "password",
			},
		},
		log: logger.Default(),
	}

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
		{"secret", "configs/postgres/secret.yaml", expectedSecretYAML},
		{"pvc", "configs/postgres/pvc.yaml", expectedPvcYAML},
		{"service", "configs/postgres/service.yaml", expectedServiceYAML},
		{"deployment", "configs/postgres/deployment.yaml", expectedDeploymentYAML},
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
