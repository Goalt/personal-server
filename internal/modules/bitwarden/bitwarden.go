package bitwarden

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"os/exec"
	"strings"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type BitwardenModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *BitwardenModule {
	return &BitwardenModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *BitwardenModule) Name() string {
	return "bitwarden"
}

func (m *BitwardenModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "bitwarden")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Bitwarden Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	pvc, service, deployment := m.prepare()

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

	// Write Service
	if err := writeYAML(service, "service"); err != nil {
		return err
	}

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 3/3 Bitwarden configurations generated successfully\n")
	return nil
}

func (m *BitwardenModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Bitwarden Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "bitwarden-claim0", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'bitwarden-claim0' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "bitwarden", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'bitwarden' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "bitwarden", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'bitwarden' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	pvc, service, deployment := m.prepare()

	// Apply PersistentVolumeClaim
	m.log.Progress("Applying PersistentVolumeClaim: bitwarden-claim0\n")
	createdPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdPVC.Name)

	// Apply Service
	m.log.Progress("\nApplying Service: bitwarden\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("\nApplying Deployment: bitwarden\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: Bitwarden configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for bitwarden module
func (m *BitwardenModule) prepare() (*corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment) {
	// Prepare PersistentVolumeClaim
	storageQuantity := resource.MustParse("100Mi")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bitwarden-claim0",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by":         "personal-server",
				"io.kompose.service": "bitwarden-claim0",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQuantity,
				},
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bitwarden",
			Namespace: m.ModuleConfig.Namespace,
			Annotations: map[string]string{
				"kompose.cmd":     "kompose --file docker-comopose.yaml convert",
				"kompose.version": "1.26.1 (HEAD)",
			},
			Labels: map[string]string{
				"managed-by":         "personal-server",
				"io.kompose.service": "bitwarden",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "80",
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
				{
					Name:       "3012",
					Port:       3012,
					TargetPort: intstr.FromInt(3012),
				},
			},
			Selector: map[string]string{
				"app": "bitwarden",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bitwarden",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "bitwarden",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &replicas,
			RevisionHistoryLimit: func() *int32 { i := int32(1); return &i }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "bitwarden",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "bitwarden",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:  "bitwarden",
							Image: "vaultwarden/server:1.32.0",
							Env: []corev1.EnvVar{
								{
									Name:  "WEBSOCKET_ENABLED",
									Value: "true",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
								{
									ContainerPort: 3012,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "bitwarden-claim0",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "bitwarden-claim0",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "bitwarden-claim0",
								},
							},
						},
					},
				},
			},
		},
	}

	return pvc, service, deployment
}

func (m *BitwardenModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Bitwarden Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: bitwarden\n")
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "bitwarden", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'bitwarden' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: bitwarden\n")
		successCount++
	}

	// Delete Service
	m.log.Info("\n🗑️  Deleting Service: bitwarden\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "bitwarden", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'bitwarden' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: bitwarden\n")
		successCount++
	}

	// Delete PersistentVolumeClaim
	m.log.Info("\n🗑️  Deleting PersistentVolumeClaim: bitwarden-claim0\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "bitwarden-claim0", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'bitwarden-claim0' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: bitwarden-claim0\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/3 bitwarden resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
		m.log.Warn("WARNING: Deleting the PVC will remove all Bitwarden data permanently!\n")
	}
	return nil
}

func (m *BitwardenModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Bitwarden resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	resourceFound := false

	// Check PersistentVolumeClaim
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "bitwarden-claim0", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'bitwarden-claim0' not found\n")
		} else {
			m.log.Error("Error checking PersistentVolumeClaim: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("PersistentVolumeClaim 'bitwarden-claim0'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Status: %s\n", pvc.Status.Phase)
		m.log.Info("   Storage: %s\n", pvc.Spec.Resources.Requests.Storage().String())
		if pvc.Spec.VolumeName != "" {
			m.log.Info("   Volume: %s\n", pvc.Spec.VolumeName)
		}
	}

	m.log.Println()

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "bitwarden", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'bitwarden' not found\n")
		} else {
			m.log.Error("Error checking service: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Service 'bitwarden'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Type: %s\n", service.Spec.Type)
		m.log.Info("   Ports:\n")
		for _, port := range service.Spec.Ports {
			m.log.Info("     - %s: %d -> %s\n", port.Name, port.Port, port.TargetPort.String())
		}
	}

	m.log.Println()

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "bitwarden", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'bitwarden' not found\n")
		} else {
			m.log.Error("Error checking deployment: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'bitwarden'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Replicas: %d desired / %d ready / %d available / %d unavailable\n",
			deployment.Status.Replicas,
			deployment.Status.ReadyReplicas,
			deployment.Status.AvailableReplicas,
			deployment.Status.UnavailableReplicas)
		m.log.Info("   Updated Replicas: %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("   Image: %s\n", deployment.Spec.Template.Spec.Containers[0].Image)
	}

	m.log.Println()

	// Get Pods for the deployment
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=bitwarden",
	})
	if err != nil {
		m.log.Error("Error listing pods: %v\n", err)
	} else if len(pods.Items) > 0 {
		resourceFound = true
		m.log.Info("PODS:\n")
		m.log.Info("%-40s %-10s %-10s %-10s\n", "NAME", "READY", "STATUS", "AGE")
		for _, pod := range pods.Items {
			ready := 0
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready++
				}
			}
			total := len(pod.Spec.Containers)
			age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
			m.log.Info("%-40s %-10s %-10s %-10s\n",
				pod.Name,
				fmt.Sprintf("%d/%d", ready, total),
				pod.Status.Phase,
				k8s.FormatAge(age))
		}
	}

	if !resourceFound {
		m.log.Println("\nNo Bitwarden resources found. Run 'bitwarden apply' to create them.")
	}
	return nil
}

func (m *BitwardenModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "bitwarden")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("bitwarden_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting Bitwarden backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 1. Backup data volume
	m.log.Info("💾 Backing up Bitwarden data (/data)...\n")

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=bitwarden",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=bitwarden")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	dataBackupFile := filepath.Join(backupDir, fmt.Sprintf("bitwarden_data_%s.tar.gz", timestamp))

	// Execute tar command in pod and stream to file
	// We need to use kubectl exec logic here.

	// Let's try to find kubectl or microk8s kubectl
	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	// Construct command: kubectl exec -n <ns> <pod> -- tar czf - /data
	cmdStr := fmt.Sprintf("%s exec -n %s %s -- tar czf - /data", kubectlCmd, m.ModuleConfig.Namespace, podName)

	cmdParts := strings.Fields(cmdStr)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	outFile, err := os.Create(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to create data backup file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr // pipe stderr to see errors

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
	metadata := fmt.Sprintf(`Bitwarden Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: bitwarden
Pod: %s

Data Archive:
%s

Restore Command:
personal-server bitwarden restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(dataBackupFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server bitwarden restore %s\n", timestamp)

	return nil
}

func (m *BitwardenModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server bitwarden restore [TIMESTAMP|latest]")
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
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "bitwarden_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "bitwarden_backup_")
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
		timestamp = strings.TrimPrefix(latestDir, "bitwarden_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("bitwarden_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	dataBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("bitwarden_data_%s.tar.gz", timestamp))
	if _, err := os.Stat(dataBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("data archive missing: %s", dataBackupFile)
	}

	m.log.Info("🔄 Starting Bitwarden restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Data will be restored from %s\n", dataBackupFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=bitwarden",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=bitwarden")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	// Restore data
	m.log.Info("💾 Restoring data...\n")

	// Let's try to find kubectl or microk8s kubectl
	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	// 1. Clean existing data
	// kubectl exec -n <ns> <pod> -- rm -rf /data/*
	cleanCmdStr := fmt.Sprintf("%s exec -n %s %s -- rm -rf /data/*", kubectlCmd, m.ModuleConfig.Namespace, podName)
	cleanCmdParts := strings.Fields(cleanCmdStr)
	cleanCmd := exec.CommandContext(ctx, cleanCmdParts[0], cleanCmdParts[1:]...)
	if err := cleanCmd.Run(); err != nil {
		// Ignore error if directory is already empty or other minor issues, but log it
		m.log.Warn("Warning during clean: %v\n", err)
	}

	// 2. Restore from tar
	// cat <file> | kubectl exec -i -n <ns> <pod> -- tar xzf - -C /
	restoreCmdStr := fmt.Sprintf("%s exec -i -n %s %s -- tar xzf - -C /", kubectlCmd, m.ModuleConfig.Namespace, podName)
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

	// Restart deployment
	m.log.Info("🔄 Restarting deployment 'bitwarden'...\n")

	// We can use rollout restart via kubectl or just delete the pod to force restart if replicas=1
	// Or use scale down/up.
	// The script used `rollout restart`.
	// Let's use `kubectl rollout restart` for simplicity and consistency with script logic
	restartCmdStr := fmt.Sprintf("%s rollout restart deployment/bitwarden -n %s", kubectlCmd, m.ModuleConfig.Namespace)
	restartCmdParts := strings.Fields(restartCmdStr)
	restartCmd := exec.CommandContext(ctx, restartCmdParts[0], restartCmdParts[1:]...)

	if err := restartCmd.Run(); err != nil {
		m.log.Warn("Failed to trigger rollout restart: %v\n", err)
	} else {
		m.log.Success("✅ Deployment restarted successfully\n")
	}

	m.log.Success("🎉 Restore complete!\n")
	return nil
}
