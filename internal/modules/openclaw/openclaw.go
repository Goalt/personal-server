package openclaw

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
	"k8s.io/apimachinery/pkg/util/intstr"
)

type OpenClawModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *OpenClawModule {
	return &OpenClawModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *OpenClawModule) Name() string {
	return "openclaw"
}

func (m *OpenClawModule) Doc(ctx context.Context) error {
	m.log.Info("Module: openclaw\n\n")
	m.log.Info("Description:\n  Deploys the OpenClaw application.\n  Manages two PersistentVolumeClaims (data and assets), a Service, and a Deployment.\n\n")
	m.log.Info("Required configuration keys (modules[].secrets):\n  dashboard_token   Gateway token for OpenClaw (OPENCLAW_GATEWAY_TOKEN)\n\n")
	m.log.Info("Subcommands:\n  generate   Write Kubernetes YAML to configs/openclaw/\n  apply      Create/update resources in the cluster\n  clean      Delete all OpenClaw resources from the cluster\n  status     Print Deployment and Pod status\n  doc        Show this documentation\n  backup     Archive data and assets volumes to the destination directory\n  restore    Restore volumes from a backup archive\n")
	return nil
}

func (m *OpenClawModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "openclaw")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating OpenClaw Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	configPVC, dataPVC, service, deployment := m.prepare()

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

	// Write Config PVC
	if err := writeYAML(configPVC, "config-pvc"); err != nil {
		return err
	}

	// Write Data PVC
	if err := writeYAML(dataPVC, "data-pvc"); err != nil {
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

	m.log.Info("\nCompleted: 4/4 OpenClaw configurations generated successfully\n")
	return nil
}

func (m *OpenClawModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying OpenClaw Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-config-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'openclaw-config-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'openclaw-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'openclaw' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'openclaw' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	configPVC, dataPVC, service, deployment := m.prepare()

	// Apply Config PVC
	m.log.Progress("Applying PersistentVolumeClaim: openclaw-config-pvc\n")
	createdConfigPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, configPVC, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdConfigPVC.Name)

	// Apply Data PVC
	m.log.Progress("Applying PersistentVolumeClaim: openclaw-data-pvc\n")
	createdDataPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, dataPVC, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdDataPVC.Name)

	// Apply Service
	m.log.Progress("\nApplying Service: openclaw\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("\nApplying Deployment: openclaw\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: OpenClaw configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for openclaw module
func (m *OpenClawModule) prepare() (*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment) {
	// Prepare Config PersistentVolumeClaim
	configStorageQuantity := resource.MustParse("100Mi")
	configPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw-config-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: configStorageQuantity,
				},
			},
		},
	}

	// Prepare Data PersistentVolumeClaim
	dataStorageQuantity := resource.MustParse("1Gi")
	dataPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: dataStorageQuantity,
				},
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       18789,
					TargetPort: intstr.FromInt(18789),
				},
			},
			Selector: map[string]string{
				"app": "openclaw",
			},
		},
	}

	// Prepare Deployment
	image := m.ModuleConfig.Image
	if image == "" {
		image = "ghcr.io/openclaw/openclaw:2026.4.2"
	}

	gatewayToken := m.ModuleConfig.Secrets["dashboard_token"]

	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &replicas,
			RevisionHistoryLimit: func() *int32 { i := int32(1); return &i }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "openclaw",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "openclaw",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:            "openclaw",
							Image:           image,
							ImagePullPolicy: k8s.DefaultImagePullPolicy(image),
							Env: []corev1.EnvVar{
								{
									Name:  "OPENCLAW_GATEWAY_TOKEN",
									Value: gatewayToken,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 18789,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "openclaw-config",
									MountPath: "/home/node/.openclaw",
								},
								{
									Name:      "openclaw-data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "openclaw-config",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "openclaw-config-pvc",
								},
							},
						},
						{
							Name: "openclaw-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "openclaw-data-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	return configPVC, dataPVC, service, deployment
}

func (m *OpenClawModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning OpenClaw Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: openclaw\n")
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "openclaw", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'openclaw' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: openclaw\n")
		successCount++
	}

	// Delete Service
	m.log.Info("\n🗑️  Deleting Service: openclaw\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "openclaw", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'openclaw' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: openclaw\n")
		successCount++
	}

	// Delete Config PVC
	m.log.Info("\n🗑️  Deleting PersistentVolumeClaim: openclaw-config-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "openclaw-config-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'openclaw-config-pvc' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: openclaw-config-pvc\n")
		successCount++
	}

	// Delete Data PVC
	m.log.Info("\n🗑️  Deleting PersistentVolumeClaim: openclaw-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "openclaw-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'openclaw-data-pvc' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: openclaw-data-pvc\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/4 OpenClaw resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
		m.log.Warn("WARNING: Deleting the PVCs will remove all OpenClaw data permanently!\n")
	}
	return nil
}

func (m *OpenClawModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking OpenClaw resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	resourceFound := false

	// Check Config PVC
	configPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-config-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'openclaw-config-pvc' not found\n")
		} else {
			m.log.Error("Error checking PersistentVolumeClaim: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(configPVC.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("PersistentVolumeClaim 'openclaw-config-pvc'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Status: %s\n", configPVC.Status.Phase)
		m.log.Info("   Storage: %s\n", configPVC.Spec.Resources.Requests.Storage().String())
		if configPVC.Spec.VolumeName != "" {
			m.log.Info("   Volume: %s\n", configPVC.Spec.VolumeName)
		}
	}

	m.log.Println()

	// Check Data PVC
	dataPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'openclaw-data-pvc' not found\n")
		} else {
			m.log.Error("Error checking PersistentVolumeClaim: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(dataPVC.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("PersistentVolumeClaim 'openclaw-data-pvc'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Status: %s\n", dataPVC.Status.Phase)
		m.log.Info("   Storage: %s\n", dataPVC.Spec.Resources.Requests.Storage().String())
		if dataPVC.Spec.VolumeName != "" {
			m.log.Info("   Volume: %s\n", dataPVC.Spec.VolumeName)
		}
	}

	m.log.Println()

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'openclaw' not found\n")
		} else {
			m.log.Error("Error checking service: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Service 'openclaw'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Type: %s\n", service.Spec.Type)
		m.log.Info("   Ports:\n")
		for _, port := range service.Spec.Ports {
			m.log.Info("     - %s: %d -> %s\n", port.Name, port.Port, port.TargetPort.String())
		}
	}

	m.log.Println()

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'openclaw' not found\n")
		} else {
			m.log.Error("Error checking deployment: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'openclaw'\n")
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
		LabelSelector: "app=openclaw",
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
		m.log.Println("\nNo OpenClaw resources found. Run 'openclaw apply' to create them.")
	}
	return nil
}

// detectKubectl returns the kubectl command path, preferring microk8s if available
func detectKubectl() string {
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		return "/snap/bin/microk8s kubectl"
	}
	return "kubectl"
}

func (m *OpenClawModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "openclaw")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("openclaw_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting OpenClaw backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=openclaw",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=openclaw")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	kubectlCmd := detectKubectl()

	// 1. Backup config volume
	m.log.Info("💾 Backing up OpenClaw config (/config)...\n")
	configBackupFile := filepath.Join(backupDir, fmt.Sprintf("openclaw_config_%s.tar.gz", timestamp))

	cmdStr := fmt.Sprintf("%s exec -n %s %s -- tar czf - /config", kubectlCmd, m.ModuleConfig.Namespace, podName)
	cmdParts := strings.Fields(cmdStr)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	configOutFile, err := os.Create(configBackupFile)
	if err != nil {
		return fmt.Errorf("failed to create config backup file: %w", err)
	}
	defer configOutFile.Close()

	cmd.Stdout = configOutFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to archive config: %w", err)
	}

	configFileInfo, err := configOutFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat config backup file: %w", err)
	}
	m.log.Success("✅ Config archived (%d bytes)\n", configFileInfo.Size())

	// 2. Backup data volume
	m.log.Info("💾 Backing up OpenClaw data (/data)...\n")
	dataBackupFile := filepath.Join(backupDir, fmt.Sprintf("openclaw_data_%s.tar.gz", timestamp))

	cmdStr = fmt.Sprintf("%s exec -n %s %s -- tar czf - /data", kubectlCmd, m.ModuleConfig.Namespace, podName)
	cmdParts = strings.Fields(cmdStr)
	cmd = exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	dataOutFile, err := os.Create(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to create data backup file: %w", err)
	}
	defer dataOutFile.Close()

	cmd.Stdout = dataOutFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to archive data: %w", err)
	}

	dataFileInfo, err := dataOutFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat data backup file: %w", err)
	}
	m.log.Success("✅ Data archived (%d bytes)\n", dataFileInfo.Size())

	// 3. Metadata
	m.log.Info("📋 Writing metadata...\n")
	metadataFile := filepath.Join(backupDir, "backup_info.txt")
	metadata := fmt.Sprintf(`OpenClaw Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: openclaw
Pod: %s

Config Archive:
%s

Data Archive:
%s

Restore Command:
personal-server openclaw restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(configBackupFile), filepath.Base(dataBackupFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server openclaw restore %s\n", timestamp)

	return nil
}

func (m *OpenClawModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server openclaw restore [TIMESTAMP|latest]")
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
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "openclaw_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "openclaw_backup_")
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
		timestamp = strings.TrimPrefix(latestDir, "openclaw_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("openclaw_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	configBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("openclaw_config_%s.tar.gz", timestamp))
	if _, err := os.Stat(configBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("config archive missing: %s", configBackupFile)
	}

	dataBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("openclaw_data_%s.tar.gz", timestamp))
	if _, err := os.Stat(dataBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("data archive missing: %s", dataBackupFile)
	}

	m.log.Info("🔄 Starting OpenClaw restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Config will be restored from %s\n", configBackupFile)
	m.log.Info("💾 Data will be restored from %s\n", dataBackupFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=openclaw",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=openclaw")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	kubectlCmd := detectKubectl()

	// 1. Restore config volume
	m.log.Info("💾 Restoring config...\n")

	// Clean existing config
	cleanCmdStr := fmt.Sprintf("%s exec -n %s %s -- rm -rf /config/*", kubectlCmd, m.ModuleConfig.Namespace, podName)
	cleanCmdParts := strings.Fields(cleanCmdStr)
	cleanCmd := exec.CommandContext(ctx, cleanCmdParts[0], cleanCmdParts[1:]...)
	if err := cleanCmd.Run(); err != nil {
		m.log.Warn("Warning during config clean: %v\n", err)
	}

	// Restore config from tar
	restoreCmdStr := fmt.Sprintf("%s exec -i -n %s %s -- tar xzf - -C /", kubectlCmd, m.ModuleConfig.Namespace, podName)
	restoreCmdParts := strings.Fields(restoreCmdStr)
	restoreCmd := exec.CommandContext(ctx, restoreCmdParts[0], restoreCmdParts[1:]...)

	configInFile, err := os.Open(configBackupFile)
	if err != nil {
		return fmt.Errorf("failed to open config backup file: %w", err)
	}
	defer configInFile.Close()

	restoreCmd.Stdin = configInFile
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr

	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}
	m.log.Success("✅ Config restored\n")

	// 2. Restore data volume
	m.log.Info("💾 Restoring data...\n")

	// Clean existing data
	cleanCmdStr = fmt.Sprintf("%s exec -n %s %s -- rm -rf /data/*", kubectlCmd, m.ModuleConfig.Namespace, podName)
	cleanCmdParts = strings.Fields(cleanCmdStr)
	cleanCmd = exec.CommandContext(ctx, cleanCmdParts[0], cleanCmdParts[1:]...)
	if err := cleanCmd.Run(); err != nil {
		m.log.Warn("Warning during data clean: %v\n", err)
	}

	// Restore data from tar
	restoreCmdStr = fmt.Sprintf("%s exec -i -n %s %s -- tar xzf - -C /", kubectlCmd, m.ModuleConfig.Namespace, podName)
	restoreCmdParts = strings.Fields(restoreCmdStr)
	restoreCmd = exec.CommandContext(ctx, restoreCmdParts[0], restoreCmdParts[1:]...)

	dataInFile, err := os.Open(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to open data backup file: %w", err)
	}
	defer dataInFile.Close()

	restoreCmd.Stdin = dataInFile
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr

	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}
	m.log.Success("✅ Data restored\n")

	// Restart deployment
	m.log.Info("🔄 Restarting deployment 'openclaw'...\n")
	restartCmdStr := fmt.Sprintf("%s rollout restart deployment/openclaw -n %s", kubectlCmd, m.ModuleConfig.Namespace)
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
