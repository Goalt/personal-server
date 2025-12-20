package petproject

import (
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestNew(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "myapp",
		Namespace: "hobby",
		Image:     "nginx:latest",
		Environment: map[string]string{
			"PORT": "8080",
		},
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	if module == nil {
		t.Fatal("Expected module to be created, got nil")
	}

	if module.Name() != "myapp" {
		t.Errorf("Expected module name to be 'myapp', got '%s'", module.Name())
	}

	if module.ProjectConfig.Image != "nginx:latest" {
		t.Errorf("Expected image to be 'nginx:latest', got '%s'", module.ProjectConfig.Image)
	}
}

func TestPrepareDeployment(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "myapp",
		Namespace: "hobby",
		Image:     "nginx:latest",
		Environment: map[string]string{
			"PORT": "8080",
			"ENV":  "production",
		},
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	deployment := module.prepareDeployment()

	if deployment == nil {
		t.Fatal("Expected deployment to be created, got nil")
	}

	expectedName := "pet-myapp"
	if deployment.Name != expectedName {
		t.Errorf("Expected deployment name to be '%s', got '%s'", expectedName, deployment.Name)
	}

	if deployment.Namespace != "hobby" {
		t.Errorf("Expected namespace to be 'hobby', got '%s'", deployment.Namespace)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]
	if container.Name != "myapp" {
		t.Errorf("Expected container name to be 'myapp', got '%s'", container.Name)
	}

	if container.Image != "nginx:latest" {
		t.Errorf("Expected container image to be 'nginx:latest', got '%s'", container.Image)
	}

	if len(container.Env) != 2 {
		t.Errorf("Expected 2 environment variables, got %d", len(container.Env))
	}

	// Check environment variables
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}

	if envMap["PORT"] != "8080" {
		t.Errorf("Expected PORT to be '8080', got '%s'", envMap["PORT"])
	}

	if envMap["ENV"] != "production" {
		t.Errorf("Expected ENV to be 'production', got '%s'", envMap["ENV"])
	}

	// Check labels
	if deployment.Labels["type"] != "pet-project" {
		t.Errorf("Expected type label to be 'pet-project', got '%s'", deployment.Labels["type"])
	}

	if deployment.Labels["managed-by"] != "personal-server" {
		t.Errorf("Expected managed-by label to be 'personal-server', got '%s'", deployment.Labels["managed-by"])
	}
}

func TestGenerate(t *testing.T) {
	// This test would require setting up a temporary directory
	// and verifying file creation. For now, we'll skip it.
	t.Skip("Skipping integration test")
}

func TestApply(t *testing.T) {
	// This test would require a Kubernetes cluster
	// For now, we'll skip it.
	t.Skip("Skipping integration test")
}

func TestClean(t *testing.T) {
	// This test would require a Kubernetes cluster
	// For now, we'll skip it.
	t.Skip("Skipping integration test")
}

func TestStatus(t *testing.T) {
	// This test would require a Kubernetes cluster
	// For now, we'll skip it.
	t.Skip("Skipping integration test")
}

func TestPrepareDeploymentWithoutEnvironment(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "simpleapp",
		Namespace: "hobby",
		Image:     "alpine:latest",
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	deployment := module.prepareDeployment()

	if deployment == nil {
		t.Fatal("Expected deployment to be created, got nil")
	}

	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]
	if len(container.Env) != 0 {
		t.Errorf("Expected 0 environment variables, got %d", len(container.Env))
	}
}
