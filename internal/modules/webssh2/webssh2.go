package webssh2

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/k8s"
	"github.com/Goalt/personal-server/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type WebSSH2Module struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *WebSSH2Module {
	return &WebSSH2Module{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *WebSSH2Module) Name() string {
	return "webssh2"
}

func (m *WebSSH2Module) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "webssh2")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating WebSSH2 Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	configMap, service, deployment := m.prepare()

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

	// Write ConfigMap
	if err := writeYAML(configMap, "configmap"); err != nil {
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

	m.log.Info("\nCompleted: 3/3 WebSSH2 configurations generated successfully\n")
	return nil
}

func (m *WebSSH2Module) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying WebSSH2 Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, "webssh2-config", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("configmap 'webssh2-config' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check configmap existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "webssh2", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'webssh2' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "webssh2", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'webssh2' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	configMap, service, deployment := m.prepare()

	// Apply ConfigMap
	m.log.Info("Creating ConfigMap...\n")
	_, err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}
	m.log.Success("ConfigMap 'webssh2-config' created successfully\n")

	// Apply Service
	m.log.Info("Creating Service...\n")
	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Service 'webssh2' created successfully\n")

	// Apply Deployment
	m.log.Info("Creating Deployment...\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Deployment 'webssh2' created successfully\n")

	m.log.Info("\nCompleted: All WebSSH2 resources applied successfully\n")
	return nil
}

func (m *WebSSH2Module) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning up WebSSH2 Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Define resources to delete
	resources := []struct {
		name string
		fn   func() error
	}{
		{
			name: "Deployment 'webssh2'",
			fn: func() error {
				return clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "webssh2", metav1.DeleteOptions{})
			},
		},
		{
			name: "Service 'webssh2'",
			fn: func() error {
				return clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "webssh2", metav1.DeleteOptions{})
			},
		},
		{
			name: "ConfigMap 'webssh2-config'",
			fn: func() error {
				return clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Delete(ctx, "webssh2-config", metav1.DeleteOptions{})
			},
		},
	}

	// Delete each resource
	for _, res := range resources {
		m.log.Info("Deleting %s...\n", res.name)
		if err := res.fn(); err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("%s not found, skipping\n", res.name)
			} else {
				return fmt.Errorf("failed to delete %s: %w", res.name, err)
			}
		} else {
			m.log.Success("%s deleted successfully\n", res.name)
		}
	}

	m.log.Info("\nCompleted: WebSSH2 cleanup finished\n")
	return nil
}

func (m *WebSSH2Module) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking WebSSH2 status in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "webssh2", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'webssh2' not found\n")
		} else {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		m.log.Info("Deployment 'webssh2':\n")
		m.log.Info("  Replicas: %d/%d\n", deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
		m.log.Info("  Available: %d\n", deployment.Status.AvailableReplicas)
		m.log.Info("  Updated: %d\n", deployment.Status.UpdatedReplicas)
	}

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "webssh2", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'webssh2' not found\n")
		} else {
			return fmt.Errorf("failed to get service: %w", err)
		}
	} else {
		m.log.Info("\nService 'webssh2':\n")
		m.log.Info("  Type: %s\n", service.Spec.Type)
		m.log.Info("  ClusterIP: %s\n", service.Spec.ClusterIP)
		for _, port := range service.Spec.Ports {
			m.log.Info("  Port: %d -> %d/%s\n", port.Port, port.TargetPort.IntVal, port.Protocol)
		}
	}

	// Check Pods
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=webssh2",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	m.log.Info("\nPods:\n")
	if len(pods.Items) == 0 {
		m.log.Warn("  No pods found\n")
	} else {
		for _, pod := range pods.Items {
			m.log.Info("  %s: %s\n", pod.Name, pod.Status.Phase)
		}
	}

	return nil
}

func (m *WebSSH2Module) prepare() (*corev1.ConfigMap, *corev1.Service, *appsv1.Deployment) {
	// Get configuration from secrets
	headerText := m.ModuleConfig.Secrets["header_text"]
	if headerText == "" {
		headerText = "WebSSH2"
	}

	sshHost := m.ModuleConfig.Secrets["ssh_host"]
	if sshHost == "" {
		sshHost = ""
	}

	authAllowed := m.ModuleConfig.Secrets["auth_allowed"]
	if authAllowed == "" {
		authAllowed = "password,publickey,keyboard-interactive"
	}

	listenPort := m.ModuleConfig.Secrets["listen_port"]
	if listenPort == "" {
		listenPort = "2222"
	}

	image := m.ModuleConfig.Secrets["image"]
	if image == "" {
		image = "ghcr.io/billchurch/webssh2:latest"
	}

	// ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webssh2-config",
			Namespace: m.ModuleConfig.Namespace,
		},
		Data: map[string]string{
			"WEBSSH2_LISTEN_PORT": listenPort,
			"WEBSSH2_HEADER_TEXT": headerText,
			"WEBSSH2_AUTH_ALLOWED": authAllowed,
		},
	}

	// Add SSH host if specified
	if sshHost != "" {
		configMap.Data["WEBSSH2_SSH_HOST"] = sshHost
	}

	// Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webssh2",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "webssh2",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "webssh2",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       2222,
					TargetPort: intstr.FromInt(2222),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	// Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webssh2",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "webssh2",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "webssh2",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "webssh2",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "webssh2",
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 2222,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "webssh2-config",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ssh",
										Port: intstr.FromInt(2222),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ssh",
										Port: intstr.FromInt(2222),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
								FailureThreshold:    3,
							},
						},
					},
				},
			},
		},
	}

	return configMap, service, deployment
}
