package cloudflare

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	corev1 "k8s.io/api/core/v1"
)

func TestCloudflareModule_Name(t *testing.T) {
	module := &CloudflareModule{}
	if module.Name() != "cloudflare" {
		t.Errorf("Name() = %s, want cloudflare", module.Name())
	}
}

func TestCloudflareModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		apiToken  string
	}{
		{
			name:      "default configuration",
			namespace: "infra",
			apiToken:  "test-api-token-123",
		},
		{
			name:      "custom namespace",
			namespace: "cloudflare-ns",
			apiToken:  "another-token-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &CloudflareModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "cloudflare",
					Namespace: tt.namespace,
				},
			}

			secret, deployment := module.prepare(tt.apiToken)

			// Verify all objects are not nil
			if secret == nil {
				t.Fatal("prepare() returned nil Secret")
			}
			if deployment == nil {
				t.Fatal("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly
			if secret.Namespace != tt.namespace {
				t.Errorf("Secret namespace = %s, want %s", secret.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
		})
	}
}

func TestCloudflareModule_PrepareSecret(t *testing.T) {
	apiToken := "my-secret-api-token"
	module := &CloudflareModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "cloudflare",
			Namespace: "test-namespace",
		},
	}

	secret, _ := module.prepare(apiToken)

	// Test Secret name
	if secret.Name != "tunnel-token" {
		t.Errorf("Secret name = %s, want tunnel-token", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "cloudflared",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := secret.Labels[key]; !ok {
			t.Errorf("Secret missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Secret label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test Secret data contains token
	if string(secret.Data["token"]) != apiToken {
		t.Errorf("Secret data[token] = %s, want %s", string(secret.Data["token"]), apiToken)
	}
}

func TestCloudflareModule_PrepareDeployment(t *testing.T) {
	module := &CloudflareModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "cloudflare",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare("test-token")

	// Test Deployment name
	if deployment.Name != "cloudflared-deployment" {
		t.Errorf("Deployment name = %s, want cloudflared-deployment", deployment.Name)
	}

	// Test Deployment labels
	expectedLabels := map[string]string{
		"managed-by": "personal-server",
		"app":        "cloudflared",
	}
	for key, expectedValue := range expectedLabels {
		if actualValue, ok := deployment.Labels[key]; !ok {
			t.Errorf("Deployment missing label: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Deployment label %s = %s, want %s", key, actualValue, expectedValue)
		}
	}

	// Test replicas - cloudflare uses 2 replicas for HA
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 2 {
		t.Errorf("Deployment replicas = %d, want 2", *deployment.Spec.Replicas)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["pod"] != "cloudflared" {
		t.Errorf("Deployment selector pod = %s, want cloudflared", deployment.Spec.Selector.MatchLabels["pod"])
	}

	// Test pod template labels
	if deployment.Spec.Template.Labels["pod"] != "cloudflared" {
		t.Errorf("Pod template label pod = %s, want cloudflared", deployment.Spec.Template.Labels["pod"])
	}
}

func TestCloudflareModule_PrepareDeploymentPodSecurityContext(t *testing.T) {
	module := &CloudflareModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "cloudflare",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare("test-token")

	// Test PodSecurityContext
	podSecurityContext := deployment.Spec.Template.Spec.SecurityContext
	if podSecurityContext == nil {
		t.Fatal("PodSecurityContext is nil")
	}

	// Test Sysctls
	if len(podSecurityContext.Sysctls) != 1 {
		t.Fatalf("Sysctls count = %d, want 1", len(podSecurityContext.Sysctls))
	}

	sysctl := podSecurityContext.Sysctls[0]
	if sysctl.Name != "net.ipv4.ping_group_range" {
		t.Errorf("Sysctl name = %s, want net.ipv4.ping_group_range", sysctl.Name)
	}
	if sysctl.Value != "65532 65532" {
		t.Errorf("Sysctl value = %s, want 65532 65532", sysctl.Value)
	}
}

func TestCloudflareModule_PrepareDeploymentContainer(t *testing.T) {
	module := &CloudflareModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "cloudflare",
			Namespace: "test-namespace",
		},
	}

	_, deployment := module.prepare("test-token")

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "cloudflared" {
		t.Errorf("Container name = %s, want cloudflared", container.Name)
	}

	// Test container image
	expectedImage := "cloudflare/cloudflared:2025.11.1"
	if container.Image != expectedImage {
		t.Errorf("Container image = %s, want %s", container.Image, expectedImage)
	}

	// Test command
	expectedCommand := []string{
		"cloudflared",
		"tunnel",
		"--no-autoupdate",
		"--loglevel",
		"debug",
		"--metrics",
		"0.0.0.0:2000",
		"run",
	}
	if len(container.Command) != len(expectedCommand) {
		t.Errorf("Container command length = %d, want %d", len(container.Command), len(expectedCommand))
	} else {
		for i, cmd := range expectedCommand {
			if container.Command[i] != cmd {
				t.Errorf("Container command[%d] = %s, want %s", i, container.Command[i], cmd)
			}
		}
	}

	// Test environment variables
	tunnelTokenFound := false
	for _, env := range container.Env {
		if env.Name == "TUNNEL_TOKEN" {
			tunnelTokenFound = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Error("TUNNEL_TOKEN should use SecretKeyRef")
			} else {
				if env.ValueFrom.SecretKeyRef.Name != "tunnel-token" {
					t.Errorf("TUNNEL_TOKEN secret name = %s, want tunnel-token", env.ValueFrom.SecretKeyRef.Name)
				}
				if env.ValueFrom.SecretKeyRef.Key != "token" {
					t.Errorf("TUNNEL_TOKEN secret key = %s, want token", env.ValueFrom.SecretKeyRef.Key)
				}
			}
		}
	}
	if !tunnelTokenFound {
		t.Error("Container missing TUNNEL_TOKEN env var")
	}

	// Test liveness probe
	if container.LivenessProbe == nil {
		t.Fatal("Container LivenessProbe is nil")
	}
	if container.LivenessProbe.HTTPGet == nil {
		t.Fatal("LivenessProbe HTTPGet is nil")
	}
	if container.LivenessProbe.HTTPGet.Path != "/ready" {
		t.Errorf("LivenessProbe path = %s, want /ready", container.LivenessProbe.HTTPGet.Path)
	}
	if container.LivenessProbe.HTTPGet.Port.IntVal != 2000 {
		t.Errorf("LivenessProbe port = %d, want 2000", container.LivenessProbe.HTTPGet.Port.IntVal)
	}
	if container.LivenessProbe.FailureThreshold != 1 {
		t.Errorf("LivenessProbe FailureThreshold = %d, want 1", container.LivenessProbe.FailureThreshold)
	}
	if container.LivenessProbe.InitialDelaySeconds != 10 {
		t.Errorf("LivenessProbe InitialDelaySeconds = %d, want 10", container.LivenessProbe.InitialDelaySeconds)
	}
	if container.LivenessProbe.PeriodSeconds != 10 {
		t.Errorf("LivenessProbe PeriodSeconds = %d, want 10", container.LivenessProbe.PeriodSeconds)
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "one minute",
			duration: 1 * time.Minute,
			expected: "1m",
		},
		{
			name:     "multiple minutes",
			duration: 45 * time.Minute,
			expected: "45m",
		},
		{
			name:     "one hour",
			duration: 1 * time.Hour,
			expected: "1h",
		},
		{
			name:     "multiple hours",
			duration: 5 * time.Hour,
			expected: "5h",
		},
		{
			name:     "one day",
			duration: 24 * time.Hour,
			expected: "1d",
		},
		{
			name:     "multiple days",
			duration: 72 * time.Hour,
			expected: "3d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := k8s.FormatAge(tt.duration)
			if result != tt.expected {
				t.Errorf("k8s.FormatAge(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestGetMapKeys(t *testing.T) {
	m := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	keys := getMapKeys(m)

	if len(keys) != 3 {
		t.Errorf("getMapKeys returned %d keys, want 3", len(keys))
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	for expectedKey := range m {
		if !keySet[expectedKey] {
			t.Errorf("getMapKeys missing key: %s", expectedKey)
		}
	}
}

//go:embed testdata/secret.yaml
var expectedSecretYAML string

//go:embed testdata/deployment.yaml
var expectedDeploymentYAML string

func TestGenerate(t *testing.T) {
	// Create a temporary directory for output
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Change to temp directory so Generate creates files there
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Create module with test configuration
	module := &CloudflareModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "cloudflare",
			Namespace: "infra",
			Secrets: map[string]string{
				"cloudflare_api_token": "password",
			},
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify generated files exist and match expected content
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{"secret", "configs/cloudflare/secret.yaml", expectedSecretYAML},
		{"deployment", "configs/cloudflare/deployment.yaml", expectedDeploymentYAML},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read generated file
			generatedPath := filepath.Join(tempDir, tc.filename)
			generatedContent, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("failed to read generated file %s: %v", tc.filename, err)
			}

			// Compare with expected
			if string(generatedContent) != tc.expected {
				t.Errorf("Generated YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s", string(generatedContent), tc.expected)
			}
		})
	}
}
