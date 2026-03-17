package xray

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

//go:embed testdata/configmap.yaml
var expectedConfigmapYAML string

//go:embed testdata/deployment.yaml
var expectedDeploymentYAML string

//go:embed testdata/service.yaml
var expectedServiceYAML string

//go:embed testdata/ingress.yaml
var expectedIngressYAML string

func TestXrayModule_Name(t *testing.T) {
	m := &XrayModule{}
	if m.Name() != "xray" {
		t.Errorf("Name() = %s, want xray", m.Name())
	}
}

func TestXrayModule_Doc(t *testing.T) {
	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig:  config.Module{Name: "xray", Namespace: "infra"},
		log:           logger.NewNopLogger(),
	}
	if err := m.Doc(context.Background()); err != nil {
		t.Errorf("Doc() returned unexpected error: %v", err)
	}
}

func TestXrayModule_Prepare(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{"infra namespace", "infra"},
		{"custom namespace", "vpn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &XrayModule{
				GeneralConfig: config.GeneralConfig{Domain: "example.com"},
				ModuleConfig: config.Module{
					Name:      "xray",
					Namespace: tt.namespace,
					Secrets: map[string]string{
						"xray_uuid": "a2fb4a5d-c720-4f71-b256-9a43d6d534c8",
						"xray_path": "/vpn-secret-path",
					},
				},
			}

			configMap, deployment, service, ingress, err := m.prepare()
			if err != nil {
				t.Fatalf("prepare() returned unexpected error: %v", err)
			}
			if configMap == nil {
				t.Fatal("prepare() returned nil ConfigMap")
			}
			if deployment == nil {
				t.Fatal("prepare() returned nil Deployment")
			}
			if service == nil {
				t.Fatal("prepare() returned nil Service")
			}
			if ingress == nil {
				t.Fatal("prepare() returned nil Ingress")
			}

			if configMap.Namespace != tt.namespace {
				t.Errorf("ConfigMap namespace = %s, want %s", configMap.Namespace, tt.namespace)
			}
			if deployment.Namespace != tt.namespace {
				t.Errorf("Deployment namespace = %s, want %s", deployment.Namespace, tt.namespace)
			}
			if service.Namespace != tt.namespace {
				t.Errorf("Service namespace = %s, want %s", service.Namespace, tt.namespace)
			}
			if ingress.Namespace != tt.namespace {
				t.Errorf("Ingress namespace = %s, want %s", ingress.Namespace, tt.namespace)
			}
		})
	}
}

func TestXrayModule_PrepareRequiresUUID(t *testing.T) {
	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig: config.Module{
			Name:      "xray",
			Namespace: "infra",
			Secrets:   map[string]string{},
		},
	}
	_, _, _, _, err := m.prepare()
	if err == nil {
		t.Error("prepare() expected error when xray_uuid is missing, got nil")
	}
}

func TestXrayModule_PrepareConfigMap(t *testing.T) {
	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig: config.Module{
			Name:      "xray",
			Namespace: "infra",
			Secrets: map[string]string{
				"xray_uuid": "a2fb4a5d-c720-4f71-b256-9a43d6d534c8",
				"xray_path": "/vpn-secret-path",
			},
		},
	}

	configMap, _, _, _, err := m.prepare()
	if err != nil {
		t.Fatalf("prepare() unexpected error: %v", err)
	}

	if configMap.Name != "xray-config" {
		t.Errorf("ConfigMap name = %s, want xray-config", configMap.Name)
	}
	if configMap.Labels["app"] != "xray" {
		t.Errorf("ConfigMap label app = %s, want xray", configMap.Labels["app"])
	}
	if configMap.Labels["managed-by"] != "personal-server" {
		t.Errorf("ConfigMap label managed-by = %s, want personal-server", configMap.Labels["managed-by"])
	}
	if _, ok := configMap.Data[xrayConfigFile]; !ok {
		t.Errorf("ConfigMap missing data key %s", xrayConfigFile)
	}
}

func TestXrayModule_PrepareDeployment(t *testing.T) {
	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig: config.Module{
			Name:      "xray",
			Namespace: "infra",
			Secrets: map[string]string{
				"xray_uuid": "a2fb4a5d-c720-4f71-b256-9a43d6d534c8",
				"xray_path": "/vpn-secret-path",
			},
		},
	}

	_, deployment, _, _, err := m.prepare()
	if err != nil {
		t.Fatalf("prepare() unexpected error: %v", err)
	}

	if deployment.Name != "xray" {
		t.Errorf("Deployment name = %s, want xray", deployment.Name)
	}
	if deployment.Labels["app"] != "xray" {
		t.Errorf("Deployment label app = %s, want xray", deployment.Labels["app"])
	}
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("Deployment replicas = %d, want 1", *deployment.Spec.Replicas)
	}

	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		t.Fatal("Deployment has no containers")
	}
	container := containers[0]
	if container.Name != "xray" {
		t.Errorf("Container name = %s, want xray", container.Name)
	}
	if container.Image != xrayImage {
		t.Errorf("Container image = %s, want %s", container.Image, xrayImage)
	}
	if len(container.Ports) == 0 || container.Ports[0].ContainerPort != xrayPort {
		t.Errorf("Container port = %v, want %d", container.Ports, xrayPort)
	}
	if len(container.VolumeMounts) == 0 || container.VolumeMounts[0].MountPath != xrayConfigMount {
		t.Errorf("Container volumeMount mountPath = %v, want %s", container.VolumeMounts, xrayConfigMount)
	}

	volumes := deployment.Spec.Template.Spec.Volumes
	if len(volumes) == 0 || volumes[0].Name != xrayConfigVolume {
		t.Errorf("Deployment volumes = %v, want volume named %s", volumes, xrayConfigVolume)
	}
}

func TestXrayModule_PrepareService(t *testing.T) {
	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig: config.Module{
			Name:      "xray",
			Namespace: "infra",
			Secrets: map[string]string{
				"xray_uuid": "a2fb4a5d-c720-4f71-b256-9a43d6d534c8",
				"xray_path": "/vpn-secret-path",
			},
		},
	}

	_, _, service, _, err := m.prepare()
	if err != nil {
		t.Fatalf("prepare() unexpected error: %v", err)
	}

	if service.Name != "xray" {
		t.Errorf("Service name = %s, want xray", service.Name)
	}
	if service.Labels["app"] != "xray" {
		t.Errorf("Service label app = %s, want xray", service.Labels["app"])
	}
	if len(service.Spec.Ports) == 0 || service.Spec.Ports[0].Port != xrayPort {
		t.Errorf("Service port = %v, want %d", service.Spec.Ports, xrayPort)
	}
	if service.Spec.Selector["app"] != "xray" {
		t.Errorf("Service selector app = %s, want xray", service.Spec.Selector["app"])
	}
}

func TestXrayModule_PrepareIngress(t *testing.T) {
	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig: config.Module{
			Name:      "xray",
			Namespace: "infra",
			Secrets: map[string]string{
				"xray_uuid": "a2fb4a5d-c720-4f71-b256-9a43d6d534c8",
				"xray_path": "/vpn-secret-path",
			},
		},
	}

	_, _, _, ingress, err := m.prepare()
	if err != nil {
		t.Fatalf("prepare() unexpected error: %v", err)
	}

	if ingress.Name != "xray" {
		t.Errorf("Ingress name = %s, want xray", ingress.Name)
	}
	if ingress.Annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] != "3600" {
		t.Errorf("Ingress missing proxy-read-timeout annotation")
	}
	if ingress.Annotations["nginx.ingress.kubernetes.io/proxy-send-timeout"] != "3600" {
		t.Errorf("Ingress missing proxy-send-timeout annotation")
	}
	if _, ok := ingress.Annotations["nginx.ingress.kubernetes.io/configuration-snippet"]; !ok {
		t.Errorf("Ingress missing configuration-snippet annotation for WebSocket support")
	}
	if len(ingress.Spec.TLS) == 0 {
		t.Error("Ingress has no TLS configuration")
	}
	if len(ingress.Spec.Rules) == 0 {
		t.Error("Ingress has no rules")
	}
	rule := ingress.Spec.Rules[0]
	if rule.Host != "example.com" {
		t.Errorf("Ingress rule host = %s, want example.com", rule.Host)
	}
	if len(rule.HTTP.Paths) == 0 || rule.HTTP.Paths[0].Path != "/vpn-secret-path" {
		t.Errorf("Ingress rule path = %v, want /vpn-secret-path", rule.HTTP.Paths)
	}
}

func TestXrayModule_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	m := &XrayModule{
		GeneralConfig: config.GeneralConfig{Domain: "example.com"},
		ModuleConfig: config.Module{
			Name:      "xray",
			Namespace: "infra",
			Secrets: map[string]string{
				"xray_uuid": "a2fb4a5d-c720-4f71-b256-9a43d6d534c8",
				"xray_path": "/vpn-secret-path",
			},
		},
		log: logger.NewNopLogger(),
	}

	if err := m.Generate(context.Background()); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	cases := []struct {
		name string
		file string
		want string
	}{
		{"configmap", "configs/xray/configmap.yaml", expectedConfigmapYAML},
		{"deployment", "configs/xray/deployment.yaml", expectedDeploymentYAML},
		{"service", "configs/xray/service.yaml", expectedServiceYAML},
		{"ingress", "configs/xray/ingress.yaml", expectedIngressYAML},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := os.ReadFile(filepath.Join(tmpDir, tc.file))
			if err != nil {
				t.Fatalf("failed to read %s: %v", tc.file, err)
			}
			if string(got) != tc.want {
				t.Errorf("Generated YAML does not match expected.\nGot:\n%s\n\nWant:\n%s", string(got), tc.want)
			}
		})
	}
}
