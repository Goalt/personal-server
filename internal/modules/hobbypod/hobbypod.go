package hobbypod

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HobbyPodModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *HobbyPodModule {
	return &HobbyPodModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *HobbyPodModule) Name() string {
	return "hobby-pod"
}

func (m *HobbyPodModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "hobbypod")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating hobby-pod Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	pvc, deployment := m.prepare()

	// Helper function to write object to YAML file
	writeYAML := func(obj interface{}, name string) error {
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to convert %s to JSON: %w", name, err)
		}
		yamlContent, err := k8s.JSONToYAML(string(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to convert %s to YAML: %w", name, err)
		}
		filename := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", name))
		if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
			return fmt.Errorf("failed to write %s to file: %w", name, err)
		}
		m.log.Success("Generated: %s\n", filename)
		return nil
	}

	// Write PVC
	if err := writeYAML(pvc, "pvc"); err != nil {
		return err
	}

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 2/2 hobby-pod configurations generated successfully\n")
	return nil
}

func (m *HobbyPodModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying hobby-pod Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "hobby-storage-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'hobby-storage-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "hobby-pod", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'hobby-pod' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	pvc, deployment := m.prepare()

	// Apply PVC
	m.log.Progress("Applying PersistentVolumeClaim: hobby-storage-pvc\n")
	createdPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdPVC.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: hobby-pod\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 2/2 resources applied successfully\n")
	return nil
}

func (m *HobbyPodModule) prepare() (*corev1.PersistentVolumeClaim, *appsv1.Deployment) {
	// Prepare PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hobby-storage-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "hobby-pod",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	privileged := true
	runAsNonRoot := false
	allowPrivilegeEscalation := true

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hobby-pod",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "hobby-pod",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "hobby-pod",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "hobby-pod",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "hobby",
							Image: "ghcr.io/goalt/workconfig:sha-595cb6c",
							Env: []corev1.EnvVar{
								{
									Name:  "DEBIAN_FRONTEND",
									Value: "noninteractive",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "hobby-storage",
									MountPath: "/data",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             &runAsNonRoot,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								Privileged:               &privileged,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{"SYS_ADMIN"},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "hobby-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "hobby-storage-pvc",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	return pvc, deployment
}

func (m *HobbyPodModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning hobby-pod Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalResources := 2 // Deployment + PVC

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: hobby-pod\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "hobby-pod", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'hobby-pod' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: hobby-pod\n")
		successCount++
	}

	// Delete PVC
	m.log.Info("🗑️  Deleting PersistentVolumeClaim: hobby-storage-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "hobby-storage-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PVC 'hobby-storage-pvc' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PVC: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PVC: hobby-storage-pvc\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d resources deleted successfully\n", successCount, totalResources)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *HobbyPodModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking hobby-pod resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	m.log.Println("DEPLOYMENT:")
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "hobby-pod", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment 'hobby-pod' not found\n")
		} else {
			m.log.Error("  Error getting deployment: %v\n", err)
		}
	} else {
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", deployment.Name)
		m.log.Info("  Namespace: %s\n", deployment.Namespace)
		m.log.Info("  Ready: %d/%d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
		m.log.Info("  Up-to-date: %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("  Available: %d\n", deployment.Status.AvailableReplicas)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
		m.log.Print("  Status: ")
		if deployment.Status.ReadyReplicas == deployment.Status.Replicas && deployment.Status.Replicas > 0 {
			m.log.Success("Healthy\n")
		} else if deployment.Status.Replicas == 0 {
			m.log.Warn("Scaled to zero\n")
		} else {
			m.log.Warn("Not ready\n")
		}
	}

	m.log.Println("\nPERSISTENT VOLUME CLAIM:")
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "hobby-storage-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  PVC 'hobby-storage-pvc' not found\n")
		} else {
			m.log.Error("  Error getting PVC: %v\n", err)
		}
	} else {
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", pvc.Name)
		m.log.Info("  Namespace: %s\n", pvc.Namespace)
		m.log.Info("  Status: %s\n", pvc.Status.Phase)
		m.log.Info("  Volume: %s\n", pvc.Spec.VolumeName)
		m.log.Info("  Capacity: %s\n", pvc.Status.Capacity.Storage().String())
		m.log.Info("  Access Modes: %v\n", pvc.Spec.AccessModes)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Get Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=hobby-pod",
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Println("  No pods found with label app=hobby-pod")
	} else {
		m.log.Info("  %-40s %-10s %-10s %-10s\n", "NAME", "READY", "STATUS", "AGE")
		for _, pod := range pods.Items {
			ready := 0
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready++
				}
			}
			total := len(pod.Spec.Containers)
			age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
			status := "✅"
			if pod.Status.Phase != corev1.PodRunning || ready != total {
				status = "⚠️ "
			}
			m.log.Info("  %s %-40s %-10s %-10s %-10s\n",
				status,
				pod.Name,
				fmt.Sprintf("%d/%d", ready, total),
				pod.Status.Phase,
				k8s.FormatAge(age))
		}
	}

	m.log.Println()
	return nil
}

func (m *HobbyPodModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "hobby-pod")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("hobby_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting Hobby Pod backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 1. Backup data volume
	m.log.Info("💾 Backing up data volume (/data)...\n")

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=hobby-pod",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=hobby-pod")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	dataBackupFile := filepath.Join(backupDir, fmt.Sprintf("hobby_data_%s.tar.gz", timestamp))

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	// Command: tar czf - -C /data .
	cmdStr := fmt.Sprintf("%s exec -n %s %s -- tar czf - -C /data .", kubectlCmd, m.ModuleConfig.Namespace, podName)

	cmdParts := strings.Fields(cmdStr)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	outFile, err := os.Create(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to create data backup file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to archive data: %w", err)
	}

	fileInfo, err := outFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat data backup file: %w", err)
	}
	m.log.Success("✅ Data archived (%d bytes)\n", fileInfo.Size())

	// 2. Metadata
	m.log.Info("📋 Writing metadata...\n")
	metadataFile := filepath.Join(backupDir, "backup_info.txt")
	metadata := fmt.Sprintf(`Hobby Pod Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: hobby-pod
Pod: %s

Data Archive:
%s

Restore Command:
personal-server hobby-pod restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(dataBackupFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server hobby-pod restore %s\n", timestamp)

	return nil
}

func (m *HobbyPodModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server hobby-pod restore [TIMESTAMP|latest]")
	}

	timestamp := args[0]
	backupDir := "backups"

	// Resolve latest
	if timestamp == "latest" {
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return fmt.Errorf("failed to read backup directory: %w", err)
		}

		var latestTime time.Time
		var latestDir string

		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "hobby_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "hobby_backup_")
				ts, err := time.Parse("20060102_150405", tsStr)
				if err == nil {
					if ts.After(latestTime) {
						latestTime = ts
						latestDir = entry.Name()
					}
				}
			}
		}

		if latestDir == "" {
			return fmt.Errorf("no backups found")
		}
		timestamp = strings.TrimPrefix(latestDir, "hobby_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("hobby_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	dataBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("hobby_data_%s.tar.gz", timestamp))
	if _, err := os.Stat(dataBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("data archive missing: %s", dataBackupFile)
	}

	m.log.Info("🔄 Starting Hobby Pod restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Data will be restored from %s\n", dataBackupFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=hobby-pod",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=hobby-pod")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	// Restore data
	m.log.Info("💾 Restoring data...\n")

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	// 1. Clean existing data
	cleanCmdStr := fmt.Sprintf("%s exec -n %s %s -- rm -rf /data/*", kubectlCmd, m.ModuleConfig.Namespace, podName)
	cleanCmdParts := strings.Fields(cleanCmdStr)
	cleanCmd := exec.CommandContext(ctx, cleanCmdParts[0], cleanCmdParts[1:]...)
	if err := cleanCmd.Run(); err != nil {
		m.log.Warn("Warning during clean: %v\n", err)
	}

	// 2. Restore from tar
	restoreCmdStr := fmt.Sprintf("%s exec -i -n %s %s -- tar xzf - -C /data", kubectlCmd, m.ModuleConfig.Namespace, podName)
	restoreCmdParts := strings.Fields(restoreCmdStr)
	restoreCmd := exec.CommandContext(ctx, restoreCmdParts[0], restoreCmdParts[1:]...)

	inFile, err := os.Open(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to open data backup file: %w", err)
	}
	defer inFile.Close()

	restoreCmd.Stdin = inFile
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr

	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}
	m.log.Success("✅ Data restored\n")

	m.log.Success("🎉 Restore complete!\n")
	return nil
}
