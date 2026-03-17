package registrysecret

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
)

func TestNew(t *testing.T) {
	registries := map[string]config.RegistryCredentials{
		"my-registry": {
			Server:    "https://registry.example.com",
			Username:  "user",
			Password:  "pass",
			Namespace: "hobby",
		},
	}

	log := logger.Default()
	m := New(registries, log)

	if m == nil {
		t.Fatal("Expected module to be created, got nil")
	}
	if m.Name() != "registry" {
		t.Errorf("Expected module name 'registry', got '%s'", m.Name())
	}
	if len(m.Registries) != 1 {
		t.Errorf("Expected 1 registry, got %d", len(m.Registries))
	}
}

func TestPrepareSecret(t *testing.T) {
	m := New(nil, logger.Default())

	creds := config.RegistryCredentials{
		Server:    "https://registry.example.com",
		Username:  "user",
		Password:  "pass",
		Email:     "user@example.com",
		Namespace: "hobby",
	}

	secret, err := m.prepareSecret("my-registry", creds)
	if err != nil {
		t.Fatalf("prepareSecret() returned error: %v", err)
	}

	if secret == nil {
		t.Fatal("Expected secret to be non-nil")
	}
	if secret.Name != "my-registry" {
		t.Errorf("Expected secret name 'my-registry', got '%s'", secret.Name)
	}
	if secret.Namespace != "hobby" {
		t.Errorf("Expected namespace 'hobby', got '%s'", secret.Namespace)
	}
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		t.Errorf("Expected secret type %s, got %s", corev1.SecretTypeDockerConfigJson, secret.Type)
	}
	if secret.Labels["managed-by"] != "personal-server" {
		t.Errorf("Expected label managed-by=personal-server, got '%s'", secret.Labels["managed-by"])
	}
	if secret.Labels["type"] != "registry-secret" {
		t.Errorf("Expected label type=registry-secret, got '%s'", secret.Labels["type"])
	}

	// Verify .dockerconfigjson content
	data, ok := secret.Data[".dockerconfigjson"]
	if !ok || len(data) == 0 {
		t.Fatal("Expected .dockerconfigjson data to be set")
	}

	var parsed map[string]map[string]map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal .dockerconfigjson: %v", err)
	}

	auths := parsed["auths"]["https://registry.example.com"]
	if auths == nil {
		t.Fatal("Expected auths entry for registry server")
	}
	if auths["username"] != "user" {
		t.Errorf("Expected username 'user', got '%v'", auths["username"])
	}
	if auths["password"] != "pass" {
		t.Errorf("Expected password 'pass', got '%v'", auths["password"])
	}
	if auths["email"] != "user@example.com" {
		t.Errorf("Expected email 'user@example.com', got '%v'", auths["email"])
	}
	if auths["auth"] == "" {
		t.Error("Expected non-empty auth field")
	}
}

func TestPrepareSecret_OmitsEmailWhenEmpty(t *testing.T) {
	m := New(nil, logger.Default())

	creds := config.RegistryCredentials{
		Server:    "https://registry.example.com",
		Username:  "user",
		Password:  "pass",
		Namespace: "hobby",
		// Email intentionally left empty
	}

	secret, err := m.prepareSecret("my-registry", creds)
	if err != nil {
		t.Fatalf("prepareSecret() returned error: %v", err)
	}

	data := secret.Data[".dockerconfigjson"]
	var parsed map[string]map[string]map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal .dockerconfigjson: %v", err)
	}

	auths := parsed["auths"]["https://registry.example.com"]
	if _, hasEmail := auths["email"]; hasEmail {
		t.Error("Expected email field to be omitted when email is empty")
	}
}

func TestPrepareSecret_NoEmail(t *testing.T) {
	m := New(nil, logger.Default())

	creds := config.RegistryCredentials{
		Server:    "https://registry.example.com",
		Username:  "user",
		Password:  "pass",
		Namespace: "hobby",
	}

	secret, err := m.prepareSecret("my-registry", creds)
	if err != nil {
		t.Fatalf("prepareSecret() returned error: %v", err)
	}
	if secret == nil {
		t.Fatal("Expected secret to be non-nil even without email")
	}
}

func TestGenerate_EmptyRegistries(t *testing.T) {
	m := New(nil, logger.Default())

	err := m.Generate(context.Background())
	if err != nil {
		t.Errorf("Generate() with empty registries should not return error, got: %v", err)
	}
}

func TestApply_EmptyRegistries(t *testing.T) {
	m := New(nil, logger.Default())

	// Should return nil without attempting to contact k8s
	err := m.Apply(context.Background())
	if err != nil {
		t.Errorf("Apply() with empty registries should not return error, got: %v", err)
	}
}

func TestClean_EmptyRegistries(t *testing.T) {
	m := New(nil, logger.Default())

	err := m.Clean(context.Background())
	if err != nil {
		t.Errorf("Clean() with empty registries should not return error, got: %v", err)
	}
}

func TestStatus_EmptyRegistries(t *testing.T) {
	m := New(nil, logger.Default())

	err := m.Status(context.Background())
	if err != nil {
		t.Errorf("Status() with empty registries should not return error, got: %v", err)
	}
}
