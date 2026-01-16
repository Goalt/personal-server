package postgresexporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"infra"},
	}
	moduleConfig := config.Module{
		Name:      "postgres-exporter",
		Namespace: "infra",
		Secrets:   map[string]string{},
	}
	log := logger.Default()

	module := New(generalConfig, moduleConfig, log)

	assert.NotNil(t, module)
	assert.Equal(t, "postgres-exporter", module.Name())
	assert.Equal(t, generalConfig, module.GeneralConfig)
	assert.Equal(t, moduleConfig, module.ModuleConfig)
}

func TestPrepare(t *testing.T) {
	tests := []struct {
		name         string
		secrets      map[string]string
		wantEnvCount int
		checkEnvVars map[string]string
	}{
		{
			name: "with all configuration",
			secrets: map[string]string{
				"data_source_uri":   "custom-db:5432/mydb?sslmode=require",
				"data_source_user":  "customuser",
				"data_source_pass":  "custompass",
				"extend_query_path": "/custom/path",
				"include_databases": "db1,db2",
			},
			wantEnvCount: 5,
			checkEnvVars: map[string]string{
				"DATA_SOURCE_URI":               "custom-db:5432/mydb?sslmode=require",
				"DATA_SOURCE_USER":              "customuser",
				"DATA_SOURCE_PASS":              "custompass",
				"PG_EXPORTER_EXTEND_QUERY_PATH": "/custom/path",
				"PG_EXPORTER_INCLUDE_DATABASES": "db1,db2",
			},
		},
		{
			name:         "with default configuration",
			secrets:      map[string]string{},
			wantEnvCount: 4,
			checkEnvVars: map[string]string{
				"DATA_SOURCE_URI":               "postgres:5432/postgres?sslmode=disable",
				"DATA_SOURCE_USER":              "postgres",
				"DATA_SOURCE_PASS":              "postgres",
				"PG_EXPORTER_INCLUDE_DATABASES": "postgres",
			},
		},
		{
			name: "with partial configuration",
			secrets: map[string]string{
				"data_source_uri": "mydb:5432/testdb",
			},
			wantEnvCount: 4,
			checkEnvVars: map[string]string{
				"DATA_SOURCE_URI":               "mydb:5432/testdb",
				"DATA_SOURCE_USER":              "postgres",
				"DATA_SOURCE_PASS":              "postgres",
				"PG_EXPORTER_INCLUDE_DATABASES": "postgres",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generalConfig := config.GeneralConfig{
				Domain:     "example.com",
				Namespaces: []string{"infra"},
			}
			moduleConfig := config.Module{
				Name:      "postgres-exporter",
				Namespace: "infra",
				Secrets:   tt.secrets,
			}
			log := logger.Default()

			module := New(generalConfig, moduleConfig, log)
			deployment, err := module.prepare()

			require.NoError(t, err)
			require.NotNil(t, deployment)

			// Check basic deployment properties
			assert.Equal(t, "postgres-exporter", deployment.Name)
			assert.Equal(t, "infra", deployment.Namespace)
			assert.Equal(t, int32(1), *deployment.Spec.Replicas)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)

			// Check labels
			assert.Equal(t, "postgres-exporter", deployment.Labels["app"])
			assert.Equal(t, "postgres-exporter", deployment.Spec.Selector.MatchLabels["app"])
			assert.Equal(t, "postgres-exporter", deployment.Spec.Template.Labels["app"])

			// Check Prometheus annotations
			assert.Equal(t, "true", deployment.Spec.Template.Annotations["prometheus.io/scrape"])
			assert.Equal(t, "/metrics", deployment.Spec.Template.Annotations["prometheus.io/path"])
			assert.Equal(t, "9187", deployment.Spec.Template.Annotations["prometheus.io/port"])

			// Check container
			require.Len(t, deployment.Spec.Template.Spec.Containers, 1)
			container := deployment.Spec.Template.Spec.Containers[0]
			assert.Equal(t, "postgres-exporter", container.Name)
			assert.Equal(t, "quay.io/prometheuscommunity/postgres-exporter:latest", container.Image)

			// Check ports
			require.Len(t, container.Ports, 1)
			assert.Equal(t, int32(9187), container.Ports[0].ContainerPort)

			// Check environment variables
			assert.Len(t, container.Env, tt.wantEnvCount)

			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}

			for key, expectedValue := range tt.checkEnvVars {
				assert.Equal(t, expectedValue, envMap[key], "Environment variable %s mismatch", key)
			}

			// Check pod spec settings
			assert.Equal(t, int64(0), *deployment.Spec.Template.Spec.TerminationGracePeriodSeconds)
		})
	}
}

func TestGenerate(t *testing.T) {
	// Create temp directory for test output
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)

	// Change to temp directory
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"infra"},
	}
	moduleConfig := config.Module{
		Name:      "postgres-exporter",
		Namespace: "infra",
		Secrets: map[string]string{
			"data_source_uri": "test-db:5432/testdb",
		},
	}
	log := logger.Default()

	module := New(generalConfig, moduleConfig, log)
	ctx := context.Background()

	err = module.Generate(ctx)
	require.NoError(t, err)

	// Verify deployment.yaml was created
	deploymentFile := filepath.Join("configs", "postgres-exporter", "deployment.yaml")
	assert.FileExists(t, deploymentFile)

	// Read and verify content
	content, err := os.ReadFile(deploymentFile)
	require.NoError(t, err)

	// Basic content checks
	contentStr := string(content)
	assert.Contains(t, contentStr, "name: postgres-exporter")
	assert.Contains(t, contentStr, "quay.io/prometheuscommunity/postgres-exporter:latest")
	assert.Contains(t, contentStr, "prometheus.io/scrape")
}

func TestGetSecretOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		secrets      map[string]string
		key          string
		defaultValue string
		want         string
	}{
		{
			name:         "secret exists",
			secrets:      map[string]string{"key1": "value1"},
			key:          "key1",
			defaultValue: "default",
			want:         "value1",
		},
		{
			name:         "secret does not exist",
			secrets:      map[string]string{"key1": "value1"},
			key:          "key2",
			defaultValue: "default",
			want:         "default",
		},
		{
			name:         "empty secrets map",
			secrets:      map[string]string{},
			key:          "key1",
			defaultValue: "default",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generalConfig := config.GeneralConfig{
				Domain:     "example.com",
				Namespaces: []string{"infra"},
			}
			moduleConfig := config.Module{
				Name:      "postgres-exporter",
				Namespace: "infra",
				Secrets:   tt.secrets,
			}
			log := logger.Default()

			module := New(generalConfig, moduleConfig, log)
			got := module.getSecretOrDefault(tt.key, tt.defaultValue)

			assert.Equal(t, tt.want, got)
		})
	}
}
