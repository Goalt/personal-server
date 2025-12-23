package drone

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
)

func TestDroneModule_Name(t *testing.T) {
	module := &DroneModule{}
	if module.Name() != "drone" {
		t.Errorf("Name() = %s, want drone", module.Name())
	}
}

func TestDroneModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		domain    string
	}{
		{
			name:      "default configuration",
			namespace: "infra",
			domain:    "example.com",
		},
		{
			name:      "custom namespace",
			namespace: "ci-cd",
			domain:    "myserver.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &DroneModule{
				GeneralConfig: config.GeneralConfig{
					Domain: tt.domain,
				},
				ModuleConfig: config.Module{
					Name:      "drone",
					Namespace: tt.namespace,
					Secrets: map[string]string{
						"drone_gitea_client_id":     "client-id",
						"drone_gitea_client_secret": "client-secret",
						"drone_rpc_secret":          "rpc-secret",
						"drone_server_proto":        "https",
						"drone_server_host":         "drone",
					},
				},
			}

			secret, role, roleBinding, deployment, runnerDeployment, service := module.prepare()

			// Verify all objects are not nil
			if secret == nil {
				t.Error("prepare() returned nil Secret")
			}
			if role == nil {
				t.Error("prepare() returned nil Role")
			}
			if roleBinding == nil {
				t.Error("prepare() returned nil RoleBinding")
			}
			if deployment == nil {
				t.Error("prepare() returned nil Deployment")
			}
			if runnerDeployment == nil {
				t.Error("prepare() returned nil Runner Deployment")
			}
			if service == nil {
				t.Error("prepare() returned nil Service")
			}

			// Verify namespace is set correctly
			if secret.Namespace != tt.namespace {
				t.Errorf("Secret namespace = %s, want %s", secret.Namespace, tt.namespace)
			}
			if role.Namespace != tt.namespace {
				t.Errorf("Role namespace = %s, want %s", role.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
			if service.Namespace != tt.namespace {
				t.Errorf("Service namespace = %s, want %s", service.Namespace, tt.namespace)
			}
		})
	}
}

func TestDroneModule_PrepareSecret(t *testing.T) {
	domain := "example.com"
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: domain,
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"drone_gitea_client_id":     "client-id",
				"drone_gitea_client_secret": "client-secret",
				"drone_rpc_secret":          "rpc-secret",
				"drone_server_proto":        "https",
			},
		},
	}

	secret, _, _, _, _, _ := module.prepare()

	// Test Secret name
	if secret.Name != "drone-secrets" {
		t.Errorf("Secret name = %s, want drone-secrets", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret data includes gitea server URL with domain
	expectedGiteaServer := "https://gitea." + domain
	if secret.StringData["drone_gitea_server"] != expectedGiteaServer {
		t.Errorf("Secret drone_gitea_server = %s, want %s", secret.StringData["drone_gitea_server"], expectedGiteaServer)
	}

	// Test Secret data includes server host with domain
	expectedServerHost := "drone." + domain
	if secret.StringData["drone_server_host"] != expectedServerHost {
		t.Errorf("Secret drone_server_host = %s, want %s", secret.StringData["drone_server_host"], expectedServerHost)
	}
}

func TestDroneModule_PrepareRole(t *testing.T) {
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, role, _, _, _, _ := module.prepare()

	// Test Role name
	if role.Name != "drone" {
		t.Errorf("Role name = %s, want drone", role.Name)
	}

	// Test Role rules
	if len(role.Rules) != 2 {
		t.Fatalf("Role rules count = %d, want 2", len(role.Rules))
	}

	// Check secrets rule
	secretsRuleFound := false
	podsRuleFound := false
	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			if resource == "secrets" {
				secretsRuleFound = true
				// Verify verbs for secrets
				verbSet := make(map[string]bool)
				for _, verb := range rule.Verbs {
					verbSet[verb] = true
				}
				if !verbSet["create"] || !verbSet["delete"] {
					t.Error("Secrets rule should have create and delete verbs")
				}
			}
			if resource == "pods" {
				podsRuleFound = true
			}
		}
	}
	if !secretsRuleFound {
		t.Error("Role missing secrets rule")
	}
	if !podsRuleFound {
		t.Error("Role missing pods rule")
	}
}

func TestDroneModule_PrepareRoleBinding(t *testing.T) {
	testNamespace := "test-namespace"
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: testNamespace,
			Secrets:   map[string]string{},
		},
	}

	_, _, roleBinding, _, _, _ := module.prepare()

	// Test RoleBinding name
	if roleBinding.Name != "drone" {
		t.Errorf("RoleBinding name = %s, want drone", roleBinding.Name)
	}

	// Test RoleRef
	if roleBinding.RoleRef.Kind != "Role" {
		t.Errorf("RoleBinding RoleRef.Kind = %s, want Role", roleBinding.RoleRef.Kind)
	}
	if roleBinding.RoleRef.Name != "drone" {
		t.Errorf("RoleBinding RoleRef.Name = %s, want drone", roleBinding.RoleRef.Name)
	}
	if roleBinding.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("RoleBinding RoleRef.APIGroup = %s, want rbac.authorization.k8s.io", roleBinding.RoleRef.APIGroup)
	}

	// Test Subjects
	if len(roleBinding.Subjects) != 1 {
		t.Fatalf("RoleBinding Subjects count = %d, want 1", len(roleBinding.Subjects))
	}
	subject := roleBinding.Subjects[0]
	if subject.Kind != "ServiceAccount" {
		t.Errorf("RoleBinding Subject.Kind = %s, want ServiceAccount", subject.Kind)
	}
	if subject.Name != "default" {
		t.Errorf("RoleBinding Subject.Name = %s, want default", subject.Name)
	}
	if subject.Namespace != testNamespace {
		t.Errorf("RoleBinding Subject.Namespace = %s, want %s", subject.Namespace, testNamespace)
	}
}

func TestDroneModule_PrepareDeployment(t *testing.T) {
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, deployment, _, _ := module.prepare()

	// Test Deployment name
	if deployment.Name != "drone" {
		t.Errorf("Deployment name = %s, want drone", deployment.Name)
	}

	// Test labels
	if deployment.Labels["app"] != "drone" {
		t.Errorf("Deployment label app = %s, want drone", deployment.Labels["app"])
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "drone" {
		t.Errorf("Deployment selector app = %s, want drone", deployment.Spec.Selector.MatchLabels["app"])
	}
}

func TestDroneModule_PrepareDeploymentContainer(t *testing.T) {
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, deployment, _, _ := module.prepare()

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "drone" {
		t.Errorf("Container name = %s, want drone", container.Name)
	}

	// Test container image
	if container.Image != "drone/drone:2" {
		t.Errorf("Container image = %s, want drone/drone:2", container.Image)
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Errorf("Container ports count = %d, want 1", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 80 {
		t.Errorf("Container port = %d, want 80", container.Ports[0].ContainerPort)
	}

	// Test probes
	if container.LivenessProbe == nil {
		t.Error("Container LivenessProbe is nil")
	}
	if container.ReadinessProbe == nil {
		t.Error("Container ReadinessProbe is nil")
	}

	// Verify env vars use secret references
	envCount := 0
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			envCount++
			if env.ValueFrom.SecretKeyRef.Name != "drone-secrets" {
				t.Errorf("Env %s secret name = %s, want drone-secrets", env.Name, env.ValueFrom.SecretKeyRef.Name)
			}
		}
	}
	if envCount < 5 {
		t.Errorf("Expected at least 5 env vars with secret refs, got %d", envCount)
	}
}

func TestDroneModule_PrepareRunnerDeployment(t *testing.T) {
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, _, runnerDeployment, _ := module.prepare()

	// Test Deployment name
	if runnerDeployment.Name != "drone-runner" {
		t.Errorf("Runner Deployment name = %s, want drone-runner", runnerDeployment.Name)
	}

	// Test labels
	if runnerDeployment.Labels["app.kubernetes.io/name"] != "drone-runner" {
		t.Errorf("Runner Deployment label app.kubernetes.io/name = %s, want drone-runner", runnerDeployment.Labels["app.kubernetes.io/name"])
	}

	// Test replicas
	if runnerDeployment.Spec.Replicas == nil {
		t.Fatal("Runner Deployment replicas is nil")
	}
	if *runnerDeployment.Spec.Replicas != 1 {
		t.Errorf("Runner Deployment replicas = %d, want 1", *runnerDeployment.Spec.Replicas)
	}

	// Verify container
	if len(runnerDeployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Runner Container count = %d, want 1", len(runnerDeployment.Spec.Template.Spec.Containers))
	}

	container := runnerDeployment.Spec.Template.Spec.Containers[0]
	if container.Name != "runner" {
		t.Errorf("Runner Container name = %s, want runner", container.Name)
	}
	if container.Image != "drone/drone-runner-kube:latest" {
		t.Errorf("Runner Container image = %s, want drone/drone-runner-kube:latest", container.Image)
	}
}

func TestDroneModule_PrepareService(t *testing.T) {
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, _, _, service := module.prepare()

	// Test Service name
	if service.Name != "drone" {
		t.Errorf("Service name = %s, want drone", service.Name)
	}

	// Test Service type
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("Service type = %s, want ClusterIP", service.Spec.Type)
	}

	// Test selector
	if service.Spec.Selector["app"] != "drone" {
		t.Errorf("Service selector app = %s, want drone", service.Spec.Selector["app"])
	}

	// Test ports
	if len(service.Spec.Ports) != 1 {
		t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}
	if service.Spec.Ports[0].Port != 80 {
		t.Errorf("Service port = %d, want 80", service.Spec.Ports[0].Port)
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			expected: "1m",
		},
		{
			name:     "multiple minutes",
			duration: 45 * time.Minute,
			expected: "45m",
		},
		{
			name:     "one hour",
			duration: 1 * time.Hour,
			expected: "1h",
		},
		{
			name:     "multiple hours",
			duration: 5 * time.Hour,
			expected: "5h",
		},
		{
			name:     "one day",
			duration: 24 * time.Hour,
			expected: "1d",
		},
		{
			name:     "multiple days",
			duration: 72 * time.Hour,
			expected: "3d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := k8s.FormatAge(tt.duration)
			if result != tt.expected {
				t.Errorf("k8s.FormatAge(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
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
	module := &DroneModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "drone",
			Namespace: "infra",
			Secrets: map[string]string{
				"drone_gitea_client_secret": "test-secret",
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
		"configs/drone/secret.yaml",
		"configs/drone/role.yaml",
		"configs/drone/rolebinding.yaml",
		"configs/drone/deployment.yaml",
		"configs/drone/runner-deployment.yaml",
		"configs/drone/service.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file %s was not generated", file)
		}
	}
}
