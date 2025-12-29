package gitea

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

type GiteaModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *GiteaModule {
	return &GiteaModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *GiteaModule) Name() string {
	return "gitea"
}

func (m *GiteaModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "gitea")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Gitea Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	secret, pvc, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare resources: %w", err)
	}

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

	// Write Secret
	if err := writeYAML(secret, "secret"); err != nil {
		return err
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

	m.log.Info("\nCompleted: 4/4 Gitea configurations generated successfully\n")
	return nil
}

func (m *GiteaModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Gitea Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "gitea-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'gitea-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "gitea-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'gitea-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "gitea", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'gitea' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "gitea", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'gitea' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	secret, pvc, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare resources: %w", err)
	}

	// Apply Secret
	m.log.Progress("Applying Secret: gitea-secrets\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	m.log.Success("Created Secret: gitea-secrets\n")

	// Apply PVC
	m.log.Progress("Applying PersistentVolumeClaim: gitea-data-pvc\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: gitea-data-pvc\n")

	// Apply Service
	m.log.Progress("Applying Service: gitea\n")
	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: gitea\n")

	// Apply Deployment
	m.log.Progress("Applying Deployment: gitea\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: gitea\n")

	m.log.Info("\nCompleted: Gitea configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for gitea module
func (m *GiteaModule) prepare() (*corev1.Secret, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment, error) {
	// Prepare Secret
	dbPassword, exists := m.ModuleConfig.Secrets["gitea_db_password"]
	if !exists {
		return nil, nil, nil, nil, fmt.Errorf("gitea_db_password not found in configuration")
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitea-secrets",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "gitea",
				"managed-by": "personal-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"GITEA__database__PASSWD": []byte(dbPassword),
		},
	}

	// Prepare PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitea-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "gitea",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: func() *string { s := "microk8s-hostpath"; return &s }(),
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitea",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "gitea",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       3000,
					TargetPort: intstr.FromInt(3000),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "ssh",
					Port:       22,
					TargetPort: intstr.FromInt(22),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "gitea",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitea",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "gitea",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "gitea",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "gitea",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "gitea",
							Image:           "gitea/gitea:1.25",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 3000,
								},
								{
									Name:          "ssh",
									ContainerPort: 22,
								},
							},
							Env: []corev1.EnvVar{
								{Name: "USER_UID", Value: "1000"},
								{Name: "USER_GID", Value: "1000"},
								{Name: "GITEA__database__DB_TYPE", Value: "postgres"},
								{Name: "GITEA__database__HOST", Value: k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "database_host", "postgres:5432")},
								{Name: "GITEA__database__NAME", Value: k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "gitea_db_user", "gitea")},
								{Name: "GITEA__database__USER", Value: k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "gitea_db_user", "gitea")},
								{
									Name: "GITEA__database__PASSWD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "gitea-secrets",
											},
											Key: "GITEA__database__PASSWD",
										},
									},
								},
								{Name: "GITEA__server__DOMAIN", Value: "gitea.local"},
								{Name: "GITEA__server__SSH_DOMAIN", Value: "gitea.local"},
								{Name: "GITEA__server__ROOT_URL", Value: "https://gitea." + m.GeneralConfig.Domain},
								{Name: "GITEA__server__HTTP_PORT", Value: "3000"},
								{Name: "GITEA__server__SSH_PORT", Value: "22"},
								{Name: "DISABLE_REGISTRATION", Value: "true"},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(3000),
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										Port: intstr.FromInt(3000),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "gitea-data",
									MountPath: "/data",
								},
								{
									Name:      "gitea-config",
									MountPath: "/etc/gitea",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "gitea-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "gitea-data-pvc",
								},
							},
						},
						{
							Name: "gitea-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return secret, pvc, service, deployment, nil
}

func (m *GiteaModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Gitea Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Processing Deployment: gitea\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "gitea", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'gitea' not found\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: gitea\n")
		successCount++
	}

	// Delete Service
	m.log.Info("🗑️  Processing Service: gitea\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "gitea", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'gitea' not found\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: gitea\n")
		successCount++
	}

	// Delete PVC
	m.log.Info("🗑️  Processing PersistentVolumeClaim: gitea-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "gitea-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'gitea-data-pvc' not found\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: gitea-data-pvc\n")
		successCount++
	}

	// Delete Secret
	m.log.Info("🗑️  Processing Secret: gitea-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "gitea-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret 'gitea-secrets' not found\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: gitea-secrets\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d Gitea resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *GiteaModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Gitea resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "gitea", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'gitea' not found\n")
		} else {
			m.log.Error("Error getting Deployment: %v\n", err)
		}
	} else {
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("DEPLOYMENT:\n")
		m.log.Info("  Name:            %s\n", deployment.Name)
		m.log.Info("  Ready:           %d/%d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
		m.log.Info("  Up-to-date:      %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("  Available:       %d\n", deployment.Status.AvailableReplicas)
		m.log.Info("  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "gitea", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'gitea' not found\n")
		} else {
			m.log.Error("Error getting Service: %v\n", err)
		}
	} else {
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("SERVICE:\n")
		m.log.Info("  Name:            %s\n", service.Name)
		m.log.Info("  Type:            %s\n", service.Spec.Type)
		m.log.Info("  Cluster-IP:      %s\n", service.Spec.ClusterIP)
		m.log.Print("  Ports:           ")
		for i, port := range service.Spec.Ports {
			if i > 0 {
				m.log.Print(", ")
			}
			m.log.Print("%d/%s", port.Port, port.Protocol)
		}
		m.log.Info("\n  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	// Check PVC
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "gitea-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'gitea-data-pvc' not found\n")
		} else {
			m.log.Error("Error getting PersistentVolumeClaim: %v\n", err)
		}
	} else {
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("PERSISTENT VOLUME CLAIM:\n")
		m.log.Info("  Name:            %s\n", pvc.Name)
		m.log.Info("  Status:          %s\n", pvc.Status.Phase)
		m.log.Info("  Volume:          %s\n", pvc.Spec.VolumeName)
		m.log.Info("  Capacity:        %s\n", pvc.Status.Capacity.Storage().String())
		m.log.Info("  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	// Check Secret
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "gitea-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Secret 'gitea-secrets' not found\n")
		} else {
			m.log.Error("Error getting Secret: %v\n", err)
		}
	} else {
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("SECRET:\n")
		m.log.Info("  Name:            %s\n", secret.Name)
		m.log.Info("  Type:            %s\n", secret.Type)
		m.log.Info("  Data keys:       %d\n", len(secret.Data))
		m.log.Info("  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	// Check Pods
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=gitea",
	})
	if err != nil {
		m.log.Error("Error listing pods: %v\n", err)
	} else if len(pods.Items) > 0 {
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
	} else {
		m.log.Println("No Gitea pods found")
	}
	return nil
}

func (m *GiteaModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "gitea")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("gitea_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting Gitea backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 1. Backup data volume
	m.log.Info("💾 Backing up Gitea data (/data)...\n")

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=gitea",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=gitea")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	dataBackupFile := filepath.Join(backupDir, fmt.Sprintf("gitea_data_%s.tar.gz", timestamp))

	// Execute tar command in pod and stream to file

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
	metadata := fmt.Sprintf(`Gitea Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: gitea
Pod: %s

Data Archive:
%s

Restore Command:
personal-server gitea restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(dataBackupFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server gitea restore %s\n", timestamp)

	return nil
}

func (m *GiteaModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server gitea restore [TIMESTAMP|latest]")
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
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "gitea_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "gitea_backup_")
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
		timestamp = strings.TrimPrefix(latestDir, "gitea_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("gitea_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	dataBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("gitea_data_%s.tar.gz", timestamp))
	if _, err := os.Stat(dataBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("data archive missing: %s", dataBackupFile)
	}

	m.log.Info("🔄 Starting Gitea restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Data will be restored from %s\n", dataBackupFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=gitea",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=gitea")
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
	m.log.Info("🔄 Restarting deployment 'gitea'...\n")

	restartCmdStr := fmt.Sprintf("%s rollout restart deployment/gitea -n %s", kubectlCmd, m.ModuleConfig.Namespace)
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
