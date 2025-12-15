package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

type PostgresModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *PostgresModule {
	return &PostgresModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *PostgresModule) Name() string {
	return "postgres"
}

func (m *PostgresModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "postgres")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Postgres Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	secret, pvc, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
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

	m.log.Info("\nCompleted: 4/4 Postgres configurations generated successfully\n")
	return nil
}

func (m *PostgresModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Postgres Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "postgres-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'postgres-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "postgres-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'postgres-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "postgres", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'postgres' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "postgres", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'postgres' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	secret, pvc, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
	}

	// Apply Secret
	m.log.Progress("Applying Secret: postgres-secrets\n")
	createdSecret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}
	m.log.Success("Created Secret: %s\n", createdSecret.Name)

	// Apply PVC
	m.log.Progress("Applying PVC: postgres-data-pvc\n")
	createdPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}
	m.log.Success("Created PVC: %s\n", createdPVC.Name)

	// Apply Service
	m.log.Progress("Applying Service: postgres\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: postgres\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 4/4 resources applied successfully\n")
	return nil
}

func (m *PostgresModule) prepare() (*corev1.Secret, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment, error) {
	// Prepare Secret
	secretData := make(map[string][]byte)

	user, exists := m.ModuleConfig.Secrets["admin_postgres_user"]
	if !exists {
		return nil, nil, nil, nil, fmt.Errorf("admin_postgres_user not found in configuration")
	}
	secretData["admin_postgres_user"] = []byte(user)

	password, exists := m.ModuleConfig.Secrets["admin_postgres_password"]
	if !exists {
		return nil, nil, nil, nil, fmt.Errorf("admin_postgres_password not found in configuration")
	}
	secretData["admin_postgres_password"] = []byte(password)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-secrets",
			Namespace: m.ModuleConfig.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	// Prepare PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "postgres",
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

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "postgres",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "postgres",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "postgres",
					Port:       5432,
					TargetPort: intstr.FromInt(5432),
				},
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "postgres",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "postgres",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "postgres",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "postgres",
							Image:           "postgres:16",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 5432,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "POSTGRES_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "postgres-secrets",
											},
											Key: "admin_postgres_user",
										},
									},
								},
								{
									Name: "POSTGRES_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "postgres-secrets",
											},
											Key: "admin_postgres_password",
										},
									},
								},
								{
									Name:  "PGDATA",
									Value: "/var/lib/postgresql/data/pgdata",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"sh",
											"-c",
											"pg_isready -U \"$POSTGRES_USER\" -h 127.0.0.1 -p 5432",
										},
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"sh",
											"-c",
											"pg_isready -U \"$POSTGRES_USER\" -h 127.0.0.1 -p 5432",
										},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/postgresql/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "postgres-data-pvc",
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

func (m *PostgresModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Postgres Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalResources := 4 // Deployment, Service, Secret, PVC

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// 1. Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: postgres\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "postgres", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: postgres\n")
		successCount++
	}

	// 2. Delete Service
	m.log.Info("🗑️  Deleting Service: postgres\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "postgres", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: postgres\n")
		successCount++
	}

	// 3. Delete Secret
	m.log.Info("🗑️  Deleting Secret: postgres-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "postgres-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: postgres-secrets\n")
		successCount++
	}

	// 4. Delete PVC
	m.log.Info("🗑️  Deleting PVC: postgres-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "postgres-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PVC not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PVC: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PVC: postgres-data-pvc\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d resources deleted successfully\n", successCount, totalResources)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *PostgresModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Postgres resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	m.log.Println("DEPLOYMENT:")
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "postgres", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Println("  Status: Not Found")
		} else {
			m.log.Error("  Error: %v\n", err)
		}
	} else {
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", deployment.Name)
		m.log.Info("  Ready: %d/%d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
		m.log.Info("  Up-to-date: %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("  Available: %d\n", deployment.Status.AvailableReplicas)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check Service
	m.log.Println("\nSERVICE:")
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "postgres", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Println("  Status: Not Found")
		} else {
			m.log.Error("  Error: %v\n", err)
		}
	} else {
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", service.Name)
		m.log.Info("  Type: %s\n", service.Spec.Type)
		m.log.Info("  Cluster-IP: %s\n", service.Spec.ClusterIP)
		m.log.Print("  Ports: ")
		for i, port := range service.Spec.Ports {
			if i > 0 {
				m.log.Print(", ")
			}
			m.log.Print("%d:%d/%s", port.Port, port.TargetPort.IntVal, port.Protocol)
		}
		m.log.Println()
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check Secret
	m.log.Println("\nSECRET:")
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "postgres-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Println("  Status: Not Found")
		} else {
			m.log.Error("  Error: %v\n", err)
		}
	} else {
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", secret.Name)
		m.log.Info("  Type: %s\n", secret.Type)
		m.log.Print("  Data keys: ")
		i := 0
		for key := range secret.Data {
			if i > 0 {
				m.log.Print(", ")
			}
			m.log.Print(key)
			i++
		}
		m.log.Println()
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check PVC
	m.log.Println("\nPERSISTENT VOLUME CLAIM:")
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "postgres-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Println("  Status: Not Found")
		} else {
			m.log.Error("  Error: %v\n", err)
		}
	} else {
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", pvc.Name)
		m.log.Info("  Status: %s\n", pvc.Status.Phase)
		m.log.Info("  Volume: %s\n", pvc.Spec.VolumeName)
		m.log.Info("  Capacity: %s\n", pvc.Status.Capacity.Storage())
		m.log.Info("  Access Modes: %v\n", pvc.Spec.AccessModes)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=postgres",
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Println("  No pods found")
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
			m.log.Info("  %-40s %-10s %-10s %-10s\n",
				pod.Name,
				fmt.Sprintf("%d/%d", ready, total),
				pod.Status.Phase,
				k8s.FormatAge(age))
		}
	}
	return nil
}

func (m *PostgresModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "postgres")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("postgres_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting Postgres backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=postgres",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=postgres")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	dumpFile := filepath.Join(backupDir, fmt.Sprintf("postgres_dump_%s.sql.gz", timestamp))

	// Execute pg_dumpall in pod and stream to file
	m.log.Info("💾 Creating full database dump (pg_dumpall)...\n")

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	// Use sh -c to simple pipe
	finalCmdStr := fmt.Sprintf(`%s exec -n %s %s -- bash -c "pg_dumpall -U \"\$POSTGRES_USER\" --clean --if-exists" | gzip > %s`, kubectlCmd, m.ModuleConfig.Namespace, podName, dumpFile)

	cmd := exec.CommandContext(ctx, "sh", "-c", finalCmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create dump: %w", err)
	}

	fileInfo, err := os.Stat(dumpFile)
	if err != nil {
		return fmt.Errorf("failed to stat dump file: %w", err)
	}
	m.log.Success("✅ Database dump created (%d bytes)\n", fileInfo.Size())

	// Metadata
	m.log.Info("📋 Writing metadata...\n")
	metadataFile := filepath.Join(backupDir, "backup_info.txt")
	metadata := fmt.Sprintf(`Postgres Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: postgres
Pod: %s

Dump File:
%s

Restore Command:
personal-server postgres restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(dumpFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server postgres restore %s\n", timestamp)

	return nil
}

func (m *PostgresModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server postgres restore [TIMESTAMP|latest]")
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
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "postgres_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "postgres_backup_")
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
		timestamp = strings.TrimPrefix(latestDir, "postgres_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("postgres_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	dumpFile := filepath.Join(targetBackupDir, fmt.Sprintf("postgres_dump_%s.sql.gz", timestamp))
	if _, err := os.Stat(dumpFile); os.IsNotExist(err) {
		return fmt.Errorf("dump file missing: %s", dumpFile)
	}

	m.log.Info("🔄 Starting Postgres restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Database will be restored from %s\n", dumpFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=postgres",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=postgres")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	m.log.Info("💾 Restoring database (this may take a while)...\n")

	// Restore command: gunzip -c file | kubectl exec ... -- psql -U $POSTGRES_USER postgres

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	finalCmdStr := fmt.Sprintf(`gunzip -c %s | %s exec -i -n %s %s -- bash -c "psql -U \"\$POSTGRES_USER\" postgres"`, dumpFile, kubectlCmd, m.ModuleConfig.Namespace, podName)

	restoreCmd := exec.CommandContext(ctx, "sh", "-c", finalCmdStr)
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr

	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	m.log.Success("✅ Database restored\n")
	m.log.Success("🎉 Restore complete!\n")

	return nil
}

func (m *PostgresModule) AddDB(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: personal-server postgres add-db <DB_NAME> <DB_USER> <DB_PASS>")
	}

	dbName := args[0]
	dbUser := args[1]
	dbPass := args[2]

	// Validation
	validator := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validator.MatchString(dbName) {
		return fmt.Errorf("invalid DB_NAME: must match ^[a-zA-Z0-9_]+$")
	}
	if !validator.MatchString(dbUser) {
		return fmt.Errorf("invalid DB_USER: must match ^[a-zA-Z0-9_]+$")
	}

	// Escape single quotes for SQL
	dbPassEsc := strings.ReplaceAll(dbPass, "'", "''")

	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=postgres",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=postgres")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	// Helper to exec command
	execPSQL := func(sql string, db string) error {
		// Use heredoc with EOF to avoid escaping issues
		cmdStr := fmt.Sprintf(`%s exec -i -n %s %s -- bash -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "%s" -v ON_ERROR_STOP=1' <<'EOF'
%s
EOF`, kubectlCmd, m.ModuleConfig.Namespace, podName, db, sql)
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("command failed: %s\nOutput: %s", err, string(output))
		}
		return nil
	}

	// Check readiness
	m.log.Info("Waiting for Postgres to be ready in pod %s...\n", podName)
	checkCmd := fmt.Sprintf(`%s exec -n %s %s -- sh -c 'pg_isready -U "$POSTGRES_USER" -h 127.0.0.1 -p 5432'`, kubectlCmd, m.ModuleConfig.Namespace, podName)
	ready := false
	for i := 0; i < 30; i++ {
		if err := exec.CommandContext(ctx, "sh", "-c", checkCmd).Run(); err == nil {
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		return fmt.Errorf("postgres is not ready")
	}

	m.log.Info("Creating/Altering role '%s'...\n", dbUser)
	createRoleSQL := fmt.Sprintf(`
DO $$
BEGIN
   IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN
      CREATE ROLE "%s" LOGIN PASSWORD '%s';
   ELSE
      ALTER ROLE "%s" WITH LOGIN PASSWORD '%s';
   END IF;
END
$$;`, dbUser, dbUser, dbPassEsc, dbUser, dbPassEsc)
	if err := execPSQL(createRoleSQL, "postgres"); err != nil {
		return err
	}

	m.log.Info("Ensuring database '%s' exists...\n", dbName)
	checkDBAuthCmd := fmt.Sprintf(`%s exec -n %s %s -- bash -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d postgres -Atqc "SELECT 1 FROM pg_database WHERE datname = ''%s'';"'`, kubectlCmd, m.ModuleConfig.Namespace, podName, dbName)
	out, _ := exec.CommandContext(ctx, "sh", "-c", checkDBAuthCmd).Output()
	if !strings.Contains(string(out), "1") {
		createDBCmd := fmt.Sprintf(`%s exec -n %s %s -- bash -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"%s\" OWNER \"%s\";"'`, kubectlCmd, m.ModuleConfig.Namespace, podName, dbName, dbUser)
		if out, err := exec.CommandContext(ctx, "sh", "-c", createDBCmd).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create database: %s\nOutput: %s", err, string(out))
		}
	}

	m.log.Info("Granting privileges on database '%s' to '%s'...\n", dbName, dbUser)
	grantsSQL := fmt.Sprintf(`
GRANT CONNECT ON DATABASE "%s" TO "%s";
GRANT USAGE ON SCHEMA public TO "%s";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "%s";
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO "%s";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO "%s";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO "%s";
ALTER SCHEMA public OWNER TO "%s";
`, dbName, dbUser, dbUser, dbUser, dbUser, dbUser, dbUser, dbUser)

	if err := execPSQL(grantsSQL, dbName); err != nil {
		m.log.Warn("Some grants might have failed or been redundant: %v\n", err)
	}

	m.log.Success("✅ Database and user setup complete for %s / %s\n", dbName, dbUser)
	return nil
}

func (m *PostgresModule) RemoveDB(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: personal-server postgres remove-db <DB_NAME> <DB_USER>")
	}

	dbName := args[0]
	dbUser := args[1]

	// Validation
	validator := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validator.MatchString(dbName) {
		return fmt.Errorf("invalid DB_NAME: must match ^[a-zA-Z0-9_]+$")
	}
	if !validator.MatchString(dbUser) {
		return fmt.Errorf("invalid DB_USER: must match ^[a-zA-Z0-9_]+$")
	}

	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=postgres",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=postgres")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	m.log.Info("Terminating active connections to database '%s'...\n", dbName)
	terminateSQL := fmt.Sprintf(`
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '%s'
AND pid <> pg_backend_pid();
`, dbName)

	cmdStr := fmt.Sprintf(`%s exec -i -n %s %s -- bash -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d "postgres" -v ON_ERROR_STOP=1' <<'EOF'
%s
EOF`, kubectlCmd, m.ModuleConfig.Namespace, podName, terminateSQL)
	// Ignore errors during termination
	_ = exec.CommandContext(ctx, "sh", "-c", cmdStr).Run()

	m.log.Info("Dropping database '%s'...\n", dbName)
	dropDBCmd := fmt.Sprintf(`%s exec -n %s %s -- bash -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS \"%s\";"'`, kubectlCmd, m.ModuleConfig.Namespace, podName, dbName)
	if out, err := exec.CommandContext(ctx, "sh", "-c", dropDBCmd).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to drop database: %s\nOutput: %s", err, string(out))
	}

	m.log.Info("Dropping role '%s'...\n", dbUser)
	dropRoleCmd := fmt.Sprintf(`%s exec -n %s %s -- bash -c 'PGPASSWORD="$POSTGRES_PASSWORD" psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 -c "DROP ROLE IF EXISTS \"%s\";"'`, kubectlCmd, m.ModuleConfig.Namespace, podName, dbUser)
	if out, err := exec.CommandContext(ctx, "sh", "-c", dropRoleCmd).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to drop role: %s\nOutput: %s", err, string(out))
	}

	m.log.Success("✅ Database '%s' and user '%s' removed\n", dbName, dbUser)
	return nil
}
