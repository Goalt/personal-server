package redis

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestRedisModule_Name(t *testing.T) {
	module := &RedisModule{}
	if module.Name() != "redis" {
		t.Errorf("Name() = %s, want redis", module.Name())
	}
}

func TestRedisModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		secrets   map[string]string
		wantErr   bool
	}{
		{
			name:      "valid configuration with password",
			namespace: "infra",
			secrets: map[string]string{
				"redis_password": "secret123",
			},
			wantErr: false,
		},
		{
			name:      "valid configuration without password",
			namespace: "infra",
			secrets:   map[string]string{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &RedisModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "redis",
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
			if secret.Labels["app"] != "redis" {
				t.Errorf("Secret label app = %s, want redis", secret.Labels["app"])
			}
			if pvc.Labels["app"] != "redis" {
				t.Errorf("PVC label app = %s, want redis", pvc.Labels["app"])
			}
			if service.Labels["app"] != "redis" {
				t.Errorf("Service label app = %s, want redis", service.Labels["app"])
			}
			if deployment.Labels["app"] != "redis" {
				t.Errorf("Deployment label app = %s, want redis", deployment.Labels["app"])
			}

			// Verify managed-by label
			if secret.Labels["managed-by"] != "personal-server" {
				t.Errorf("Secret label managed-by = %s, want personal-server", secret.Labels["managed-by"])
			}
		})
	}
}

func TestRedisModule_Generate(t *testing.T) {
	// Create a temporary directory for test outputs
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Change to temporary directory
	os.Chdir(tmpDir)

	module := &RedisModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "redis",
			Namespace: "infra",
			Secrets: map[string]string{
				"redis_password": "secret123",
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

	// Verify that config files were generated
	configsDir := filepath.Join(tmpDir, "configs", "redis")
	files := []string{"secret.yaml", "pvc.yaml", "service.yaml", "deployment.yaml"}

	for _, file := range files {
		path := filepath.Join(configsDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist, but it doesn't", file)
		}
	}
}
