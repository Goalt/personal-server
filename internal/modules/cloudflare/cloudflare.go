package cloudflare

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
	"k8s.io/apimachinery/pkg/util/intstr"
)

type CloudflareModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *CloudflareModule {
	return &CloudflareModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *CloudflareModule) Name() string {
	return "cloudflare"
}

func (m *CloudflareModule) Generate(ctx context.Context) error {
	// Get cloudflare API token
	apiToken, exists := m.ModuleConfig.Secrets["cloudflare_api_token"]
	if !exists || apiToken == "" {
		return fmt.Errorf("cloudflare API token not found in module secrets")
	}

	// Define output directory
	outputDir := filepath.Join("configs", "cloudflare")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Cloudflare Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	secret, deployment := m.prepare(apiToken)

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

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 2/2 Cloudflare configurations generated successfully\n")
	return nil
}

func (m *CloudflareModule) Apply(ctx context.Context) error {
	// Get cloudflare API token
	apiToken, exists := m.ModuleConfig.Secrets["cloudflare_api_token"]
	if !exists || apiToken == "" {
		return fmt.Errorf("cloudflare API token not found in module secrets")
	}

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Cloudflare Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "tunnel-token", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'tunnel-token' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "cloudflared-deployment", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'cloudflared-deployment' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	secret, deployment := m.prepare(apiToken)

	// Apply Secret
	m.log.Progress("Applying Secret: tunnel-token\n")
	createdSecret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	m.log.Success("Created Secret: %s\n", createdSecret.Name)

	// Apply Deployment
	m.log.Progress("\nApplying Deployment: cloudflared-deployment\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: Cloudflare configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for cloudflare module
func (m *CloudflareModule) prepare(apiToken string) (*corev1.Secret, *appsv1.Deployment) {
	// Prepare Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tunnel-token",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "cloudflared",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte(apiToken),
		},
	}

	// Prepare Deployment
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudflared-deployment",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "cloudflared",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"pod": "cloudflared",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"pod": "cloudflared",
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						Sysctls: []corev1.Sysctl{
							{
								Name:  "net.ipv4.ping_group_range",
								Value: "65532 65532",
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "cloudflared",
							Image: "cloudflare/cloudflared:2025.11.1",
							Env: []corev1.EnvVar{
								{
									Name: "TUNNEL_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "tunnel-token",
											},
											Key: "token",
										},
									},
								},
							},
							Command: []string{
								"cloudflared",
								"tunnel",
								"--no-autoupdate",
								"--loglevel",
								"debug",
								"--metrics",
								"0.0.0.0:2000",
								"run",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(2000),
									},
								},
								FailureThreshold:    1,
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
						},
					},
				},
			},
		},
	}

	return secret, deployment
}

func (m *CloudflareModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Cloudflare Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: cloudflared-deployment\n")
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "cloudflared-deployment", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'cloudflared-deployment' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: cloudflared-deployment\n")
		successCount++
	}

	// Delete Secret
	m.log.Info("\n🗑️  Deleting Secret: tunnel-token\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "tunnel-token", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret 'tunnel-token' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: tunnel-token\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/2 cloudflare resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *CloudflareModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Cloudflare resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	resourceFound := false

	// Check Secret
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "tunnel-token", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Secret 'tunnel-token' not found\n")
		} else {
			m.log.Error("Error checking secret: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Secret 'tunnel-token'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Type: %s\n", secret.Type)
		m.log.Info("   Data keys: %v\n", getMapKeys(secret.Data))
	}

	m.log.Println()

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "cloudflared-deployment", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'cloudflared-deployment' not found\n")
		} else {
			m.log.Error("Error checking deployment: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'cloudflared-deployment'\n")
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
		LabelSelector: "pod=cloudflared",
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
		m.log.Println("\nNo Cloudflare resources found. Run 'cloudflare apply' to create them.")
	}
	return nil
}

// getMapKeys returns the keys of a map as a slice
func getMapKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
