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

// TestGiteaE2E tests the complete gitea module lifecycle
func TestGiteaE2E(t *testing.T) {
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
		t.Log("Cleaning up gitea resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "gitea"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete service
		serviceName := "gitea"
		err = client.CoreV1().Services(testNamespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service %s: %v", serviceName, err)
		} else {
			t.Logf("Deleted service: %s", serviceName)
		}

		// Delete secret
		secretName := "gitea-secrets"
		err = client.CoreV1().Secrets(testNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete secret %s: %v", secretName, err)
		} else {
			t.Logf("Deleted secret: %s", secretName)
		}

		// Delete PVC
		pvcName := "gitea-data-pvc"
		err = client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC %s: %v", pvcName, err)
		} else {
			t.Logf("Deleted PVC: %s", pvcName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/gitea"); err != nil {
			t.Logf("Warning: failed to remove configs/gitea: %v", err)
		}
	}()

	// Test 1: Generate gitea configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "gitea", "generate")
		if err != nil {
			t.Fatalf("failed to generate gitea configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"secret.yaml", "pvc.yaml", "service.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "gitea", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply gitea configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "gitea", "apply")
		if err != nil {
			t.Fatalf("failed to apply gitea configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify Secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, "gitea-secrets", metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret gitea-secrets was not created: %v", err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
		}

		// Verify PVC was created
		pvc, err := client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "gitea-data-pvc", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC gitea-data-pvc was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", pvc.Name)
		}

		// Verify service was created
		service, err := client.CoreV1().Services(testNamespace).Get(ctx, "gitea", metav1.GetOptions{})
		if err != nil {
			t.Errorf("service gitea was not created: %v", err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)
			// Verify service has both HTTP and SSH ports
			if len(service.Spec.Ports) != 2 {
				t.Errorf("service has %d ports, expected 2 (HTTP and SSH)", len(service.Spec.Ports))
			}
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "gitea", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment gitea was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}

		// Add a sleep to wait for the pod to be ready
		time.Sleep(120 * time.Second)

		// Verify pod is running - get pod from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment gitea: %v", err)
		} else if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment gitea")
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
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "gitea", "status")
		if err != nil {
			t.Fatalf("failed to get gitea status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "gitea") {
			t.Errorf("status output does not contain deployment name: gitea")
		}
	})

	// Test 4: Test backup command (note: may fail if pod is not ready)
	t.Run("Backup", func(t *testing.T) {
		testBackupCommand(t, "gitea")
	})

	// Test 5: Clean up gitea resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "gitea", "clean")
		if err != nil {
			t.Fatalf("failed to clean gitea: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "gitea", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment gitea still exists after clean")
		} else {
			t.Logf("Deployment gitea deleted successfully")
		}

		// Verify service is deleted or being deleted
		_, err = client.CoreV1().Services(testNamespace).Get(ctx, "gitea", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service gitea still exists after clean")
		} else {
			t.Logf("Service gitea deleted successfully")
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, "gitea-secrets", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret gitea-secrets still exists after clean")
		} else {
			t.Logf("Secret gitea-secrets deleted successfully")
		}

		// Verify PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "gitea-data-pvc", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC gitea-data-pvc still exists after clean")
		} else {
			t.Logf("PVC gitea-data-pvc deleted successfully")
		}
	})
}

