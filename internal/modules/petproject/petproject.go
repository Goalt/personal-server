package petproject

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

type PetProjectModule struct {
	GeneralConfig config.GeneralConfig
	ProjectConfig config.PetProject
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, projectConfig config.PetProject, log logger.Logger) *PetProjectModule {
	return &PetProjectModule{
		GeneralConfig: generalConfig,
		ProjectConfig: projectConfig,
		log:           log,
	}
}

func (m *PetProjectModule) Name() string {
	return m.ProjectConfig.Name
}

func (m *PetProjectModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "pet-projects", m.ProjectConfig.Name)

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating pet project '%s' Kubernetes configurations...\n", m.ProjectConfig.Name)
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	deployment := m.prepareDeployment()

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

	m.log.Info("\nCompleted: pet project '%s' configurations generated successfully\n", m.ProjectConfig.Name)
	return nil
}

func (m *PetProjectModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying pet project '%s' Kubernetes configurations...\n", m.ProjectConfig.Name)
	m.log.Info("Target namespace: %s\n\n", m.ProjectConfig.Namespace)

	deploymentName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)

	// Check if deployment already exists
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.AppsV1().Deployments(m.ProjectConfig.Namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment '%s' already exists in namespace '%s'", deploymentName, m.ProjectConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	deployment := m.prepareDeployment()

	// Apply Deployment
	m.log.Progress("Applying Deployment: %s\n", deploymentName)
	createdDeployment, err := clientset.AppsV1().Deployments(m.ProjectConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: pet project '%s' resources applied successfully\n", m.ProjectConfig.Name)
	return nil
}

func (m *PetProjectModule) prepareDeployment() *appsv1.Deployment {
	replicas := int32(1)
	deploymentName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)

	// Convert environment map to EnvVar slice
	envVars := make([]corev1.EnvVar, 0, len(m.ProjectConfig.Environment))
	for key, value := range m.ProjectConfig.Environment {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: m.ProjectConfig.Namespace,
			Labels: map[string]string{
				"app":        deploymentName,
				"managed-by": "personal-server",
				"type":       "pet-project",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  deploymentName,
						"type": "pet-project",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  m.ProjectConfig.Name,
							Image: m.ProjectConfig.Image,
							Env:   envVars,
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	return deployment
}

func (m *PetProjectModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	deploymentName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)

	m.log.Info("Cleaning pet project '%s' Kubernetes resources...\n", m.ProjectConfig.Name)
	m.log.Info("Target namespace: %s\n\n", m.ProjectConfig.Namespace)

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: %s\n", deploymentName)
	err = clientset.AppsV1().Deployments(m.ProjectConfig.Namespace).Delete(ctx, deploymentName, deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment '%s' not found (already deleted or never existed)\n", deploymentName)
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
			return err
		}
	} else {
		m.log.Success("Deleted Deployment: %s\n", deploymentName)
	}

	m.log.Info("\nCompleted: pet project '%s' resources deleted successfully\n", m.ProjectConfig.Name)
	m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	return nil
}

func (m *PetProjectModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	deploymentName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)

	m.log.Info("Checking pet project '%s' resources in namespace '%s'...\n\n", m.ProjectConfig.Name, m.ProjectConfig.Namespace)

	// Check Deployment
	m.log.Println("DEPLOYMENT:")
	deployment, err := clientset.AppsV1().Deployments(m.ProjectConfig.Namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment '%s' not found\n", deploymentName)
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

	// Get Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ProjectConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", deploymentName),
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Println("  No pods found with label app=%s", deploymentName)
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
