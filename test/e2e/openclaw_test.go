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

// TestOpenClawE2E tests the complete openclaw module lifecycle
func TestOpenClawE2E(t *testing.T) {
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
		t.Log("Cleaning up openclaw resources...")
		ctx := context.Background()

		// Delete deployment
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, "openclaw", metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment openclaw: %v", err)
		} else {
			t.Logf("Deleted deployment: openclaw")
		}

		// Delete service
		err = client.CoreV1().Services(testNamespace).Delete(ctx, "openclaw", metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service openclaw: %v", err)
		} else {
			t.Logf("Deleted service: openclaw")
		}

		// Delete config PVC
		err = client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "openclaw-config-pvc", metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC openclaw-config-pvc: %v", err)
		} else {
			t.Logf("Deleted PVC: openclaw-config-pvc")
		}

		// Delete data PVC
		err = client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, "openclaw-data-pvc", metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC openclaw-data-pvc: %v", err)
		} else {
			t.Logf("Deleted PVC: openclaw-data-pvc")
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/openclaw"); err != nil {
			t.Logf("Warning: failed to remove configs/openclaw: %v", err)
		}
	}()

	// Test 1: Generate openclaw configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "openclaw", "generate")
		if err != nil {
			t.Fatalf("failed to generate openclaw configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"config-pvc.yaml", "data-pvc.yaml", "service.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "openclaw", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply openclaw configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "openclaw", "apply")
		if err != nil {
			t.Fatalf("failed to apply openclaw configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify config PVC was created
		configPVC, err := client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "openclaw-config-pvc", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC openclaw-config-pvc was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", configPVC.Name)
		}

		// Verify data PVC was created
		dataPVC, err := client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "openclaw-data-pvc", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC openclaw-data-pvc was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", dataPVC.Name)
		}

		// Verify service was created
		service, err := client.CoreV1().Services(testNamespace).Get(ctx, "openclaw", metav1.GetOptions{})
		if err != nil {
			t.Errorf("service openclaw was not created: %v", err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "openclaw", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment openclaw was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}

		// Add a sleep to wait for the pod to be ready
		time.Sleep(8 * time.Second)

		// Verify pod is running - get pod from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment openclaw: %v", err)
		} else if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment openclaw")
		} else {
			pod := &pods.Items[0]
			t.Logf("Verified pod exists: %s", pod.Name)

			if pod.Status.Phase != corev1.PodRunning {
				t.Logf("Warning: pod %s is not running: %s", pod.Name, pod.Status.Phase)
			} else {
				t.Logf("Verified pod is running: %s", pod.Name)
			}
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "openclaw", "status")
		if err != nil {
			t.Fatalf("failed to get openclaw status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "openclaw") {
			t.Errorf("status output does not contain deployment name: openclaw")
		}
	})

	// Test 4: Test backup command (note: may fail if pod is not ready)
	t.Run("Backup", func(t *testing.T) {
		testBackupCommand(t, "openclaw")
	})

	// Test 5: Clean up openclaw resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "openclaw", "clean")
		if err != nil {
			t.Fatalf("failed to clean openclaw: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "openclaw", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment openclaw still exists after clean")
		} else {
			t.Logf("Deployment openclaw deleted successfully")
		}

		// Verify service is deleted or being deleted
		_, err = client.CoreV1().Services(testNamespace).Get(ctx, "openclaw", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service openclaw still exists after clean")
		} else {
			t.Logf("Service openclaw deleted successfully")
		}

		// Verify config PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "openclaw-config-pvc", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC openclaw-config-pvc still exists after clean")
		} else {
			t.Logf("PVC openclaw-config-pvc deleted successfully")
		}

		// Verify data PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "openclaw-data-pvc", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC openclaw-data-pvc still exists after clean")
		} else {
			t.Logf("PVC openclaw-data-pvc deleted successfully")
		}
	})
}
