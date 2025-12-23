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

func TestRollout(t *testing.T) {
	// This test would require a Kubernetes cluster
	// For now, we'll skip it.
	t.Skip("Skipping integration test")
}

func TestRolloutValidation(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "testapp",
		Namespace: "hobby",
		Image:     "nginx:latest",
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	// Test with no args
	err := module.Rollout(nil, []string{})
	if err == nil {
		t.Error("Expected error when no args provided, got nil")
	}

	// Test with invalid operation
	err = module.Rollout(nil, []string{"invalid"})
	if err == nil {
		t.Error("Expected error for invalid operation, got nil")
	}
}

func TestPrepareService(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "myapp",
		Namespace: "hobby",
		Image:     "nginx:latest",
		Service: &config.ServiceConfig{
			Ports: []config.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: 8080,
				},
				{
					Name:       "https",
					Port:       443,
					TargetPort: 8443,
				},
			},
		},
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	service := module.prepareService()

	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}

	expectedName := "pet-myapp"
	if service.Name != expectedName {
		t.Errorf("Expected service name to be '%s', got '%s'", expectedName, service.Name)
	}

	if service.Namespace != "hobby" {
		t.Errorf("Expected namespace to be 'hobby', got '%s'", service.Namespace)
	}

	// Check labels
	if service.Labels["type"] != "pet-project" {
		t.Errorf("Expected type label to be 'pet-project', got '%s'", service.Labels["type"])
	}

	if service.Labels["managed-by"] != "personal-server" {
		t.Errorf("Expected managed-by label to be 'personal-server', got '%s'", service.Labels["managed-by"])
	}

	if service.Labels["app"] != expectedName {
		t.Errorf("Expected app label to be '%s', got '%s'", expectedName, service.Labels["app"])
	}

	// Check selector
	if service.Spec.Selector["app"] != expectedName {
		t.Errorf("Expected selector app to be '%s', got '%s'", expectedName, service.Spec.Selector["app"])
	}

	// Check ports
	if len(service.Spec.Ports) != 2 {
		t.Fatalf("Expected 2 service ports, got %d", len(service.Spec.Ports))
	}

	// Check first port
	if service.Spec.Ports[0].Name != "http" {
		t.Errorf("Expected first port name to be 'http', got '%s'", service.Spec.Ports[0].Name)
	}

	if service.Spec.Ports[0].Port != 80 {
		t.Errorf("Expected first port to be 80, got %d", service.Spec.Ports[0].Port)
	}

	if service.Spec.Ports[0].TargetPort.IntVal != 8080 {
		t.Errorf("Expected first targetPort to be 8080, got %d", service.Spec.Ports[0].TargetPort.IntVal)
	}

	// Check second port
	if service.Spec.Ports[1].Name != "https" {
		t.Errorf("Expected second port name to be 'https', got '%s'", service.Spec.Ports[1].Name)
	}

	if service.Spec.Ports[1].Port != 443 {
		t.Errorf("Expected second port to be 443, got %d", service.Spec.Ports[1].Port)
	}

	if service.Spec.Ports[1].TargetPort.IntVal != 8443 {
		t.Errorf("Expected second targetPort to be 8443, got %d", service.Spec.Ports[1].TargetPort.IntVal)
	}
}

func TestPrepareServiceWithoutConfig(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "myapp",
		Namespace: "hobby",
		Image:     "nginx:latest",
		Service:   nil, // No service configuration
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	service := module.prepareService()

	if service != nil {
		t.Error("Expected service to be nil when no service configuration is provided")
	}
}

func TestPrepareServiceWithEmptyPorts(t *testing.T) {
	generalConfig := config.GeneralConfig{
		Domain:     "example.com",
		Namespaces: []string{"hobby"},
	}

	projectConfig := config.PetProject{
		Name:      "myapp",
		Namespace: "hobby",
		Image:     "nginx:latest",
		Service: &config.ServiceConfig{
			Ports: []config.ServicePort{}, // Empty ports
		},
	}

	log := logger.Default()
	module := New(generalConfig, projectConfig, log)

	service := module.prepareService()

	if service != nil {
		t.Error("Expected service to be nil when service has no ports")
	}
}
