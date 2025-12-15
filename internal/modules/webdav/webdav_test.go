package webdav

import (
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestWebdavModule_Name(t *testing.T) {
	module := &WebdavModule{}
	if module.Name() != "webdav" {
		t.Errorf("Name() = %s, want webdav", module.Name())
	}
}

func TestWebdavModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "default configuration",
			namespace: "infra",
		},
		{
			name:      "custom namespace",
			namespace: "storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &WebdavModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "webdav",
					Namespace: tt.namespace,
				},
			}

			configMap, secret, pvc, service, deployment := module.prepare()

			// Verify all objects are not nil
			if configMap == nil {
				t.Fatal("prepare() returned nil ConfigMap")
			}
			if secret == nil {
				t.Fatal("prepare() returned nil Secret")
			}
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
			if configMap.Namespace != tt.namespace {
				t.Errorf("ConfigMap namespace = %s, want %s", configMap.Namespace, tt.namespace)
			}
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

func TestWebdavModule_PrepareConfigMap(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	configMap, _, _, _, _ := module.prepare()

	// Test ConfigMap name
	if configMap.Name != "webdav-config" {
		t.Errorf("ConfigMap name = %s, want webdav-config", configMap.Name)
	}

	// Test ConfigMap labels
	expectedLabels := map[string]string{
		"app":        "webdav",
		"managed-by": "personal-server",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := configMap.Labels[key]; !ok {
			t.Errorf("ConfigMap missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("ConfigMap label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test ConfigMap data contains config.yaml
	if _, ok := configMap.Data["config.yaml"]; !ok {
		t.Error("ConfigMap missing config.yaml key")
	}

	// Verify config.yaml contains expected settings
	configYaml := configMap.Data["config.yaml"]
	if !strings.Contains(configYaml, "port: 8080") {
		t.Error("config.yaml missing port: 8080")
	}
	if !strings.Contains(configYaml, "directory: /data") {
		t.Error("config.yaml missing directory: /data")
	}
	if !strings.Contains(configYaml, "permissions: CRUD") {
		t.Error("config.yaml missing permissions: CRUD")
	}
	if !strings.Contains(configYaml, "behindProxy: true") {
		t.Error("config.yaml missing behindProxy: true")
	}
}

func TestWebdavModule_PrepareSecret(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"webdav_username": "testuser",
				"webdav_password": "testpass123",
			},
		},
	}

	_, secret, _, _, _ := module.prepare()

	// Test Secret name
	if secret.Name != "webdav-secrets" {
		t.Errorf("Secret name = %s, want webdav-secrets", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret labels
	expectedLabels := map[string]string{
		"app":        "webdav",
		"managed-by": "personal-server",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := secret.Labels[key]; !ok {
			t.Errorf("Secret missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Secret label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test Secret data
	if secret.StringData["webdav_username"] != "testuser" {
		t.Errorf("Secret webdav_username = %s, want testuser", secret.StringData["webdav_username"])
	}
	if secret.StringData["webdav_password"] != "testpass123" {
		t.Errorf("Secret webdav_password = %s, want testpass123", secret.StringData["webdav_password"])
	}
}

func TestWebdavModule_PrepareSecretDefaults(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
			Secrets:   map[string]string{}, // Empty secrets to test defaults
		},
	}

	_, secret, _, _, _ := module.prepare()

	// Test default values are used
	if secret.StringData["webdav_username"] != "admin" {
		t.Errorf("Secret webdav_username default = %s, want admin", secret.StringData["webdav_username"])
	}
	if secret.StringData["webdav_password"] != "abc" {
		t.Errorf("Secret webdav_password default = %s, want abc", secret.StringData["webdav_password"])
	}
}

func TestWebdavModule_PreparePVC(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	_, _, pvc, _, _ := module.prepare()

	// Test PVC name
	if pvc.Name != "webdav-data-pvc" {
		t.Errorf("PVC name = %s, want webdav-data-pvc", pvc.Name)
	}

	// Test PVC labels
	expectedLabels := map[string]string{
		"app":        "webdav",
		"managed-by": "personal-server",
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

	// Test storage request - 20Gi
	expectedStorage := resource.MustParse("20Gi")
	actualStorage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if actualStorage.Cmp(expectedStorage) != 0 {
		t.Errorf("PVC storage request = %s, want %s", actualStorage.String(), expectedStorage.String())
	}
}

func TestWebdavModule_PrepareService(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	_, _, _, service, _ := module.prepare()

	// Test Service name
	if service.Name != "webdav-service" {
		t.Errorf("Service name = %s, want webdav-service", service.Name)
	}

	// Test Service type
	if service.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("Service type = %s, want ClusterIP", service.Spec.Type)
	}

	// Test Service labels
	expectedLabels := map[string]string{
		"app":        "webdav",
		"managed-by": "personal-server",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := service.Labels[key]; !ok {
			t.Errorf("Service missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Service label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test selector
	if service.Spec.Selector["app"] != "webdav" {
		t.Errorf("Service selector app = %s, want webdav", service.Spec.Selector["app"])
	}

	// Test Service ports
	if len(service.Spec.Ports) != 1 {
		t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}

	port := service.Spec.Ports[0]
	if port.Name != "http" {
		t.Errorf("Service port name = %s, want http", port.Name)
	}
	if port.Port != 8080 {
		t.Errorf("Service port = %d, want 8080", port.Port)
	}
	if port.TargetPort.IntVal != 8080 {
		t.Errorf("Service targetPort = %d, want 8080", port.TargetPort.IntVal)
	}
	if port.Protocol != corev1.ProtocolTCP {
		t.Errorf("Service port protocol = %s, want TCP", port.Protocol)
	}
}

func TestWebdavModule_PrepareDeployment(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, deployment := module.prepare()

	// Test Deployment name
	if deployment.Name != "webdav" {
		t.Errorf("Deployment name = %s, want webdav", deployment.Name)
	}

	// Test Deployment labels
	expectedLabels := map[string]string{
		"app":        "webdav",
		"managed-by": "personal-server",
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

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "webdav" {
		t.Errorf("Deployment selector app = %s, want webdav", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test restart policy
	if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("Pod restart policy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
	}

	// Test pod security context
	podSecurityContext := deployment.Spec.Template.Spec.SecurityContext
	if podSecurityContext == nil {
		t.Fatal("PodSecurityContext is nil")
	}
	if podSecurityContext.FSGroup == nil || *podSecurityContext.FSGroup != 1000 {
		t.Error("PodSecurityContext FSGroup should be 1000")
	}
}

func TestWebdavModule_PrepareDeploymentContainer(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, deployment := module.prepare()

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "webdav" {
		t.Errorf("Container name = %s, want webdav", container.Name)
	}

	// Test container image
	if container.Image != "ghcr.io/hacdias/webdav:latest" {
		t.Errorf("Container image = %s, want ghcr.io/hacdias/webdav:latest", container.Image)
	}

	// Test container args
	expectedArgs := []string{"-c", "/config/config.yaml"}
	if len(container.Args) != len(expectedArgs) {
		t.Errorf("Container args length = %d, want %d", len(container.Args), len(expectedArgs))
	} else {
		for i, arg := range expectedArgs {
			if container.Args[i] != arg {
				t.Errorf("Container args[%d] = %s, want %s", i, container.Args[i], arg)
			}
		}
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Errorf("Container ports count = %d, want 1", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 8080 {
		t.Errorf("Container port = %d, want 8080", container.Ports[0].ContainerPort)
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 2 {
		t.Errorf("Container volume mounts count = %d, want 2", len(container.VolumeMounts))
	}

	configMountFound := false
	dataMountFound := false
	for _, vm := range container.VolumeMounts {
		if vm.Name == "webdav-config" && vm.MountPath == "/config" && vm.ReadOnly {
			configMountFound = true
		}
		if vm.Name == "webdav-data" && vm.MountPath == "/data" {
			dataMountFound = true
		}
	}
	if !configMountFound {
		t.Error("Container missing webdav-config volume mount")
	}
	if !dataMountFound {
		t.Error("Container missing webdav-data volume mount")
	}

	// Test environment variables
	envNames := make(map[string]bool)
	for _, env := range container.Env {
		envNames[env.Name] = true
		if env.Name == "WEBDAV_USERNAME" || env.Name == "WEBDAV_PASSWORD" {
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Errorf("%s should use SecretKeyRef", env.Name)
			} else if env.ValueFrom.SecretKeyRef.Name != "webdav-secrets" {
				t.Errorf("%s secret name = %s, want webdav-secrets", env.Name, env.ValueFrom.SecretKeyRef.Name)
			}
		}
	}
	if !envNames["WEBDAV_USERNAME"] {
		t.Error("Container missing WEBDAV_USERNAME env var")
	}
	if !envNames["WEBDAV_PASSWORD"] {
		t.Error("Container missing WEBDAV_PASSWORD env var")
	}
}

func TestWebdavModule_PrepareDeploymentContainerSecurityContext(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, deployment := module.prepare()

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test security context
	if container.SecurityContext == nil {
		t.Fatal("Container SecurityContext is nil")
	}

	sc := container.SecurityContext

	// Test RunAsNonRoot
	if sc.RunAsNonRoot == nil || *sc.RunAsNonRoot != true {
		t.Error("RunAsNonRoot should be true")
	}

	// Test RunAsUser
	if sc.RunAsUser == nil || *sc.RunAsUser != 1000 {
		t.Error("RunAsUser should be 1000")
	}

	// Test RunAsGroup
	if sc.RunAsGroup == nil || *sc.RunAsGroup != 1000 {
		t.Error("RunAsGroup should be 1000")
	}

	// Test AllowPrivilegeEscalation
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation != false {
		t.Error("AllowPrivilegeEscalation should be false")
	}

	// Test ReadOnlyRootFilesystem
	if sc.ReadOnlyRootFilesystem == nil || *sc.ReadOnlyRootFilesystem != true {
		t.Error("ReadOnlyRootFilesystem should be true")
	}

	// Test Capabilities
	if sc.Capabilities == nil {
		t.Fatal("Capabilities is nil")
	}
	if len(sc.Capabilities.Drop) != 1 {
		t.Errorf("Capabilities.Drop count = %d, want 1", len(sc.Capabilities.Drop))
	}
	if sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("Capabilities.Drop[0] = %s, want ALL", sc.Capabilities.Drop[0])
	}
}

func TestWebdavModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &WebdavModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webdav",
			Namespace: "test-namespace",
		},
	}

	_, _, _, _, deployment := module.prepare()

	// Test volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 2 {
		t.Fatalf("Volumes count = %d, want 2", len(deployment.Spec.Template.Spec.Volumes))
	}

	configVolumeFound := false
	dataVolumeFound := false
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == "webdav-config" {
			configVolumeFound = true
			if volume.ConfigMap == nil {
				t.Error("webdav-config volume should use ConfigMap")
			} else if volume.ConfigMap.Name != "webdav-config" {
				t.Errorf("webdav-config ConfigMap name = %s, want webdav-config", volume.ConfigMap.Name)
			}
		}
		if volume.Name == "webdav-data" {
			dataVolumeFound = true
			if volume.PersistentVolumeClaim == nil {
				t.Error("webdav-data volume should use PersistentVolumeClaim")
			} else if volume.PersistentVolumeClaim.ClaimName != "webdav-data-pvc" {
				t.Errorf("webdav-data PVC claim name = %s, want webdav-data-pvc", volume.PersistentVolumeClaim.ClaimName)
			}
		}
	}
	if !configVolumeFound {
		t.Error("Deployment missing webdav-config volume")
	}
	if !dataVolumeFound {
		t.Error("Deployment missing webdav-data volume")
	}
}
