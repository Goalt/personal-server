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
  - name: cloudflare
    namespace: infra
    secrets:
      cloudflare_api_token: old_token
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

	err = app.handleConfigEditCommand(cfg, []string{"cloudflare", "cloudflare_api_token", "new_token"})
	if err != nil {
		t.Fatalf("handleConfigEditCommand failed: %v", err)
	}

	// Verify the file was updated
	reloaded, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if reloaded.Modules[0].Secrets["cloudflare_api_token"] != "new_token" {
		t.Errorf("Expected 'new_token', got '%s'", reloaded.Modules[0].Secrets["cloudflare_api_token"])
	}

	output := logBuf.String()
	if !strings.Contains(output, "cloudflare") {
		t.Errorf("Expected log output to mention 'cloudflare', got: %s", output)
	}
}

func TestHandleConfigEditCommand_NewSecret(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `general:
  domain: example.com
  namespaces: [infra]
modules:
  - name: bitwarden
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

	err = app.handleConfigEditCommand(cfg, []string{"bitwarden", "new_key", "new_value"})
	if err != nil {
		t.Fatalf("handleConfigEditCommand failed: %v", err)
	}

	// Verify the file was updated
	reloaded, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if reloaded.Modules[0].Secrets["new_key"] != "new_value" {
		t.Errorf("Expected 'new_value', got '%s'", reloaded.Modules[0].Secrets["new_key"])
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

	err = app.handleConfigEditCommand(cfg, []string{"nonexistent", "key", "value"})
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
		{"cloudflare", "key"},
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
