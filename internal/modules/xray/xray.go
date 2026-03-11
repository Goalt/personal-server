package xray

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

const (
	deploymentName = "xray"
	serviceName    = "xray-service"
	configMapName  = "xray-config"
	secretName     = "xray-secrets"
	containerPort  = int32(10000)
	defaultImage   = "ghcr.io/xtls/xray-core:latest"
	defaultWsPath  = "/vpn-ws"
)

// XrayModule manages the Xray VPN (VLESS + WebSocket) Kubernetes resources.
type XrayModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

// New creates a new XrayModule.
func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *XrayModule {
	return &XrayModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

// Name returns the module name.
func (m *XrayModule) Name() string {
	return "xray"
}

// Generate writes Kubernetes manifests for the Xray module to disk.
func (m *XrayModule) Generate(ctx context.Context) error {
	outputDir := filepath.Join("configs", "xray")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Xray VPN Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	secret, configMap, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare resources: %w", err)
	}

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

	if err := writeYAML(secret, "secret"); err != nil {
		return err
	}
	if err := writeYAML(configMap, "configmap"); err != nil {
		return err
	}
	if err := writeYAML(service, "service"); err != nil {
		return err
	}
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 4/4 Xray configurations generated successfully\n")
	return nil
}

// Apply creates Kubernetes resources for the Xray module in the cluster.
func (m *XrayModule) Apply(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Xray VPN Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	m.log.Info("Checking for existing resources...\n")

	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Secret '%s' already exists in namespace '%s'", secretName, m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Secret existence: %w", err)
	}

	_, err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ConfigMap '%s' already exists in namespace '%s'", configMapName, m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ConfigMap existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Service '%s' already exists in namespace '%s'", serviceName, m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Deployment '%s' already exists in namespace '%s'", deploymentName, m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	secret, configMap, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare resources: %w", err)
	}

	m.log.Progress("Applying Secret: %s\n", secretName)
	createdSecret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}
	m.log.Success("Created Secret: %s\n", createdSecret.Name)

	m.log.Progress("Applying ConfigMap: %s\n", configMapName)
	createdCM, err := clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}
	m.log.Success("Created ConfigMap: %s\n", createdCM.Name)

	m.log.Progress("Applying Service: %s\n", serviceName)
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	m.log.Progress("Applying Deployment: %s\n", deploymentName)
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 4/4 resources applied successfully\n")
	return nil
}

// Clean removes all Kubernetes resources created by the Xray module.
func (m *XrayModule) Clean(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Xray VPN Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalResources := 4

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	m.log.Info("🗑️  Deleting Deployment: %s\n", deploymentName)
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, deploymentName, deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment '%s' not found (already deleted or never existed)\n", deploymentName)
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: %s\n", deploymentName)
		successCount++
	}

	m.log.Info("🗑️  Deleting Service: %s\n", serviceName)
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, serviceName, deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service '%s' not found (already deleted or never existed)\n", serviceName)
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: %s\n", serviceName)
		successCount++
	}

	m.log.Info("🗑️  Deleting ConfigMap: %s\n", configMapName)
	err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Delete(ctx, configMapName, deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ConfigMap '%s' not found (already deleted or never existed)\n", configMapName)
		} else {
			m.log.Error("Failed to delete ConfigMap: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ConfigMap: %s\n", configMapName)
		successCount++
	}

	m.log.Info("🗑️  Deleting Secret: %s\n", secretName)
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, secretName, deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret '%s' not found (already deleted or never existed)\n", secretName)
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: %s\n", secretName)
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d resources deleted successfully\n", successCount, totalResources)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

// Status displays the current status of Xray VPN Kubernetes resources.
func (m *XrayModule) Status(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Xray VPN resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	m.log.Println("DEPLOYMENT:")
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment '%s' not found\n", deploymentName)
		} else {
			m.log.Error("  Error getting deployment: %v\n", err)
		}
	} else {
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", deployment.Name)
		m.log.Info("  Ready: %d/%d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
		m.log.Info("  Up-to-date: %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("  Available: %d\n", deployment.Status.AvailableReplicas)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	m.log.Println("\nSERVICE:")
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Service '%s' not found\n", serviceName)
		} else {
			m.log.Error("  Error getting service: %v\n", err)
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

	m.log.Println("\nCONFIGMAP:")
	cm, err := clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ConfigMap '%s' not found\n", configMapName)
		} else {
			m.log.Error("  Error getting ConfigMap: %v\n", err)
		}
	} else {
		age := time.Since(cm.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", cm.Name)
		m.log.Info("  Data keys: %d\n", len(cm.Data))
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	m.log.Println("\nSECRET:")
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Secret '%s' not found\n", secretName)
		} else {
			m.log.Error("  Error getting Secret: %v\n", err)
		}
	} else {
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", secret.Name)
		m.log.Info("  Type: %s\n", secret.Type)
		m.log.Info("  Data keys: %d\n", len(secret.Data))
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	return nil
}

// prepare builds the Kubernetes objects for the Xray VPN module.
func (m *XrayModule) prepare() (*corev1.Secret, *corev1.ConfigMap, *corev1.Service, *appsv1.Deployment, error) {
	uuid, exists := m.ModuleConfig.Secrets["xray_uuid"]
	if !exists || uuid == "" {
		return nil, nil, nil, nil, fmt.Errorf("xray_uuid not found in configuration — generate one with 'xray uuid' or 'uuidgen'")
	}
	wsPath := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "xray_websocket_path", defaultWsPath)
	image := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "xray_image", defaultImage)

	// Secret stores the UUID for reference / external tooling
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "xray",
				"managed-by": "personal-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"xray_uuid": uuid,
		},
	}

	// ConfigMap holds the Xray config.json (VLESS + WebSocket, no TLS — TLS is terminated at Ingress)
	configJSON := fmt.Sprintf(`{
    "log": {
        "loglevel": "warning"
    },
    "inbounds": [
        {
            "port": %d,
            "listen": "0.0.0.0",
            "protocol": "vless",
            "settings": {
                "clients": [
                    {
                        "id": "%s",
                        "level": 0
                    }
                ],
                "decryption": "none"
            },
            "streamSettings": {
                "network": "ws",
                "security": "none",
                "wsSettings": {
                    "path": "%s"
                }
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom",
            "settings": {}
        },
        {
            "protocol": "blackhole",
            "settings": {},
            "tag": "blocked"
        }
    ]
}`, containerPort, uuid, wsPath)

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "xray",
				"managed-by": "personal-server",
			},
		},
		Data: map[string]string{
			"config.json": configJSON,
		},
	}

	// Service exposes the Xray WebSocket listener as an internal ClusterIP service
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "xray",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "ws",
					Port:       containerPort,
					TargetPort: intstr.FromInt(int(containerPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "xray",
			},
		},
	}

	// Deployment runs the Xray core container
	replicas := int32(1)
	allowPrivilegeEscalation := false
	runAsNonRoot := true
	readOnlyRootFilesystem := true

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "xray",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "xray",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "xray",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:  "xray",
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "ws",
									ContainerPort: containerPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "xray-config",
									MountPath: "/etc/xray",
									ReadOnly:  true,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								RunAsNonRoot:             &runAsNonRoot,
								ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "xray-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return secret, configMap, service, deployment, nil
}
