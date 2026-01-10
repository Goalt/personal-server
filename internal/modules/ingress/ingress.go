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
	// Check if ingress rules are defined
	if len(m.IngressConfig.Rules) == 0 {
		return fmt.Errorf("no ingress rules found in configuration")
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
	m.log.Info("Total rules: %d\n\n", len(m.IngressConfig.Rules))

	// Prepare Kubernetes objects
	ingress := m.prepare()

	// Convert to JSON
	jsonBytes, err := json.Marshal(ingress)
	if err != nil {
		return fmt.Errorf("failed to convert Ingress to JSON: %w", err)
	}

	// Convert JSON to YAML
	yamlContent, err := k8s.JSONToYAML(string(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to convert Ingress to YAML: %w", err)
	}

	// Write to file
	filename := filepath.Join(outputDir, "ingress.yaml")
	if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write Ingress to file: %w", err)
	}

	m.log.Success("Generated: %s\n", filename)
	m.log.Info("\nCompleted: Ingress configuration generated successfully\n")
	return nil
}

func (m *IngressModule) Apply(ctx context.Context) error {
	// Check if ingress rules are defined
	if len(m.IngressConfig.Rules) == 0 {
		return fmt.Errorf("no ingress rules found in configuration")
	}

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Ingress Kubernetes configurations...\n")
	m.log.Info("Ingress name: %s\n", m.IngressConfig.Name)
	m.log.Info("Target namespace: %s\n\n", m.IngressConfig.Namespace)

	// Check if resource already exists
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Get(ctx, m.IngressConfig.Name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Ingress '%s' already exists in namespace '%s'", m.IngressConfig.Name, m.IngressConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Ingress existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	ingress := m.prepare()

	// Apply Ingress
	m.log.Progress("Applying Ingress: %s\n", m.IngressConfig.Name)
	createdIngress, err := clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Ingress: %w", err)
	}
	m.log.Success("Created Ingress: %s\n", createdIngress.Name)

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

func (m *IngressModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Ingress resources...\n")
	m.log.Info("Ingress name: %s\n", m.IngressConfig.Name)
	m.log.Info("Namespace: %s\n\n", m.IngressConfig.Namespace)

	// Try to delete the Ingress
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

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

	// Get Ingress
	ingress, err := clientset.NetworkingV1().Ingresses(m.IngressConfig.Namespace).Get(ctx, m.IngressConfig.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Ingress '%s' not found in namespace '%s'\n", m.IngressConfig.Name, m.IngressConfig.Namespace)
			return nil
		}
		return fmt.Errorf("failed to get Ingress: %w", err)
	}

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
	return nil
}
