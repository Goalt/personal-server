package openreplay

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestOpenReplayModule_Name(t *testing.T) {
	module := &OpenReplayModule{}
	if module.Name() != "openreplay" {
		t.Errorf("Name() = %s, want openreplay", module.Name())
	}
}

func TestOpenReplayModule_Prepare(t *testing.T) {
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
				"domain_name": "openreplay.example.com",
				"pg_password": "secret123",
				"jwt_secret":  "my-jwt-secret",
			},
			wantErr: false,
		},
		{
			name:      "valid configuration without secrets uses defaults",
			namespace: "infra",
			secrets:   map[string]string{},
			wantErr:   false,
		},
		{
			name:      "valid configuration with partial secrets",
			namespace: "infra",
			secrets: map[string]string{
				"pg_password": "secret123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &OpenReplayModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "openreplay",
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

			// Verify labels
			if secret.Labels["app"] != "openreplay" {
				t.Errorf("Secret label app = %s, want openreplay", secret.Labels["app"])
			}
			if pvc.Labels["app"] != "openreplay" {
				t.Errorf("PVC label app = %s, want openreplay", pvc.Labels["app"])
			}
			if service.Labels["app"] != "openreplay" {
				t.Errorf("Service label app = %s, want openreplay", service.Labels["app"])
			}
			if deployment.Labels["app"] != "openreplay" {
				t.Errorf("Deployment label app = %s, want openreplay", deployment.Labels["app"])
			}

			// Verify managed-by label
			if secret.Labels["managed-by"] != "personal-server" {
				t.Errorf("Secret label managed-by = %s, want personal-server", secret.Labels["managed-by"])
			}

			// Verify service port
			if len(service.Spec.Ports) != 1 {
				t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
			} else if service.Spec.Ports[0].Port != 80 {
				t.Errorf("Service port = %d, want 80", service.Spec.Ports[0].Port)
			}

			// Verify secret data keys
			if _, ok := secret.Data["domain-name"]; !ok {
				t.Error("Secret missing key 'domain-name'")
			}
			if _, ok := secret.Data["pg-password"]; !ok {
				t.Error("Secret missing key 'pg-password'")
			}
			if _, ok := secret.Data["jwt-secret"]; !ok {
				t.Error("Secret missing key 'jwt-secret'")
			}

			// Verify deployment uses domain from GeneralConfig when no secret provided
			if len(tt.secrets) == 0 {
				domainValue := string(secret.Data["domain-name"])
				if domainValue != "example.com" {
					t.Errorf("domain-name = %s, want example.com (from GeneralConfig)", domainValue)
				}
			}
		})
	}
}

func TestOpenReplayModule_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	os.Chdir(tmpDir)

	module := &OpenReplayModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "openreplay",
			Namespace: "infra",
			Secrets: map[string]string{
				"domain_name": "openreplay.example.com",
				"pg_password": "secret123",
				"jwt_secret":  "my-jwt-secret",
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

	configsDir := filepath.Join(tmpDir, "configs", "openreplay")
	files := []string{"secret.yaml", "pvc.yaml", "service.yaml", "deployment.yaml"}

	for _, file := range files {
		path := filepath.Join(configsDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist, but it doesn't", file)
		}
	}
}
