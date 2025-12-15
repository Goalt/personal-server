package drone

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

type DroneModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *DroneModule {
	return &DroneModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *DroneModule) Name() string {
	return "drone"
}

func (m *DroneModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "drone")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating Drone Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	secret, role, roleBinding, deployment, runnerDeployment, service := m.prepare()

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

	// Write Role
	if err := writeYAML(role, "role"); err != nil {
		return err
	}

	// Write RoleBinding
	if err := writeYAML(roleBinding, "rolebinding"); err != nil {
		return err
	}

	// Write Deployment
	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	// Write Runner Deployment
	if err := writeYAML(runnerDeployment, "runner-deployment"); err != nil {
		return err
	}

	// Write Service
	if err := writeYAML(service, "service"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 6/6 Drone configurations generated successfully\n")
	return nil
}

func (m *DroneModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying Drone Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "drone-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'drone-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.RbacV1().Roles(m.ModuleConfig.Namespace).Get(ctx, "drone", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("role 'drone' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check role existence: %w", err)
	}

	_, err = clientset.RbacV1().RoleBindings(m.ModuleConfig.Namespace).Get(ctx, "drone", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("roleBinding 'drone' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check roleBinding existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "drone", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'drone' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "drone-runner", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'drone-runner' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "drone", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'drone' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	secret, role, roleBinding, deployment, runnerDeployment, service := m.prepare()

	// Apply Secret
	m.log.Progress("Applying Secret: drone-secrets\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	m.log.Success("Created Secret: drone-secrets\n")

	// Apply Role
	m.log.Progress("Applying Role: drone\n")
	_, err = clientset.RbacV1().Roles(m.ModuleConfig.Namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}
	m.log.Success("Created Role: drone\n")

	// Apply RoleBinding
	m.log.Progress("Applying RoleBinding: drone\n")
	_, err = clientset.RbacV1().RoleBindings(m.ModuleConfig.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create roleBinding: %w", err)
	}
	m.log.Success("Created RoleBinding: drone\n")

	// Apply Drone Server Deployment
	m.log.Progress("Applying Deployment: drone\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: drone\n")

	// Apply Drone Runner Deployment
	m.log.Progress("Applying Deployment: drone-runner\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, runnerDeployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: drone-runner\n")

	// Apply Service
	m.log.Progress("Applying Service: drone\n")
	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: drone\n")

	m.log.Info("\nCompleted: Drone configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for drone module
func (m *DroneModule) prepare() (*corev1.Secret, *rbacv1.Role, *rbacv1.RoleBinding, *appsv1.Deployment, *appsv1.Deployment, *corev1.Service) {
	// Prepare Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drone-secrets",
			Namespace: m.ModuleConfig.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"drone_gitea_server":        "https://gitea." + m.GeneralConfig.Domain,
			"drone_gitea_client_id":     m.ModuleConfig.Secrets["drone_gitea_client_id"],
			"drone_gitea_client_secret": m.ModuleConfig.Secrets["drone_gitea_client_secret"],
			"drone_rpc_secret":          m.ModuleConfig.Secrets["drone_rpc_secret"],
			"drone_server_host":         "drone." + m.GeneralConfig.Domain,
			"drone_server_proto":        m.ModuleConfig.Secrets["drone_server_proto"],
		},
	}

	// Prepare Role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drone",
			Namespace: m.ModuleConfig.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "pods/log"},
				Verbs:     []string{"get", "create", "delete", "list", "watch", "update"},
			},
		},
	}

	// Prepare RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drone",
			Namespace: m.ModuleConfig.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: m.ModuleConfig.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "drone",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Prepare Drone Server Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drone",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "drone",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "drone",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "drone",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "drone",
							Image:           "drone/drone:2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "DRONE_SERVER_HOST",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_server_host",
										},
									},
								},
								{
									Name: "DRONE_SERVER_PROTO",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_server_proto",
										},
									},
								},
								{
									Name: "DRONE_GITEA_SERVER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_gitea_server",
										},
									},
								},
								{
									Name: "DRONE_GITEA_CLIENT_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_gitea_client_id",
										},
									},
								},
								{
									Name: "DRONE_GITEA_CLIENT_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_gitea_client_secret",
										},
									},
								},
								{
									Name: "DRONE_RPC_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_rpc_secret",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(80),
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(80),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
							},
						},
					},
				},
			},
		},
	}

	// Prepare Drone Runner Deployment
	runnerDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drone-runner",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "drone-runner",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "drone-runner",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "drone-runner",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "runner",
							Image: "drone/drone-runner-kube:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 3000,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DRONE_RPC_HOST",
									Value: m.ModuleConfig.Secrets["drone_server_host"],
								},
								{
									Name:  "DRONE_RPC_PROTO",
									Value: "http",
								},
								{
									Name: "DRONE_RPC_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "drone-secrets",
											},
											Key: "drone_rpc_secret",
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

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "drone",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app": "drone",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "drone",
			},
		},
	}

	return secret, role, roleBinding, deployment, runnerDeployment, service
}

func (m *DroneModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning Drone Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: drone\n")
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "drone", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'drone' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: drone\n")
		successCount++
	}

	// Delete Runner Deployment
	m.log.Info("🗑️  Deleting Deployment: drone-runner\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "drone-runner", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'drone-runner' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: drone-runner\n")
		successCount++
	}

	// Delete Service
	m.log.Info("\n🗑️  Deleting Service: drone\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "drone", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'drone' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: drone\n")
		successCount++
	}

	// Delete Secret
	m.log.Info("\n🗑️  Deleting Secret: drone-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "drone-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret 'drone-secrets' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: drone-secrets\n")
		successCount++
	}

	// Delete Role
	m.log.Info("\n🗑️  Deleting Role: drone\n")
	err = clientset.RbacV1().Roles(m.ModuleConfig.Namespace).Delete(ctx, "drone", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Role 'drone' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete role: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Role: drone\n")
		successCount++
	}

	// Delete RoleBinding
	m.log.Info("\n🗑️  Deleting RoleBinding: drone\n")
	err = clientset.RbacV1().RoleBindings(m.ModuleConfig.Namespace).Delete(ctx, "drone", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("RoleBinding 'drone' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete rolebinding: %v\n", err)
		}
	} else {
		m.log.Success("Deleted RoleBinding: drone\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/6 drone resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *DroneModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking Drone resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	resourceFound := false

	// Check Secret
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "drone-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Secret 'drone-secrets' not found\n")
		} else {
			m.log.Error("Error checking secret: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Secret 'drone-secrets'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Type: %s\n", secret.Type)
	}

	m.log.Println()

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "drone", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'drone' not found\n")
		} else {
			m.log.Error("Error checking service: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Service 'drone'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Type: %s\n", service.Spec.Type)
		m.log.Info("   Ports:\n")
		for _, port := range service.Spec.Ports {
			m.log.Info("     - %s: %d -> %s\n", port.Name, port.Port, port.TargetPort.String())
		}
	}

	m.log.Println()

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "drone", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'drone' not found\n")
		} else {
			m.log.Error("Error checking deployment: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'drone'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Replicas: %d desired / %d ready / %d available / %d unavailable\n",
			deployment.Status.Replicas,
			deployment.Status.ReadyReplicas,
			deployment.Status.AvailableReplicas,
			deployment.Status.UnavailableReplicas)
		m.log.Info("   Updated Replicas: %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("   Image: %s\n", deployment.Spec.Template.Spec.Containers[0].Image)
	}

	m.log.Println()

	// Check Runner Deployment
	runnerDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "drone-runner", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'drone-runner' not found\n")
		} else {
			m.log.Error("Error checking deployment: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(runnerDeployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'drone-runner'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Replicas: %d desired / %d ready / %d available / %d unavailable\n",
			runnerDeployment.Status.Replicas,
			runnerDeployment.Status.ReadyReplicas,
			runnerDeployment.Status.AvailableReplicas,
			runnerDeployment.Status.UnavailableReplicas)
		m.log.Info("   Updated Replicas: %d\n", runnerDeployment.Status.UpdatedReplicas)
		m.log.Info("   Image: %s\n", runnerDeployment.Spec.Template.Spec.Containers[0].Image)
	}

	m.log.Println()

	// Get Pods for the deployment
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=drone",
	})
	if err != nil {
		m.log.Error("Error listing pods: %v\n", err)
	} else if len(pods.Items) > 0 {
		resourceFound = true
		m.log.Info("PODS (drone):\n")
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

	// Get Pods for the runner deployment
	runnerPods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=drone-runner",
	})
	if err != nil {
		m.log.Error("Error listing runner pods: %v\n", err)
	} else if len(runnerPods.Items) > 0 {
		resourceFound = true
		m.log.Info("\nPODS (drone-runner):\n")
		m.log.Info("%-40s %-10s %-10s %-10s\n", "NAME", "READY", "STATUS", "AGE")
		for _, pod := range runnerPods.Items {
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

	if !resourceFound {
		m.log.Println("\nNo Drone resources found. Run 'drone apply' to create them.")
	}
	return nil
}
