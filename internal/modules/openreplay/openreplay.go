package openreplay

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

type OpenReplayModule struct {
	GeneralConfig config.GeneralConfig
	ModuleConfig  config.Module
	log           logger.Logger
}

func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *OpenReplayModule {
	return &OpenReplayModule{
		GeneralConfig: generalConfig,
		ModuleConfig:  moduleConfig,
		log:           log,
	}
}

func (m *OpenReplayModule) Name() string {
	return "openreplay"
}

func (m *OpenReplayModule) Generate(ctx context.Context) error {
	outputDir := filepath.Join("configs", "openreplay")

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
	}

	m.log.Info("Generating OpenReplay Kubernetes configurations...\n")
	m.log.Info("Output directory: %s\n\n", outputDir)

	secret, pvc, service, deployment, err := m.prepare()
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

	if err := writeYAML(secret, "secret"); err != nil {
		return err
	}

	if err := writeYAML(pvc, "pvc"); err != nil {
		return err
	}

	if err := writeYAML(service, "service"); err != nil {
		return err
	}

	if err := writeYAML(deployment, "deployment"); err != nil {
		return err
	}

	m.log.Info("\nCompleted: 4/4 OpenReplay configurations generated successfully\n")
	return nil
}

func (m *OpenReplayModule) Apply(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Applying OpenReplay Kubernetes configurations...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	m.log.Info("Checking for existing resources...\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "openreplay-secrets", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("secret 'openreplay-secrets' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check secret existence: %w", err)
	}

	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openreplay-data-pvc", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("PersistentVolumeClaim 'openreplay-data-pvc' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check PersistentVolumeClaim existence: %w", err)
	}

	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "openreplay", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service 'openreplay' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "openreplay", metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("deployment 'openreplay' already exists in namespace '%s'", m.ModuleConfig.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check deployment existence: %w", err)
	}

	m.log.Info("No existing resources found, proceeding with creation...\n\n")

	secret, pvc, service, deployment, err := m.prepare()
	if err != nil {
		return fmt.Errorf("failed to prepare resources: %w", err)
	}

	m.log.Progress("Applying Secret: openreplay-secrets\n")
	_, err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	m.log.Success("Created Secret: openreplay-secrets\n")

	m.log.Progress("Applying PersistentVolumeClaim: openreplay-data-pvc\n")
	_, err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaim: %w", err)
	}
	m.log.Success("Created PersistentVolumeClaim: openreplay-data-pvc\n")

	m.log.Progress("Applying Service: openreplay\n")
	_, err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	m.log.Success("Created Service: openreplay\n")

	m.log.Progress("Applying Deployment: openreplay\n")
	_, err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}
	m.log.Success("Created Deployment: openreplay\n")

	m.log.Info("\nCompleted: OpenReplay configurations applied successfully\n")
	return nil
}

// prepare creates and returns the Kubernetes objects for the openreplay module
func (m *OpenReplayModule) prepare() (*corev1.Secret, *corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment, error) {
	domainName := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "domain_name", m.GeneralConfig.Domain)
	pgPassword := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "pg_password", "openreplay")
	jwtSecret := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "jwt_secret", "")
	openreplayImage := k8s.GetSecretOrDefault(m.ModuleConfig.Secrets, "openreplay_image", "openreplay/openreplay-standalone:v1.21.0")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openreplay-secrets",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "openreplay",
				"managed-by": "personal-server",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"domain-name": []byte(domainName),
			"pg-password": []byte(pgPassword),
			"jwt-secret":  []byte(jwtSecret),
		},
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openreplay-data-pvc",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "openreplay",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openreplay",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "openreplay",
				"managed-by": "personal-server",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(9000),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "openreplay",
			},
		},
	}

	replicas := int32(1)
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openreplay",
			Namespace: m.ModuleConfig.Namespace,
			Labels: map[string]string{
				"app":        "openreplay",
				"managed-by": "personal-server",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "openreplay",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "openreplay",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "openreplay",
							Image:           openreplayImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 9000,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "DOMAIN_NAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "openreplay-secrets",
											},
											Key: "domain-name",
										},
									},
								},
								{
									Name: "PG_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "openreplay-secrets",
											},
											Key: "pg-password",
										},
									},
								},
								{
									Name: "JWT_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "openreplay-secrets",
											},
											Key: "jwt-secret",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 120,
								PeriodSeconds:       15,
								TimeoutSeconds:      5,
								FailureThreshold:    10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       10,
								TimeoutSeconds:      3,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "openreplay-data",
									MountPath: "/var/lib/openreplay",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "openreplay-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "openreplay-data-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	return secret, pvc, service, deployment, nil
}

func (m *OpenReplayModule) Clean(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Cleaning OpenReplay Kubernetes resources...\n")
	m.log.Info("Target namespace: %s\n\n", m.ModuleConfig.Namespace)

	successCount := 0
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	m.log.Info("🗑️  Processing Deployment: openreplay\n")
	err = clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Delete(ctx, "openreplay", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Deployment 'openreplay' not found\n")
		} else {
			m.log.Error("Failed to delete Deployment: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Deployment: openreplay\n")
		successCount++
	}

	m.log.Info("🗑️  Processing Service: openreplay\n")
	err = clientset.CoreV1().Services(m.ModuleConfig.Namespace).Delete(ctx, "openreplay", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Service 'openreplay' not found\n")
		} else {
			m.log.Error("Failed to delete Service: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Service: openreplay\n")
		successCount++
	}

	m.log.Info("🗑️  Processing PersistentVolumeClaim: openreplay-data-pvc\n")
	err = clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Delete(ctx, "openreplay-data-pvc", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("PersistentVolumeClaim 'openreplay-data-pvc' not found\n")
		} else {
			m.log.Error("Failed to delete PersistentVolumeClaim: %v\n", err)
		}
	} else {
		m.log.Success("Deleted PersistentVolumeClaim: openreplay-data-pvc\n")
		successCount++
	}

	m.log.Info("🗑️  Processing Secret: openreplay-secrets\n")
	err = clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Delete(ctx, "openreplay-secrets", deleteOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Warn("Secret 'openreplay-secrets' not found\n")
		} else {
			m.log.Error("Failed to delete Secret: %v\n", err)
		}
	} else {
		m.log.Success("Deleted Secret: openreplay-secrets\n")
		successCount++
	}

	m.log.Info("\nCompleted: %d OpenReplay resources deleted successfully\n", successCount)
	if successCount > 0 {
		m.log.Println("\nNote: Resource deletion is asynchronous and may take some time to complete.")
	}
	return nil
}

func (m *OpenReplayModule) Status(ctx context.Context) error {
	clientset, err := k8s.CreateKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	m.log.Info("Checking OpenReplay resources in namespace '%s'...\n\n", m.ModuleConfig.Namespace)

	deployment, err := clientset.AppsV1().Deployments(m.ModuleConfig.Namespace).Get(ctx, "openreplay", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Deployment 'openreplay' not found\n")
		} else {
			m.log.Error("Error getting Deployment: %v\n", err)
		}
	} else {
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("DEPLOYMENT:\n")
		m.log.Info("  Name:            %s\n", deployment.Name)
		m.log.Info("  Ready:           %d/%d\n", deployment.Status.ReadyReplicas, deployment.Status.Replicas)
		m.log.Info("  Up-to-date:      %d\n", deployment.Status.UpdatedReplicas)
		m.log.Info("  Available:       %d\n", deployment.Status.AvailableReplicas)
		m.log.Info("  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	service, err := clientset.CoreV1().Services(m.ModuleConfig.Namespace).Get(ctx, "openreplay", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Service 'openreplay' not found\n")
		} else {
			m.log.Error("Error getting Service: %v\n", err)
		}
	} else {
		age := time.Since(service.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("SERVICE:\n")
		m.log.Info("  Name:            %s\n", service.Name)
		m.log.Info("  Type:            %s\n", service.Spec.Type)
		m.log.Info("  Cluster-IP:      %s\n", service.Spec.ClusterIP)
		m.log.Print("  Ports:           ")
		for i, port := range service.Spec.Ports {
			if i > 0 {
				m.log.Print(", ")
			}
			m.log.Print("%d/%s", port.Port, port.Protocol)
		}
		m.log.Info("\n  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	pvc, err := clientset.CoreV1().PersistentVolumeClaims(m.ModuleConfig.Namespace).Get(ctx, "openreplay-data-pvc", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("PersistentVolumeClaim 'openreplay-data-pvc' not found\n")
		} else {
			m.log.Error("Error getting PersistentVolumeClaim: %v\n", err)
		}
	} else {
		age := time.Since(pvc.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("PERSISTENT VOLUME CLAIM:\n")
		m.log.Info("  Name:            %s\n", pvc.Name)
		m.log.Info("  Status:          %s\n", pvc.Status.Phase)
		m.log.Info("  Volume:          %s\n", pvc.Spec.VolumeName)
		m.log.Info("  Capacity:        %s\n", pvc.Status.Capacity.Storage().String())
		m.log.Info("  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	secret, err := clientset.CoreV1().Secrets(m.ModuleConfig.Namespace).Get(ctx, "openreplay-secrets", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			m.log.Error("Secret 'openreplay-secrets' not found\n")
		} else {
			m.log.Error("Error getting Secret: %v\n", err)
		}
	} else {
		age := time.Since(secret.CreationTimestamp.Time).Round(time.Second)
		m.log.Info("SECRET:\n")
		m.log.Info("  Name:            %s\n", secret.Name)
		m.log.Info("  Type:            %s\n", secret.Type)
		m.log.Info("  Data keys:       %d\n", len(secret.Data))
		m.log.Info("  Age:             %s\n", k8s.FormatAge(age))
		m.log.Println()
	}

	pods, err := clientset.CoreV1().Pods(m.ModuleConfig.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=openreplay",
	})
	if err != nil {
		m.log.Error("Error listing pods: %v\n", err)
	} else if len(pods.Items) > 0 {
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
	} else {
		m.log.Println("No OpenReplay pods found")
	}
	return nil
}
