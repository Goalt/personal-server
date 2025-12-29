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

// TestWebdavE2E tests the complete webdav module lifecycle
func TestWebdavE2E(t *testing.T) {
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
		t.Log("Cleaning up webdav resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "webdav"
		// Using testNamespace constant
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete service
		serviceName := "webdav-service"
		err = client.CoreV1().Services(testNamespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service %s: %v", serviceName, err)
		} else {
			t.Logf("Deleted service: %s", serviceName)
		}

		// Delete PVC
		pvcName := "webdav-data-pvc"
		err = client.CoreV1().PersistentVolumeClaims(testNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete PVC %s: %v", pvcName, err)
		} else {
			t.Logf("Deleted PVC: %s", pvcName)
		}

		// Delete secret
		secretName := "webdav-secrets"
		err = client.CoreV1().Secrets(testNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete secret %s: %v", secretName, err)
		} else {
			t.Logf("Deleted secret: %s", secretName)
		}

		// Delete configmap
		configmapName := "webdav-config"
		err = client.CoreV1().ConfigMaps(testNamespace).Delete(ctx, configmapName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete configmap %s: %v", configmapName, err)
		} else {
			t.Logf("Deleted configmap: %s", configmapName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/webdav"); err != nil {
			t.Logf("Warning: failed to remove configs/webdav: %v", err)
		}
	}()

	// Test 1: Generate webdav configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "webdav", "generate")
		if err != nil {
			t.Fatalf("failed to generate webdav configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"configmap.yaml", "secret.yaml", "pvc.yaml", "service.yaml", "deployment.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "webdav", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply webdav configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "webdav", "apply")
		if err != nil {
			t.Fatalf("failed to apply webdav configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()
		// Using testNamespace constant

		// Verify configmap was created
		configmap, err := client.CoreV1().ConfigMaps(testNamespace).Get(ctx, "webdav-config", metav1.GetOptions{})
		if err != nil {
			t.Errorf("configmap webdav-config was not created: %v", err)
		} else {
			t.Logf("Verified configmap exists: %s", configmap.Name)
		}

		// Verify secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, "webdav-secrets", metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret webdav-secrets was not created: %v", err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
		}

		// Verify PVC was created
		pvc, err := client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "webdav-data-pvc", metav1.GetOptions{})
		if err != nil {
			t.Errorf("PVC webdav-data-pvc was not created: %v", err)
		} else {
			t.Logf("Verified PVC exists: %s", pvc.Name)
		}

		// Verify service was created
		service, err := client.CoreV1().Services(testNamespace).Get(ctx, "webdav-service", metav1.GetOptions{})
		if err != nil {
			t.Errorf("service webdav-service was not created: %v", err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "webdav", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment webdav was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "webdav", "status")
		if err != nil {
			t.Fatalf("failed to get webdav status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "webdav") {
			t.Errorf("status output does not contain deployment name: webdav")
		}
	})

	// Test 4: Test backup command (note: may fail if pod is not ready)
	t.Run("Backup", func(t *testing.T) {
		testBackupCommand(t, "webdav")
	})

	// Test 5: Clean up webdav resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, binaryPath, "-config", testConfigPath, "webdav", "clean")
		if err != nil {
			t.Fatalf("failed to clean webdav: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()
		// Using testNamespace constant

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "webdav", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment webdav still exists after clean")
		} else {
			t.Logf("Deployment webdav deleted successfully")
		}

		// Verify service is deleted or being deleted
		_, err = client.CoreV1().Services(testNamespace).Get(ctx, "webdav-service", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service webdav-service still exists after clean")
		} else {
			t.Logf("Service webdav-service deleted successfully")
		}

		// Verify PVC is deleted or being deleted
		_, err = client.CoreV1().PersistentVolumeClaims(testNamespace).Get(ctx, "webdav-data-pvc", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: PVC webdav-data-pvc still exists after clean")
		} else {
			t.Logf("PVC webdav-data-pvc deleted successfully")
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, "webdav-secrets", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret webdav-secrets still exists after clean")
		} else {
			t.Logf("Secret webdav-secrets deleted successfully")
		}

		// Verify configmap is deleted or being deleted
		_, err = client.CoreV1().ConfigMaps(testNamespace).Get(ctx, "webdav-config", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: configmap webdav-config still exists after clean")
		} else {
			t.Logf("Configmap webdav-config deleted successfully")
		}
	})
}
