package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestHandleConfigEditCommand_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `general:
  domain: example.com
  namespaces: [infra]
modules:
  - name: hobby-pod
    namespace: infra
    image: ghcr.io/goalt/work-config:old-tag
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	app := &App{logger: log}

	err = app.handleConfigEditCommand(cfg, []string{"hobby-pod", "image", "ghcr.io/goalt/work-config:new-tag"})
	if err != nil {
		t.Fatalf("handleConfigEditCommand failed: %v", err)
	}

	// Verify the file was updated
	reloaded, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if reloaded.Modules[0].Image != "ghcr.io/goalt/work-config:new-tag" {
		t.Errorf("Expected 'ghcr.io/goalt/work-config:new-tag', got '%s'", reloaded.Modules[0].Image)
	}

	output := logBuf.String()
	if !strings.Contains(output, "hobby-pod") {
		t.Errorf("Expected log output to mention 'hobby-pod', got: %s", output)
	}
}

func TestHandleConfigEditCommand_SetNewImage(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `general:
  domain: example.com
  namespaces: [infra]
modules:
  - name: prometheus
    namespace: infra
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	app := &App{logger: log}

	err = app.handleConfigEditCommand(cfg, []string{"prometheus", "image", "prom/prometheus:v3.0.0"})
	if err != nil {
		t.Fatalf("handleConfigEditCommand failed: %v", err)
	}

	// Verify the file was updated
	reloaded, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if reloaded.Modules[0].Image != "prom/prometheus:v3.0.0" {
		t.Errorf("Expected 'prom/prometheus:v3.0.0', got '%s'", reloaded.Modules[0].Image)
	}
}

func TestHandleConfigEditCommand_RejectsNonImageKey(t *testing.T) {
	cfg := &config.Config{
		Modules: []config.Module{
			{Name: "cloudflare", Namespace: "infra"},
		},
	}

	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	app := &App{logger: log}

	err := app.handleConfigEditCommand(cfg, []string{"cloudflare", "cloudflare_api_token", "new_token"})
	if err == nil {
		t.Error("Expected error for non-image key, got nil")
	}

	if !strings.Contains(err.Error(), "only the 'image' field can be edited") {
		t.Errorf("Expected error about image field, got: %v", err)
	}
}

func TestHandleConfigEditCommand_ModuleNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `general:
  domain: example.com
  namespaces: [infra]
modules:
  - name: cloudflare
    namespace: infra
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	app := &App{logger: log}

	err = app.handleConfigEditCommand(cfg, []string{"nonexistent", "image", "value"})
	if err == nil {
		t.Error("Expected error for non-existent module, got nil")
	}

	if !strings.Contains(err.Error(), "module not found: nonexistent") {
		t.Errorf("Expected error to mention 'module not found', got: %v", err)
	}
}

func TestHandleConfigEditCommand_InsufficientArgs(t *testing.T) {
	cfg := &config.Config{}

	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	app := &App{logger: log}

	testCases := [][]string{
		{},
		{"cloudflare"},
		{"cloudflare", "image"},
	}

	for _, args := range testCases {
		err := app.handleConfigEditCommand(cfg, args)
		if err == nil {
			t.Errorf("Expected error for args %v, got nil", args)
		}
		if !strings.Contains(err.Error(), "usage:") {
			t.Errorf("Expected usage error for args %v, got: %v", args, err)
		}
	}
}
