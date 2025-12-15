package pgadmin

import (
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func TestPgadminModule_Name(t *testing.T) {
	module := &PgadminModule{}
	if module.Name() != "pgadmin" {
		t.Errorf("Name() = %s, want pgadmin", module.Name())
	}
}

func TestPgadminModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		secrets   map[string]string
		wantErr   bool
	}{
		{
			name:      "valid configuration",
			namespace: "infra",
			secrets: map[string]string{
				"pgadmin_default_email":  "admin@example.com",
				"pgadmin_admin_password": "secret123",
			},
			wantErr: false,
		},
		{
			name:      "missing pgadmin_default_email",
			namespace: "infra",
			secrets: map[string]string{
				"pgadmin_admin_password": "secret123",
			},
			wantErr: true,
		},
		{
			name:      "missing pgadmin_admin_password",
			namespace: "infra",
			secrets: map[string]string{
				"pgadmin_default_email": "admin@example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &PgadminModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				ModuleConfig: config.Module{
					Name:      "pgadmin",
					Namespace: tt.namespace,
					Secrets:   tt.secrets,
				},
			}

			secret, service, deployment, err := module.prepare()

			if tt.wantErr {
				if err == nil {
					t.Error("prepare() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("prepare() unexpected error: %v", err)
				return
			}

			// Verify all objects are not nil
			if secret == nil {
				t.Error("prepare() returned nil Secret")
			}
			if service == nil {
				t.Error("prepare() returned nil Service")
			}
			if deployment == nil {
				t.Error("prepare() returned nil Deployment")
			}

			// Verify namespace is set correctly
			if secret.Namespace != tt.namespace {
				t.Errorf("Secret namespace = %s, want %s", secret.Namespace, tt.namespace)
			}
			if service.Namespace != tt.namespace {
				t.Errorf("Service namespace = %s, want %s", service.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
		})
	}
}

func TestPgadminModule_PrepareSecret(t *testing.T) {
	email := "admin@example.com"
	password := "secret123"
	module := &PgadminModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "pgadmin",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"pgadmin_default_email":  email,
				"pgadmin_admin_password": password,
			},
		},
	}

	secret, _, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Secret name
	if secret.Name != "pgadmin-secrets" {
		t.Errorf("Secret name = %s, want pgadmin-secrets", secret.Name)
	}

	// Test Secret type
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("Secret type = %s, want Opaque", secret.Type)
	}

	// Test Secret labels
	if secret.Labels["app"] != "pgadmin" {
		t.Errorf("Secret label app = %s, want pgadmin", secret.Labels["app"])
	}
	if secret.Labels["managed-by"] != "personal-server" {
		t.Errorf("Secret label managed-by = %s, want personal-server", secret.Labels["managed-by"])
	}

	// Test Secret data
	if string(secret.Data["pgadmin_default_email"]) != email {
		t.Errorf("Secret data[pgadmin_default_email] = %s, want %s", string(secret.Data["pgadmin_default_email"]), email)
	}
	if string(secret.Data["pgadmin_admin_password"]) != password {
		t.Errorf("Secret data[pgadmin_admin_password] = %s, want %s", string(secret.Data["pgadmin_admin_password"]), password)
	}
}

func TestPgadminModule_PrepareService(t *testing.T) {
	module := &PgadminModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "pgadmin",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"pgadmin_default_email":  "admin@example.com",
				"pgadmin_admin_password": "secret123",
			},
		},
	}

	_, service, _, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Service name
	if service.Name != "pgadmin" {
		t.Errorf("Service name = %s, want pgadmin", service.Name)
	}

	// Test Service labels
	if service.Labels["app"] != "pgadmin" {
		t.Errorf("Service label app = %s, want pgadmin", service.Labels["app"])
	}
	if service.Labels["managed-by"] != "personal-server" {
		t.Errorf("Service label managed-by = %s, want personal-server", service.Labels["managed-by"])
	}

	// Test selector
	if service.Spec.Selector["app"] != "pgadmin" {
		t.Errorf("Service selector app = %s, want pgadmin", service.Spec.Selector["app"])
	}

	// Test Service ports
	if len(service.Spec.Ports) != 1 {
		t.Errorf("Service ports count = %d, want 1", len(service.Spec.Ports))
	}

	port := service.Spec.Ports[0]
	if port.Name != "http" {
		t.Errorf("Service port name = %s, want http", port.Name)
	}
	if port.Port != 80 {
		t.Errorf("Service port = %d, want 80", port.Port)
	}
	if port.TargetPort.IntVal != 80 {
		t.Errorf("Service targetPort = %d, want 80", port.TargetPort.IntVal)
	}
	if port.Protocol != corev1.ProtocolTCP {
		t.Errorf("Service port protocol = %s, want TCP", port.Protocol)
	}
}

func TestPgadminModule_PrepareDeployment(t *testing.T) {
	module := &PgadminModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "pgadmin",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"pgadmin_default_email":  "admin@example.com",
				"pgadmin_admin_password": "secret123",
			},
		},
	}

	_, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Test Deployment name
	if deployment.Name != "pgadmin" {
		t.Errorf("Deployment name = %s, want pgadmin", deployment.Name)
	}

	// Test labels
	if deployment.Labels["app"] != "pgadmin" {
		t.Errorf("Deployment label app = %s, want pgadmin", deployment.Labels["app"])
	}
	if deployment.Labels["managed-by"] != "personal-server" {
		t.Errorf("Deployment label managed-by = %s, want personal-server", deployment.Labels["managed-by"])
	}

	// Test replicas
	if deployment.Spec.Replicas == nil {
		t.Fatal("Deployment replicas is nil")
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	// Test revision history limit
	if deployment.Spec.RevisionHistoryLimit == nil {
		t.Fatal("Deployment revisionHistoryLimit is nil")
	}
	if *deployment.Spec.RevisionHistoryLimit != 1 {
		t.Errorf("Deployment revisionHistoryLimit = %d, want 1", *deployment.Spec.RevisionHistoryLimit)
	}

	// Test selector
	if deployment.Spec.Selector.MatchLabels["app"] != "pgadmin" {
		t.Errorf("Deployment selector app = %s, want pgadmin", deployment.Spec.Selector.MatchLabels["app"])
	}

	// Test restart policy
	if deployment.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		t.Errorf("Pod restart policy = %s, want Always", deployment.Spec.Template.Spec.RestartPolicy)
	}

	// Test termination grace period
	if deployment.Spec.Template.Spec.TerminationGracePeriodSeconds == nil {
		t.Fatal("TerminationGracePeriodSeconds is nil")
	}
	if *deployment.Spec.Template.Spec.TerminationGracePeriodSeconds != 0 {
		t.Errorf("TerminationGracePeriodSeconds = %d, want 0", *deployment.Spec.Template.Spec.TerminationGracePeriodSeconds)
	}
}

func TestPgadminModule_PrepareDeploymentContainer(t *testing.T) {
	module := &PgadminModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "pgadmin",
			Namespace: "test-namespace",
			Secrets: map[string]string{
				"pgadmin_default_email":  "admin@example.com",
				"pgadmin_admin_password": "secret123",
			},
		},
	}

	_, _, deployment, err := module.prepare()
	if err != nil {
		t.Fatalf("prepare() error: %v", err)
	}

	// Verify container count
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Container count = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Test container name
	if container.Name != "pgadmin" {
		t.Errorf("Container name = %s, want pgadmin", container.Name)
	}

	// Test container image
	if container.Image != "dpage/pgadmin4:9.10.0" {
		t.Errorf("Container image = %s, want dpage/pgadmin4:9.10.0", container.Image)
	}

	// Test image pull policy
	if container.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("Container ImagePullPolicy = %s, want Always", container.ImagePullPolicy)
	}

	// Test container ports
	if len(container.Ports) != 1 {
		t.Errorf("Container ports count = %d, want 1", len(container.Ports))
	}
	if container.Ports[0].ContainerPort != 80 {
		t.Errorf("Container port = %d, want 80", container.Ports[0].ContainerPort)
	}
	if container.Ports[0].Name != "http" {
		t.Errorf("Container port name = %s, want http", container.Ports[0].Name)
	}

	// Test environment variables
	envNames := make(map[string]bool)
	for _, env := range container.Env {
		envNames[env.Name] = true
		if env.Name == "PGADMIN_CONFIG_ENHANCED_COOKIE_PROTECTION" && env.Value != "False" {
			t.Errorf("PGADMIN_CONFIG_ENHANCED_COOKIE_PROTECTION = %s, want False", env.Value)
		}
	}

	// Verify required env vars exist
	requiredEnvs := []string{"PGADMIN_DEFAULT_EMAIL", "PGADMIN_DEFAULT_PASSWORD", "PGADMIN_CONFIG_ENHANCED_COOKIE_PROTECTION"}
	for _, envName := range requiredEnvs {
		if !envNames[envName] {
			t.Errorf("Container missing env var: %s", envName)
		}
	}

	// Verify secret references for sensitive env vars
	for _, env := range container.Env {
		if env.Name == "PGADMIN_DEFAULT_EMAIL" || env.Name == "PGADMIN_DEFAULT_PASSWORD" {
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Errorf("%s should use SecretKeyRef", env.Name)
			} else if env.ValueFrom.SecretKeyRef.Name != "pgadmin-secrets" {
				t.Errorf("%s secret name = %s, want pgadmin-secrets", env.Name, env.ValueFrom.SecretKeyRef.Name)
			}
		}
	}
}
