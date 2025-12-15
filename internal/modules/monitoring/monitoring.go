package monitoring

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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitoringModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *MonitoringModule {
	return &MonitoringModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *MonitoringModule) Name() string {
	return "monitoring"
}

func (m *MonitoringModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "monitoring")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Sentry Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	serviceAccount, clusterRole, clusterRoleBinding, secret, deployment, err := m.prepare()
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

	// Write ServiceAccount
	if err := writeYAML(serviceAccount, "serviceaccount"); err != nil {
		return err
	}

	// Write ClusterRole
	if err := writeYAML(clusterRole, "clusterrole"); err != nil {
		return err
	}

	// Write ClusterRoleBinding
	if err := writeYAML(clusterRoleBinding, "clusterrolebinding"); err != nil {
		return err
	}

	// Write Secret
	if err := writeYAML(secret, "secret"); err != nil {
		return err
	}

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 5/5 Sentry configurations generated successfully\n")
	return nil
}

func (m *MonitoringModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Sentry Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ServiceAccount 'monitor-sentry-kubernetes' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ServiceAccount existence: %w", err)
	}

	_, err = clientset.RbacV1().ClusterRoles().Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ClusterRole 'monitor-sentry-kubernetes' already exists")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ClusterRole existence: %w", err)
	}

	_, err = clientset.RbacV1().ClusterRoleBindings().Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ClusterRoleBinding 'monitor-sentry-kubernetes' already exists")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ClusterRoleBinding existence: %w", err)
	}

	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Secret 'monitor-sentry-kubernetes' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Secret existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Deployment 'monitor-sentry-kubernetes' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	serviceAccount, clusterRole, clusterRoleBinding, secret, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
	}

	// Apply ServiceAccount
	m.log.Progress("Applying ServiceAccount: monitor-sentry-kubernetes\n")
	createdSA, err := clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ServiceAccount: %w", err)
	}
	m.log.Success("Created ServiceAccount: %s\n", createdSA.Name)

	// Apply ClusterRole
	m.log.Progress("Applying ClusterRole: monitor-sentry-kubernetes\n")
	createdCR, err := clientset.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ClusterRole: %w", err)
	}
	m.log.Success("Created ClusterRole: %s\n", createdCR.Name)

	// Apply ClusterRoleBinding
	m.log.Progress("Applying ClusterRoleBinding: monitor-sentry-kubernetes\n")
	createdCRB, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
	}
	m.log.Success("Created ClusterRoleBinding: %s\n", createdCRB.Name)

	// Apply Secret
	m.log.Progress("Applying Secret: monitor-sentry-kubernetes\n")
	createdSecret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}
	m.log.Success("Created Secret: %s\n", createdSecret.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: monitor-sentry-kubernetes\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 5/5 Sentry configurations applied successfully\n")
	return nil
}

func (m *MonitoringModule) prepare() (*corev1.ServiceAccount, *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding, *corev1.Secret, *appsv1.Deployment, error) {
	// Prepare ServiceAccount
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-sentry-kubernetes",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":      "sentry-kubernetes",
				"heritage": "Helm",
				"release":  "monitor",
				"chart":    "sentry-kubernetes-0.2.6",
			},
		},
	}

	// Prepare ClusterRole
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "monitor-sentry-kubernetes",
			Labels: map[string]string{
				"app":      "sentry-kubernetes",
				"heritage": "Helm",
				"release":  "monitor",
				"chart":    "sentry-kubernetes-0.2.6",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	// Prepare ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "monitor-sentry-kubernetes",
			Labels: map[string]string{
				"app":      "sentry-kubernetes",
				"heritage": "Helm",
				"release":  "monitor",
				"chart":    "sentry-kubernetes-0.2.6",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "monitor-sentry-kubernetes",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "monitor-sentry-kubernetes",
				Namespace: m.ModuleConfig.Namespace,
			},
		},
	}

	// Prepare Secret
	dsn, exists := m.ModuleConfig.Secrets["sentry_dsn"]
	if !exists {
		return nil, nil, nil, nil, nil, fmt.Errorf("sentry_dsn not found in configuration")
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-sentry-kubernetes",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":      "sentry-kubernetes",
				"heritage": "Helm",
				"release":  "monitor",
				"chart":    "sentry-kubernetes-0.2.6",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"sentry.dsn": []byte(dsn),
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitor-sentry-kubernetes",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":      "sentry-kubernetes",
				"heritage": "Helm",
				"release":  "monitor",
				"chart":    "sentry-kubernetes-0.2.6",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "sentry-kubernetes",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "sentry-kubernetes",
						"release": "monitor",
					},
					Annotations: map[string]string{
						"checksum/secrets": "static-place-holder",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "monitor-sentry-kubernetes",
					Containers: []corev1.Container{
						{
							Name:            "sentry-kubernetes",
							Image:           "ghcr.io/goalt/sentry-kubernetes:0b536b48eee946b00cac35e161561f3f31fb1a79",
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								{
									Name: "SENTRY_DSN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "monitor-sentry-kubernetes",
											},
											Key: "sentry.dsn",
										},
									},
								},
								{
									Name:  "SENTRY_K8S_MONITOR_CRONJOBS",
									Value: "true",
								},
								{
									Name:  "SENTRY_K8S_WATCH_NAMESPACES",
									Value: "__all__",
								},
							},
						},
					},
				},
			},
		},
	}

	return serviceAccount, clusterRole, clusterRoleBinding, secret, deployment, nil
}

func (m *MonitoringModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Sentry Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalSteps := 5

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// 1. Delete Deployment
	m.log.Info("🗑️  Deleting Deployment...\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "monitor-sentry-kubernetes", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: monitor-sentry-kubernetes\n")
		successCount++
	}

	// 2. Delete Secret
	m.log.Info("🗑️  Deleting Secret...\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "monitor-sentry-kubernetes", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: monitor-sentry-kubernetes\n")
		successCount++
	}

	// 3. Delete ClusterRoleBinding
	m.log.Info("🗑️  Deleting ClusterRoleBinding...\n")
	err = clientset.RbacV1().ClusterRoleBindings().Delete(ctx, "monitor-sentry-kubernetes", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ClusterRoleBinding not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ClusterRoleBinding: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ClusterRoleBinding: monitor-sentry-kubernetes\n")
		successCount++
	}

	// 4. Delete ClusterRole
	m.log.Info("🗑️  Deleting ClusterRole...\n")
	err = clientset.RbacV1().ClusterRoles().Delete(ctx, "monitor-sentry-kubernetes", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ClusterRole not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ClusterRole: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ClusterRole: monitor-sentry-kubernetes\n")
		successCount++
	}

	// 5. Delete ServiceAccount
	m.log.Info("🗑️  Deleting ServiceAccount...\n")
	err = clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Delete(ctx, "monitor-sentry-kubernetes", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ServiceAccount not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ServiceAccount: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ServiceAccount: monitor-sentry-kubernetes\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d Sentry resources deleted successfully\n", successCount, totalSteps)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *MonitoringModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Sentry resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check ServiceAccount
	m.log.Println("SERVICE ACCOUNT:")
	sa, err := clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ServiceAccount 'monitor-sentry-kubernetes' not found\n")
		} else {
			m.log.Error("  Error getting ServiceAccount: %v\n", err)
		}
	} else {
		age := time.Since(sa.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ServiceAccount: monitor-sentry-kubernetes (Age: %s)\n", k8s.FormatAge(age))
	}

	// Check ClusterRole
	m.log.Println("\nCLUSTER ROLE:")
	cr, err := clientset.RbacV1().ClusterRoles().Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ClusterRole 'monitor-sentry-kubernetes' not found\n")
		} else {
			m.log.Error("  Error getting ClusterRole: %v\n", err)
		}
	} else {
		age := time.Since(cr.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ClusterRole: monitor-sentry-kubernetes (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Rules: %d\n", len(cr.Rules))
	}

	// Check ClusterRoleBinding
	m.log.Println("\nCLUSTER ROLE BINDING:")
	crb, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ClusterRoleBinding 'monitor-sentry-kubernetes' not found\n")
		} else {
			m.log.Error("  Error getting ClusterRoleBinding: %v\n", err)
		}
	} else {
		age := time.Since(crb.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ClusterRoleBinding: monitor-sentry-kubernetes (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Role: %s\n", crb.RoleRef.Name)
		m.log.Info("     Subjects: %d\n", len(crb.Subjects))
	}

	// Check Secret
	m.log.Println("\nSECRET:")
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Secret 'monitor-sentry-kubernetes' not found\n")
		} else {
			m.log.Error("  Error getting Secret: %v\n", err)
		}
	} else {
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  Secret: monitor-sentry-kubernetes (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Type: %s\n", secret.Type)
		m.log.Info("     Keys: %d\n", len(secret.Data))
	}

	// Check Deployment
	m.log.Println("\nDEPLOYMENT:")
	dep, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "monitor-sentry-kubernetes", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment 'monitor-sentry-kubernetes' not found\n")
		} else {
			m.log.Error("  Error getting Deployment: %v\n", err)
		}
	} else {
		age := time.Since(dep.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  Deployment: monitor-sentry-kubernetes (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Replicas: %d/%d ready\n", dep.Status.ReadyReplicas, dep.Status.Replicas)
		m.log.Info("     Updated: %d\n", dep.Status.UpdatedReplicas)
		m.log.Info("     Available: %d\n", dep.Status.AvailableReplicas)
	}

	// Check Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=sentry-kubernetes",
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Warn("  No pods found with label 'app=sentry-kubernetes'\n")
	} else {
		m.log.Info("  Found %d pod(s):\n", len(pods.Items))
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
			if pod.Status.Phase != corev1.PodRunning {
				status = "⚠️"
			}
			m.log.Info("  %s %-40s [%d/%d] %-10s (Age: %s)\n",
				status, pod.Name, ready, total, pod.Status.Phase, k8s.FormatAge(age))
		}
	}

	m.log.Println()
	return nil
}
