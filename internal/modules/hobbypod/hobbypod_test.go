package hobbypod

import (
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestHobbyPodModule_Name(t *testing.T) {
	module := &HobbyPodModule{}
	if module.Name() != "hobby-pod" {
		t.Errorf("Name() = %s, want hobby-pod", module.Name())
	}
}

func TestHobbyPodModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "default namespace",
			namespace: "hobby",
		},
		{
			name:      "custom namespace",
			namespace: "custom-hobby",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &HobbyPodModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "hobby-pod",
					Namespace: tt.namespace,
				},
			}

			pvc, deployment := module.prepare()

			// Verify all objects are not nil
			if pvc == nil {
				t.Fatal("prepare() returned nil PVC")
			}
			if deployment == nil {
				t.Fatal("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly
			if pvc.Namespace != tt.namespace {
				t.Errorf("PVC namespace = %s, want %s", pvc.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
		})
	}
}

func TestHobbyPodModule_PreparePVC(t *testing.T) {
	module := &HobbyPodModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "hobby-pod",
			Namespace: "test-namespace",
		},
	}

	pvc, _ := module.prepare()

	// Test PVC name
	if pvc.Name != "hobby-storage-pvc" {
		t.Errorf("PVC name = %s, want hobby-storage-pvc", pvc.Name)
	}

	// Test PVC labels
	expectedLabels := map[string]string{
		"app":        "hobby-pod",
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

	// Test storage request
	expectedStorage := resource.MustParse("10Gi")
	actualStorage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if actualStorage.Cmp(expectedStorage) != 0 {
		t.Errorf("PVC storage request = %s, want %s", actualStorage.String(), expectedStorage.String())
	}
}

func TestHobbyPodModule_PrepareDeployment(t *testing.T) {
	module := &HobbyPodModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "hobby-pod",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare()

	// Test Deployment name
	if deployment.Name != "hobby-pod" {
		t.Errorf("Deployment name = %s, want hobby-pod", deployment.Name)
	}

	// Test labels
	expectedLabels := map[string]string{
		"app":        "hobby-pod",
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
	if deployment.Spec.Selector.MatchLabels["app"] != "hobby-pod" {
		t.Errorf("Deployment selector app = %s, want hobby-pod", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test restart policy
	if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("Pod restart policy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
	}
}

func TestHobbyPodModule_PrepareDeploymentContainer(t *testing.T) {
	module := &HobbyPodModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "hobby-pod",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare()

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "hobby" {
		t.Errorf("Container name = %s, want hobby", container.Name)
	}

	// Test container image
	expectedImage := "ghcr.io/goalt/workconfig:sha-595cb6c"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test environment variables
	debianFrontendFound := false
	for _, env := range container.Env {
		if env.Name == "DEBIAN_FRONTEND" {
			debianFrontendFound = true
			if env.Value != "noninteractive" {
				t.Errorf("DEBIAN_FRONTEND = %s, want noninteractive", env.Value)
			}
		}
	}
	if !debianFrontendFound {
		t.Error("Container missing DEBIAN_FRONTEND env var")
	}

	// Test volume mounts
	if len(container.VolumeMounts) != 1 {
		t.Errorf("Container volume mounts count = %d, want 1", len(container.VolumeMounts))
	}

	volumeMount := container.VolumeMounts[0]
	if volumeMount.Name != "hobby-storage" {
		t.Errorf("VolumeMount name = %s, want hobby-storage", volumeMount.Name)
	}
	if volumeMount.MountPath != "/data" {
		t.Errorf("VolumeMount mountPath = %s, want /data", volumeMount.MountPath)
	}
}

func TestHobbyPodModule_PrepareDeploymentContainerSecurityContext(t *testing.T) {
	module := &HobbyPodModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "hobby-pod",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare()

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test security context
	if container.SecurityContext == nil {
		t.Fatal("Container SecurityContext is nil")
	}

	sc := container.SecurityContext

	// Test RunAsNonRoot
	if sc.RunAsNonRoot == nil {
		t.Error("RunAsNonRoot is nil")
	} else if *sc.RunAsNonRoot != false {
		t.Errorf("RunAsNonRoot = %v, want false", *sc.RunAsNonRoot)
	}

	// Test AllowPrivilegeEscalation
	if sc.AllowPrivilegeEscalation == nil {
		t.Error("AllowPrivilegeEscalation is nil")
	} else if *sc.AllowPrivilegeEscalation != true {
		t.Errorf("AllowPrivilegeEscalation = %v, want true", *sc.AllowPrivilegeEscalation)
	}

	// Test Privileged
	if sc.Privileged == nil {
		t.Error("Privileged is nil")
	} else if *sc.Privileged != true {
		t.Errorf("Privileged = %v, want true", *sc.Privileged)
	}

	// Test Capabilities
	if sc.Capabilities == nil {
		t.Fatal("Capabilities is nil")
	}
	if len(sc.Capabilities.Add) != 1 {
		t.Errorf("Capabilities.Add count = %d, want 1", len(sc.Capabilities.Add))
	}
	if sc.Capabilities.Add[0] != "SYS_ADMIN" {
		t.Errorf("Capabilities.Add[0] = %s, want SYS_ADMIN", sc.Capabilities.Add[0])
	}
}

func TestHobbyPodModule_PrepareDeploymentVolumes(t *testing.T) {
	module := &HobbyPodModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "hobby-pod",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare()

	// Test volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("Volumes count = %d, want 1", len(deployment.Spec.Template.Spec.Volumes))
	}

	volume := deployment.Spec.Template.Spec.Volumes[0]
	if volume.Name != "hobby-storage" {
		t.Errorf("Volume name = %s, want hobby-storage", volume.Name)
	}

	if volume.PersistentVolumeClaim == nil {
		t.Fatal("Volume PersistentVolumeClaim is nil")
	}

	if volume.PersistentVolumeClaim.ClaimName != "hobby-storage-pvc" {
		t.Errorf("Volume PVC claim name = %s, want hobby-storage-pvc", volume.PersistentVolumeClaim.ClaimName)
	}
}
