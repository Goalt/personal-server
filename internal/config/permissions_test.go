package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadConfig_InsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check is not enforced on Windows")
	}

	configContent := `general:
  domain: example.com
  namespaces: [infra]
`

	insecureModes := []os.FileMode{0644, 0640, 0604, 0666, 0660, 0606}
	for _, mode := range insecureModes {
		mode := mode
		t.Run("mode_"+modeString(mode), func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}
			// WriteFile honors umask on create, so explicitly chmod.
			if err := os.Chmod(configFile, mode); err != nil {
				t.Fatalf("Failed to chmod test config file: %v", err)
			}

			_, err := LoadConfig(configFile)
			if err == nil {
				t.Fatalf("Expected error for insecure permissions %#o, got nil", mode)
			}
			if !strings.Contains(err.Error(), "insecure permissions") {
				t.Errorf("Expected error message to mention 'insecure permissions', got: %v", err)
			}
		})
	}
}

func TestLoadConfig_SecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check is not enforced on Windows")
	}

	configContent := `general:
  domain: example.com
  namespaces: [infra]
`

	secureModes := []os.FileMode{0600, 0400, 0700, 0500}
	for _, mode := range secureModes {
		mode := mode
		t.Run("mode_"+modeString(mode), func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}
			if err := os.Chmod(configFile, mode); err != nil {
				t.Fatalf("Failed to chmod test config file: %v", err)
			}

			cfg, err := LoadConfig(configFile)
			if err != nil {
				t.Fatalf("LoadConfig failed for secure mode %#o: %v", mode, err)
			}
			if cfg.General.Domain != "example.com" {
				t.Errorf("Expected domain 'example.com', got '%s'", cfg.General.Domain)
			}
		})
	}
}

func modeString(m os.FileMode) string {
	const digits = "01234567"
	b := []byte{digits[(m>>9)&7], digits[(m>>6)&7], digits[(m>>3)&7], digits[m&7]}
	return string(b)
}
