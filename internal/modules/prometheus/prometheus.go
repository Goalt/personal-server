package prometheus

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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type PrometheusModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *PrometheusModule {
	return &PrometheusModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *PrometheusModule) Name() string {
	return "prometheus"
}

func (m *PrometheusModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "prometheus")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Prometheus Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	serviceAccount, clusterRole, clusterRoleBinding, configMap, pvc, service, deployment, err := m.prepare()
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

	// Write ConfigMap
	if err := writeYAML(configMap, "configmap"); err != nil {
		return err
	}

	// Write PVC
	if err := writeYAML(pvc, "pvc"); err != nil {
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

	m.log.Info("\nCompleted: 7/7 Prometheus configurations generated successfully\n")
	return nil
}

func (m *PrometheusModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Prometheus Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ServiceAccount 'prometheus' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ServiceAccount existence: %w", err)
	}

	_, err = clientset.RbacV1().ClusterRoles().Get(ctx, "prometheus", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ClusterRole 'prometheus' already exists")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ClusterRole existence: %w", err)
	}

	_, err = clientset.RbacV1().ClusterRoleBindings().Get(ctx, "prometheus", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ClusterRoleBinding 'prometheus' already exists")
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ClusterRoleBinding existence: %w", err)
	}

	_, err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, "prometheus-config", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ConfigMap 'prometheus-config' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ConfigMap existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "prometheus-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'prometheus-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Service 'prometheus' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Deployment 'prometheus' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	serviceAccount, clusterRole, clusterRoleBinding, configMap, pvc, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare Kubernetes objects: %w", err)
	}

	// Apply ServiceAccount
	m.log.Progress("Applying ServiceAccount: prometheus\n")
	createdSA, err := clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ServiceAccount: %w", err)
	}
	m.log.Success("Created ServiceAccount: %s\n", createdSA.Name)

	// Apply ClusterRole
	m.log.Progress("Applying ClusterRole: prometheus\n")
	createdCR, err := clientset.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ClusterRole: %w", err)
	}
	m.log.Success("Created ClusterRole: %s\n", createdCR.Name)

	// Apply ClusterRoleBinding
	m.log.Progress("Applying ClusterRoleBinding: prometheus\n")
	createdCRB, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
	}
	m.log.Success("Created ClusterRoleBinding: %s\n", createdCRB.Name)

	// Apply ConfigMap
	m.log.Progress("Applying ConfigMap: prometheus-config\n")
	createdCM, err := clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}
	m.log.Success("Created ConfigMap: %s\n", createdCM.Name)

	// Apply PVC
	m.log.Progress("Applying PersistentVolumeClaim: prometheus-data-pvc\n")
	createdPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdPVC.Name)

	// Apply Service
	m.log.Progress("Applying Service: prometheus\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: prometheus\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 7/7 Prometheus configurations applied successfully\n")
	return nil
}

func (m *PrometheusModule) prepare() (*corev1.ServiceAccount, *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding, *corev1.ConfigMap, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment, error) {
	// Prepare ServiceAccount
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
	}

	// Prepare ClusterRole
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus",
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/proxy", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}

	// Prepare ClusterRoleBinding
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus",
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "prometheus",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "prometheus",
				Namespace: m.ModuleConfig.Namespace,
			},
		},
	}

	// Prepare ConfigMap with Prometheus configuration
	prometheusConfig := `global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'kubernetes-apiservers'
    kubernetes_sd_configs:
      - role: endpoints
    scheme: https
    tls_config:
      ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    relabel_configs:
      - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]
        action: keep
        regex: default;kubernetes;https

  - job_name: 'kubernetes-nodes'
    kubernetes_sd_configs:
      - role: node
    scheme: https
    tls_config:
      ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    relabel_configs:
      - action: labelmap
        regex: __meta_kubernetes_node_label_(.+)
      - target_label: __address__
        replacement: kubernetes.default.svc:443
      - source_labels: [__meta_kubernetes_node_name]
        regex: (.+)
        target_label: __metrics_path__
        replacement: /api/v1/nodes/${1}/proxy/metrics

  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
      - action: labelmap
        regex: __meta_kubernetes_pod_label_(.+)
      - source_labels: [__meta_kubernetes_namespace]
        action: replace
        target_label: kubernetes_namespace
      - source_labels: [__meta_kubernetes_pod_name]
        action: replace
        target_label: kubernetes_pod_name

  - job_name: 'kubernetes-service-endpoints'
    kubernetes_sd_configs:
      - role: endpoints
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
        action: replace
        target_label: __scheme__
        regex: (https?)
      - source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
        action: replace
        target_label: __address__
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
      - action: labelmap
        regex: __meta_kubernetes_service_label_(.+)
      - source_labels: [__meta_kubernetes_namespace]
        action: replace
        target_label: kubernetes_namespace
      - source_labels: [__meta_kubernetes_service_name]
        action: replace
        target_label: kubernetes_name
`

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus-config",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
		Data: map[string]string{
			"prometheus.yml": prometheusConfig,
		},
	}

	// Prepare PVC
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       9090,
					TargetPort: intstr.FromInt(9090),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "prometheus",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prometheus",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "prometheus",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "prometheus",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "prometheus",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "prometheus",
					Containers: []corev1.Container{
						{
							Name:            "prometheus",
							Image:           "prom/prometheus:v2.48.0",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--config.file=/etc/prometheus/prometheus.yml",
								"--storage.tsdb.path=/prometheus",
								"--web.console.libraries=/usr/share/prometheus/console_libraries",
								"--web.console.templates=/usr/share/prometheus/consoles",
								"--web.enable-lifecycle",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 9090,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "prometheus-config",
									MountPath: "/etc/prometheus",
								},
								{
									Name:      "prometheus-data",
									MountPath: "/prometheus",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/healthy",
										Port: intstr.FromInt(9090),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/ready",
										Port: intstr.FromInt(9090),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "prometheus-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "prometheus-config",
									},
								},
							},
						},
						{
							Name: "prometheus-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "prometheus-data-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	return serviceAccount, clusterRole, clusterRoleBinding, configMap, pvc, service, deployment, nil
}

func (m *PrometheusModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Prometheus Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalSteps := 7

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// 1. Delete Deployment
	m.log.Info("🗑️  Deleting Deployment...\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "prometheus", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: prometheus\n")
		successCount++
	}

	// 2. Delete Service
	m.log.Info("🗑️  Deleting Service...\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "prometheus", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: prometheus\n")
		successCount++
	}

	// 3. Delete PVC
	m.log.Info("🗑️  Deleting PersistentVolumeClaim...\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "prometheus-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: prometheus-data-pvc\n")
		successCount++
	}

	// 4. Delete ConfigMap
	m.log.Info("🗑️  Deleting ConfigMap...\n")
	err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Delete(ctx, "prometheus-config", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ConfigMap not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ConfigMap: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ConfigMap: prometheus-config\n")
		successCount++
	}

	// 5. Delete ClusterRoleBinding
	m.log.Info("🗑️  Deleting ClusterRoleBinding...\n")
	err = clientset.RbacV1().ClusterRoleBindings().Delete(ctx, "prometheus", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ClusterRoleBinding not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ClusterRoleBinding: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ClusterRoleBinding: prometheus\n")
		successCount++
	}

	// 6. Delete ClusterRole
	m.log.Info("🗑️  Deleting ClusterRole...\n")
	err = clientset.RbacV1().ClusterRoles().Delete(ctx, "prometheus", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ClusterRole not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ClusterRole: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ClusterRole: prometheus\n")
		successCount++
	}

	// 7. Delete ServiceAccount
	m.log.Info("🗑️  Deleting ServiceAccount...\n")
	err = clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Delete(ctx, "prometheus", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ServiceAccount not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ServiceAccount: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ServiceAccount: prometheus\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d Prometheus resources deleted successfully\n", successCount, totalSteps)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *PrometheusModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Prometheus resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check ServiceAccount
	m.log.Println("SERVICE ACCOUNT:")
	sa, err := clientset.CoreV1().ServiceAccounts(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ServiceAccount 'prometheus' not found\n")
		} else {
			m.log.Error("  Error getting ServiceAccount: %v\n", err)
		}
	} else {
		age := time.Since(sa.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ServiceAccount: prometheus (Age: %s)\n", k8s.FormatAge(age))
	}

	// Check ClusterRole
	m.log.Println("\nCLUSTER ROLE:")
	cr, err := clientset.RbacV1().ClusterRoles().Get(ctx, "prometheus", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ClusterRole 'prometheus' not found\n")
		} else {
			m.log.Error("  Error getting ClusterRole: %v\n", err)
		}
	} else {
		age := time.Since(cr.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ClusterRole: prometheus (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Rules: %d\n", len(cr.Rules))
	}

	// Check ClusterRoleBinding
	m.log.Println("\nCLUSTER ROLE BINDING:")
	crb, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "prometheus", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ClusterRoleBinding 'prometheus' not found\n")
		} else {
			m.log.Error("  Error getting ClusterRoleBinding: %v\n", err)
		}
	} else {
		age := time.Since(crb.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ClusterRoleBinding: prometheus (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Role: %s\n", crb.RoleRef.Name)
		m.log.Info("     Subjects: %d\n", len(crb.Subjects))
	}

	// Check ConfigMap
	m.log.Println("\nCONFIG MAP:")
	cm, err := clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, "prometheus-config", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ConfigMap 'prometheus-config' not found\n")
		} else {
			m.log.Error("  Error getting ConfigMap: %v\n", err)
		}
	} else {
		age := time.Since(cm.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  ConfigMap: prometheus-config (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Data keys: %d\n", len(cm.Data))
	}

	// Check PVC
	m.log.Println("\nPERSISTENT VOLUME CLAIM:")
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "prometheus-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  PersistentVolumeClaim 'prometheus-data-pvc' not found\n")
		} else {
			m.log.Error("  Error getting PersistentVolumeClaim: %v\n", err)
		}
	} else {
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  PersistentVolumeClaim: prometheus-data-pvc (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Status: %s\n", pvc.Status.Phase)
		if pvc.Status.Capacity.Storage() != nil {
			m.log.Info("     Capacity: %s\n", pvc.Status.Capacity.Storage().String())
		}
	}

	// Check Service
	m.log.Println("\nSERVICE:")
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Service 'prometheus' not found\n")
		} else {
			m.log.Error("  Error getting Service: %v\n", err)
		}
	} else {
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  Service: prometheus (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Type: %s\n", service.Spec.Type)
		m.log.Info("     Cluster-IP: %s\n", service.Spec.ClusterIP)
		m.log.Print("     Ports: ")
		for i, port := range service.Spec.Ports {
			if i > 0 {
				m.log.Print(", ")
			}
			m.log.Print("%d/%s", port.Port, port.Protocol)
		}
		m.log.Println()
	}

	// Check Deployment
	m.log.Println("\nDEPLOYMENT:")
	dep, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment 'prometheus' not found\n")
		} else {
			m.log.Error("  Error getting Deployment: %v\n", err)
		}
	} else {
		age := time.Since(dep.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("  Deployment: prometheus (Age: %s)\n", k8s.FormatAge(age))
		m.log.Info("     Replicas: %d/%d ready\n", dep.Status.ReadyReplicas, dep.Status.Replicas)
		m.log.Info("     Updated: %d\n", dep.Status.UpdatedReplicas)
		m.log.Info("     Available: %d\n", dep.Status.AvailableReplicas)
	}

	// Check Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=prometheus",
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Warn("  No pods found with label 'app=prometheus'\n")
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

func (m *PrometheusModule) Rollout(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("rollout command requires an action: restart, status, history, or undo")
	}

	action := args[0]
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	switch action {
	case "restart":
		m.log.Info("Restarting Prometheus deployment...\n")
		deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}

		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}
		deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

		_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to restart deployment: %w", err)
		}
		m.log.Success("Deployment restart initiated\n")

	case "status":
		m.log.Info("Checking rollout status...\n")
		deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "prometheus", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
		m.log.Info("Replicas: %d desired | %d updated | %d available | %d ready\n",
			*deployment.Spec.Replicas,
			deployment.Status.UpdatedReplicas,
			deployment.Status.AvailableReplicas,
			deployment.Status.ReadyReplicas)

	case "history":
		m.log.Info("Rollout history:\n")
		m.log.Warn("History viewing is limited in this implementation\n")

	case "undo":
		m.log.Info("Rolling back deployment...\n")
		m.log.Warn("Rollback is limited in this implementation\n")

	default:
		return fmt.Errorf("unknown rollout action: %s (valid actions: restart, status, history, undo)", action)
	}

	return nil
}
