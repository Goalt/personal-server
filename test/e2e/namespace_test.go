package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	testConfigPath = "test/e2e/test-config.yaml"
	binaryPath     = "../../bin/personal-server"
	timeout        = 5 * time.Minute
)

// createKubeClient creates a Kubernetes client using the default kubeconfig
func createKubeClient(t *testing.T) *kubernetes.Clientset {
	t.Helper()

	// Get kubeconfig path
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("failed to get home directory: %v", err)
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	// Build config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatalf("failed to build kubeconfig: %v", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}

	return clientset
}

// runCommand executes a command and returns its output
func runCommand(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", os.Getenv("KUBECONFIG")))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// TestNamespaceE2E tests the complete namespace lifecycle
func TestNamespaceE2E(t *testing.T) {
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

	// Test namespaces
	testNamespaces := []string{"e2e-test-infra", "e2e-test-hobby"}

	// Cleanup function - runs at the end
	defer func() {
		t.Log("Cleaning up test namespaces...")
		ctx := context.Background()
		for _, ns := range testNamespaces {
			err := client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
			if err != nil {
				t.Logf("Warning: failed to delete namespace %s: %v", ns, err)
			} else {
				t.Logf("Deleted namespace: %s", ns)
			}
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/namespace"); err != nil {
			t.Logf("Warning: failed to remove configs/namespace: %v", err)
		}
	}()

	// Ensure namespaces don't exist before test
	ctx := context.Background()
	for _, ns := range testNamespaces {
		_, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
		if err == nil {
			t.Fatalf("namespace %s already exists, please clean up before running test", ns)
		}
	}

	// Test 1: Generate namespace configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "namespace", "generate")
		if err != nil {
			t.Fatalf("failed to generate namespace configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		for _, ns := range testNamespaces {
			configFile := filepath.Join("configs", "namespace", fmt.Sprintf("%s.yaml", ns))
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply namespace configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "namespace", "apply")
		if err != nil {
			t.Fatalf("failed to apply namespace configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Verify namespaces were created in Kubernetes
		for _, ns := range testNamespaces {
			namespace, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			if err != nil {
				t.Errorf("namespace %s was not created: %v", ns, err)
			} else {
				t.Logf("Verified namespace exists: %s", namespace.Name)

				// Verify labels
				if namespace.Labels["managed-by"] != "personal-server" {
					t.Errorf("namespace %s missing expected label 'managed-by: personal-server'", ns)
				}
			}
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "namespace", "status")
		if err != nil {
			t.Fatalf("failed to get namespace status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains namespace names
		for _, ns := range testNamespaces {
			if !contains(output, ns) {
				t.Errorf("status output does not contain namespace: %s", ns)
			}
		}
	})

	// Test 4: Clean up namespaces
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "namespace", "clean")
		if err != nil {
			t.Fatalf("failed to clean namespaces: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		// Verify namespaces are being deleted or are gone
		for _, ns := range testNamespaces {
			namespace, err := client.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			if err == nil {
				// Namespace still exists, check if it's being deleted
				if namespace.DeletionTimestamp == nil {
					t.Errorf("namespace %s still exists and is not being deleted", ns)
				} else {
					t.Logf("Namespace %s is being deleted (expected)", ns)
				}
			} else {
				t.Logf("Namespace %s deleted successfully", ns)
			}
		}
	})
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
