package pgadmin

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

type PgadminModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *PgadminModule {
	return &PgadminModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *PgadminModule) Name() string {
	return "pgadmin"
}

func (m *PgadminModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "pgadmin")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating pgadmin Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	secret, service, deployment, err := m.prepare()
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

	// Write Service
	if err := writeYAML(service, "service"); err != nil {
		return err
	}

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 3/3 pgadmin configurations generated successfully\n")
	return nil
}

func (m *PgadminModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying pgadmin Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "pgadmin-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'pgadmin-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "pgadmin", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'pgadmin' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "pgadmin", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'pgadmin' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	secret, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
	}

	// Apply Secret
	m.log.Progress("Applying Secret: pgadmin-secrets\n")
	createdSecret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}
	m.log.Success("Created Secret: %s\n", createdSecret.Name)

	// Apply Service
	m.log.Progress("Applying Service: pgadmin\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: pgadmin\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 3/3 pgadmin resources applied successfully\n")
	return nil
}

func (m *PgadminModule) prepare() (*corev1.Secret, *corev1.Service, *appsv1.Deployment, error) {
	// Prepare Secret
	secretData := make(map[string][]byte)

	email, exists := m.ModuleConfig.Secrets["pgadmin_default_email"]
	if !exists {
		return nil, nil, nil, fmt.Errorf("pgadmin_default_email not found in configuration")
	}
	secretData["pgadmin_default_email"] = []byte(email)

	password, exists := m.ModuleConfig.Secrets["pgadmin_admin_password"]
	if !exists {
		return nil, nil, nil, fmt.Errorf("pgadmin_admin_password not found in configuration")
	}
	secretData["pgadmin_admin_password"] = []byte(password)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pgadmin-secrets",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "pgadmin",
				"managed-by": "personal-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pgadmin",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "pgadmin",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "pgadmin",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	revisionHistoryLimit := int32(1)
	terminationGracePeriodSeconds := int64(0)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pgadmin",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "pgadmin",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &replicas,
			RevisionHistoryLimit: &revisionHistoryLimit,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "pgadmin",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "pgadmin",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "pgadmin",
							Image:           "dpage/pgadmin4:9.10.0",
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{
									Name: "PGADMIN_DEFAULT_EMAIL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "pgadmin-secrets",
											},
											Key:      "pgadmin_default_email",
											Optional: k8s.BoolPtr(false),
										},
									},
								},
								{
									Name:  "PGADMIN_CONFIG_ENHANCED_COOKIE_PROTECTION",
									Value: "False",
								},
								{
									Name: "PGADMIN_DEFAULT_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "pgadmin-secrets",
											},
											Key:      "pgadmin_admin_password",
											Optional: k8s.BoolPtr(false),
										},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
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

	return secret, service, deployment, nil
}

func (m *PgadminModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning pgadmin Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Processing Deployment: pgadmin\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "pgadmin", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: pgadmin\n")
		successCount++
	}

	// Delete Service
	m.log.Info("🗑️  Processing Service: pgadmin\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "pgadmin", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: pgadmin\n")
		successCount++
	}

	// Delete Secret
	m.log.Info("🗑️  Processing Secret: pgadmin-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "pgadmin-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: pgadmin-secrets\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/3 pgadmin resources deleted successfully\n", successCount)
	return nil
}

func (m *PgadminModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking pgadmin resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "pgadmin", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'pgadmin': Not found\n")
		} else {
			m.log.Error("Deployment 'pgadmin': Error: %v\n", err)
		}
	} else {
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'pgadmin': Found\n")
		m.log.Info("  Replicas: %d/%d ready\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
		m.log.Info("  Image: %s\n", deployment.Spec.Template.Spec.Containers[0].Image)
	}

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "pgadmin", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("\nService 'pgadmin': Not found\n")
		} else {
			m.log.Error("\nService 'pgadmin': Error: %v\n", err)
		}
	} else {
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("\nService 'pgadmin': Found\n")
		m.log.Info("  Type: %s\n", service.Spec.Type)
		m.log.Info("  Cluster IP: %s\n", service.Spec.ClusterIP)
		m.log.Info("  Ports: %v\n", service.Spec.Ports[0].Port)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check Secret
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "pgadmin-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("\nSecret 'pgadmin-secrets': Not found\n")
		} else {
			m.log.Error("\nSecret 'pgadmin-secrets': Error: %v\n", err)
		}
	} else {
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("\nSecret 'pgadmin-secrets': Found\n")
		m.log.Info("  Type: %s\n", secret.Type)
		m.log.Info("  Data keys: %d\n", len(secret.Data))
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// List Pods
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=pgadmin",
	})
	if err != nil {
		m.log.Error("\nError listing pods: %v\n", err)
	} else if len(pods.Items) > 0 {
		m.log.Info("\nPods:\n")
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
		m.log.Info("\nNo pods found with label app=pgadmin\n")
	}
	return nil
}
