package workpod

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestWorkPodModule_Name(t *testing.T) {
	module := &WorkPodModule{}
	if module.Name() != "workpod" {
		t.Errorf("Name() = %s, want workpod", module.Name())
	}
}

func TestWorkPodModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "default namespace",
			namespace: "default",
		},
		{
			name:      "custom namespace",
			namespace: "workloads",
		},
		{
			name:      "infra namespace",
			namespace: "infra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &WorkPodModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "workpod",
					Namespace: tt.namespace,
				},
			}

			pvc, deployment := module.prepare()

			// Verify PVC is not nil
			if pvc == nil {
				t.Fatal("prepare() returned nil PersistentVolumeClaim")
			}

			// Verify Deployment is not nil
			if deployment == nil {
				t.Fatal("prepare() returned nil Deployment")
			}

			// Verify PVC properties
			if pvc.Name != "work-storage-pvc" {
				t.Errorf("PVC name = %s, want work-storage-pvc", pvc.Name)
			}
			if pvc.Namespace != tt.namespace {
				t.Errorf("PVC namespace = %s, want %s", pvc.Namespace, tt.namespace)
			}
			if pvc.Labels["app"] != "work-pod" {
				t.Errorf("PVC label app = %s, want work-pod", pvc.Labels["app"])
			}

			// Verify PVC storage size
			expectedStorage := resource.MustParse("10Gi")
			actualStorage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if !actualStorage.Equal(expectedStorage) {
				t.Errorf("PVC storage = %s, want %s", actualStorage.String(), expectedStorage.String())
			}

			// Verify PVC access mode
			if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
				t.Errorf("PVC access modes = %v, want [ReadWriteOnce]", pvc.Spec.AccessModes)
			}

			// Verify Deployment properties
			if deployment.Name != "work-pod" {
				t.Errorf("Deployment name = %s, want work-pod", deployment.Name)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
			if deployment.Labels["app"] != "work-pod" {
				t.Errorf("Deployment label app = %s, want work-pod", deployment.Labels["app"])
			}

			// Verify Deployment replicas
			if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
				t.Errorf("Deployment replicas = %v, want 1", deployment.Spec.Replicas)
			}

			// Verify Deployment selector
			if deployment.Spec.Selector == nil {
				t.Fatal("Deployment selector is nil")
			}
			if deployment.Spec.Selector.MatchLabels["app"] != "work-pod" {
				t.Errorf("Deployment selector app = %s, want work-pod", deployment.Spec.Selector.MatchLabels["app"])
			}

			// Verify Pod template labels
			if deployment.Spec.Template.Labels["app"] != "work-pod" {
				t.Errorf("Pod template label app = %s, want work-pod", deployment.Spec.Template.Labels["app"])
			}

			// Verify container configuration
			containers := deployment.Spec.Template.Spec.Containers
			if len(containers) != 1 {
				t.Fatalf("Expected 1 container, got %d", len(containers))
			}

			container := containers[0]
			if container.Name != "debian" {
				t.Errorf("Container name = %s, want debian", container.Name)
			}
			if container.Image != "ghcr.io/goalt/workconfig:sha-afd12b4" {
				t.Errorf("Container image = %s, want ghcr.io/goalt/workconfig:sha-afd12b4", container.Image)
			}

			// Verify environment variables
			foundEnv := false
			for _, env := range container.Env {
				if env.Name == "DEBIAN_FRONTEND" && env.Value == "noninteractive" {
					foundEnv = true
					break
				}
			}
			if !foundEnv {
				t.Error("Container missing DEBIAN_FRONTEND=noninteractive env var")
			}

			// Verify volume mounts
			if len(container.VolumeMounts) != 1 {
				t.Fatalf("Expected 1 volume mount, got %d", len(container.VolumeMounts))
			}
			if container.VolumeMounts[0].Name != "work-storage" {
				t.Errorf("VolumeMount name = %s, want work-storage", container.VolumeMounts[0].Name)
			}
			if container.VolumeMounts[0].MountPath != "/data" {
				t.Errorf("VolumeMount mountPath = %s, want /data", container.VolumeMounts[0].MountPath)
			}

			// Verify security context
			if container.SecurityContext == nil {
				t.Fatal("Container SecurityContext is nil")
			}
			if container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
				t.Error("Container should be privileged")
			}
			if container.SecurityContext.RunAsNonRoot == nil || *container.SecurityContext.RunAsNonRoot {
				t.Error("Container RunAsNonRoot should be false")
			}
			if container.SecurityContext.AllowPrivilegeEscalation == nil || !*container.SecurityContext.AllowPrivilegeEscalation {
				t.Error("Container AllowPrivilegeEscalation should be true")
			}

			// Verify SYS_ADMIN capability
			if container.SecurityContext.Capabilities == nil {
				t.Fatal("Container Capabilities is nil")
			}
			foundCapability := false
			for _, cap := range container.SecurityContext.Capabilities.Add {
				if cap == "SYS_ADMIN" {
					foundCapability = true
					break
				}
			}
			if !foundCapability {
				t.Error("Container missing SYS_ADMIN capability")
			}

			// Verify volumes
			volumes := deployment.Spec.Template.Spec.Volumes
			if len(volumes) != 1 {
				t.Fatalf("Expected 1 volume, got %d", len(volumes))
			}
			if volumes[0].Name != "work-storage" {
				t.Errorf("Volume name = %s, want work-storage", volumes[0].Name)
			}
			if volumes[0].PersistentVolumeClaim == nil {
				t.Fatal("Volume PersistentVolumeClaim is nil")
			}
			if volumes[0].PersistentVolumeClaim.ClaimName != "work-storage-pvc" {
				t.Errorf("Volume PVC claim name = %s, want work-storage-pvc", volumes[0].PersistentVolumeClaim.ClaimName)
			}

			// Verify restart policy
			if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
				t.Errorf("RestartPolicy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
			}
		})
	}
}

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

	module := &WorkPodModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "workpod",
			Namespace: "hobby",
		},
		log: logger.Default(),
	}

	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	expectedFiles := []string{
		"configs/workpod/pvc.yaml",
		"configs/workpod/deployment.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(tempDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("expected file %s was not generated", file)
		}
	}
}
