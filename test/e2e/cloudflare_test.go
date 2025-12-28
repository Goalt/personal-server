package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCloudflareE2E tests the complete cloudflare module lifecycle
func TestCloudflareE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Change to the repository root
	repoRoot := filepath.Join("..", "..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("failed to change to repo root: %v", err)
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("binary not found at %s. Run 'make build' first", binaryPath)
	}

	// Create Kubernetes client
	client := createKubeClient(t)

	// Cleanup function - runs at the end
	defer func() {
		t.Log("Cleaning up cloudflare resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "cloudflared"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete secret
		secretName := "cloudflare-secret"
		err = client.CoreV1().Secrets(testNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete secret %s: %v", secretName, err)
		} else {
			t.Logf("Deleted secret: %s", secretName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/cloudflare"); err != nil {
			t.Logf("Warning: failed to remove configs/cloudflare: %v", err)
		}
	}()

	// Test 1: Generate cloudflare configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "cloudflare", "generate")
		if err != nil {
			t.Fatalf("failed to generate cloudflare configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"secret.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "cloudflare", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply cloudflare configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "cloudflare", "apply")
		if err != nil {
			t.Fatalf("failed to apply cloudflare configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, "cloudflare-secret", metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret cloudflare-secret was not created: %v", err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "cloudflared", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment cloudflared was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "cloudflare", "status")
		if err != nil {
			t.Fatalf("failed to get cloudflare status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "cloudflared") {
			t.Errorf("status output does not contain deployment name: cloudflared")
		}
	})

	// Test 4: Clean up cloudflare resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "cloudflare", "clean")
		if err != nil {
			t.Fatalf("failed to clean cloudflare: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "cloudflared", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment cloudflared still exists after clean")
		} else {
			t.Logf("Deployment cloudflared deleted successfully")
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, "cloudflare-secret", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret cloudflare-secret still exists after clean")
		} else {
			t.Logf("Secret cloudflare-secret deleted successfully")
		}
	})
}
