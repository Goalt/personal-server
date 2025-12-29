package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCloudflareE2E tests the complete cloudflare module lifecycle
func TestCloudflareE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// Construct full path to binary from test directory
	fullBinaryPath := filepath.Join("..", "..", binaryPath)
	// Construct full path to config from test directory
	fullConfigPath := filepath.Join("..", "..", testConfigPath)

	// Verify binary exists
	if _, err := os.Stat(fullBinaryPath); os.IsNotExist(err) {
		t.Fatalf("binary not found at %s. Run 'make build' first", fullBinaryPath)
	}

	// Create Kubernetes client
	client := createKubeClient(t)

	// Ensure the test namespace exists
	ctx := context.Background()
	_, err := client.CoreV1().Namespaces().Get(ctx, testNamespace, metav1.GetOptions{})
	if err != nil {
		// Namespace doesn't exist, create it
		t.Logf("Creating test namespace: %s", testNamespace)
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
				Labels: map[string]string{
					"managed-by": "personal-server",
				},
			},
		}
		_, err = client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create test namespace %s: %v", testNamespace, err)
		}
		t.Logf("Created test namespace: %s", testNamespace)
	} else {
		t.Logf("Test namespace %s already exists", testNamespace)
	}

	// Cleanup function - runs at the end
	defer func() {
		t.Log("Cleaning up cloudflare resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "cloudflared-deployment"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete secret
		secretName := "tunnel-token"
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
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "cloudflare", "generate")
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
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "cloudflare", "apply")
		if err != nil {
			t.Fatalf("failed to apply cloudflare configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, "tunnel-token", metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret tunnel-token was not created: %v", err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "cloudflared-deployment", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment cloudflared-deployment was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}

		// Add a sleep to wait for the pod to be ready
		time.Sleep(5 * time.Second)

		// Verify pod is running - get pod from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment cloudflared-deployment: %v", err)
		}
		if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment cloudflared-deployment")
		} else {
			t.Logf("Verified pod exists: %s", pods.Items[0].Name)
		}

		// Verify pod is running
		if pods.Items[0].Status.Phase != corev1.PodRunning {
			t.Errorf("pod cloudflared is not running: %s", pods.Items[0].Status.Phase)
		} else {
			t.Logf("Verified pod is running: %s", pods.Items[0].Name)
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "cloudflare", "status")
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
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "cloudflare", "clean")
		if err != nil {
			t.Fatalf("failed to clean cloudflare: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "cloudflared-deployment", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment cloudflared-deployment still exists after clean")
		} else {
			t.Logf("Deployment cloudflared-deployment deleted successfully")
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, "tunnel-token", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret tunnel-token still exists after clean")
		} else {
			t.Logf("Secret tunnel-token deleted successfully")
		}
	})
}
