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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	xrayPort         = 10000
	xrayImage        = "ghcr.io/xtls/xray-core:latest"
	xrayConfigVolume = "xray-config"
	xrayConfigMount  = "/etc/xray"
	xrayConfigFile   = "config.json"
)

// xrayConfig is the top-level Xray configuration structure.
type xrayConfig struct {
	Inbounds  []xrayInbound  `json:"inbounds"`
	Outbounds []xrayOutbound `json:"outbounds"`
}

type xrayInbound struct {
	Port           int                  `json:"port"`
	Listen         string               `json:"listen"`
	Protocol       string               `json:"protocol"`
	Settings       xrayInboundSettings  `json:"settings"`
	StreamSettings xrayStreamSettings   `json:"streamSettings"`
}

type xrayInboundSettings struct {
	Clients    []xrayClient `json:"clients"`
	Decryption string       `json:"decryption"`
}

type xrayClient struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
}

type xrayStreamSettings struct {
	Network    string         `json:"network"`
	WsSettings xrayWsSettings `json:"wsSettings"`
}

type xrayWsSettings struct {
	Path string `json:"path"`
}

type xrayOutbound struct {
	Protocol string `json:"protocol"`
}

// XrayModule manages a VLESS + WebSocket + TLS VPN deployment using xray-core.
type XrayModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

// New constructs an XrayModule.
func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *XrayModule {
	return &XrayModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *XrayModule) Name() string {
	return "xray"
}

// Doc prints human-readable documentation about the module.
func (m *XrayModule) Doc(ctx context.Context) error {
	m.log.Info("Module: xray\n\n")
	m.log.Info("Description:\n  Deploys Xray (VLESS + WebSocket + TLS) VPN inside Kubernetes.\n  Traffic is hidden behind the existing Ingress Controller on port 443,\n  masquerading as regular HTTPS traffic routed to a secret WebSocket path.\n\n")
	m.log.Info("Required configuration keys (modules[].secrets):\n  xray_uuid   UUID for the VLESS user (generate with: xray uuid)\n  xray_path   Secret WebSocket path served by the Ingress (e.g. /vpn-secret-path)\n\n")
	m.log.Info("Optional configuration keys (modules[].secrets):\n  xray_image  Custom container image (default: ghcr.io/xtls/xray-core:latest)\n\n")
	m.log.Info("TLS prerequisites:\n  A Kubernetes TLS secret named 'xray-tls' must exist in the target namespace\n  before applying the Ingress. You can create it with cert-manager or manually:\n    kubectl create secret tls xray-tls --cert=tls.crt --key=tls.key -n <namespace>\n\n")
	m.log.Info("Subcommands:\n  generate   Write Kubernetes YAML to configs/xray/\n  apply      Create/update resources in the cluster\n  clean      Delete all Xray resources from the cluster\n  status     Print Deployment and Pod status\n  doc        Show this documentation\n")
	return nil
}

// prepare returns all Kubernetes objects needed for the Xray deployment.
func (m *XrayModule) prepare() (*corev1.ConfigMap, *appsv1.Deployment, *corev1.Service, *networkingv1.Ingress, error) {
	uuid := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "xray_uuid", "")
	path := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "xray_path", "/xray")
	image := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "xray_image", xrayImage)

	if uuid == "" {
		return nil, nil, nil, nil, fmt.Errorf("xray_uuid secret is required")
	}

	ns := m.ModuleConfig.Namespace
	labels := map[string]string{
		"app":        "xray",
		"managed-by": "personal-server",
	}

	// Build xray config.json
	cfg := xrayConfig{
		Inbounds: []xrayInbound{
			{
				Port:     xrayPort,
				Listen:   "0.0.0.0",
				Protocol: "vless",
				Settings: xrayInboundSettings{
					Clients:    []xrayClient{{ID: uuid, Level: 0}},
					Decryption: "none",
				},
				StreamSettings: xrayStreamSettings{
					Network:    "ws",
					WsSettings: xrayWsSettings{Path: path},
				},
			},
		},
		Outbounds: []xrayOutbound{
			{Protocol: "freedom"},
		},
	}
	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to marshal xray config: %w", err)
	}

	// ConfigMap
	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xray-config",
			Namespace: ns,
			Labels:    labels,
		},
		Data: map[string]string{
			xrayConfigFile: string(configJSON),
		},
	}

	// Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xray",
			Namespace: ns,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "xray"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "xray"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "xray",
							Image:           image,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									Name:          "xray",
									ContainerPort: xrayPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      xrayConfigVolume,
									MountPath: xrayConfigMount,
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: xrayConfigVolume,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "xray-config",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Service (ClusterIP)
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xray",
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "xray",
					Port:       xrayPort,
					TargetPort: intstr.FromInt(xrayPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{"app": "xray"},
		},
	}

	// Ingress with WebSocket annotations
	pathType := networkingv1.PathTypePrefix
	domain := m.GeneralConfig.Domain
	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "xray",
			Namespace: ns,
			Labels:    labels,
			Annotations: map[string]string{
				"managed-by": "personal-server",
				"nginx.ingress.kubernetes.io/proxy-read-timeout":  "3600",
				"nginx.ingress.kubernetes.io/proxy-send-timeout":  "3600",
				"nginx.ingress.kubernetes.io/proxy-body-size":     "0",
				"nginx.ingress.kubernetes.io/proxy-http-version":  "1.1",
				"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_set_header Upgrade $http_upgrade;\nproxy_set_header Connection \"upgrade\";\n",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{domain},
					SecretName: "xray-tls",
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: domain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "xray",
											Port: networkingv1.ServiceBackendPort{
												Number: xrayPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return configMap, deployment, service, ingress, nil
}

// Generate writes Kubernetes YAML files to configs/xray/.
func (m *XrayModule) Generate(ctx context.Context) error {
	outputDir := filepath.Join("configs", "xray")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Xray Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	configMap, deployment, service, ingress, err := m.prepare()
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

	if err := writeYAML(configMap, "configmap"); err != nil {
		return err
	}
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}
	if err := writeYAML(service, "service"); err != nil {
		return err
	}
	if err := writeYAML(ingress, "ingress"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 4/4 Xray configurations generated successfully\n")
	return nil
}

// Apply creates all Xray Kubernetes resources.
func (m *XrayModule) Apply(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Xray Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	ns := m.ModuleConfig.Namespace

	// Check existing resources
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().ConfigMaps(ns).Get(ctx, "xray-config", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ConfigMap 'xray-config' already exists in namespace '%s'", ns)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ConfigMap existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(ns).Get(ctx, "xray", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Deployment 'xray' already exists in namespace '%s'", ns)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Deployment existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(ns).Get(ctx, "xray", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Service 'xray' already exists in namespace '%s'", ns)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Service existence: %w", err)
	}

	_, err = clientset.NetworkingV1().Ingresses(ns).Get(ctx, "xray", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Ingress 'xray' already exists in namespace '%s'", ns)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Ingress existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	configMap, deployment, service, ingress, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare resources: %w", err)
	}

	m.log.Progress("Applying ConfigMap: xray-config\n")
	_, err = clientset.CoreV1().ConfigMaps(ns).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}
	m.log.Success("Created ConfigMap: xray-config\n")

	m.log.Progress("Applying Deployment: xray\n")
	_, err = clientset.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: xray\n")

	m.log.Progress("Applying Service: xray\n")
	_, err = clientset.CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: xray\n")

	m.log.Progress("Applying Ingress: xray\n")
	_, err = clientset.NetworkingV1().Ingresses(ns).Create(ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Ingress: %w", err)
	}
	m.log.Success("Created Ingress: xray\n")

	m.log.Info("\nCompleted: Xray configurations applied successfully\n")
	return nil
}

// Clean removes all Xray Kubernetes resources.
func (m *XrayModule) Clean(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Xray Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	ns := m.ModuleConfig.Namespace
	del := metav1.DeleteOptions{}

	m.log.Progress("Deleting Ingress: xray\n")
	if err := clientset.NetworkingV1().Ingresses(ns).Delete(ctx, "xray", del); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Ingress: %w", err)
	}
	m.log.Success("Deleted Ingress: xray\n")

	m.log.Progress("Deleting Service: xray\n")
	if err := clientset.CoreV1().Services(ns).Delete(ctx, "xray", del); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Service: %w", err)
	}
	m.log.Success("Deleted Service: xray\n")

	m.log.Progress("Deleting Deployment: xray\n")
	if err := clientset.AppsV1().Deployments(ns).Delete(ctx, "xray", del); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Deployment: %w", err)
	}
	m.log.Success("Deleted Deployment: xray\n")

	m.log.Progress("Deleting ConfigMap: xray-config\n")
	if err := clientset.CoreV1().ConfigMaps(ns).Delete(ctx, "xray-config", del); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ConfigMap: %w", err)
	}
	m.log.Success("Deleted ConfigMap: xray-config\n")

	m.log.Info("\nCompleted: Xray resources cleaned successfully\n")
	return nil
}

// Status prints the current state of all Xray Kubernetes resources.
func (m *XrayModule) Status(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ns := m.ModuleConfig.Namespace

	dep, err := clientset.AppsV1().Deployments(ns).Get(ctx, "xray", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Info("Deployment 'xray' not found in namespace '%s'\n", ns)
			return nil
		}
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	m.log.Info("Deployment: %s  Ready: %d/%d  Age: %s\n",
		dep.Name,
		dep.Status.ReadyReplicas,
		dep.Status.Replicas,
		k8s.FormatAge(time.Since(dep.CreationTimestamp.Time)),
	)

	// List pods
	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "app=xray",
	})
	if err != nil {
		return fmt.Errorf("failed to list Pods: %w", err)
	}

	for _, pod := range pods.Items {
		m.log.Info("Pod: %s  Phase: %s  Age: %s\n",
			pod.Name,
			pod.Status.Phase,
			k8s.FormatAge(time.Since(pod.CreationTimestamp.Time)),
		)
	}

	return nil
}
