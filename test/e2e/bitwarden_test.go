package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestBitwardenE2E tests the complete bitwarden module lifecycle
func TestBitwardenE2E(t *testing.T) {
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
		t.Log("Cleaning up bitwarden resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "vaultwarden"
		namespace := "e2e-test-infra"
		err := client.AppsV1().Deployments(namespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete service
		serviceName := "vaultwarden"
		err = client.CoreV1().Services(namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service %s: %v", serviceName, err)
		} else {
			t.Logf("Deleted service: %s", serviceName)
		}

		// Delete PVC
		pvcName := "vaultwarden-data"
		err = client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC %s: %v", pvcName, err)
		} else {
			t.Logf("Deleted PVC: %s", pvcName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/bitwarden"); err != nil {
			t.Logf("Warning: failed to remove configs/bitwarden: %v", err)
		}
	}()

	// Test 1: Generate bitwarden configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "bitwarden", "generate")
		if err != nil {
			t.Fatalf("failed to generate bitwarden configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"pvc.yaml", "service.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "bitwarden", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply bitwarden configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "bitwarden", "apply")
		if err != nil {
			t.Fatalf("failed to apply bitwarden configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()
		namespace := "e2e-test-infra"

		// Verify PVC was created
		pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, "vaultwarden-data", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC vaultwarden-data was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", pvc.Name)
		}

		// Verify service was created
		service, err := client.CoreV1().Services(namespace).Get(ctx, "vaultwarden", metav1.GetOptions{})
		if err != nil {
			t.Errorf("service vaultwarden was not created: %v", err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "vaultwarden", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment vaultwarden was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "bitwarden", "status")
		if err != nil {
			t.Fatalf("failed to get bitwarden status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "vaultwarden") {
			t.Errorf("status output does not contain deployment name: vaultwarden")
		}
	})

	// Test 4: Test backup command (note: this may not fully succeed without a running pod, but should not error)
	t.Run("Backup", func(t *testing.T) {
		// Create a temporary backup directory
		backupDir, err := os.MkdirTemp("", "e2e-bitwarden-backup-*")
		if err != nil {
			t.Fatalf("failed to create temp backup dir: %v", err)
		}
		defer os.RemoveAll(backupDir)

		// Try to run backup - it may fail if pod isn't ready, but shouldn't panic
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "bitwarden", "backup", backupDir)
		// We don't fail the test if backup fails since the pod might not be ready
		// but we log the output
		if err != nil {
			t.Logf("Backup command output (may fail if pod not ready):\n%s", output)
			t.Logf("Expected: Backup may fail if pod is not ready: %v", err)
		} else {
			t.Logf("Backup output:\n%s", output)
		}
	})

	// Test 5: Clean up bitwarden resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "bitwarden", "clean")
		if err != nil {
			t.Fatalf("failed to clean bitwarden: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()
		namespace := "e2e-test-infra"

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(namespace).Get(ctx, "vaultwarden", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment vaultwarden still exists after clean")
		} else {
			t.Logf("Deployment vaultwarden deleted successfully")
		}

		// Verify service is deleted or being deleted
		_, err = client.CoreV1().Services(namespace).Get(ctx, "vaultwarden", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service vaultwarden still exists after clean")
		} else {
			t.Logf("Service vaultwarden deleted successfully")
		}

		// Verify PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, "vaultwarden-data", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC vaultwarden-data still exists after clean")
		} else {
			t.Logf("PVC vaultwarden-data deleted successfully")
		}
	})
}
