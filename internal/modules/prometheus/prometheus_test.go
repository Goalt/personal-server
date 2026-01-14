package prometheus

import (
	"context"
	"os"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
)

func TestPrometheusModule_Name(t *testing.T) {
	module := &PrometheusModule{}
	if module.Name() != "prometheus" {
		t.Errorf("Name() = %s, want prometheus", module.Name())
	}
}

func TestPrometheusModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{
			name:      "valid configuration",
			namespace: "infra",
			wantErr:   false,
		},
		{
			name:      "custom namespace",
			namespace: "monitoring",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &PrometheusModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "prometheus",
					Namespace: tt.namespace,
				},
			}

			sa, cr, crb, cm, pvc, service, deployment, err := module.prepare()

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
			if cm == nil {
				t.Error("prepare() returned nil ConfigMap")
			}
			if pvc == nil {
				t.Error("prepare() returned nil PersistentVolumeClaim")
			}
			if service == nil {
				t.Error("prepare() returned nil Service")
			}
			if deployment == nil {
				t.Error("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly
			if sa.Namespace != tt.namespace {
				t.Errorf("ServiceAccount namespace = %s, want %s", sa.Namespace, tt.namespace)
			}
			if cm.Namespace != tt.namespace {
				t.Errorf("ConfigMap namespace = %s, want %s", cm.Namespace, tt.namespace)
			}
			if pvc.Namespace != tt.namespace {
				t.Errorf("PersistentVolumeClaim namespace = %s, want %s", pvc.Namespace, tt.namespace)
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

func TestPrometheusModule_PrepareServiceAccount(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	sa, _, _, _, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ServiceAccount name
	if sa.Name != "prometheus" {
		t.Errorf("ServiceAccount name = %s, want prometheus", sa.Name)
	}

	// Test ServiceAccount labels
	expectedLabels := map[string]string{
		"app":        "prometheus",
		"managed-by": "personal-server",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := sa.Labels[key]; !ok {
			t.Errorf("ServiceAccount missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("ServiceAccount label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}
}

func TestPrometheusModule_PrepareClusterRole(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	_, cr, _, _, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ClusterRole name
	if cr.Name != "prometheus" {
		t.Errorf("ClusterRole name = %s, want prometheus", cr.Name)
	}

	// Test ClusterRole rules
	if len(cr.Rules) != 3 {
		t.Fatalf("ClusterRole rules count = %d, want 3", len(cr.Rules))
	}

	// Test first rule (core resources)
	rule := cr.Rules[0]
	if len(rule.APIGroups) != 1 || rule.APIGroups[0] != "" {
		t.Errorf("ClusterRole rule[0] APIGroups = %v, want [\"\"]", rule.APIGroups)
	}
	expectedResources := []string{"nodes", "nodes/proxy", "services", "endpoints", "pods"}
	if len(rule.Resources) != len(expectedResources) {
		t.Errorf("ClusterRole rule[0] Resources count = %d, want %d", len(rule.Resources), len(expectedResources))
	}

	expectedVerbs := []string{"get", "list", "watch"}
	if len(rule.Verbs) != len(expectedVerbs) {
		t.Errorf("ClusterRole rule[0] Verbs count = %d, want %d", len(rule.Verbs), len(expectedVerbs))
	}
}

func TestPrometheusModule_PrepareClusterRoleBinding(t *testing.T) {
	testNamespace := "test-namespace"
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: testNamespace,
		},
	}

	_, _, crb, _, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ClusterRoleBinding name
	if crb.Name != "prometheus" {
		t.Errorf("ClusterRoleBinding name = %s, want prometheus", crb.Name)
	}

	// Test RoleRef
	if crb.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("ClusterRoleBinding RoleRef.APIGroup = %s, want rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
	}
	if crb.RoleRef.Kind != "ClusterRole" {
		t.Errorf("ClusterRoleBinding RoleRef.Kind = %s, want ClusterRole", crb.RoleRef.Kind)
	}
	if crb.RoleRef.Name != "prometheus" {
		t.Errorf("ClusterRoleBinding RoleRef.Name = %s, want prometheus", crb.RoleRef.Name)
	}

	// Test Subjects
	if len(crb.Subjects) != 1 {
		t.Fatalf("ClusterRoleBinding Subjects count = %d, want 1", len(crb.Subjects))
	}
	subject := crb.Subjects[0]
	if subject.Kind != "ServiceAccount" {
		t.Errorf("ClusterRoleBinding Subject.Kind = %s, want ServiceAccount", subject.Kind)
	}
	if subject.Name != "prometheus" {
		t.Errorf("ClusterRoleBinding Subject.Name = %s, want prometheus", subject.Name)
	}
	if subject.Namespace != testNamespace {
		t.Errorf("ClusterRoleBinding Subject.Namespace = %s, want %s", subject.Namespace, testNamespace)
	}
}

func TestPrometheusModule_PrepareConfigMap(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	_, _, _, cm, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ConfigMap name
	if cm.Name != "prometheus-config" {
		t.Errorf("ConfigMap name = %s, want prometheus-config", cm.Name)
	}

	// Test ConfigMap data
	if _, ok := cm.Data["prometheus.yml"]; !ok {
		t.Error("ConfigMap missing prometheus.yml key")
	}

	// Verify the configuration contains expected scrape configs
	prometheusConfig := cm.Data["prometheus.yml"]
	expectedConfigs := []string{
		"job_name: 'prometheus'",
		"job_name: 'kubernetes-apiservers'",
		"job_name: 'kubernetes-nodes'",
		"job_name: 'kubernetes-pods'",
		"job_name: 'kubernetes-service-endpoints'",
	}

	for _, expected := range expectedConfigs {
		if !contains(prometheusConfig, expected) {
			t.Errorf("ConfigMap prometheus.yml missing expected config: %s", expected)
		}
	}
}

func TestPrometheusModule_PreparePVC(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, pvc, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test PVC name
	if pvc.Name != "prometheus-data-pvc" {
		t.Errorf("PVC name = %s, want prometheus-data-pvc", pvc.Name)
	}

	// Test access modes
	if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
		t.Errorf("PVC access modes = %v, want [ReadWriteOnce]", pvc.Spec.AccessModes)
	}

	// Test storage size
	storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	expectedStorage := "10Gi"
	if storage.String() != expectedStorage {
		t.Errorf("PVC storage = %s, want %s", storage.String(), expectedStorage)
	}
}

func TestPrometheusModule_PrepareService(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, _, service, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Service name
	if service.Name != "prometheus" {
		t.Errorf("Service name = %s, want prometheus", service.Name)
	}

	// Test Service type
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("Service type = %s, want ClusterIP", service.Spec.Type)
	}

	// Test Service ports
	if len(service.Spec.Ports) != 1 {
		t.Fatalf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}

	port := service.Spec.Ports[0]
	if port.Name != "http" {
		t.Errorf("Service port name = %s, want http", port.Name)
	}
	if port.Port != 9090 {
		t.Errorf("Service port = %d, want 9090", port.Port)
	}
	if port.TargetPort.IntVal != 9090 {
		t.Errorf("Service targetPort = %d, want 9090", port.TargetPort.IntVal)
	}

	// Test selector
	if service.Spec.Selector["app"] != "prometheus" {
		t.Errorf("Service selector app = %s, want prometheus", service.Spec.Selector["app"])
	}
}

func TestPrometheusModule_PrepareDeployment(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Deployment name
	if deployment.Name != "prometheus" {
		t.Errorf("Deployment name = %s, want prometheus", deployment.Name)
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "prometheus" {
		t.Errorf("Deployment selector app = %s, want prometheus", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test pod template
	if deployment.Spec.Template.Spec.ServiceAccountName != "prometheus" {
		t.Errorf("Deployment ServiceAccountName = %s, want prometheus", deployment.Spec.Template.Spec.ServiceAccountName)
	}
}

func TestPrometheusModule_PrepareDeploymentContainer(t *testing.T) {
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "prometheus" {
		t.Errorf("Container name = %s, want prometheus", container.Name)
	}

	// Test container image
	expectedImage := "prom/prometheus:v2.48.0"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test image pull policy
	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("Container ImagePullPolicy = %s, want IfNotPresent", container.ImagePullPolicy)
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Fatalf("Container ports count = %d, want 1", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 9090 {
		t.Errorf("Container port = %d, want 9090", container.Ports[0].ContainerPort)
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 2 {
		t.Fatalf("Container volume mounts count = %d, want 2", len(container.VolumeMounts))
	}

	// Test liveness probe
	if container.LivenessProbe == nil {
		t.Error("Container missing liveness probe")
	} else {
		if container.LivenessProbe.HTTPGet.Path != "/-/healthy" {
			t.Errorf("Liveness probe path = %s, want /-/healthy", container.LivenessProbe.HTTPGet.Path)
		}
	}

	// Test readiness probe
	if container.ReadinessProbe == nil {
		t.Error("Container missing readiness probe")
	} else {
		if container.ReadinessProbe.HTTPGet.Path != "/-/ready" {
			t.Errorf("Readiness probe path = %s, want /-/ready", container.ReadinessProbe.HTTPGet.Path)
		}
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
	module := &PrometheusModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "prometheus",
			Namespace: "infra",
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify generated files exist
	expectedFiles := []string{
		"configs/prometheus/serviceaccount.yaml",
		"configs/prometheus/clusterrole.yaml",
		"configs/prometheus/clusterrolebinding.yaml",
		"configs/prometheus/configmap.yaml",
		"configs/prometheus/pvc.yaml",
		"configs/prometheus/service.yaml",
		"configs/prometheus/deployment.yaml",
	}

	for _, filename := range expectedFiles {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", filename)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && 
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
