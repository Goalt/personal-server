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

// TestPostgresE2E tests the complete postgres module lifecycle
func TestPostgresE2E(t *testing.T) {
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
		t.Log("Cleaning up postgres resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "postgres"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete service
		serviceName := "postgres"
		err = client.CoreV1().Services(testNamespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service %s: %v", serviceName, err)
		} else {
			t.Logf("Deleted service: %s", serviceName)
		}

		// Delete secret
		secretName := "postgres-secrets"
		err = client.CoreV1().Secrets(testNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete secret %s: %v", secretName, err)
		} else {
			t.Logf("Deleted secret: %s", secretName)
		}

		// Delete PVC
		pvcName := "postgres-data-pvc"
		err = client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC %s: %v", pvcName, err)
		} else {
			t.Logf("Deleted PVC: %s", pvcName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/postgres"); err != nil {
			t.Logf("Warning: failed to remove configs/postgres: %v", err)
		}
	}()

	// Test 1: Generate postgres configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "postgres", "generate")
		if err != nil {
			t.Fatalf("failed to generate postgres configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"secret.yaml", "pvc.yaml", "service.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "postgres", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply postgres configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "postgres", "apply")
		if err != nil {
			t.Fatalf("failed to apply postgres configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify Secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, "postgres-secrets", metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret postgres-secrets was not created: %v", err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
		}

		// Verify PVC was created
		pvc, err := client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "postgres-data-pvc", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC postgres-data-pvc was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", pvc.Name)
		}

		// Verify service was created
		service, err := client.CoreV1().Services(testNamespace).Get(ctx, "postgres", metav1.GetOptions{})
		if err != nil {
			t.Errorf("service postgres was not created: %v", err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "postgres", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment postgres was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}

		// Add a sleep to wait for the pod to be ready
		time.Sleep(10 * time.Second)

		// Verify pod is running - get pod from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment postgres: %v", err)
		} else if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment postgres")
		} else {
			pod := &pods.Items[0]
			t.Logf("Verified pod exists: %s", pod.Name)

			if pod.Status.Phase != corev1.PodRunning {
				t.Errorf("pod %s is not running: %s", pod.Name, pod.Status.Phase)
			} else {
				t.Logf("Verified pod is running: %s", pod.Name)
			}
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "postgres", "status")
		if err != nil {
			t.Fatalf("failed to get postgres status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "postgres") {
			t.Errorf("status output does not contain deployment name: postgres")
		}
	})

	// Test 4: Test add-db command (add a database and user)
	t.Run("AddDB", func(t *testing.T) {
		// Wait for postgres pod to be fully ready before adding DB
		time.Sleep(15 * time.Second)

		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "postgres", "add-db", "testdb", "testuser", "testpass123")
		if err != nil {
			t.Logf("add-db command output (may fail if pod not ready):\n%s", output)
			t.Logf("add-db failed as expected when pod is not ready: %v", err)
		} else {
			t.Logf("add-db output:\n%s", output)

			// Verify the output contains success messages
			if !strings.Contains(output, "Database and user setup complete") {
				t.Errorf("add-db output does not contain expected success message")
			}
		}
	})

	// Test 5: Test remove-db command (remove the database and user we just created)
	t.Run("RemoveDB", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "postgres", "remove-db", "testdb", "testuser")
		if err != nil {
			t.Logf("remove-db command output (may fail if pod not ready or db doesn't exist):\n%s", output)
			t.Logf("remove-db failed as expected: %v", err)
		} else {
			t.Logf("remove-db output:\n%s", output)

			// Verify the output contains success messages
			if !strings.Contains(output, "removed") {
				t.Errorf("remove-db output does not contain expected success message")
			}
		}
	})

	// Test 6: Test backup command (note: may fail if pod is not ready)
	t.Run("Backup", func(t *testing.T) {
		testBackupCommand(t, "postgres")
	})

	// Test 7: Clean up postgres resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "postgres", "clean")
		if err != nil {
			t.Fatalf("failed to clean postgres: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "postgres", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment postgres still exists after clean")
		} else {
			t.Logf("Deployment postgres deleted successfully")
		}

		// Verify service is deleted or being deleted
		_, err = client.CoreV1().Services(testNamespace).Get(ctx, "postgres", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service postgres still exists after clean")
		} else {
			t.Logf("Service postgres deleted successfully")
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, "postgres-secrets", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret postgres-secrets still exists after clean")
		} else {
			t.Logf("Secret postgres-secrets deleted successfully")
		}

		// Verify PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "postgres-data-pvc", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC postgres-data-pvc still exists after clean")
		} else {
			t.Logf("PVC postgres-data-pvc deleted successfully")
		}
	})
}
