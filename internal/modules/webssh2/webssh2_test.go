package webssh2

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSSH2Module_Name(t *testing.T) {
	module := &WebSSH2Module{}
	assert.Equal(t, "webssh2", module.Name())
}

func TestWebSSH2Module_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		secrets   map[string]string
		wantErr   bool
	}{
		{
			name:      "valid configuration with custom settings",
			namespace: "infra",
			secrets: map[string]string{
				"header_text":  "My SSH Client",
				"ssh_host":     "ssh.example.com",
				"auth_allowed": "password,publickey",
			},
			wantErr: false,
		},
		{
			name:      "valid configuration with defaults",
			namespace: "infra",
			secrets:   map[string]string{},
			wantErr:   false,
		},
		{
			name:      "valid configuration with custom image",
			namespace: "infra",
			secrets: map[string]string{
				"image": "ghcr.io/billchurch/webssh2:2.3.2",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &WebSSH2Module{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "webssh2",
					Namespace: tt.namespace,
					Secrets:   tt.secrets,
				},
				log: logger.NewNopLogger(),
			}

			configMap, service, deployment := module.prepare()

			// Verify ConfigMap
			assert.NotNil(t, configMap)
			assert.Equal(t, "webssh2-config", configMap.Name)
			assert.Equal(t, tt.namespace, configMap.Namespace)
			assert.NotEmpty(t, configMap.Data)

			// Verify Service
			assert.NotNil(t, service)
			assert.Equal(t, "webssh2", service.Name)
			assert.Equal(t, tt.namespace, service.Namespace)
			assert.Len(t, service.Spec.Ports, 1)
			assert.Equal(t, int32(2222), service.Spec.Ports[0].Port)

			// Verify Deployment
			assert.NotNil(t, deployment)
			assert.Equal(t, "webssh2", deployment.Name)
			assert.Equal(t, tt.namespace, deployment.Namespace)
			assert.Equal(t, int32(1), *deployment.Spec.Replicas)
			assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)

			container := deployment.Spec.Template.Spec.Containers[0]
			assert.Equal(t, "webssh2", container.Name)
			
			// Verify image
			if tt.secrets["image"] != "" {
				assert.Equal(t, tt.secrets["image"], container.Image)
			} else {
				assert.Equal(t, "ghcr.io/billchurch/webssh2:latest", container.Image)
			}

			// Verify environment
			assert.Len(t, container.EnvFrom, 1)
			assert.Equal(t, "webssh2-config", container.EnvFrom[0].ConfigMapRef.Name)

			// Verify probes
			assert.NotNil(t, container.LivenessProbe)
			assert.NotNil(t, container.ReadinessProbe)
		})
	}
}

func TestWebSSH2Module_Generate(t *testing.T) {
	// Create a temporary directory for test output
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	module := &WebSSH2Module{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webssh2",
			Namespace: "infra",
			Secrets: map[string]string{
				"header_text": "Test SSH",
			},
		},
		log: logger.NewNopLogger(),
	}

	ctx := context.Background()
	err = module.Generate(ctx)
	require.NoError(t, err)

	// Verify generated files exist
	outputDir := filepath.Join(tempDir, "configs", "webssh2")
	files := []string{
		"configmap.yaml",
		"service.yaml",
		"deployment.yaml",
	}

	for _, file := range files {
		path := filepath.Join(outputDir, file)
		assert.FileExists(t, path, "Expected file %s to exist", file)

		// Verify file is not empty
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Greater(t, info.Size(), int64(0), "File %s should not be empty", file)
	}
}

func TestWebSSH2Module_PrepareWithDefaults(t *testing.T) {
	module := &WebSSH2Module{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webssh2",
			Namespace: "infra",
			Secrets:   map[string]string{},
		},
		log: logger.NewNopLogger(),
	}

	configMap, _, deployment := module.prepare()

	// Verify default values
	assert.Equal(t, "WebSSH2", configMap.Data["WEBSSH2_HEADER_TEXT"])
	assert.Equal(t, "password,publickey,keyboard-interactive", configMap.Data["WEBSSH2_AUTH_ALLOWED"])
	assert.Equal(t, "2222", configMap.Data["WEBSSH2_LISTEN_PORT"])
	assert.Equal(t, "ghcr.io/billchurch/webssh2:latest", deployment.Spec.Template.Spec.Containers[0].Image)
}

func TestWebSSH2Module_PrepareWithCustomValues(t *testing.T) {
	module := &WebSSH2Module{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "webssh2",
			Namespace: "infra",
			Secrets: map[string]string{
				"header_text":  "Custom SSH",
				"ssh_host":     "ssh.custom.com",
				"auth_allowed": "password",
				"listen_port":  "3333",
				"image":        "ghcr.io/billchurch/webssh2:2.3.2",
			},
		},
		log: logger.NewNopLogger(),
	}

	configMap, service, deployment := module.prepare()

	// Verify custom values
	assert.Equal(t, "Custom SSH", configMap.Data["WEBSSH2_HEADER_TEXT"])
	assert.Equal(t, "ssh.custom.com", configMap.Data["WEBSSH2_SSH_HOST"])
	assert.Equal(t, "password", configMap.Data["WEBSSH2_AUTH_ALLOWED"])
	assert.Equal(t, "3333", configMap.Data["WEBSSH2_LISTEN_PORT"])
	assert.Equal(t, "ghcr.io/billchurch/webssh2:2.3.2", deployment.Spec.Template.Spec.Containers[0].Image)
	
	// Service port should still be 2222 (default WebSSH2 port)
	assert.Equal(t, int32(2222), service.Spec.Ports[0].Port)
}
