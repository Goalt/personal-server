package monitoring

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
)

func TestMonitoringModule_Name(t *testing.T) {
	module := &MonitoringModule{}
	if module.Name() != "monitoring" {
		t.Errorf("Name() = %s, want monitoring", module.Name())
	}
}

func TestMonitoringModule_Prepare(t *testing.T) {
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
				"sentry_dsn": "https://test@sentry.io/123",
			},
			wantErr: false,
		},
		{
			name:      "missing sentry_dsn",
			namespace: "infra",
			secrets:   map[string]string{},
			wantErr:   true,
		},
		{
			name:      "custom namespace",
			namespace: "monitoring",
			secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &MonitoringModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "monitoring",
					Namespace: tt.namespace,
					Secrets:   tt.secrets,
				},
			}

			sa, cr, crb, secret, deployment, err := module.prepare()

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
			if sa == nil {
				t.Error("prepare() returned nil ServiceAccount")
			}
			if cr == nil {
				t.Error("prepare() returned nil ClusterRole")
			}
			if crb == nil {
				t.Error("prepare() returned nil ClusterRoleBinding")
			}
			if secret == nil {
				t.Error("prepare() returned nil Secret")
			}
			if deployment == nil {
				t.Error("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly
			if sa.Namespace != tt.namespace {
				t.Errorf("ServiceAccount namespace = %s, want %s", sa.Namespace, tt.namespace)
			}
			if secret.Namespace != tt.namespace {
				t.Errorf("Secret namespace = %s, want %s", secret.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
		})
	}
}

func TestMonitoringModule_PrepareServiceAccount(t *testing.T) {
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/123",
			},
		},
	}

	sa, _, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ServiceAccount name
	if sa.Name != "monitor-sentry-kubernetes" {
		t.Errorf("ServiceAccount name = %s, want monitor-sentry-kubernetes", sa.Name)
	}

	// Test ServiceAccount labels
	expectedLabels := map[string]string{
		"app":      "sentry-kubernetes",
		"heritage": "Helm",
		"release":  "monitor",
		"chart":    "sentry-kubernetes-0.2.6",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := sa.Labels[key]; !ok {
			t.Errorf("ServiceAccount missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("ServiceAccount label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}
}

func TestMonitoringModule_PrepareClusterRole(t *testing.T) {
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/123",
			},
		},
	}

	_, cr, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ClusterRole name
	if cr.Name != "monitor-sentry-kubernetes" {
		t.Errorf("ClusterRole name = %s, want monitor-sentry-kubernetes", cr.Name)
	}

	// Test ClusterRole rules
	if len(cr.Rules) != 1 {
		t.Fatalf("ClusterRole rules count = %d, want 1", len(cr.Rules))
	}

	rule := cr.Rules[0]
	if len(rule.APIGroups) != 1 || rule.APIGroups[0] != "" {
		t.Errorf("ClusterRole rule APIGroups = %v, want [\"\"]", rule.APIGroups)
	}
	if len(rule.Resources) != 1 || rule.Resources[0] != "events" {
		t.Errorf("ClusterRole rule Resources = %v, want [\"events\"]", rule.Resources)
	}

	expectedVerbs := []string{"get", "list", "watch"}
	if len(rule.Verbs) != len(expectedVerbs) {
		t.Errorf("ClusterRole rule Verbs count = %d, want %d", len(rule.Verbs), len(expectedVerbs))
	}
	for i, verb := range expectedVerbs {
		if rule.Verbs[i] != verb {
			t.Errorf("ClusterRole rule Verbs[%d] = %s, want %s", i, rule.Verbs[i], verb)
		}
	}
}

func TestMonitoringModule_PrepareClusterRoleBinding(t *testing.T) {
	testNamespace := "test-namespace"
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: testNamespace,
			Secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/123",
			},
		},
	}

	_, _, crb, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ClusterRoleBinding name
	if crb.Name != "monitor-sentry-kubernetes" {
		t.Errorf("ClusterRoleBinding name = %s, want monitor-sentry-kubernetes", crb.Name)
	}

	// Test RoleRef
	if crb.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("ClusterRoleBinding RoleRef.APIGroup = %s, want rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
	}
	if crb.RoleRef.Kind != "ClusterRole" {
		t.Errorf("ClusterRoleBinding RoleRef.Kind = %s, want ClusterRole", crb.RoleRef.Kind)
	}
	if crb.RoleRef.Name != "monitor-sentry-kubernetes" {
		t.Errorf("ClusterRoleBinding RoleRef.Name = %s, want monitor-sentry-kubernetes", crb.RoleRef.Name)
	}

	// Test Subjects
	if len(crb.Subjects) != 1 {
		t.Fatalf("ClusterRoleBinding Subjects count = %d, want 1", len(crb.Subjects))
	}
	subject := crb.Subjects[0]
	if subject.Kind != "ServiceAccount" {
		t.Errorf("ClusterRoleBinding Subject.Kind = %s, want ServiceAccount", subject.Kind)
	}
	if subject.Name != "monitor-sentry-kubernetes" {
		t.Errorf("ClusterRoleBinding Subject.Name = %s, want monitor-sentry-kubernetes", subject.Name)
	}
	if subject.Namespace != testNamespace {
		t.Errorf("ClusterRoleBinding Subject.Namespace = %s, want %s", subject.Namespace, testNamespace)
	}
}

func TestMonitoringModule_PrepareSecret(t *testing.T) {
	sentryDSN := "https://test@sentry.io/123"
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"sentry_dsn": sentryDSN,
			},
		},
	}

	_, _, _, secret, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Secret name
	if secret.Name != "monitor-sentry-kubernetes" {
		t.Errorf("Secret name = %s, want monitor-sentry-kubernetes", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret data
	if string(secret.Data["sentry.dsn"]) != sentryDSN {
		t.Errorf("Secret data[sentry.dsn] = %s, want %s", string(secret.Data["sentry.dsn"]), sentryDSN)
	}
}

func TestMonitoringModule_PrepareDeployment(t *testing.T) {
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/123",
			},
		},
	}

	_, _, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Deployment name
	if deployment.Name != "monitor-sentry-kubernetes" {
		t.Errorf("Deployment name = %s, want monitor-sentry-kubernetes", deployment.Name)
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "sentry-kubernetes" {
		t.Errorf("Deployment selector app = %s, want sentry-kubernetes", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test pod template
	if deployment.Spec.Template.Spec.ServiceAccountName != "monitor-sentry-kubernetes" {
		t.Errorf("Deployment ServiceAccountName = %s, want monitor-sentry-kubernetes", deployment.Spec.Template.Spec.ServiceAccountName)
	}
}

func TestMonitoringModule_PrepareDeploymentContainer(t *testing.T) {
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/123",
			},
		},
	}

	_, _, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "sentry-kubernetes" {
		t.Errorf("Container name = %s, want sentry-kubernetes", container.Name)
	}

	// Test container image
	expectedImage := "ghcr.io/goalt/sentry-kubernetes:0b536b48eee946b00cac35e161561f3f31fb1a79"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test image pull policy
	if container.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("Container ImagePullPolicy = %s, want Always", container.ImagePullPolicy)
	}

	// Test environment variables
	expectedEnvs := map[string]string{
		"SENTRY_K8S_MONITOR_CRONJOBS": "true",
		"SENTRY_K8S_WATCH_NAMESPACES": "__all__",
	}

	for _, env := range container.Env {
		if expectedValue, ok := expectedEnvs[env.Name]; ok {
			if env.Value != expectedValue {
				t.Errorf("Container env %s = %s, want %s", env.Name, env.Value, expectedValue)
			}
		}
	}

	// Verify SENTRY_DSN uses secret reference
	sentryDSNFound := false
	for _, env := range container.Env {
		if env.Name == "SENTRY_DSN" {
			sentryDSNFound = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Error("SENTRY_DSN should use SecretKeyRef")
			} else {
				if env.ValueFrom.SecretKeyRef.Name != "monitor-sentry-kubernetes" {
					t.Errorf("SENTRY_DSN secret name = %s, want monitor-sentry-kubernetes", env.ValueFrom.SecretKeyRef.Name)
				}
				if env.ValueFrom.SecretKeyRef.Key != "sentry.dsn" {
					t.Errorf("SENTRY_DSN secret key = %s, want sentry.dsn", env.ValueFrom.SecretKeyRef.Key)
				}
			}
		}
	}
	if !sentryDSNFound {
		t.Error("Container missing SENTRY_DSN env var")
	}
}

func TestMonitoringModule_PrepareMissingSentryDSN(t *testing.T) {
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, _, _, err := module.prepare()
	if err == nil {
		t.Error("prepare() expected error for missing sentry_dsn, got nil")
	}

	expectedErr := "sentry_dsn not found in configuration"
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
	module := &MonitoringModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "monitoring",
			Namespace: "infra",
			Secrets: map[string]string{
				"sentry_dsn": "https://test@sentry.io/123",
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
		"configs/monitoring/serviceaccount.yaml",
		"configs/monitoring/clusterrole.yaml",
		"configs/monitoring/clusterrolebinding.yaml",
		"configs/monitoring/secret.yaml",
		"configs/monitoring/deployment.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file %s was not generated", file)
		}
	}

	// Verify deployment contains expected content
	deploymentPath := filepath.Join(tempDir, "configs/monitoring/deployment.yaml")
	deploymentContent, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatalf("failed to read deployment.yaml: %v", err)
	}
	deploymentStr := string(deploymentContent)

	expectedStrings := []string{
		"monitor-sentry-kubernetes",
		"infra",
		"sentry-kubernetes",
	}
	for _, expected := range expectedStrings {
		if !strings.Contains(deploymentStr, expected) {
			t.Errorf("deployment.yaml missing expected content: %s", expected)
		}
	}
}
