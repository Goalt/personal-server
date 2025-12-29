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

// TestDroneE2E tests the complete drone module lifecycle
func TestDroneE2E(t *testing.T) {
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
		t.Log("Cleaning up drone resources...")
		ctx := context.Background()

		// Delete drone server deployment
		deploymentName := "drone"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete drone runner deployment
		runnerDeploymentName := "drone-runner"
		err = client.AppsV1().Deployments(testNamespace).Delete(ctx, runnerDeploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", runnerDeploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", runnerDeploymentName)
		}

		// Delete service
		serviceName := "drone"
		err = client.CoreV1().Services(testNamespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service %s: %v", serviceName, err)
		} else {
			t.Logf("Deleted service: %s", serviceName)
		}

		// Delete secret
		secretName := "drone-secrets"
		err = client.CoreV1().Secrets(testNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete secret %s: %v", secretName, err)
		} else {
			t.Logf("Deleted secret: %s", secretName)
		}

		// Delete role
		roleName := "drone"
		err = client.RbacV1().Roles(testNamespace).Delete(ctx, roleName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete role %s: %v", roleName, err)
		} else {
			t.Logf("Deleted role: %s", roleName)
		}

		// Delete rolebinding
		roleBindingName := "drone"
		err = client.RbacV1().RoleBindings(testNamespace).Delete(ctx, roleBindingName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete rolebinding %s: %v", roleBindingName, err)
		} else {
			t.Logf("Deleted rolebinding: %s", roleBindingName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/drone"); err != nil {
			t.Logf("Warning: failed to remove configs/drone: %v", err)
		}
	}()

	// Test 1: Generate drone configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "drone", "generate")
		if err != nil {
			t.Fatalf("failed to generate drone configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{
			"secret.yaml",
			"role.yaml",
			"rolebinding.yaml",
			"deployment.yaml",
			"runner-deployment.yaml",
			"service.yaml",
		}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "drone", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply drone configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "drone", "apply")
		if err != nil {
			t.Fatalf("failed to apply drone configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify Secret was created
		secret, err := client.CoreV1().Secrets(testNamespace).Get(ctx, "drone-secrets", metav1.GetOptions{})
		if err != nil {
			t.Errorf("secret drone-secrets was not created: %v", err)
		} else {
			t.Logf("Verified secret exists: %s", secret.Name)
			// Verify secret contains expected keys
			expectedKeys := []string{
				"drone_gitea_server",
				"drone_gitea_client_id",
				"drone_gitea_client_secret",
				"drone_rpc_secret",
				"drone_server_host",
				"drone_server_proto",
			}
			for _, key := range expectedKeys {
				if _, ok := secret.Data[key]; !ok {
					t.Errorf("secret is missing key: %s", key)
				}
			}
		}

		// Verify Role was created
		role, err := client.RbacV1().Roles(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err != nil {
			t.Errorf("role drone was not created: %v", err)
		} else {
			t.Logf("Verified role exists: %s", role.Name)
		}

		// Verify RoleBinding was created
		roleBinding, err := client.RbacV1().RoleBindings(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err != nil {
			t.Errorf("rolebinding drone was not created: %v", err)
		} else {
			t.Logf("Verified rolebinding exists: %s", roleBinding.Name)
		}

		// Verify service was created
		service, err := client.CoreV1().Services(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err != nil {
			t.Errorf("service drone was not created: %v", err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)
			// Verify service has HTTP port
			if len(service.Spec.Ports) != 1 {
				t.Errorf("service has %d ports, expected 1 (HTTP)", len(service.Spec.Ports))
			}
		}

		// Verify drone server deployment was created
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment drone was not created: %v", err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)
		}

		// Verify drone runner deployment was created
		runnerDeployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, "drone-runner", metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment drone-runner was not created: %v", err)
		} else {
			t.Logf("Verified runner deployment exists: %s", runnerDeployment.Name)
		}

		// Add a sleep to wait for the pods to be ready
		time.Sleep(10 * time.Second)

		// Verify drone server pod is running - get pod from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment drone: %v", err)
		} else if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment drone")
		} else {
			pod := &pods.Items[0]
			t.Logf("Verified drone server pod exists: %s", pod.Name)

			if pod.Status.Phase != corev1.PodRunning {
				t.Logf("Warning: pod %s is not running: %s", pod.Name, pod.Status.Phase)
			} else {
				t.Logf("Verified drone server pod is running: %s", pod.Name)
			}
		}

		// Verify drone runner pod exists
		runnerLabelSelector := metav1.FormatLabelSelector(runnerDeployment.Spec.Selector)
		runnerPods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: runnerLabelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment drone-runner: %v", err)
		} else if len(runnerPods.Items) == 0 {
			t.Errorf("no pods found for deployment drone-runner")
		} else {
			runnerPod := &runnerPods.Items[0]
			t.Logf("Verified drone runner pod exists: %s", runnerPod.Name)

			if runnerPod.Status.Phase != corev1.PodRunning {
				t.Logf("Warning: runner pod %s is not running: %s", runnerPod.Name, runnerPod.Status.Phase)
			} else {
				t.Logf("Verified drone runner pod is running: %s", runnerPod.Name)
			}
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "drone", "status")
		if err != nil {
			t.Fatalf("failed to get drone status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment names
		if !strings.Contains(output, "drone") {
			t.Errorf("status output does not contain deployment name: drone")
		}
		if !strings.Contains(output, "drone-runner") {
			t.Errorf("status output does not contain deployment name: drone-runner")
		}
	})

	// Test 4: Clean up drone resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "drone", "clean")
		if err != nil {
			t.Fatalf("failed to clean drone: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify drone server deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment drone still exists after clean")
		} else {
			t.Logf("Deployment drone deleted successfully")
		}

		// Verify drone runner deployment is deleted or being deleted
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, "drone-runner", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment drone-runner still exists after clean")
		} else {
			t.Logf("Deployment drone-runner deleted successfully")
		}

		// Verify service is deleted or being deleted
		_, err = client.CoreV1().Services(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service drone still exists after clean")
		} else {
			t.Logf("Service drone deleted successfully")
		}

		// Verify secret is deleted or being deleted
		_, err = client.CoreV1().Secrets(testNamespace).Get(ctx, "drone-secrets", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: secret drone-secrets still exists after clean")
		} else {
			t.Logf("Secret drone-secrets deleted successfully")
		}

		// Verify role is deleted or being deleted
		_, err = client.RbacV1().Roles(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: role drone still exists after clean")
		} else {
			t.Logf("Role drone deleted successfully")
		}

		// Verify rolebinding is deleted or being deleted
		_, err = client.RbacV1().RoleBindings(testNamespace).Get(ctx, "drone", metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: rolebinding drone still exists after clean")
		} else {
			t.Logf("RoleBinding drone deleted successfully")
		}
	})
}

