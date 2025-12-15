package namespace

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceModule struct {
	GeneralConfig config.GeneralConfig
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, log logger.Logger) *NamespaceModule {
	return &NamespaceModule{
		GeneralConfig: generalConfig,
		log:           log,
	}
}

func (m *NamespaceModule) Name() string {
	return "namespace"
}

func (m *NamespaceModule) Generate(ctx context.Context) error {
	// Check if namespaces are defined
	if len(m.GeneralConfig.Namespaces) == 0 {
		return fmt.Errorf("no namespaces found in configuration file")
	}

	// Define output directory
	outputDir := filepath.Join("configs", "namespace")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Kubernetes namespace configurations...\n")
	m.log.Info("Output directory: %s\n", outputDir)
	m.log.Info("Total namespaces to generate: %d\n\n", len(m.GeneralConfig.Namespaces))

	// Prepare Kubernetes objects
	namespaces := m.prepare()

	// Generate YAML files for each namespace
	for _, namespace := range namespaces {
		// Convert to JSON
		jsonBytes, err := json.Marshal(namespace)
		if err != nil {
			return fmt.Errorf("failed to convert namespace '%s' to JSON: %w", namespace.Name, err)
		}

		// Convert JSON to YAML
		yamlContent, err := k8s.JSONToYAML(string(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to convert namespace '%s' to YAML: %w", namespace.Name, err)
		}

		// Write to file
		filename := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", namespace.Name))
		if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
			return fmt.Errorf("failed to write namespace '%s' to file: %w", namespace.Name, err)
		}

		m.log.Success("Generated: %s\n", filename)
	}

	m.log.Info("\nCompleted: %d/%d namespace configurations generated successfully\n", len(namespaces), len(namespaces))
	return nil
}

func (m *NamespaceModule) Apply(ctx context.Context) error {
	// Check if namespaces are defined
	if len(m.GeneralConfig.Namespaces) == 0 {
		return fmt.Errorf("no namespaces found in configuration file")
	}

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Kubernetes namespace configurations...\n")
	m.log.Info("Total namespaces to apply: %d\n\n", len(m.GeneralConfig.Namespaces))

	// Check if resources already exist
	m.log.Info("Checking for existing namespaces...\n")
	for _, namespaceName := range m.GeneralConfig.Namespaces {
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
		if err == nil {
			return fmt.Errorf("namespace '%s' already exists", namespaceName)
		} else if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check namespace '%s' existence: %w", namespaceName, err)
		}
	}

	m.log.Info("No existing namespaces found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	namespaces := m.prepare()

	// Apply namespaces
	for _, namespace := range namespaces {
		m.log.Progress("Applying Namespace: %s\n", namespace.Name)
		createdNs, err := clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace '%s': %w", namespace.Name, err)
		}
		m.log.Success("Created Namespace: %s\n", createdNs.Name)
	}

	m.log.Info("\nCompleted: %d/%d namespaces applied successfully\n", len(namespaces), len(namespaces))
	return nil
}

func (m *NamespaceModule) prepare() []*corev1.Namespace {
	namespaces := make([]*corev1.Namespace, 0, len(m.GeneralConfig.Namespaces))

	for _, namespaceName := range m.GeneralConfig.Namespaces {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
				Labels: map[string]string{
					"managed-by": "personal-server",
				},
			},
		}
		namespaces = append(namespaces, namespace)
	}

	return namespaces
}

func (m *NamespaceModule) Clean(ctx context.Context) error {
	// Check if namespaces are defined
	if len(m.GeneralConfig.Namespaces) == 0 {
		return fmt.Errorf("no namespaces found in configuration file")
	}

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Kubernetes namespaces...\n")
	m.log.Info("Total namespaces to delete: %d\n\n", len(m.GeneralConfig.Namespaces))

	successCount := 0

	for _, namespaceName := range m.GeneralConfig.Namespaces {
		m.log.Info("🗑️  Processing namespace: %s\n", namespaceName)

		// Try to delete the namespace
		deletePolicy := metav1.DeletePropagationForeground
		deleteOptions := metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		}

		err := clientset.CoreV1().Namespaces().Delete(ctx, namespaceName, deleteOptions)
		if err != nil {
			if errors.IsNotFound(err) {
				m.log.Warn("Namespace '%s' not found (already deleted or never existed)\n", namespaceName)
			} else {
				m.log.Error("Failed to delete namespace '%s': %v\n", namespaceName, err)
				continue
			}
		} else {
			m.log.Success("Deleted namespace: %s\n", namespaceName)
			successCount++
		}
	}

	m.log.Info("\nCompleted: %d/%d namespaces deleted successfully\n", successCount, len(m.GeneralConfig.Namespaces))
	if successCount > 0 {
		m.log.Println("\nNote: Namespace deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *NamespaceModule) Status(ctx context.Context) error {
	// Check if namespaces are defined
	if len(m.GeneralConfig.Namespaces) == 0 {
		return fmt.Errorf("no namespaces found in configuration file")
	}

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// First, check which namespaces exist
	existingNamespaces := make(map[string]bool)
	for _, namespaceName := range m.GeneralConfig.Namespaces {
		_, err := clientset.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
		if err == nil {
			existingNamespaces[namespaceName] = true
		}
	}

	// Display namespace status
	if len(existingNamespaces) > 0 {
		m.log.Println("Existing namespaces:")
		namespaceList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			m.log.Error("Error listing namespaces: %v\n", err)
		} else {
			m.log.Info("%-20s %-10s %-20s\n", "NAME", "STATUS", "AGE")
			for _, ns := range namespaceList.Items {
				if existingNamespaces[ns.Name] {
					age := time.Since(ns.CreationTimestamp.Time).Round(time.Second)
					m.log.Info("%-20s %-10s %-20s\n", ns.Name, ns.Status.Phase, k8s.FormatAge(age))
				}
			}
		}
	} else {
		m.log.Println("None of the configured namespaces exist.")
	}

	m.log.Println()

	// List resources in each namespace
	for _, namespaceName := range m.GeneralConfig.Namespaces {
		if !existingNamespaces[namespaceName] {
			m.log.Info("Namespace '%s' not found.\n\n", namespaceName)
			continue
		}

		m.log.Info("Resources in namespace '%s':\n", namespaceName)
		m.log.Println("---")

		// Get Pods
		pods, err := clientset.CoreV1().Pods(namespaceName).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.log.Error("Error listing pods: %v\n", err)
		} else if len(pods.Items) > 0 {
			m.log.Info("\nPODS:\n")
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

		// Get Services
		services, err := clientset.CoreV1().Services(namespaceName).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.log.Error("Error listing services: %v\n", err)
		} else if len(services.Items) > 0 {
			m.log.Info("\nSERVICES:\n")
			m.log.Info("%-40s %-15s %-15s %-10s\n", "NAME", "TYPE", "CLUSTER-IP", "AGE")
			for _, svc := range services.Items {
				age := time.Since(svc.CreationTimestamp.Time).Round(time.Second)
				m.log.Info("%-40s %-15s %-15s %-10s\n",
					svc.Name,
					svc.Spec.Type,
					svc.Spec.ClusterIP,
					k8s.FormatAge(age))
			}
		}

		// Get Deployments
		deployments, err := clientset.AppsV1().Deployments(namespaceName).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.log.Error("Error listing deployments: %v\n", err)
		} else if len(deployments.Items) > 0 {
			m.log.Info("\nDEPLOYMENTS:\n")
			m.log.Info("%-40s %-10s %-10s %-10s\n", "NAME", "READY", "UP-TO-DATE", "AGE")
			for _, dep := range deployments.Items {
				age := time.Since(dep.CreationTimestamp.Time).Round(time.Second)
				m.log.Info("%-40s %-10s %-10d %-10s\n",
					dep.Name,
					fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, dep.Status.Replicas),
					dep.Status.UpdatedReplicas,
					k8s.FormatAge(age))
			}
		}

		// Get StatefulSets
		statefulsets, err := clientset.AppsV1().StatefulSets(namespaceName).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.log.Error("Error listing statefulsets: %v\n", err)
		} else if len(statefulsets.Items) > 0 {
			m.log.Info("\nSTATEFULSETS:\n")
			m.log.Info("%-40s %-10s %-10s\n", "NAME", "READY", "AGE")
			for _, sts := range statefulsets.Items {
				age := time.Since(sts.CreationTimestamp.Time).Round(time.Second)
				m.log.Info("%-40s %-10s %-10s\n",
					sts.Name,
					fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, sts.Status.Replicas),
					k8s.FormatAge(age))
			}
		}

		// Get PersistentVolumeClaims
		pvcs, err := clientset.CoreV1().PersistentVolumeClaims(namespaceName).List(ctx, metav1.ListOptions{})
		if err != nil {
			m.log.Error("Error listing PVCs: %v\n", err)
		} else if len(pvcs.Items) > 0 {
			m.log.Info("\nPERSISTENT VOLUME CLAIMS:\n")
			m.log.Info("%-40s %-10s %-15s %-10s\n", "NAME", "STATUS", "VOLUME", "AGE")
			for _, pvc := range pvcs.Items {
				age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
				m.log.Info("%-40s %-10s %-15s %-10s\n",
					pvc.Name,
					pvc.Status.Phase,
					pvc.Spec.VolumeName,
					k8s.FormatAge(age))
			}
		}

		// Check if namespace is empty
		if len(pods.Items) == 0 && len(services.Items) == 0 && len(deployments.Items) == 0 &&
			len(statefulsets.Items) == 0 && len(pvcs.Items) == 0 {
			m.log.Println("No resources found in this namespace.")
		}

		m.log.Println()
	}
	return nil
}
