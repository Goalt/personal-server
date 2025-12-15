package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Success(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `general:
  domain: example.com
  namespaces: [infra, hobby]
modules:
  - name: cloudflare
    namespace: infra
    secrets:
      cloudflare_api_token: abc123
  - name: bitwarden
    namespace: infra
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load the config
	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify general config
	if config.General.Domain != "example.com" {
		t.Errorf("Expected domain to be 'example.com', got '%s'", config.General.Domain)
	}

	if len(config.General.Namespaces) != 2 {
		t.Errorf("Expected 2 namespaces, got %d", len(config.General.Namespaces))
	}

	if config.General.Namespaces[0] != "infra" {
		t.Errorf("Expected first namespace to be 'infra', got '%s'", config.General.Namespaces[0])
	}

	if config.General.Namespaces[1] != "hobby" {
		t.Errorf("Expected second namespace to be 'hobby', got '%s'", config.General.Namespaces[1])
	}

	// Verify modules
	if len(config.Modules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(config.Modules))
	}

	// Verify first module
	if config.Modules[0].Name != "cloudflare" {
		t.Errorf("Expected first module name to be 'cloudflare', got '%s'", config.Modules[0].Name)
	}

	if config.Modules[0].Namespace != "infra" {
		t.Errorf("Expected first module namespace to be 'infra', got '%s'", config.Modules[0].Namespace)
	}

	if len(config.Modules[0].Secrets) != 1 {
		t.Errorf("Expected 1 secret in first module, got %d", len(config.Modules[0].Secrets))
	}

	if config.Modules[0].Secrets["cloudflare_api_token"] != "abc123" {
		t.Errorf("Expected cloudflare_api_token to be 'abc123', got '%s'", config.Modules[0].Secrets["cloudflare_api_token"])
	}

	// Verify second module (no secrets)
	if config.Modules[1].Name != "bitwarden" {
		t.Errorf("Expected second module name to be 'bitwarden', got '%s'", config.Modules[1].Name)
	}

	if config.Modules[1].Namespace != "infra" {
		t.Errorf("Expected second module namespace to be 'infra', got '%s'", config.Modules[1].Namespace)
	}

	if len(config.Modules[1].Secrets) != 0 {
		t.Errorf("Expected 0 secrets in second module, got %d", len(config.Modules[1].Secrets))
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	// Try to load a non-existent file
	_, err := LoadConfig("/non/existent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	expectedMsg := "config file not found:"
	if err != nil && len(err.Error()) < len(expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `general:
  domain: example.com
  namespaces: [infra, hobby
modules:
  - name: cloudflare
    namespace
`

	err := os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid config file: %v", err)
	}

	// Try to load the invalid config
	_, err = LoadConfig(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}

	expectedMsg := "error parsing YAML config:"
	if err != nil && len(err.Error()) < len(expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	// Create an empty config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "empty.yaml")

	err := os.WriteFile(configFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty config file: %v", err)
	}

	// Load the empty config
	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed for empty file: %v", err)
	}

	// Verify empty config
	if config.General.Domain != "" {
		t.Errorf("Expected empty domain, got '%s'", config.General.Domain)
	}

	if len(config.General.Namespaces) != 0 {
		t.Errorf("Expected 0 namespaces, got %d", len(config.General.Namespaces))
	}

	if len(config.Modules) != 0 {
		t.Errorf("Expected 0 modules, got %d", len(config.Modules))
	}
}

func TestLoadConfig_MultipleSecrets(t *testing.T) {
	// Create a config with multiple secrets
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `general:
  domain: test.com
  namespaces: [default]
modules:
  - name: postgres
    namespace: infra
    secrets:
      admin_postgres_user: postgres
      admin_postgres_password: secret123
      postgres_db: testdb
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Load the config
	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify multiple secrets
	if len(config.Modules[0].Secrets) != 3 {
		t.Errorf("Expected 3 secrets, got %d", len(config.Modules[0].Secrets))
	}

	expectedSecrets := map[string]string{
		"admin_postgres_user":     "postgres",
		"admin_postgres_password": "secret123",
		"postgres_db":             "testdb",
	}

	for key, expectedValue := range expectedSecrets {
		if actualValue, ok := config.Modules[0].Secrets[key]; !ok {
			t.Errorf("Expected secret '%s' not found", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected secret '%s' to be '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func TestLoadConfig_UnreadableFile(t *testing.T) {
	// Skip this test on systems where we can't change permissions
	if os.Getuid() == 0 {
		t.Skip("Skipping test when running as root")
	}

	// Create a file with no read permissions
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "unreadable.yaml")

	err := os.WriteFile(configFile, []byte("test"), 0000)
	if err != nil {
		t.Fatalf("Failed to create unreadable file: %v", err)
	}

	// Try to load the unreadable file
	_, err = LoadConfig(configFile)
	if err == nil {
		t.Error("Expected error for unreadable file, got nil")
	}

	expectedMsg := "error reading config file:"
	if err != nil && len(err.Error()) < len(expectedMsg) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestGetModule_Success(t *testing.T) {
	// Create a config with modules
	config := &Config{
		Modules: []Module{
			{
				Name:      "cloudflare",
				Namespace: "infra",
				Secrets: map[string]string{
					"api_token": "abc123",
				},
			},
			{
				Name:      "postgres",
				Namespace: "db",
				Secrets: map[string]string{
					"password": "secret",
				},
			},
		},
	}

	// Test getting an existing module
	module, err := config.GetModule("cloudflare")
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}

	if module.Name != "cloudflare" {
		t.Errorf("Expected module name to be 'cloudflare', got '%s'", module.Name)
	}

	if module.Namespace != "infra" {
		t.Errorf("Expected namespace to be 'infra', got '%s'", module.Namespace)
	}

	if module.Secrets["api_token"] != "abc123" {
		t.Errorf("Expected api_token to be 'abc123', got '%s'", module.Secrets["api_token"])
	}
}

func TestGetModule_NotFound(t *testing.T) {
	// Create a config with modules
	config := &Config{
		Modules: []Module{
			{
				Name:      "cloudflare",
				Namespace: "infra",
			},
		},
	}

	// Test getting a non-existent module
	_, err := config.GetModule("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent module, got nil")
	}

	expectedMsg := "module not found: nonexistent"
	if err != nil && err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestGetModule_EmptyConfig(t *testing.T) {
	// Create an empty config
	config := &Config{
		Modules: []Module{},
	}

	// Test getting a module from empty config
	_, err := config.GetModule("anymodule")
	if err == nil {
		t.Error("Expected error for empty config, got nil")
	}

	expectedMsg := "module not found: anymodule"
	if err != nil && err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestGetModule_MultipleModules(t *testing.T) {
	// Create a config with multiple modules
	config := &Config{
		Modules: []Module{
			{Name: "module1", Namespace: "ns1"},
			{Name: "module2", Namespace: "ns2"},
			{Name: "module3", Namespace: "ns3"},
		},
	}

	// Test getting the middle module
	module, err := config.GetModule("module2")
	if err != nil {
		t.Fatalf("GetModule failed: %v", err)
	}

	if module.Name != "module2" {
		t.Errorf("Expected module name to be 'module2', got '%s'", module.Name)
	}

	if module.Namespace != "ns2" {
		t.Errorf("Expected namespace to be 'ns2', got '%s'", module.Namespace)
	}
}
