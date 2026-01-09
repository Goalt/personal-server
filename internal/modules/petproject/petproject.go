package petproject

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	// Write Service if configured
	service := m.prepareService()
	if service != nil {
		if err := writeYAML(service, "service"); err != nil {
			return err
		}
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

	// Apply Service if configured
	service := m.prepareService()
	if service != nil {
		serviceName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)
		m.log.Progress("Applying Service: %s\n", serviceName)
		createdService, err := clientset.CoreV1().Services(m.ProjectConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Service: %w", err)
		}
		m.log.Success("Created Service: %s\n", createdService.Name)
	}

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
					ImagePullSecrets: func() []corev1.LocalObjectReference {
						if m.ProjectConfig.ImagePullSecret == "" {
							return nil
						}
						return []corev1.LocalObjectReference{
							{Name: m.ProjectConfig.ImagePullSecret},
						}
					}(),
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	return deployment
}

func (m *PetProjectModule) prepareService() *corev1.Service {
	// Only create service if configured
	if m.ProjectConfig.Service == nil || len(m.ProjectConfig.Service.Ports) == 0 {
		return nil
	}

	serviceName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)

	// Convert config ports to Kubernetes ServicePort
	ports := make([]corev1.ServicePort, 0, len(m.ProjectConfig.Service.Ports))
	for _, port := range m.ProjectConfig.Service.Ports {
		ports = append(ports, corev1.ServicePort{
			Name:       port.Name,
			Port:       port.Port,
			TargetPort: intstr.FromInt(int(port.TargetPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: m.ProjectConfig.Namespace,
			Labels: map[string]string{
				"app":        serviceName,
				"managed-by": "personal-server",
				"type":       "pet-project",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": serviceName,
			},
			Ports: ports,
		},
	}

	return service
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

	// Delete Service if configured
	if m.ProjectConfig.Service != nil && len(m.ProjectConfig.Service.Ports) > 0 {
		serviceName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)
		m.log.Info("🗑️  Deleting Service: %s\n", serviceName)
		err = clientset.CoreV1().Services(m.ProjectConfig.Namespace).Delete(ctx, serviceName, deleteOptions)
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("Service '%s' not found (already deleted or never existed)\n", serviceName)
			} else {
				m.log.Error("Failed to delete Service: %v\n", err)
				return err
			}
		} else {
			m.log.Success("Deleted Service: %s\n", serviceName)
		}
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
		m.log.Info("  No pods found with label app=%s\n", deploymentName)
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

	// Check Service if configured
	if m.ProjectConfig.Service != nil && len(m.ProjectConfig.Service.Ports) > 0 {
		m.log.Println("\nSERVICE:")
		serviceName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)
		service, err := clientset.CoreV1().Services(m.ProjectConfig.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Error("  Service '%s' not found\n", serviceName)
			} else {
				m.log.Error("  Error getting service: %v\n", err)
			}
		} else {
			age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
			m.log.Info("  Name: %s\n", service.Name)
			m.log.Info("  Namespace: %s\n", service.Namespace)
			m.log.Info("  Type: %s\n", service.Spec.Type)
			m.log.Info("  Age: %s\n", k8s.FormatAge(age))
			m.log.Info("  Ports:\n")
			for _, port := range service.Spec.Ports {
				m.log.Info("    - %s: %d -> %s\n", port.Name, port.Port, port.TargetPort.String())
			}
			m.log.Success("  Status: Ready\n")
		}
	}

	m.log.Println()
	return nil
}

// Rollout performs kubectl rollout operations on the pet project deployment
func (m *PetProjectModule) Rollout(ctx context.Context, args []string) error {
	deploymentName := fmt.Sprintf("pet-%s", m.ProjectConfig.Name)

	if len(args) == 0 {
		return fmt.Errorf("usage: %s rollout <restart|status|history|undo>\nAvailable rollout commands: restart, status, history, undo", m.ProjectConfig.Name)
	}

	operation := args[0]

	// Validate operation
	validOps := map[string]bool{
		"restart": true,
		"status":  true,
		"history": true,
		"undo":    true,
	}
	if !validOps[operation] {
		return fmt.Errorf("unknown rollout operation: %s\nAvailable rollout commands: restart, status, history, undo", operation)
	}

	kubectlCmd := "kubectl"
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s kubectl"
	}

	m.log.Info("🔄 Executing rollout %s for pet project '%s'...\n", operation, m.ProjectConfig.Name)

	// Build kubectl rollout command
	cmdStr := fmt.Sprintf("%s rollout %s deployment/%s -n %s", kubectlCmd, operation, deploymentName, m.ProjectConfig.Namespace)
	cmdParts := strings.Fields(cmdStr)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)

	// Capture output for status, history operations
	if operation == "status" || operation == "history" {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("rollout %s failed: %w\nOutput: %s", operation, err, string(output))
		}
		m.log.Info("%s", string(output))
		m.log.Success("✅ Rollout %s completed successfully\n", operation)
	} else {
		// For restart and undo, just execute
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("rollout %s failed: %w", operation, err)
		}
		m.log.Success("✅ Rollout %s completed successfully\n", operation)
	}

	return nil
}
