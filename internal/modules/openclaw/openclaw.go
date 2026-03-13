package openclaw

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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type OpenClawModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *OpenClawModule {
	return &OpenClawModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *OpenClawModule) Name() string {
	return "openclaw"
}

func (m *OpenClawModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "openclaw")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating OpenClaw Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	configPVC, dataPVC, service, deployment := m.prepare()

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

	// Write Config PVC
	if err := writeYAML(configPVC, "config-pvc"); err != nil {
		return err
	}

	// Write Data PVC
	if err := writeYAML(dataPVC, "data-pvc"); err != nil {
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

	m.log.Info("\nCompleted: 4/4 OpenClaw configurations generated successfully\n")
	return nil
}

func (m *OpenClawModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying OpenClaw Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-config-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'openclaw-config-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'openclaw-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'openclaw' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'openclaw' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	configPVC, dataPVC, service, deployment := m.prepare()

	// Apply Config PVC
	m.log.Progress("Applying PersistentVolumeClaim: openclaw-config-pvc\n")
	createdConfigPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, configPVC, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdConfigPVC.Name)

	// Apply Data PVC
	m.log.Progress("Applying PersistentVolumeClaim: openclaw-data-pvc\n")
	createdDataPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, dataPVC, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdDataPVC.Name)

	// Apply Service
	m.log.Progress("\nApplying Service: openclaw\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("\nApplying Deployment: openclaw\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: OpenClaw configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for openclaw module
func (m *OpenClawModule) prepare() (*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment) {
	// Prepare Config PersistentVolumeClaim
	configStorageQuantity := resource.MustParse("100Mi")
	configPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw-config-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: configStorageQuantity,
				},
			},
		},
	}

	// Prepare Data PersistentVolumeClaim
	dataStorageQuantity := resource.MustParse("1Gi")
	dataPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: dataStorageQuantity,
				},
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       5000,
					TargetPort: intstr.FromInt(5000),
				},
			},
			Selector: map[string]string{
				"app": "openclaw",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openclaw",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"managed-by": "personal-server",
				"app":        "openclaw",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &replicas,
			RevisionHistoryLimit: func() *int32 { i := int32(1); return &i }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "openclaw",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "openclaw",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:  "openclaw",
							Image: "openclaw/openclaw:latest",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 5000,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "openclaw-config",
									MountPath: "/config",
								},
								{
									Name:      "openclaw-data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "openclaw-config",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "openclaw-config-pvc",
								},
							},
						},
						{
							Name: "openclaw-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "openclaw-data-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	return configPVC, dataPVC, service, deployment
}

func (m *OpenClawModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning OpenClaw Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: openclaw\n")
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "openclaw", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'openclaw' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: openclaw\n")
		successCount++
	}

	// Delete Service
	m.log.Info("\n🗑️  Deleting Service: openclaw\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "openclaw", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'openclaw' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: openclaw\n")
		successCount++
	}

	// Delete Config PVC
	m.log.Info("\n🗑️  Deleting PersistentVolumeClaim: openclaw-config-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "openclaw-config-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'openclaw-config-pvc' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: openclaw-config-pvc\n")
		successCount++
	}

	// Delete Data PVC
	m.log.Info("\n🗑️  Deleting PersistentVolumeClaim: openclaw-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "openclaw-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'openclaw-data-pvc' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: openclaw-data-pvc\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/4 OpenClaw resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
		m.log.Warn("WARNING: Deleting the PVCs will remove all OpenClaw data permanently!\n")
	}
	return nil
}

func (m *OpenClawModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking OpenClaw resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	resourceFound := false

	// Check Config PVC
	configPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-config-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'openclaw-config-pvc' not found\n")
		} else {
			m.log.Error("Error checking PersistentVolumeClaim: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(configPVC.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("PersistentVolumeClaim 'openclaw-config-pvc'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Status: %s\n", configPVC.Status.Phase)
		m.log.Info("   Storage: %s\n", configPVC.Spec.Resources.Requests.Storage().String())
		if configPVC.Spec.VolumeName != "" {
			m.log.Info("   Volume: %s\n", configPVC.Spec.VolumeName)
		}
	}

	m.log.Println()

	// Check Data PVC
	dataPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openclaw-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'openclaw-data-pvc' not found\n")
		} else {
			m.log.Error("Error checking PersistentVolumeClaim: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(dataPVC.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("PersistentVolumeClaim 'openclaw-data-pvc'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Status: %s\n", dataPVC.Status.Phase)
		m.log.Info("   Storage: %s\n", dataPVC.Spec.Resources.Requests.Storage().String())
		if dataPVC.Spec.VolumeName != "" {
			m.log.Info("   Volume: %s\n", dataPVC.Spec.VolumeName)
		}
	}

	m.log.Println()

	// Check Service
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'openclaw' not found\n")
		} else {
			m.log.Error("Error checking service: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Service 'openclaw'\n")
		m.log.Info("   Age: %s\n", k8s.FormatAge(age))
		m.log.Info("   Type: %s\n", service.Spec.Type)
		m.log.Info("   Ports:\n")
		for _, port := range service.Spec.Ports {
			m.log.Info("     - %s: %d -> %s\n", port.Name, port.Port, port.TargetPort.String())
		}
	}

	m.log.Println()

	// Check Deployment
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "openclaw", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'openclaw' not found\n")
		} else {
			m.log.Error("Error checking deployment: %v\n", err)
		}
	} else {
		resourceFound = true
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Success("Deployment 'openclaw'\n")
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

	// Get Pods for the deployment
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=openclaw",
	})
	if err != nil {
		m.log.Error("Error listing pods: %v\n", err)
	} else if len(pods.Items) > 0 {
		resourceFound = true
		m.log.Info("PODS:\n")
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

	if !resourceFound {
		m.log.Println("\nNo OpenClaw resources found. Run 'openclaw apply' to create them.")
	}
	return nil
}
