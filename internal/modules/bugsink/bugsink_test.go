package bugsink

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestBugsinkModule_Name(t *testing.T) {
	module := &BugsinkModule{}
	if module.Name() != "bugsink" {
		t.Errorf("Name() = %s, want bugsink", module.Name())
	}
}

func TestBugsinkModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		secrets   map[string]string
		wantErr   bool
	}{
		{
			name:      "valid configuration with all secrets",
			namespace: "infra",
			secrets: map[string]string{
				"bugsink_secret_key":     "supersecretkey",
				"bugsink_admin_user":     "admin",
				"bugsink_admin_password": "secret123",
			},
		},
		{
			name:      "valid configuration with empty secrets uses defaults",
			namespace: "infra",
			secrets:   map[string]string{},
		},
		{
			name:      "valid configuration with custom image",
			namespace: "infra",
			secrets: map[string]string{
				"bugsink_image":          "bugsink/bugsink:1.0.0",
				"bugsink_secret_key":     "supersecretkey",
				"bugsink_admin_user":     "myadmin",
				"bugsink_admin_password": "mypassword",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &BugsinkModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "bugsink",
					Namespace: tt.namespace,
					Secrets:   tt.secrets,
				},
			}

			secret, pvc, service, deployment := module.prepare()

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

			// Verify labels
			for _, obj := range []map[string]string{secret.Labels, pvc.Labels, service.Labels, deployment.Labels} {
				if obj["app"] != appName {
					t.Errorf("label app = %s, want %s", obj["app"], appName)
				}
				if obj["managed-by"] != "personal-server" {
					t.Errorf("label managed-by = %s, want personal-server", obj["managed-by"])
				}
			}

			// Verify secret keys
			if _, ok := secret.Data["secret-key"]; !ok {
				t.Error("Secret is missing 'secret-key' key")
			}
			if _, ok := secret.Data["create-superuser"]; !ok {
				t.Error("Secret is missing 'create-superuser' key")
			}

			// Verify service port
			if len(service.Spec.Ports) != 1 {
				t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
			} else if service.Spec.Ports[0].Port != servicePort {
				t.Errorf("Service port = %d, want %d", service.Spec.Ports[0].Port, servicePort)
			}

			// Verify deployment replicas
			if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
				t.Error("Deployment replicas should be 1")
			}

			// Verify volume mount
			if len(deployment.Spec.Template.Spec.Volumes) != 1 {
				t.Errorf("Deployment volumes count = %d, want 1", len(deployment.Spec.Template.Spec.Volumes))
			}

			// Verify container has correct image when custom image is set
			if img, ok := tt.secrets["bugsink_image"]; ok {
				if deployment.Spec.Template.Spec.Containers[0].Image != img {
					t.Errorf("Container image = %s, want %s", deployment.Spec.Template.Spec.Containers[0].Image, img)
				}
			}

			// Verify create-superuser format (user:password)
			user := tt.secrets["bugsink_admin_user"]
			password := tt.secrets["bugsink_admin_password"]
			if user == "" {
				user = "admin"
			}
			if password == "" {
				password = "admin"
			}
			expectedSuperuser := user + ":" + password
			if string(secret.Data["create-superuser"]) != expectedSuperuser {
				t.Errorf("create-superuser = %s, want %s", secret.Data["create-superuser"], expectedSuperuser)
			}
		})
	}
}

func TestBugsinkModule_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	os.Chdir(tmpDir)

	module := &BugsinkModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "bugsink",
			Namespace: "infra",
			Secrets: map[string]string{
				"bugsink_secret_key":     "supersecretkey",
				"bugsink_admin_user":     "admin",
				"bugsink_admin_password": "secret123",
			},
		},
		log: logger.NewNopLogger(),
	}

	ctx := context.Background()
	err := module.Generate(ctx)
	if err != nil {
		t.Errorf("Generate() unexpected error: %v", err)
		return
	}

	configsDir := filepath.Join(tmpDir, "configs", "bugsink")
	files := []string{"secret.yaml", "pvc.yaml", "service.yaml", "deployment.yaml"}

	for _, file := range files {
		path := filepath.Join(configsDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist, but it doesn't", file)
		}
	}
}
