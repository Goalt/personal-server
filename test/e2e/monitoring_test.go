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

// TestMonitoringE2E tests the complete monitoring module lifecycle
func TestMonitoringE2E(t *testing.T) {
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
		t.Log("Cleaning up monitoring resources...")
		ctx := context.Background()

		resourceName := "monitor-sentry-kubernetes"

		// Delete deployment
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, resourceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", resourceName, err)
		} else {
			t.Logf("Deleted deployment: %s", resourceName)
		}

		// Delete secret
		err = client.CoreV1().Secrets(testNamespace).Delete(ctx, resourceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete secret %s: %v", resourceName, err)
		} else {
			t.Logf("Deleted secret: %s", resourceName)
		}

		// Delete cluster role binding
		err = client.RbacV1().ClusterRoleBindings().Delete(ctx, resourceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete cluster role binding %s: %v", resourceName, err)
		} else {
			t.Logf("Deleted cluster role binding: %s", resourceName)
		}

		// Delete cluster role
		err = client.RbacV1().ClusterRoles().Delete(ctx, resourceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete cluster role %s: %v", resourceName, err)
		} else {
			t.Logf("Deleted cluster role: %s", resourceName)
		}

		// Delete service account
		err = client.CoreV1().ServiceAccounts(testNamespace).Delete(ctx, resourceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service account %s: %v", resourceName, err)
		} else {
			t.Logf("Deleted service account: %s", resourceName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/monitoring"); err != nil {
			t.Logf("Warning: failed to remove configs/monitoring: %v", err)
		}
	}()

	// Test 1: Generate monitoring configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "monitoring", "generate")
		if err != nil {
			t.Fatalf("failed to generate monitoring configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{
			"serviceaccount.yaml",
			"clusterrole.yaml",
			"clusterrolebinding.yaml",
			"secret.yaml",
			"deployment.yaml",
		}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "monitoring", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply monitoring configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "monitoring", "apply")
		if err != nil {
			t.Fatalf("failed to apply monitoring configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()
		resourceName := "monitor-sentry-kubernetes"

		// Verify service account was created
		sa, err := client.CoreV1().ServiceAccounts(testNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("service account %s was not created: %v", resourceName, err)
		} else {
			t.Logf("Verified service account exists: %s", sa.Name)
		}

		// Verify cluster role was created
		cr, err := client.RbacV1().ClusterRoles().Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("cluster role %s was not created: %v", resourceName, err)
		} else {
			t.Logf("Verified cluster role exists: %s", cr.Name)
		}

		// Verify cluster role binding was created
		crb, err := client.RbacV1().ClusterRoleBindings().Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("cluster role binding %s was not created: %v", resourceName, err)
		} else {
			t.Logf("Verified cluster role binding exists: %s", crb.Name)
		}

		// Verify secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret %s was not created: %v", resourceName, err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
			// Verify secret contains the sentry.dsn key
			if _, ok := secret.Data["sentry.dsn"]; !ok {
				t.Errorf("secret %s does not contain sentry.dsn key", resourceName)
			}
		}

		// Verify deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment %s was not created: %v", resourceName, err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}

		// Wait for pod to be created
		time.Sleep(5 * time.Second)

		// Verify pod is created from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment %s: %v", resourceName, err)
		}
		if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment %s", resourceName)
		} else {
			t.Logf("Verified pod exists: %s (Phase: %s)", pods.Items[0].Name, pods.Items[0].Status.Phase)
			// Note: We don't require the pod to be Running since it needs a valid Sentry DSN
			// and might be in CrashLoopBackOff in test environments
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "monitoring", "status")
		if err != nil {
			t.Fatalf("failed to get monitoring status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains resource names
		expectedStrings := []string{
			"monitor-sentry-kubernetes",
			"SERVICE ACCOUNT",
			"CLUSTER ROLE",
			"CLUSTER ROLE BINDING",
			"SECRET",
			"DEPLOYMENT",
			"PODS",
		}
		for _, expected := range expectedStrings {
			if !strings.Contains(output, expected) {
				t.Errorf("status output does not contain expected string: %s", expected)
			}
		}
	})

	// Test 4: Test Apply idempotency (should fail on second apply)
	t.Run("ApplyIdempotency", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "monitoring", "apply")
		if err == nil {
			t.Errorf("expected apply to fail on second run, but it succeeded")
		} else {
			t.Logf("Apply correctly failed on second run (expected behavior): %v", err)
			t.Logf("Output:\n%s", output)
			// Verify error message indicates resources already exist
			if !strings.Contains(output, "already exists") {
				t.Errorf("expected error message to mention resources already exist, got: %s", output)
			}
		}
	})

	// Test 5: Clean up monitoring resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "monitoring", "clean")
		if err != nil {
			t.Fatalf("failed to clean monitoring: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()
		resourceName := "monitor-sentry-kubernetes"

		// Verify deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment %s still exists after clean", resourceName)
		} else {
			t.Logf("Deployment %s deleted successfully", resourceName)
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret %s still exists after clean", resourceName)
		} else {
			t.Logf("Secret %s deleted successfully", resourceName)
		}

		// Verify cluster role binding is deleted or being deleted
		_, err = client.RbacV1().ClusterRoleBindings().Get(ctx, resourceName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: cluster role binding %s still exists after clean", resourceName)
		} else {
			t.Logf("Cluster role binding %s deleted successfully", resourceName)
		}

		// Verify cluster role is deleted or being deleted
		_, err = client.RbacV1().ClusterRoles().Get(ctx, resourceName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: cluster role %s still exists after clean", resourceName)
		} else {
			t.Logf("Cluster role %s deleted successfully", resourceName)
		}

		// Verify service account is deleted or being deleted
		_, err = client.CoreV1().ServiceAccounts(testNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service account %s still exists after clean", resourceName)
		} else {
			t.Logf("Service account %s deleted successfully", resourceName)
		}
	})
}


