package redis

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

type RedisModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *RedisModule {
	return &RedisModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *RedisModule) Name() string {
	return "redis"
}

func (m *RedisModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "redis")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Redis Kubernetes configurations...\n")
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

	m.log.Info("\nCompleted: 4/4 Redis configurations generated successfully\n")
	return nil
}

func (m *RedisModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Redis Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "redis-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'redis-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "redis-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'redis-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "redis", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'redis' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "redis", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'redis' already exists in namespace '%s'", m.ModuleConfig.Namespace)
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
	m.log.Progress("Applying Secret: redis-secrets\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	m.log.Success("Created Secret: redis-secrets\n")

	// Apply PVC
	m.log.Progress("Applying PersistentVolumeClaim: redis-data-pvc\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: redis-data-pvc\n")

	// Apply Service
	m.log.Progress("Applying Service: redis\n")
	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: redis\n")

	// Apply Deployment
	m.log.Progress("Applying Deployment: redis\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: redis\n")

	m.log.Info("\nCompleted: Redis configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for redis module
func (m *RedisModule) prepare() (*corev1.Secret, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment, error) {
	// Prepare Secret
	redisPassword := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "redis_password", "")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-secrets",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "redis",
				"managed-by": "personal-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"redis-password": []byte(redisPassword),
		},
	}

	// Prepare PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "redis",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "redis",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis",
					Port:       6379,
					TargetPort: intstr.FromInt(6379),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "redis",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "redis",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "redis",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "redis",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "redis",
							Image:           "redis:7.2-alpine",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "redis",
									ContainerPort: 6379,
								},
							},
							Args: func() []string {
								if redisPassword != "" {
									return []string{"--requirepass", "$(REDIS_PASSWORD)"}
								}
								return []string{}
							}(),
							Env: func() []corev1.EnvVar {
								if redisPassword != "" {
									return []corev1.EnvVar{
										{
											Name: "REDIS_PASSWORD",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "redis-secrets",
													},
													Key: "redis-password",
												},
											},
										},
									}
								}
								return []corev1.EnvVar{}
							}(),
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: func() []string {
											if redisPassword != "" {
												return []string{"redis-cli", "-a", "$(REDIS_PASSWORD)", "ping"}
											}
											return []string{"redis-cli", "ping"}
										}(),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: func() []string {
											if redisPassword != "" {
												return []string{"redis-cli", "-a", "$(REDIS_PASSWORD)", "ping"}
											}
											return []string{"redis-cli", "ping"}
										}(),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "redis-data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "redis-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "redis-data-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	return secret, pvc, service, deployment, nil
}

func (m *RedisModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Redis Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Processing Deployment: redis\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "redis", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'redis' not found\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: redis\n")
		successCount++
	}

	// Delete Service
	m.log.Info("🗑️  Processing Service: redis\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "redis", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'redis' not found\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: redis\n")
		successCount++
	}

	// Delete PVC
	m.log.Info("🗑️  Processing PersistentVolumeClaim: redis-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "redis-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'redis-data-pvc' not found\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: redis-data-pvc\n")
		successCount++
	}

	// Delete Secret
	m.log.Info("🗑️  Processing Secret: redis-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "redis-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret 'redis-secrets' not found\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: redis-secrets\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d Redis resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *RedisModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Redis resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "redis", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'redis' not found\n")
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
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "redis", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'redis' not found\n")
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
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "redis-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'redis-data-pvc' not found\n")
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
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "redis-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Secret 'redis-secrets' not found\n")
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
		LabelSelector: "app=redis",
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
		m.log.Println("No Redis pods found")
	}
	return nil
}

func (m *RedisModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "redis")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("redis_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting Redis backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=redis",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=redis")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	// 1. Trigger Redis SAVE command to ensure data is persisted to disk
	m.log.Info("💾 Triggering Redis SAVE...\n")
	kubectlCmd := "kubectl"
	kubectlArgs := []string{"kubectl"}
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s"
		kubectlArgs = []string{kubectlCmd, "kubectl"}
	}

	redisPassword := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "redis_password", "")
	var saveCmd *exec.Cmd
	if redisPassword != "" {
		// Use REDISCLI_AUTH environment variable to pass password securely
		args := append(kubectlArgs[1:], "exec", "-n", m.ModuleConfig.Namespace, podName, "--", "sh", "-c", "redis-cli SAVE")
		saveCmd = exec.CommandContext(ctx, kubectlArgs[0], args...)
		saveCmd.Env = append(os.Environ(), fmt.Sprintf("REDISCLI_AUTH=%s", redisPassword))
	} else {
		args := append(kubectlArgs[1:], "exec", "-n", m.ModuleConfig.Namespace, podName, "--", "redis-cli", "SAVE")
		saveCmd = exec.CommandContext(ctx, kubectlArgs[0], args...)
	}

	if err := saveCmd.Run(); err != nil {
		m.log.Warn("Warning: Failed to trigger SAVE command: %v\n", err)
	} else {
		m.log.Success("✅ Redis SAVE completed\n")
	}

	// 2. Backup data volume
	m.log.Info("💾 Backing up Redis data (/data)...\n")

	dataBackupFile := filepath.Join(backupDir, fmt.Sprintf("redis_data_%s.tar.gz", timestamp))

	// Execute tar command in pod and stream to file
	args := append(kubectlArgs[1:], "exec", "-n", m.ModuleConfig.Namespace, podName, "--", "tar", "czf", "-", "/data")
	cmd := exec.CommandContext(ctx, kubectlArgs[0], args...)

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

	// 3. Metadata
	m.log.Info("📋 Writing metadata...\n")
	metadataFile := filepath.Join(backupDir, "backup_info.txt")
	metadata := fmt.Sprintf(`Redis Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: redis
Pod: %s

Data Archive:
%s

Restore Command:
personal-server redis restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(dataBackupFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server redis restore %s\n", timestamp)

	return nil
}

func (m *RedisModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server redis restore [TIMESTAMP|latest]")
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
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "redis_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "redis_backup_")
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
		timestamp = strings.TrimPrefix(latestDir, "redis_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("redis_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	dataBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("redis_data_%s.tar.gz", timestamp))
	if _, err := os.Stat(dataBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("data archive missing: %s", dataBackupFile)
	}

	m.log.Info("🔄 Starting Redis restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Data will be restored from %s\n", dataBackupFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=redis",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=redis")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	// Restore data
	m.log.Info("💾 Restoring data...\n")

	kubectlCmd := "kubectl"
	kubectlArgs := []string{"kubectl"}
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s"
		kubectlArgs = []string{kubectlCmd, "kubectl"}
	}

	// 1. Clean existing data
	cleanArgs := append(kubectlArgs[1:], "exec", "-n", m.ModuleConfig.Namespace, podName, "--", "rm", "-rf", "/data/*")
	cleanCmd := exec.CommandContext(ctx, kubectlArgs[0], cleanArgs...)
	if err := cleanCmd.Run(); err != nil {
		m.log.Warn("Warning during clean: %v\n", err)
	}

	// 2. Restore from tar
	restoreArgs := append(kubectlArgs[1:], "exec", "-i", "-n", m.ModuleConfig.Namespace, podName, "--", "tar", "xzf", "-", "-C", "/")
	restoreCmd := exec.CommandContext(ctx, kubectlArgs[0], restoreArgs...)

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
	m.log.Info("🔄 Restarting deployment 'redis'...\n")

	restartArgs := append(kubectlArgs[1:], "rollout", "restart", "deployment/redis", "-n", m.ModuleConfig.Namespace)
	restartCmd := exec.CommandContext(ctx, kubectlArgs[0], restartArgs...)

	if err := restartCmd.Run(); err != nil {
		m.log.Warn("Failed to trigger rollout restart: %v\n", err)
	} else {
		m.log.Success("✅ Deployment restarted successfully\n")
	}

	m.log.Success("🎉 Restore complete!\n")
	return nil
}
