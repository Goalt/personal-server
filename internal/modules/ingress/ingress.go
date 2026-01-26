package ingress

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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IngressModule struct {
	GeneralConfig config.GeneralConfig
	IngressConfig config.IngressConfig
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, ingressConfig config.IngressConfig, log logger.Logger) *IngressModule {
	return &IngressModule{
		GeneralConfig: generalConfig,
		IngressConfig: ingressConfig,
		log:           log,
	}
}

func (m *IngressModule) Name() string {
	return "ingress"
}

func (m *IngressModule) Generate(ctx context.Context) error {
	// Check if ingress rules or TCP/UDP services are defined
	if len(m.IngressConfig.Rules) == 0 && len(m.IngressConfig.TCPServices) == 0 && len(m.IngressConfig.UDPServices) == 0 {
		return fmt.Errorf("no ingress rules, TCP services, or UDP services found in configuration")
	}

	// Define output directory
	outputDir := filepath.Join("configs", "ingress", m.IngressConfig.Name)

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Ingress Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n", outputDir)
	m.log.Info("Ingress name: %s\n", m.IngressConfig.Name)
	m.log.Info("Namespace: %s\n", m.IngressConfig.Namespace)
	m.log.Info("Total HTTP rules: %d\n", len(m.IngressConfig.Rules))
	m.log.Info("Total TCP services: %d\n", len(m.IngressConfig.TCPServices))
	m.log.Info("Total UDP services: %d\n\n", len(m.IngressConfig.UDPServices))

	// Helper function to write YAML
	writeYAML := func(obj interface{}, filename string) error {
		jsonBytes, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to convert to JSON: %w", err)
		}

		yamlContent, err := k8s.JSONToYAML(string(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to convert to YAML: %w", err)
		}

		fullPath := filepath.Join(outputDir, filename)
		if err := os.WriteFile(fullPath, []byte(yamlContent), 0644); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}

		m.log.Success("Generated: %s\n", fullPath)
		return nil
	}

	// Generate HTTP Ingress if rules are defined
	if len(m.IngressConfig.Rules) > 0 {
		ingress := m.prepare()
		if err := writeYAML(ingress, "ingress.yaml"); err != nil {
			return err
		}
	}

	// Generate TCP ConfigMap if TCP services are defined
	if tcpConfigMap := m.prepareTCPConfigMap(); tcpConfigMap != nil {
		if err := writeYAML(tcpConfigMap, "tcp-configmap.yaml"); err != nil {
			return err
		}
	}

	// Generate UDP ConfigMap if UDP services are defined
	if udpConfigMap := m.prepareUDPConfigMap(); udpConfigMap != nil {
		if err := writeYAML(udpConfigMap, "udp-configmap.yaml"); err != nil {
			return err
		}
	}

	m.log.Info("\nCompleted: Ingress configuration generated successfully\n")
	return nil
}

func (m *IngressModule) Apply(ctx context.Context) error {
	// Check if ingress rules or TCP/UDP services are defined
	if len(m.IngressConfig.Rules) == 0 && len(m.IngressConfig.TCPServices) == 0 && len(m.IngressConfig.UDPServices) == 0 {
		return fmt.Errorf("no ingress rules, TCP services, or UDP services found in configuration")
	}

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Ingress Kubernetes configurations...\n")
	m.log.Info("Ingress name: %s\n", m.IngressConfig.Name)
	m.log.Info("Target namespace: %s\n\n", m.IngressConfig.Namespace)

	// Apply HTTP Ingress if rules are defined
	if len(m.IngressConfig.Rules) > 0 {
		// Check if resource already exists
		m.log.Info("Checking for existing Ingress...\n")
		_, err = clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Get(ctx, m.IngressConfig.Name, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("Ingress '%s' already exists in namespace '%s'", m.IngressConfig.Name, m.IngressConfig.Namespace)
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check Ingress existence: %w", err)
		}

		m.log.Info("No existing Ingress found, proceeding with creation...\n\n")

		// Prepare and apply Ingress
		ingress := m.prepare()
		m.log.Progress("Applying Ingress: %s\n", m.IngressConfig.Name)
		createdIngress, err := clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create Ingress: %w", err)
		}
		m.log.Success("Created Ingress: %s\n", createdIngress.Name)
	}

	// Apply TCP ConfigMap if TCP services are defined
	if tcpConfigMap := m.prepareTCPConfigMap(); tcpConfigMap != nil {
		tcpName := tcpConfigMap.Name
		m.log.Info("Checking for existing TCP ConfigMap...\n")
		_, err = clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Get(ctx, tcpName, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("TCP ConfigMap '%s' already exists in namespace '%s'", tcpName, m.IngressConfig.Namespace)
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check TCP ConfigMap existence: %w", err)
		}

		m.log.Progress("Applying TCP ConfigMap: %s\n", tcpName)
		_, err = clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Create(ctx, tcpConfigMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create TCP ConfigMap: %w", err)
		}
		m.log.Success("Created TCP ConfigMap: %s\n", tcpName)
	}

	// Apply UDP ConfigMap if UDP services are defined
	if udpConfigMap := m.prepareUDPConfigMap(); udpConfigMap != nil {
		udpName := udpConfigMap.Name
		m.log.Info("Checking for existing UDP ConfigMap...\n")
		_, err = clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Get(ctx, udpName, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("UDP ConfigMap '%s' already exists in namespace '%s'", udpName, m.IngressConfig.Namespace)
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check UDP ConfigMap existence: %w", err)
		}

		m.log.Progress("Applying UDP ConfigMap: %s\n", udpName)
		_, err = clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Create(ctx, udpConfigMap, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create UDP ConfigMap: %w", err)
		}
		m.log.Success("Created UDP ConfigMap: %s\n", udpName)
	}

	m.log.Info("\nCompleted: Ingress applied successfully\n")
	return nil
}

func (m *IngressModule) prepare() *networkingv1.Ingress {
	// Default path type if not specified
	defaultPathType := networkingv1.PathTypePrefix

	// Build ingress rules
	var ingressRules []networkingv1.IngressRule
	var tlsHosts []string

	// Group rules by host
	hostRules := make(map[string][]networkingv1.HTTPIngressPath)
	for _, rule := range m.IngressConfig.Rules {
		host := rule.Host
		if host == "" {
			host = m.GeneralConfig.Domain
		}

		path := rule.Path
		if path == "" {
			path = "/"
		}

		pathType := &defaultPathType
		if rule.PathType != "" {
			switch rule.PathType {
			case "Exact":
				pt := networkingv1.PathTypeExact
				pathType = &pt
			case "Prefix":
				pt := networkingv1.PathTypePrefix
				pathType = &pt
			case "ImplementationSpecific":
				pt := networkingv1.PathTypeImplementationSpecific
				pathType = &pt
			}
		}

		httpPath := networkingv1.HTTPIngressPath{
			Path:     path,
			PathType: pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: rule.ServiceName,
					Port: networkingv1.ServiceBackendPort{
						Number: rule.ServicePort,
					},
				},
			},
		}

		hostRules[host] = append(hostRules[host], httpPath)

		// Collect hosts for TLS if enabled
		if m.IngressConfig.TLS {
			tlsHosts = append(tlsHosts, host)
		}
	}

	// Convert map to IngressRule slice
	for host, paths := range hostRules {
		ingressRule := networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		}
		ingressRules = append(ingressRules, ingressRule)
	}

	// Build Ingress object
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.IngressConfig.Name,
			Namespace: m.IngressConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: ingressRules,
		},
	}

	// Add TLS configuration if enabled
	if m.IngressConfig.TLS && len(tlsHosts) > 0 {
		// Remove duplicates from tlsHosts
		uniqueHosts := make(map[string]bool)
		var hosts []string
		for _, host := range tlsHosts {
			if !uniqueHosts[host] {
				uniqueHosts[host] = true
				hosts = append(hosts, host)
			}
		}

		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      hosts,
				SecretName: fmt.Sprintf("%s-tls", m.IngressConfig.Name),
			},
		}
	}

	return ingress
}

// preparePortConfigMap creates a ConfigMap for TCP or UDP services
func (m *IngressModule) preparePortConfigMap(services interface{}, suffix string) *corev1.ConfigMap {
	var data map[string]string

	// Handle both TCP and UDP service types
	switch svc := services.(type) {
	case []config.TCPService:
		if len(svc) == 0 {
			return nil
		}
		data = make(map[string]string)
		for _, s := range svc {
			namespace := s.Namespace
			if namespace == "" {
				namespace = m.IngressConfig.Namespace
			}
			key := fmt.Sprintf("%d", s.Port)
			value := fmt.Sprintf("%s/%s:%d", namespace, s.ServiceName, s.ServicePort)
			data[key] = value
		}
	case []config.UDPService:
		if len(svc) == 0 {
			return nil
		}
		data = make(map[string]string)
		for _, s := range svc {
			namespace := s.Namespace
			if namespace == "" {
				namespace = m.IngressConfig.Namespace
			}
			key := fmt.Sprintf("%d", s.Port)
			value := fmt.Sprintf("%s/%s:%d", namespace, s.ServiceName, s.ServicePort)
			data[key] = value
		}
	default:
		return nil
	}

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", m.IngressConfig.Name, suffix),
			Namespace: m.IngressConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
			},
		},
		Data: data,
	}
}

func (m *IngressModule) prepareTCPConfigMap() *corev1.ConfigMap {
	return m.preparePortConfigMap(m.IngressConfig.TCPServices, "tcp")
}

func (m *IngressModule) prepareUDPConfigMap() *corev1.ConfigMap {
	return m.preparePortConfigMap(m.IngressConfig.UDPServices, "udp")
}

func (m *IngressModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Ingress resources...\n")
	m.log.Info("Ingress name: %s\n", m.IngressConfig.Name)
	m.log.Info("Namespace: %s\n\n", m.IngressConfig.Namespace)

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Try to delete the Ingress
	if len(m.IngressConfig.Rules) > 0 {
		err = clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Delete(ctx, m.IngressConfig.Name, deleteOptions)
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("Ingress '%s' not found (already deleted or never existed)\n", m.IngressConfig.Name)
			} else {
				return fmt.Errorf("failed to delete Ingress '%s': %w", m.IngressConfig.Name, err)
			}
		} else {
			m.log.Success("Deleted Ingress: %s\n", m.IngressConfig.Name)
		}
	}

	// Try to delete TCP ConfigMap
	if len(m.IngressConfig.TCPServices) > 0 {
		tcpConfigMapName := fmt.Sprintf("%s-tcp", m.IngressConfig.Name)
		err = clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Delete(ctx, tcpConfigMapName, deleteOptions)
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("TCP ConfigMap '%s' not found (already deleted or never existed)\n", tcpConfigMapName)
			} else {
				return fmt.Errorf("failed to delete TCP ConfigMap '%s': %w", tcpConfigMapName, err)
			}
		} else {
			m.log.Success("Deleted TCP ConfigMap: %s\n", tcpConfigMapName)
		}
	}

	// Try to delete UDP ConfigMap
	if len(m.IngressConfig.UDPServices) > 0 {
		udpConfigMapName := fmt.Sprintf("%s-udp", m.IngressConfig.Name)
		err = clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Delete(ctx, udpConfigMapName, deleteOptions)
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("UDP ConfigMap '%s' not found (already deleted or never existed)\n", udpConfigMapName)
			} else {
				return fmt.Errorf("failed to delete UDP ConfigMap '%s': %w", udpConfigMapName, err)
			}
		} else {
			m.log.Success("Deleted UDP ConfigMap: %s\n", udpConfigMapName)
		}
	}

	m.log.Info("\nCompleted: Ingress cleanup finished\n")
	return nil
}

func (m *IngressModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Ingress status...\n")
	m.log.Info("Ingress name: %s\n", m.IngressConfig.Name)
	m.log.Info("Namespace: %s\n\n", m.IngressConfig.Namespace)

	// Get and display HTTP Ingress if rules are defined
	if len(m.IngressConfig.Rules) > 0 {
		ingress, err := clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Get(ctx, m.IngressConfig.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("Ingress '%s' not found in namespace '%s'\n", m.IngressConfig.Name, m.IngressConfig.Namespace)
			} else {
				return fmt.Errorf("failed to get Ingress: %w", err)
			}
		} else {
			// Display Ingress information
			m.log.Info("INGRESS DETAILS:\n")
			m.log.Info("---\n")
			m.log.Info("Name: %s\n", ingress.Name)
			m.log.Info("Namespace: %s\n", ingress.Namespace)
			age := time.Since(ingress.CreationTimestamp.Time).Round(time.Second)
			m.log.Info("Age: %s\n", k8s.FormatAge(age))

			// Display rules
			if len(ingress.Spec.Rules) > 0 {
				m.log.Info("\nRULES:\n")
				for _, rule := range ingress.Spec.Rules {
					host := rule.Host
					if host == "" {
						host = "*"
					}
					m.log.Info("  Host: %s\n", host)
					if rule.HTTP != nil {
						for _, path := range rule.HTTP.Paths {
							pathType := "Prefix"
							if path.PathType != nil {
								pathType = string(*path.PathType)
							}
							m.log.Info("    Path: %s (%s) -> %s:%d\n",
								path.Path,
								pathType,
								path.Backend.Service.Name,
								path.Backend.Service.Port.Number)
						}
					}
				}
			}

			// Display TLS configuration
			if len(ingress.Spec.TLS) > 0 {
				m.log.Info("\nTLS:\n")
				for _, tls := range ingress.Spec.TLS {
					m.log.Info("  Secret: %s\n", tls.SecretName)
					m.log.Info("  Hosts: %v\n", tls.Hosts)
				}
			}

			// Display load balancer ingress
			if len(ingress.Status.LoadBalancer.Ingress) > 0 {
				m.log.Info("\nLOAD BALANCER:\n")
				for _, lb := range ingress.Status.LoadBalancer.Ingress {
					if lb.IP != "" {
						m.log.Info("  IP: %s\n", lb.IP)
					}
					if lb.Hostname != "" {
						m.log.Info("  Hostname: %s\n", lb.Hostname)
					}
				}
			}
			m.log.Println()
		}
	}

	// Get and display TCP ConfigMap if TCP services are defined
	if len(m.IngressConfig.TCPServices) > 0 {
		tcpConfigMapName := fmt.Sprintf("%s-tcp", m.IngressConfig.Name)
		tcpCM, err := clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Get(ctx, tcpConfigMapName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("TCP ConfigMap '%s' not found in namespace '%s'\n\n", tcpConfigMapName, m.IngressConfig.Namespace)
			} else {
				return fmt.Errorf("failed to get TCP ConfigMap: %w", err)
			}
		} else {
			m.log.Info("TCP SERVICES:\n")
			m.log.Info("---\n")
			m.log.Info("ConfigMap: %s\n", tcpCM.Name)
			age := time.Since(tcpCM.CreationTimestamp.Time).Round(time.Second)
			m.log.Info("Age: %s\n", k8s.FormatAge(age))
			m.log.Info("\nTCP Port Mappings:\n")
			for port, backend := range tcpCM.Data {
				m.log.Info("  %s -> %s\n", port, backend)
			}
			m.log.Println()
		}
	}

	// Get and display UDP ConfigMap if UDP services are defined
	if len(m.IngressConfig.UDPServices) > 0 {
		udpConfigMapName := fmt.Sprintf("%s-udp", m.IngressConfig.Name)
		udpCM, err := clientset.CoreV1().ConfigMaps(m.IngressConfig.Namespace).Get(ctx, udpConfigMapName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("UDP ConfigMap '%s' not found in namespace '%s'\n\n", udpConfigMapName, m.IngressConfig.Namespace)
			} else {
				return fmt.Errorf("failed to get UDP ConfigMap: %w", err)
			}
		} else {
			m.log.Info("UDP SERVICES:\n")
			m.log.Info("---\n")
			m.log.Info("ConfigMap: %s\n", udpCM.Name)
			age := time.Since(udpCM.CreationTimestamp.Time).Round(time.Second)
			m.log.Info("Age: %s\n", k8s.FormatAge(age))
			m.log.Info("\nUDP Port Mappings:\n")
			for port, backend := range udpCM.Data {
				m.log.Info("  %s -> %s\n", port, backend)
			}
			m.log.Println()
		}
	}

	return nil
}
