package dashboard

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
	"k8s.io/apimachinery/pkg/util/intstr"
)

type DashboardModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *DashboardModule {
	return &DashboardModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *DashboardModule) Name() string {
	return "dashboard"
}

func (m *DashboardModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "dashboard")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Kubernetes Dashboard configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	serviceAccount, clusterRoleBinding, service, deployment, err := m.prepare()
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

	// Write ClusterRoleBinding
	if err := writeYAML(clusterRoleBinding, "clusterrolebinding"); err != nil {
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

	m.log.Info("\nCompleted: 4/4 Kubernetes Dashboard configurations generated successfully\n")
	return nil
}

func (m *DashboardModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Kubernetes Dashboard configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ServiceAccount 'kubernetes-dashboard' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ServiceAccount existence: %w", err)
	}

	_, err = clientset.RbacV1().ClusterRoleBindings().Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ClusterRoleBinding 'kubernetes-dashboard' already exists")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ClusterRoleBinding existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Service 'kubernetes-dashboard' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Deployment 'kubernetes-dashboard' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	serviceAccount, clusterRoleBinding, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
	}

	// Apply ServiceAccount
	m.log.Progress("Applying ServiceAccount: kubernetes-dashboard\n")
	createdSA, err := clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ServiceAccount: %w", err)
	}
	m.log.Success("Created ServiceAccount: %s\n", createdSA.Name)

	// Apply ClusterRoleBinding
	m.log.Progress("Applying ClusterRoleBinding: kubernetes-dashboard\n")
	createdCRB, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
	}
	m.log.Success("Created ClusterRoleBinding: %s\n", createdCRB.Name)

	// Apply Service
	m.log.Progress("Applying Service: kubernetes-dashboard\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: kubernetes-dashboard\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 4/4 Kubernetes Dashboard configurations applied successfully\n")
	return nil
}

func (m *DashboardModule) prepare() (*corev1.ServiceAccount, *rbacv1.ClusterRoleBinding, *corev1.Service, *appsv1.Deployment, error) {
	// Prepare ServiceAccount
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-dashboard",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"k8s-app":    "kubernetes-dashboard",
			},
		},
	}

	// Prepare ClusterRoleBinding
	// Note: Uses cluster-admin role for full cluster access in personal server environment.
	// For production use, consider using a more restricted role like 'view' or a custom role.
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubernetes-dashboard",
			Labels: map[string]string{
				"managed-by": "personal-server",
				"k8s-app":    "kubernetes-dashboard",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "kubernetes-dashboard",
				Namespace: m.ModuleConfig.Namespace,
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-dashboard",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"k8s-app":    "kubernetes-dashboard",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "https",
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"k8s-app": "kubernetes-dashboard",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-dashboard",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"k8s-app":    "kubernetes-dashboard",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "kubernetes-dashboard",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kubernetes-dashboard",
					Containers: []corev1.Container{
						{
							Name:  "kubernetes-dashboard",
							Image: "kubernetesui/dashboard:v2.7.0",
							Args: []string{
								"--auto-generate-certificates",
								"--namespace=" + m.ModuleConfig.Namespace,
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/",
										Port:   intstr.FromInt(8443),
										Scheme: corev1.URISchemeHTTPS,
									},
								},
								InitialDelaySeconds: 30,
								TimeoutSeconds:      30,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: func() *bool { b := false; return &b }(),
								ReadOnlyRootFilesystem:   func() *bool { b := true; return &b }(),
								RunAsUser:                func() *int64 { i := int64(1001); return &i }(),
								RunAsGroup:               func() *int64 { i := int64(2001); return &i }(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kubernetes-dashboard-certs",
									MountPath: "/certs",
								},
								{
									Name:      "tmp-volume",
									MountPath: "/tmp",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kubernetes-dashboard-certs",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "tmp-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return serviceAccount, clusterRoleBinding, service, deployment, nil
}

func (m *DashboardModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Kubernetes Dashboard resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalSteps := 4

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// 1. Delete Deployment
	m.log.Info("🗑️  Deleting Deployment...\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "kubernetes-dashboard", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: kubernetes-dashboard\n")
		successCount++
	}

	// 2. Delete Service
	m.log.Info("🗑️  Deleting Service...\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "kubernetes-dashboard", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: kubernetes-dashboard\n")
		successCount++
	}

	// 3. Delete ClusterRoleBinding
	m.log.Info("🗑️  Deleting ClusterRoleBinding...\n")
	err = clientset.RbacV1().ClusterRoleBindings().Delete(ctx, "kubernetes-dashboard", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ClusterRoleBinding not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ClusterRoleBinding: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ClusterRoleBinding: kubernetes-dashboard\n")
		successCount++
	}

	// 4. Delete ServiceAccount
	m.log.Info("🗑️  Deleting ServiceAccount...\n")
	err = clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Delete(ctx, "kubernetes-dashboard", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ServiceAccount not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ServiceAccount: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ServiceAccount: kubernetes-dashboard\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d Kubernetes Dashboard resources deleted successfully\n", successCount, totalSteps)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *DashboardModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Kubernetes Dashboard resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check ServiceAccount
	m.log.Println("SERVICE ACCOUNT:")
	sa, err := clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ServiceAccount 'kubernetes-dashboard' not found\n")
		} else {
			m.log.Error("  Error getting ServiceAccount: %v\n", err)
		}
	} else {
		age := time.Since(sa.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ServiceAccount: kubernetes-dashboard (Age: %s)\n", k8s.FormatAge(age))
	}

	// Check ClusterRoleBinding
	m.log.Println("\nCLUSTER ROLE BINDING:")
	crb, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ClusterRoleBinding 'kubernetes-dashboard' not found\n")
		} else {
			m.log.Error("  Error getting ClusterRoleBinding: %v\n", err)
		}
	} else {
		age := time.Since(crb.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ClusterRoleBinding: kubernetes-dashboard (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Role: %s\n", crb.RoleRef.Name)
		m.log.Info("     Subjects: %d\n", len(crb.Subjects))
	}

	// Check Service
	m.log.Println("\nSERVICE:")
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Service 'kubernetes-dashboard' not found\n")
		} else {
			m.log.Error("  Error getting Service: %v\n", err)
		}
	} else {
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  Service: kubernetes-dashboard (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Type: %s\n", service.Spec.Type)
		m.log.Info("     Ports:\n")
		for _, port := range service.Spec.Ports {
			m.log.Info("       - %s: %d (%s)\n", port.Name, port.Port, port.Protocol)
		}
	}

	// Check Deployment
	m.log.Println("\nDEPLOYMENT:")
	dep, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "kubernetes-dashboard", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment 'kubernetes-dashboard' not found\n")
		} else {
			m.log.Error("  Error getting Deployment: %v\n", err)
		}
	} else {
		age := time.Since(dep.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  Deployment: kubernetes-dashboard (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Replicas: %d/%d ready\n", dep.Status.ReadyReplicas, dep.Status.Replicas)
		m.log.Info("     Updated: %d\n", dep.Status.UpdatedReplicas)
		m.log.Info("     Available: %d\n", dep.Status.AvailableReplicas)
	}

	// Check Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kubernetes-dashboard",
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Warn("  No pods found with label 'k8s-app=kubernetes-dashboard'\n")
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
