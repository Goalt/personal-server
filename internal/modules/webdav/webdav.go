package webdav

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

type WebdavModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *WebdavModule {
	return &WebdavModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *WebdavModule) Name() string {
	return "webdav"
}

func (m *WebdavModule) Generate(ctx context.Context) error {
	// Define output directory
	outputDir := filepath.Join("configs", "webdav")

	// Check and create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating WebDAV Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	// Prepare Kubernetes objects
	configMap, secret, pvc, service, deployment := m.prepare()

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

	// Write ConfigMap
	if err := writeYAML(configMap, "configmap"); err != nil {
		return err
	}

	// Write Secret
	if err := writeYAML(secret, "secret"); err != nil {
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

	m.log.Info("\nCompleted: 5/5 WebDAV configurations generated successfully\n")
	return nil
}

func (m *WebdavModule) Apply(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying WebDAV Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	// Check if resources already exist
	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, "webdav-config", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("ConfigMap 'webdav-config' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check ConfigMap existence: %w", err)
	}

	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "webdav-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Secret 'webdav-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Secret existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "webdav-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'webdav-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "webdav-service", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Service 'webdav-service' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "webdav", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Deployment 'webdav' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check Deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	// Prepare Kubernetes objects
	configMap, secret, pvc, service, deployment := m.prepare()

	// Apply ConfigMap
	m.log.Progress("Applying ConfigMap: webdav-config\n")
	createdCM, err := clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}
	m.log.Success("Created ConfigMap: %s\n", createdCM.Name)

	// Apply Secret
	m.log.Progress("Applying Secret: webdav-secrets\n")
	createdSecret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Secret: %w", err)
	}
	m.log.Success("Created Secret: %s\n", createdSecret.Name)

	// Apply PVC
	m.log.Progress("Applying PersistentVolumeClaim: webdav-data-pvc\n")
	createdPVC, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: %s\n", createdPVC.Name)

	// Apply Service
	m.log.Progress("Applying Service: webdav-service\n")
	createdService, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}
	m.log.Success("Created Service: %s\n", createdService.Name)

	// Apply Deployment
	m.log.Progress("Applying Deployment: webdav\n")
	createdDeployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Deployment: %w", err)
	}
	m.log.Success("Created Deployment: %s\n", createdDeployment.Name)

	m.log.Info("\nCompleted: 5/5 resources applied successfully\n")
	return nil
}

func (m *WebdavModule) prepare() (*corev1.ConfigMap, *corev1.Secret, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment) {
	// Prepare ConfigMap
	configMapData := `# WebDAV Server Configuration
address: 0.0.0.0
port: 8080

# Prefix to apply to the WebDAV path-ing. Default is '/'.
prefix: /

# Enable or disable debug logging. Default is 'false'.
debug: false

# Disable sniffing the files to detect their content type. Default is 'false'.
noSniff: false

# Whether the server runs behind a trusted proxy or not. When this is true,
# the header X-Forwarded-For will be used for logging the remote addresses
# of logging attempts (if available).
behindProxy: true

# The directory that will be able to be accessed by the users when connecting.
directory: /data

# The default permissions for users. This is a case insensitive option. Possible
# permissions: C (Create), R (Read), U (Update), D (Delete).
permissions: CRUD

# Logging configuration
log:
  format: json
  colors: false
  outputs:
  - stderr

# CORS configuration
cors:
  enabled: true
  credentials: true
  allowed_headers:
    - Depth
    - Content-Type
    - Authorization
  allowed_methods:
    - GET
    - HEAD
    - POST
    - PUT
    - DELETE
    - OPTIONS
    - PROPFIND
    - PROPPATCH
    - MKCOL
    - COPY
    - MOVE
    - LOCK
    - UNLOCK
  exposed_headers:
    - Content-Length
    - Content-Range
    - DAV

# User authentication
users:
  - username: "{env}WEBDAV_USERNAME"
    password: "{env}WEBDAV_PASSWORD"
    permissions: CRUD
`

	configMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webdav-config",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "webdav",
				"managed-by": "personal-server",
			},
		},
		Data: map[string]string{
			"config.yaml": configMapData,
		},
	}

	// Prepare Secret
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webdav-secrets",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "webdav",
				"managed-by": "personal-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"webdav_username": k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "webdav_username", "admin"),
			"webdav_password": k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "webdav_password", "abc"),
		},
	}

	// Prepare PVC
	storageQuantity := resource.MustParse("20Gi")
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webdav-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "webdav",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQuantity,
				},
			},
		},
	}

	// Prepare Service
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webdav-service",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "webdav",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "webdav",
			},
		},
	}

	// Prepare Deployment
	replicas := int32(1)
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)
	readOnlyRootFilesystem := true
	runAsNonRoot := true
	allowPrivilegeEscalation := false

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webdav",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "webdav",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "webdav",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "webdav",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: &fsGroup,
					},
					Containers: []corev1.Container{
						{
							Name:  "webdav",
							Image: "ghcr.io/hacdias/webdav:latest",
							Args: []string{
								"-c",
								"/config/config.yaml",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "WEBDAV_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "webdav-secrets",
											},
											Key: "webdav_username",
										},
									},
								},
								{
									Name: "WEBDAV_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "webdav-secrets",
											},
											Key: "webdav_password",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "webdav-config",
									MountPath: "/config",
									ReadOnly:  true,
								},
								{
									Name:      "webdav-data",
									MountPath: "/data",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             &runAsNonRoot,
								RunAsUser:                &runAsUser,
								RunAsGroup:               &runAsGroup,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
						{
							Name:  "backup-helper",
							Image: "busybox:latest",
							Command: []string{
								"sh",
								"-c",
								"while true; do sleep 3600; done",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "webdav-data",
									MountPath: "/data",
									ReadOnly:  false,
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             &runAsNonRoot,
								RunAsUser:                &runAsUser,
								RunAsGroup:               &runAsGroup,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								ReadOnlyRootFilesystem:   &readOnlyRootFilesystem,
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "webdav-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "webdav-config",
									},
								},
							},
						},
						{
							Name: "webdav-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "webdav-data-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	return configMap, secret, pvc, service, deployment
}

func (m *WebdavModule) Clean(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning WebDAV Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	totalResources := 5 // Deployment, Service, PVC, Secret, ConfigMap

	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	// Delete Deployment
	m.log.Info("🗑️  Deleting Deployment: webdav\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "webdav", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'webdav' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: webdav\n")
		successCount++
	}

	// Delete Service
	m.log.Info("🗑️  Deleting Service: webdav-service\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "webdav-service", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'webdav-service' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: webdav-service\n")
		successCount++
	}

	// Delete PVC
	m.log.Info("🗑️  Deleting PersistentVolumeClaim: webdav-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "webdav-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PVC 'webdav-data-pvc' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete PVC: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PVC: webdav-data-pvc\n")
		successCount++
	}

	// Delete Secret
	m.log.Info("🗑️  Deleting Secret: webdav-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "webdav-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret 'webdav-secrets' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: webdav-secrets\n")
		successCount++
	}

	// Delete ConfigMap
	m.log.Info("🗑️  Deleting ConfigMap: webdav-config\n")
	err = clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Delete(ctx, "webdav-config", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("ConfigMap 'webdav-config' not found (already deleted or never existed)\n")
		} else {
			m.log.Error("Failed to delete ConfigMap: %v\n", err)
		}
	} else {
		m.log.Success("Deleted ConfigMap: webdav-config\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d/%d resources deleted successfully\n", successCount, totalResources)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *WebdavModule) Status(ctx context.Context) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking WebDAV resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	// Check Deployment
	m.log.Println("DEPLOYMENT:")
	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "webdav", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Deployment 'webdav' not found\n")
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

	// Check Service
	m.log.Println("\nSERVICE:")
	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "webdav-service", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Service 'webdav-service' not found\n")
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

	// Check ConfigMap
	m.log.Println("\nCONFIGMAP:")
	cm, err := clientset.CoreV1().ConfigMaps(m.ModuleConfig.Namespace).Get(ctx, "webdav-config", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  ConfigMap 'webdav-config' not found\n")
		} else {
			m.log.Error("  Error getting ConfigMap: %v\n", err)
		}
	} else {
		age := time.Since(cm.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", cm.Name)
		m.log.Info("  Data keys: %d\n", len(cm.Data))
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check Secret
	m.log.Println("\nSECRET:")
	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "webdav-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  Secret 'webdav-secrets' not found\n")
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

	// Check PVC
	m.log.Println("\nPERSISTENT VOLUME CLAIM:")
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "webdav-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("  PVC 'webdav-data-pvc' not found\n")
		} else {
			m.log.Error("  Error getting PVC: %v\n", err)
		}
	} else {
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("  Name: %s\n", pvc.Name)
		m.log.Info("  Status: %s\n", pvc.Status.Phase)
		m.log.Info("  Volume: %s\n", pvc.Spec.VolumeName)
		m.log.Info("  Capacity: %s\n", pvc.Status.Capacity.Storage())
		m.log.Info("  Access Modes: %v\n", pvc.Spec.AccessModes)
		m.log.Info("  Age: %s\n", k8s.FormatAge(age))
	}

	// Check Pods
	m.log.Println("\nPODS:")
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=webdav",
	})
	if err != nil {
		m.log.Error("  Error listing pods: %v\n", err)
	} else if len(pods.Items) == 0 {
		m.log.Println("  No pods found")
	} else {
		m.log.Info("  %-40s %-10s %-10s %-10s\n", "NAME", "READY", "STATUS", "AGE")
		for _, pod := range pods.Items {
			ready := 0
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready++
				}
			}
			total := len(pod.Spec.Containers)
			age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
			m.log.Info("  %-40s %-10s %-10s %-10s\n",
				pod.Name,
				fmt.Sprintf("%d/%d", ready, total),
				pod.Status.Phase,
				k8s.FormatAge(age))
		}
	}

	m.log.Println()
	return nil
}

func (m *WebdavModule) Backup(ctx context.Context, destDir string) error {
	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	var backupDir string
	if destDir != "" {
		backupDir = filepath.Join(destDir, "webdav")
	} else {
		backupDir = filepath.Join("backups", fmt.Sprintf("webdav_backup_%s", timestamp))
	}

	m.log.Info("🔄 Starting WebDAV backup...\n")
	m.log.Info("Backup directory: %s\n", backupDir)

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// 1. Backup data volume
	m.log.Info("💾 Backing up data volume (/data)...\n")

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=webdav",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=webdav")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	dataBackupFile := filepath.Join(backupDir, fmt.Sprintf("webdav_data_%s.tar.gz", timestamp))

	kubectlCmd := "kubectl"
	kubectlArgs := []string{}
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s"
		kubectlArgs = append(kubectlArgs, "kubectl")
	}

	// Use the backup-helper sidecar container which has tar
	m.log.Info("📦 Creating archive using backup-helper container...\n")

	// kubectl exec -n <namespace> <pod> -c backup-helper -- tar czf - -C /data .
	execArgs := append(kubectlArgs, "exec", "-n", m.ModuleConfig.Namespace, podName,
		"-c", "backup-helper",
		"--",
		"tar", "czf", "-", "-C", "/data", ".")

	execCmd := exec.CommandContext(ctx, kubectlCmd, execArgs...)

	outFile, err := os.Create(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to create data backup file: %w", err)
	}
	defer outFile.Close()

	execCmd.Stdout = outFile
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	fileInfo, err := outFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat data backup file: %w", err)
	}
	m.log.Success("✅ Data archived (%d bytes)\n", fileInfo.Size())

	// 2. Metadata
	m.log.Info("📋 Writing metadata...\n")
	metadataFile := filepath.Join(backupDir, "backup_info.txt")
	metadata := fmt.Sprintf(`WebDAV Backup Information
===========================
Backup Date: %s
Backup Directory: %s
Namespace: %s
Deployment: webdav
Pod: %s

Data Archive:
%s

Restore Command:
personal-server webdav restore %s
`, time.Now().Format(time.RFC1123), backupDir, m.ModuleConfig.Namespace, podName, filepath.Base(dataBackupFile), timestamp)

	if err := os.WriteFile(metadataFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	m.log.Success("✅ Metadata written\n")

	m.log.Success("🎉 Backup complete!\n")
	m.log.Info("💡 To restore: personal-server webdav restore %s\n", timestamp)

	return nil
}

func (m *WebdavModule) Restore(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: personal-server webdav restore [TIMESTAMP|latest]")
	}

	timestamp := args[0]
	backupDir := "backups"

	// Resolve latest
	if timestamp == "latest" {
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return fmt.Errorf("failed to read backup directory: %w", err)
		}

		var latestTime time.Time
		var latestDir string

		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "webdav_backup_") {
				tsStr := strings.TrimPrefix(entry.Name(), "webdav_backup_")
				ts, err := time.Parse("20060102_150405", tsStr)
				if err == nil {
					if ts.After(latestTime) {
						latestTime = ts
						latestDir = entry.Name()
					}
				}
			}
		}

		if latestDir == "" {
			return fmt.Errorf("no backups found")
		}
		timestamp = strings.TrimPrefix(latestDir, "webdav_backup_")
		m.log.Info("Using latest backup: %s\n", timestamp)
	}

	targetBackupDir := filepath.Join(backupDir, fmt.Sprintf("webdav_backup_%s", timestamp))
	if _, err := os.Stat(targetBackupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup not found: %s", targetBackupDir)
	}

	dataBackupFile := filepath.Join(targetBackupDir, fmt.Sprintf("webdav_data_%s.tar.gz", timestamp))
	if _, err := os.Stat(dataBackupFile); os.IsNotExist(err) {
		return fmt.Errorf("data archive missing: %s", dataBackupFile)
	}

	m.log.Info("🔄 Starting WebDAV restore (timestamp: %s)...\n", timestamp)
	m.log.Info("💾 Data will be restored from %s\n", dataBackupFile)

	// Create Kubernetes client
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Find Pod
	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=webdav",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("no running pod found for app=webdav")
	}
	podName := pods.Items[0].Name
	m.log.Info("📦 Using pod: %s\n", podName)

	// Restore data
	m.log.Info("💾 Restoring data...\n")

	kubectlCmd := "kubectl"
	kubectlArgs := []string{}
	if _, err := os.Stat("/snap/bin/microk8s"); err == nil {
		kubectlCmd = "/snap/bin/microk8s"
		kubectlArgs = append(kubectlArgs, "kubectl")
	}

	// 1. Clean existing data using backup-helper container
	m.log.Info("🗑️  Cleaning existing data...\n")
	cleanArgs := append(kubectlArgs, "exec", "-n", m.ModuleConfig.Namespace, podName,
		"-c", "backup-helper",
		"--",
		"sh", "-c", "rm -rf /data/*")
	cleanCmd := exec.CommandContext(ctx, kubectlCmd, cleanArgs...)
	cleanCmd.Stderr = os.Stderr
	if err := cleanCmd.Run(); err != nil {
		m.log.Warn("Warning during clean: %v\n", err)
	}

	// 2. Restore from tar using backup-helper container (it has tar and write access)
	m.log.Info("📦 Restoring from archive...\n")
	restoreArgs := append(kubectlArgs, "exec", "-i", "-n", m.ModuleConfig.Namespace, podName,
		"-c", "backup-helper",
		"--",
		"tar", "xzf", "-", "-C", "/data")
	restoreCmd := exec.CommandContext(ctx, kubectlCmd, restoreArgs...)

	inFile, err := os.Open(dataBackupFile)
	if err != nil {
		return fmt.Errorf("failed to open data backup file: %w", err)
	}
	defer inFile.Close()

	restoreCmd.Stdin = inFile
	restoreCmd.Stdout = os.Stdout
	restoreCmd.Stderr = os.Stderr

	if err := restoreCmd.Run(); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}
	m.log.Success("✅ Data restored\n")

	m.log.Success("🎉 Restore complete!\n")
	return nil
}
