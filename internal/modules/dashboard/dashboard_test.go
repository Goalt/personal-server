package dashboard

import (
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func TestDashboardModule_Name(t *testing.T) {
	module := &DashboardModule{}
	if module.Name() != "dashboard" {
		t.Errorf("Name() = %s, want dashboard", module.Name())
	}
}

func TestDashboardModule_Prepare(t *testing.T) {
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
			namespace: "kube-system",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &DashboardModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "dashboard",
					Namespace: tt.namespace,
					Secrets:   map[string]string{},
				},
			}

			sa, crb, service, deployment, err := module.prepare()

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
			if crb == nil {
				t.Error("prepare() returned nil ClusterRoleBinding")
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
			if service.Namespace != tt.namespace {
				t.Errorf("Service namespace = %s, want %s", service.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
		})
	}
}

func TestDashboardModule_PrepareServiceAccount(t *testing.T) {
	module := &DashboardModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "dashboard",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	sa, _, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ServiceAccount name
	if sa.Name != "kubernetes-dashboard" {
		t.Errorf("ServiceAccount name = %s, want kubernetes-dashboard", sa.Name)
	}

	// Test ServiceAccount labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"k8s-app":    "kubernetes-dashboard",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := sa.Labels[key]; !ok {
			t.Errorf("ServiceAccount missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("ServiceAccount label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}
}

func TestDashboardModule_PrepareClusterRoleBinding(t *testing.T) {
	testNamespace := "test-namespace"
	module := &DashboardModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "dashboard",
			Namespace: testNamespace,
			Secrets:   map[string]string{},
		},
	}

	_, crb, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test ClusterRoleBinding name
	if crb.Name != "kubernetes-dashboard" {
		t.Errorf("ClusterRoleBinding name = %s, want kubernetes-dashboard", crb.Name)
	}

	// Test RoleRef
	if crb.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("ClusterRoleBinding RoleRef.APIGroup = %s, want rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
	}
	if crb.RoleRef.Kind != "ClusterRole" {
		t.Errorf("ClusterRoleBinding RoleRef.Kind = %s, want ClusterRole", crb.RoleRef.Kind)
	}
	if crb.RoleRef.Name != "cluster-admin" {
		t.Errorf("ClusterRoleBinding RoleRef.Name = %s, want cluster-admin", crb.RoleRef.Name)
	}

	// Test Subjects
	if len(crb.Subjects) != 1 {
		t.Fatalf("ClusterRoleBinding Subjects count = %d, want 1", len(crb.Subjects))
	}
	subject := crb.Subjects[0]
	if subject.Kind != "ServiceAccount" {
		t.Errorf("ClusterRoleBinding Subject.Kind = %s, want ServiceAccount", subject.Kind)
	}
	if subject.Name != "kubernetes-dashboard" {
		t.Errorf("ClusterRoleBinding Subject.Name = %s, want kubernetes-dashboard", subject.Name)
	}
	if subject.Namespace != testNamespace {
		t.Errorf("ClusterRoleBinding Subject.Namespace = %s, want %s", subject.Namespace, testNamespace)
	}
}

func TestDashboardModule_PrepareService(t *testing.T) {
	module := &DashboardModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "dashboard",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, service, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Service name
	if service.Name != "kubernetes-dashboard" {
		t.Errorf("Service name = %s, want kubernetes-dashboard", service.Name)
	}

	// Test Service ports
	if len(service.Spec.Ports) != 1 {
		t.Fatalf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}

	port := service.Spec.Ports[0]
	if port.Name != "https" {
		t.Errorf("Service port name = %s, want https", port.Name)
	}
	if port.Port != 443 {
		t.Errorf("Service port = %d, want 443", port.Port)
	}
	if port.Protocol != corev1.ProtocolTCP {
		t.Errorf("Service protocol = %s, want TCP", port.Protocol)
	}

	// Test Service selector
	if service.Spec.Selector["k8s-app"] != "kubernetes-dashboard" {
		t.Errorf("Service selector k8s-app = %s, want kubernetes-dashboard", service.Spec.Selector["k8s-app"])
	}
}

func TestDashboardModule_PrepareDeployment(t *testing.T) {
	module := &DashboardModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "dashboard",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Deployment name
	if deployment.Name != "kubernetes-dashboard" {
		t.Errorf("Deployment name = %s, want kubernetes-dashboard", deployment.Name)
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["k8s-app"] != "kubernetes-dashboard" {
		t.Errorf("Deployment selector k8s-app = %s, want kubernetes-dashboard", deployment.Spec.Selector.MatchLabels["k8s-app"])
	}

	// Test pod template
	if deployment.Spec.Template.Spec.ServiceAccountName != "kubernetes-dashboard" {
		t.Errorf("Deployment ServiceAccountName = %s, want kubernetes-dashboard", deployment.Spec.Template.Spec.ServiceAccountName)
	}
}

func TestDashboardModule_PrepareDeploymentContainer(t *testing.T) {
	testNamespace := "test-namespace"
	module := &DashboardModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "dashboard",
			Namespace: testNamespace,
			Secrets:   map[string]string{},
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
	if container.Name != "kubernetes-dashboard" {
		t.Errorf("Container name = %s, want kubernetes-dashboard", container.Name)
	}

	// Test container image
	expectedImage := "kubernetesui/dashboard:v2.7.0"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test container args
	expectedArgs := []string{
		"--auto-generate-certificates",
		"--namespace=" + testNamespace,
	}
	if len(container.Args) != len(expectedArgs) {
		t.Fatalf("Container args count = %d, want %d", len(container.Args), len(expectedArgs))
	}
	for i, arg := range expectedArgs {
		if container.Args[i] != arg {
			t.Errorf("Container args[%d] = %s, want %s", i, container.Args[i], arg)
		}
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Fatalf("Container ports count = %d, want 1", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 8443 {
		t.Errorf("Container port = %d, want 8443", container.Ports[0].ContainerPort)
	}

	// Test liveness probe
	if container.LivenessProbe == nil {
		t.Fatal("Container liveness probe is nil")
	}
	if container.LivenessProbe.HTTPGet == nil {
		t.Fatal("Container liveness probe HTTPGet is nil")
	}
	if container.LivenessProbe.HTTPGet.Path != "/" {
		t.Errorf("Liveness probe path = %s, want /", container.LivenessProbe.HTTPGet.Path)
	}
	if container.LivenessProbe.HTTPGet.Scheme != corev1.URISchemeHTTPS {
		t.Errorf("Liveness probe scheme = %s, want HTTPS", container.LivenessProbe.HTTPGet.Scheme)
	}

	// Test security context
	if container.SecurityContext == nil {
		t.Fatal("Container security context is nil")
	}
	if container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation != false {
		t.Error("Container AllowPrivilegeEscalation should be false")
	}
	if container.SecurityContext.ReadOnlyRootFilesystem == nil || *container.SecurityContext.ReadOnlyRootFilesystem != true {
		t.Error("Container ReadOnlyRootFilesystem should be true")
	}
	if container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser != 1001 {
		t.Error("Container RunAsUser should be 1001")
	}
	if container.SecurityContext.RunAsGroup == nil || *container.SecurityContext.RunAsGroup != 2001 {
		t.Error("Container RunAsGroup should be 2001")
	}

	// Test volume mounts
	expectedVolumeMounts := map[string]string{
		"kubernetes-dashboard-certs": "/certs",
		"tmp-volume":                 "/tmp",
	}
	if len(container.VolumeMounts) != len(expectedVolumeMounts) {
		t.Errorf("Volume mounts count = %d, want %d", len(container.VolumeMounts), len(expectedVolumeMounts))
	}
	for _, vm := range container.VolumeMounts {
		if expectedPath, ok := expectedVolumeMounts[vm.Name]; ok {
			if vm.MountPath != expectedPath {
				t.Errorf("Volume mount %s path = %s, want %s", vm.Name, vm.MountPath, expectedPath)
			}
		} else {
			t.Errorf("Unexpected volume mount: %s", vm.Name)
		}
	}
}

func TestDashboardModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &DashboardModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "dashboard",
			Namespace: "test-namespace",
			Secrets:   map[string]string{},
		},
	}

	_, _, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test volumes
	expectedVolumes := []string{
		"kubernetes-dashboard-certs",
		"tmp-volume",
	}
	if len(deployment.Spec.Template.Spec.Volumes) != len(expectedVolumes) {
		t.Fatalf("Volumes count = %d, want %d", len(deployment.Spec.Template.Spec.Volumes), len(expectedVolumes))
	}

	for _, expectedName := range expectedVolumes {
		found := false
		for _, vol := range deployment.Spec.Template.Spec.Volumes {
			if vol.Name == expectedName {
				found = true
				// Verify it's an EmptyDir volume
				if vol.EmptyDir == nil {
					t.Errorf("Volume %s should be EmptyDir", expectedName)
				}
				break
			}
		}
		if !found {
			t.Errorf("Volume %s not found", expectedName)
		}
	}
}
