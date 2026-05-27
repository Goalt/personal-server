package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	testConfigPath = "test/e2e/test-config.yaml"
	binaryPath     = "bin/personal-server"
	timeout        = 5 * time.Minute

	// Test namespace used for deploying test resources
	testNamespace = "e2e-test-infra"
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

// testBackupCommand tests a module's backup functionality
// It creates a temporary backup directory and runs the backup command
// Returns true if backup succeeded, false if it failed (expected when pod is not ready)
func testBackupCommand(t *testing.T, moduleName string) bool {
	t.Helper()

	// Create a temporary backup directory
	backupDir, err := os.MkdirTemp("", fmt.Sprintf("e2e-%s-backup-*", moduleName))
	if err != nil {
		t.Fatalf("failed to create temp backup dir: %v", err)
	}
	defer os.RemoveAll(backupDir)

	// Try to run backup - expect it to fail if pod isn't ready
	output, err := runCommand(t, binaryPath, "-config", testConfigPath, moduleName, "backup", backupDir)
	if err != nil {
		t.Logf("Backup command output (expected to fail if pod not ready):\n%s", output)
		t.Logf("Backup failed as expected when pod is not ready: %v", err)
		return false
	}
	t.Logf("Backup output:\n%s", output)
	return true
}

// waitForPodRunning waits until at least one pod matching labelSelector in
// namespace ns reaches the Running phase, or until timeout is exceeded.
// Fails the test if no pod becomes Running in time.
func waitForPodRunning(t *testing.T, client *kubernetes.Clientset, ns, labelSelector string, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pods, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err == nil {
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					t.Logf("Pod %s is running", pod.Name)
					return
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("no pod with selector %q in namespace %q became Running within %s", labelSelector, ns, timeout)
}
