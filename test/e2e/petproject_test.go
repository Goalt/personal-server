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

// TestPetProjectE2E tests the complete petproject module lifecycle
func TestPetProjectE2E(t *testing.T) {
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
		t.Log("Cleaning up petproject resources...")
		ctx := context.Background()

		// Delete deployment
		deploymentName := "pet-test-pet-project"
		err := client.AppsV1().Deployments(testNamespace).Delete(ctx, deploymentName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete deployment %s: %v", deploymentName, err)
		} else {
			t.Logf("Deleted deployment: %s", deploymentName)
		}

		// Delete service
		serviceName := "pet-test-pet-project"
		err = client.CoreV1().Services(testNamespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
		if err != nil {
			t.Logf("Warning: failed to delete service %s: %v", serviceName, err)
		} else {
			t.Logf("Deleted service: %s", serviceName)
		}

		// Clean up generated configs
		if err := os.RemoveAll("configs/pet-projects"); err != nil {
			t.Logf("Warning: failed to remove configs/pet-projects: %v", err)
		}
	}()

	// Test 1: Generate petproject configurations
	t.Run("Generate", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "generate")
		if err != nil {
			t.Fatalf("failed to generate petproject configs: %v", err)
		}
		t.Logf("Generate output:\n%s", output)

		// Verify generated files exist
		expectedFiles := []string{"deployment.yaml", "service.yaml"}
		for _, file := range expectedFiles {
			configFile := filepath.Join("configs", "pet-projects", "test-pet-project", file)
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("expected config file %s was not generated", configFile)
			} else {
				t.Logf("Generated config file: %s", configFile)
			}
		}
	})

	// Test 2: Apply petproject configurations
	t.Run("Apply", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "apply")
		if err != nil {
			t.Fatalf("failed to apply petproject configs: %v", err)
		}
		t.Logf("Apply output:\n%s", output)

		// Wait a moment for resources to be created
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment was created
		deploymentName := "pet-test-pet-project"
		deployment, err := client.AppsV1().Deployments(testNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("deployment %s was not created: %v", deploymentName, err)
		} else {
			t.Logf("Verified deployment exists: %s", deployment.Name)

			// Verify deployment has correct labels
			if deployment.Labels["type"] != "pet-project" {
				t.Errorf("deployment missing 'type: pet-project' label")
			}
			if deployment.Labels["managed-by"] != "personal-server" {
				t.Errorf("deployment missing 'managed-by: personal-server' label")
			}
		}

		// Verify service was created (since we configured it with ports)
		serviceName := "pet-test-pet-project"
		service, err := client.CoreV1().Services(testNamespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("service %s was not created: %v", serviceName, err)
		} else {
			t.Logf("Verified service exists: %s", service.Name)

			// Verify service has correct labels
			if service.Labels["type"] != "pet-project" {
				t.Errorf("service missing 'type: pet-project' label")
			}

			// Verify service has correct port
			if len(service.Spec.Ports) == 0 {
				t.Errorf("service has no ports configured")
			} else {
				port := service.Spec.Ports[0]
				if port.Name != "http" {
					t.Errorf("expected port name 'http', got '%s'", port.Name)
				}
				if port.Port != 80 {
					t.Errorf("expected port 80, got %d", port.Port)
				}
				if port.TargetPort.IntVal != 80 {
					t.Errorf("expected target port 80, got %d", port.TargetPort.IntVal)
				}
			}
		}

		// Add a sleep to wait for the pod to be ready
		time.Sleep(8 * time.Second)

		// Verify pod is running - get pod from deployment
		labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
		pods, err := client.CoreV1().Pods(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			t.Errorf("failed to list pods for deployment %s: %v", deploymentName, err)
		} else if len(pods.Items) == 0 {
			t.Errorf("no pods found for deployment %s", deploymentName)
		} else {
			pod := &pods.Items[0]
			t.Logf("Verified pod exists: %s", pod.Name)

			// Verify pod has correct labels
			if pod.Labels["type"] != "pet-project" {
				t.Errorf("pod missing 'type: pet-project' label")
			}

			// Verify environment variables
			if len(pod.Spec.Containers) > 0 {
				container := pod.Spec.Containers[0]
				envMap := make(map[string]string)
				for _, env := range container.Env {
					envMap[env.Name] = env.Value
				}

				if envMap["TEST_ENV"] != "test-value" {
					t.Errorf("expected TEST_ENV=test-value, got %s", envMap["TEST_ENV"])
				}
				if envMap["ANOTHER_VAR"] != "another-value" {
					t.Errorf("expected ANOTHER_VAR=another-value, got %s", envMap["ANOTHER_VAR"])
				}
			}

			if pod.Status.Phase != corev1.PodRunning {
				t.Logf("Warning: pod %s is not running: %s", pod.Name, pod.Status.Phase)
			} else {
				t.Logf("Verified pod is running: %s", pod.Name)
			}
		}
	})

	// Test 3: Check status
	t.Run("Status", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "status")
		if err != nil {
			t.Fatalf("failed to get petproject status: %v", err)
		}
		t.Logf("Status output:\n%s", output)

		// Verify output contains deployment name
		if !strings.Contains(output, "pet-test-pet-project") {
			t.Errorf("status output does not contain deployment name: pet-test-pet-project")
		}

		// Verify output mentions service (since we have one)
		if !strings.Contains(strings.ToUpper(output), "SERVICE") {
			t.Errorf("status output should contain SERVICE section")
		}
	})

	// Test 4: Test rollout restart
	t.Run("RolloutRestart", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "rollout", "restart")
		if err != nil {
			t.Fatalf("failed to rollout restart petproject: %v", err)
		}
		t.Logf("Rollout restart output:\n%s", output)

		// Verify the command succeeded
		if !strings.Contains(output, "completed successfully") && !strings.Contains(output, "restarted") {
			t.Logf("Warning: rollout restart output doesn't indicate success")
		}
	})

	// Test 5: Test rollout status
	t.Run("RolloutStatus", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "rollout", "status")
		if err != nil {
			// Rollout status might fail if deployment is still in progress, which is okay
			t.Logf("Rollout status output (may fail if in progress):\n%s", output)
			t.Logf("Rollout status failed (expected if deployment in progress): %v", err)
		} else {
			t.Logf("Rollout status output:\n%s", output)
		}
	})

	// Test 6: Test apply idempotency (should fail as deployment already exists)
	t.Run("ApplyIdempotency", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "apply")
		if err == nil {
			t.Errorf("expected apply to fail when deployment already exists, but it succeeded")
		} else {
			t.Logf("Apply correctly failed when deployment exists:\n%s", output)
			if !strings.Contains(output, "already exists") {
				t.Errorf("error message should mention 'already exists'")
			}
		}
	})

	// Test 7: Clean up petproject resources
	t.Run("Clean", func(t *testing.T) {
		output, err := runCommand(t, fullBinaryPath, "-config", fullConfigPath, "test-pet-project", "clean")
		if err != nil {
			t.Fatalf("failed to clean petproject: %v", err)
		}
		t.Logf("Clean output:\n%s", output)

		// Wait a moment for deletion to start
		time.Sleep(2 * time.Second)

		ctx := context.Background()

		// Verify deployment is deleted or being deleted
		deploymentName := "pet-test-pet-project"
		_, err = client.AppsV1().Deployments(testNamespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: deployment %s still exists after clean", deploymentName)
		} else {
			t.Logf("Deployment %s deleted successfully", deploymentName)
		}

		// Verify service is deleted or being deleted
		serviceName := "pet-test-pet-project"
		_, err = client.CoreV1().Services(testNamespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err == nil {
			t.Logf("Warning: service %s still exists after clean", serviceName)
		} else {
			t.Logf("Service %s deleted successfully", serviceName)
		}
	})
}

