package postgresexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PostgresExporterModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *PostgresExporterModule {
	return &PostgresExporterModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *PostgresExporterModule) Name() string {
	return "postgres-exporter"
}

func (m *PostgresExporterModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "postgres-exporter")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Postgres Exporter Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	deployment, err := m.prepare()
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

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 1/1 Postgres Exporter configurations generated successfully\n")
	return nil
}

func (m *PostgresExporterModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Postgres Exporter Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "postgres-exporter", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'postgres-exporter' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
	}

	// Apply Deployment
	m.log.Progress("Applying Deployment: postgres-exporter\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 1/1 resources applied successfully\n")
	return nil
}

func (m *PostgresExporterModule) prepare() (*appsv1.Deployment, error) {
	// Get configuration values with defaults
	dataSourceURI := m.getSecretOrDefault("data_source_uri", "postgres:5432/postgres?sslmode=disable")
	dataSourceUser := m.getSecretOrDefault("data_source_user", "postgres")
	dataSourcePass := m.getSecretOrDefault("data_source_pass", "postgres")
	extendQueryPath := m.getSecretOrDefault("extend_query_path", "")
	includeDatabases := m.getSecretOrDefault("include_databases", "postgres")

	// Prepare Deployment
	replicas := int32(1)
	revisionHistoryLimit := int32(1)
	terminationGracePeriodSeconds := int64(0)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-exporter",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "postgres-exporter",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &replicas,
			RevisionHistoryLimit: &revisionHistoryLimit,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "postgres-exporter",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "postgres-exporter",
					},
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/path":   "/metrics",
						"prometheus.io/port":   "9187",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "postgres-exporter",
							Image:           "quay.io/prometheuscommunity/postgres-exporter:latest",
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9187,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DATA_SOURCE_URI",
									Value: dataSourceURI,
								},
								{
									Name:  "DATA_SOURCE_USER",
									Value: dataSourceUser,
								},
								{
									Name:  "DATA_SOURCE_PASS",
									Value: dataSourcePass,
								},
								{
									Name:  "PG_EXPORTER_INCLUDE_DATABASES",
									Value: includeDatabases,
								},
							},
						},
					},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	// Add optional environment variables if they are set
	if extendQueryPath != "" {
		deployment.Spec.Template.Spec.Containers[0].Env = append(
			deployment.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "PG_EXPORTER_EXTEND_QUERY_PATH",
				Value: extendQueryPath,
			},
		)
	}

	return deployment, nil
}

func (m *PostgresExporterModule) getSecretOrDefault(key, defaultValue string) string {
	if value, exists := m.ModuleConfig.Secrets[key]; exists {
		return value
	}
	return defaultValue
}

func (m *PostgresExporterModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Postgres Exporter Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalResources := 1 // Deployment

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: postgres-exporter\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "postgres-exporter", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: postgres-exporter\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d resources deleted successfully\n", successCount, totalResources)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *PostgresExporterModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Postgres Exporter resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	m.log.Println("DEPLOYMENT:")
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "postgres-exporter", metav1.GetOptions{})
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

	// Check Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=postgres-exporter",
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
