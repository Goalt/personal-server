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

// TestHobbypodE2E tests the complete hobbypod module lifecycle
func TestHobbypodE2E(t *testing.T) {
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
		t.Log("Cleaning up hobbypod resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "hobby-pod"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete PVC
		pvcName := "hobby-storage-pvc"
		err = client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC %s: %v", pvcName, err)
		} else {
			t.Logf("Deleted PVC: %s", pvcName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/hobbypod"); err != nil {
			t.Logf("Warning: failed to remove configs/hobbypod: %v", err)
		}
	}()

	// Test 1: Generate hobbypod configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "hobby-pod", "generate")
		if err != nil {
			t.Fatalf("failed to generate hobbypod configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"pvc.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "hobbypod", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply hobbypod configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "hobby-pod", "apply")
		if err != nil {
			t.Fatalf("failed to apply hobbypod configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify PVC was created
		pvc, err := client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "hobby-storage-pvc", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC hobby-storage-pvc was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", pvc.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "hobby-pod", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment hobby-pod was not created: %v", err)
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
			t.Errorf("failed to list pods for deployment hobby-pod: %v", err)
		} else if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment hobby-pod")
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
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "hobby-pod", "status")
		if err != nil {
			t.Fatalf("failed to get hobbypod status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "hobby-pod") {
			t.Errorf("status output does not contain deployment name: hobby-pod")
		}
	})

	// Test 4: Test backup command (note: may fail if pod is not ready)
	t.Run("Backup", func(t *testing.T) {
		testBackupCommand(t, "hobby-pod")
	})

	// Test 5: Clean up hobbypod resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "hobby-pod", "clean")
		if err != nil {
			t.Fatalf("failed to clean hobbypod: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "hobby-pod", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment hobby-pod still exists after clean")
		} else {
			t.Logf("Deployment hobby-pod deleted successfully")
		}

		// Verify PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "hobby-storage-pvc", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC hobby-storage-pvc still exists after clean")
		} else {
			t.Logf("PVC hobby-storage-pvc deleted successfully")
		}
	})
}

