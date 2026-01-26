package ingress

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestIngressModule_Name(t *testing.T) {
	module := &IngressModule{}
	if module.Name() != "ingress" {
		t.Errorf("Name() = %s, want ingress", module.Name())
	}
}

func TestIngressModule_Prepare(t *testing.T) {
	tests := []struct {
		name           string
		ingressConfig  config.IngressConfig
		generalConfig  config.GeneralConfig
		wantRulesCount int
		wantTLS        bool
	}{
		{
			name: "single rule",
			generalConfig: config.GeneralConfig{
				Domain: "example.com",
			},
			ingressConfig: config.IngressConfig{
				Name:      "test-ingress",
				Namespace: "default",
				Rules: []config.IngressRule{
					{
						Host:        "test.example.com",
						Path:        "/",
						PathType:    "Prefix",
						ServiceName: "test-service",
						ServicePort: 80,
					},
				},
				TLS: false,
			},
			wantRulesCount: 1,
			wantTLS:        false,
		},
		{
			name: "multiple rules with TLS",
			generalConfig: config.GeneralConfig{
				Domain: "example.com",
			},
			ingressConfig: config.IngressConfig{
				Name:      "multi-ingress",
				Namespace: "default",
				Rules: []config.IngressRule{
					{
						Host:        "app1.example.com",
						Path:        "/",
						PathType:    "Prefix",
						ServiceName: "app1-service",
						ServicePort: 8080,
					},
					{
						Host:        "app2.example.com",
						Path:        "/api",
						PathType:    "Prefix",
						ServiceName: "app2-service",
						ServicePort: 3000,
					},
				},
				TLS: true,
			},
			wantRulesCount: 2,
			wantTLS:        true,
		},
		{
			name: "rule with default host",
			generalConfig: config.GeneralConfig{
				Domain: "example.com",
			},
			ingressConfig: config.IngressConfig{
				Name:      "default-host-ingress",
				Namespace: "default",
				Rules: []config.IngressRule{
					{
						Host:        "",
						Path:        "/",
						PathType:    "Prefix",
						ServiceName: "test-service",
						ServicePort: 80,
					},
				},
				TLS: false,
			},
			wantRulesCount: 1,
			wantTLS:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &IngressModule{
				GeneralConfig: tt.generalConfig,
				IngressConfig: tt.ingressConfig,
			}

			ingress := module.prepare()

			if ingress.Name != tt.ingressConfig.Name {
				t.Errorf("Ingress name = %s, want %s", ingress.Name, tt.ingressConfig.Name)
			}

			if ingress.Namespace != tt.ingressConfig.Namespace {
				t.Errorf("Ingress namespace = %s, want %s", ingress.Namespace, tt.ingressConfig.Namespace)
			}

			if len(ingress.Spec.Rules) != tt.wantRulesCount {
				t.Errorf("Ingress rules count = %d, want %d", len(ingress.Spec.Rules), tt.wantRulesCount)
			}

			if tt.wantTLS && len(ingress.Spec.TLS) == 0 {
				t.Error("Expected TLS configuration but got none")
			}

			if !tt.wantTLS && len(ingress.Spec.TLS) > 0 {
				t.Error("Expected no TLS configuration but got some")
			}
		})
	}
}

func TestIngressModule_PreparePathTypes(t *testing.T) {
	tests := []struct {
		name         string
		pathType     string
		wantPathType string
	}{
		{
			name:         "Prefix path type",
			pathType:     "Prefix",
			wantPathType: "Prefix",
		},
		{
			name:         "Exact path type",
			pathType:     "Exact",
			wantPathType: "Exact",
		},
		{
			name:         "ImplementationSpecific path type",
			pathType:     "ImplementationSpecific",
			wantPathType: "ImplementationSpecific",
		},
		{
			name:         "Empty path type defaults to Prefix",
			pathType:     "",
			wantPathType: "Prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &IngressModule{
				GeneralConfig: config.GeneralConfig{
					Domain: "example.com",
				},
				IngressConfig: config.IngressConfig{
					Name:      "test-ingress",
					Namespace: "default",
					Rules: []config.IngressRule{
						{
							Host:        "test.example.com",
							Path:        "/",
							PathType:    tt.pathType,
							ServiceName: "test-service",
							ServicePort: 80,
						},
					},
				},
			}

			ingress := module.prepare()

			if len(ingress.Spec.Rules) == 0 {
				t.Fatal("No rules generated")
			}

			rule := ingress.Spec.Rules[0]
			if rule.HTTP == nil || len(rule.HTTP.Paths) == 0 {
				t.Fatal("No HTTP paths generated")
			}

			path := rule.HTTP.Paths[0]
			if path.PathType == nil {
				t.Fatal("PathType is nil")
			}

			if string(*path.PathType) != tt.wantPathType {
				t.Errorf("PathType = %s, want %s", string(*path.PathType), tt.wantPathType)
			}
		})
	}
}

func TestIngressModule_PrepareLabels(t *testing.T) {
	module := &IngressModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		IngressConfig: config.IngressConfig{
			Name:      "test-ingress",
			Namespace: "default",
			Rules: []config.IngressRule{
				{
					Host:        "test.example.com",
					Path:        "/",
					ServiceName: "test-service",
					ServicePort: 80,
				},
			},
		},
	}

	ingress := module.prepare()

	if ingress.Labels == nil {
		t.Fatal("Labels is nil")
	}

	if ingress.Labels["managed-by"] != "personal-server" {
		t.Errorf("Label managed-by = %s, want personal-server", ingress.Labels["managed-by"])
	}
}

func TestIngressModule_PrepareTLS(t *testing.T) {
	module := &IngressModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		IngressConfig: config.IngressConfig{
			Name:      "tls-ingress",
			Namespace: "default",
			Rules: []config.IngressRule{
				{
					Host:        "app1.example.com",
					Path:        "/",
					ServiceName: "app1-service",
					ServicePort: 80,
				},
				{
					Host:        "app2.example.com",
					Path:        "/",
					ServiceName: "app2-service",
					ServicePort: 80,
				},
			},
			TLS: true,
		},
	}

	ingress := module.prepare()

	if len(ingress.Spec.TLS) == 0 {
		t.Fatal("Expected TLS configuration but got none")
	}

	tls := ingress.Spec.TLS[0]

	if tls.SecretName != "tls-ingress-tls" {
		t.Errorf("TLS SecretName = %s, want tls-ingress-tls", tls.SecretName)
	}

	if len(tls.Hosts) != 2 {
		t.Errorf("TLS Hosts count = %d, want 2", len(tls.Hosts))
	}
}

func TestIngressModule_PrepareMultiplePathsSameHost(t *testing.T) {
	module := &IngressModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		IngressConfig: config.IngressConfig{
			Name:      "multi-path-ingress",
			Namespace: "default",
			Rules: []config.IngressRule{
				{
					Host:        "example.com",
					Path:        "/api",
					ServiceName: "api-service",
					ServicePort: 8080,
				},
				{
					Host:        "example.com",
					Path:        "/web",
					ServiceName: "web-service",
					ServicePort: 80,
				},
			},
		},
	}

	ingress := module.prepare()

	// Should have 1 rule with 2 paths
	if len(ingress.Spec.Rules) != 1 {
		t.Errorf("Expected 1 rule (same host), got %d", len(ingress.Spec.Rules))
	}

	rule := ingress.Spec.Rules[0]
	if rule.HTTP == nil {
		t.Fatal("HTTP is nil")
	}

	if len(rule.HTTP.Paths) != 2 {
		t.Errorf("Expected 2 paths for same host, got %d", len(rule.HTTP.Paths))
	}
}

//go:embed testdata/ingress.yaml
var expectedIngressYAML string

func TestGenerate(t *testing.T) {
	// Create a temporary directory for output
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Change to temp directory so Generate creates files there
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Create module with test configuration
	module := &IngressModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		IngressConfig: config.IngressConfig{
			Name:      "test-ingress",
			Namespace: "default",
			Rules: []config.IngressRule{
				{
					Host:        "test.example.com",
					Path:        "/",
					PathType:    "Prefix",
					ServiceName: "test-service",
					ServicePort: 80,
				},
			},
			TLS: false,
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify generated file exists
	generatedPath := filepath.Join(tempDir, "configs/ingress/test-ingress/ingress.yaml")
	generatedContent, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	// Compare with expected
	if string(generatedContent) != expectedIngressYAML {
		t.Errorf("Generated YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s", string(generatedContent), expectedIngressYAML)
	}
}

func TestGenerate_NoRules(t *testing.T) {
	module := &IngressModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		IngressConfig: config.IngressConfig{
			Name:      "empty-ingress",
			Namespace: "default",
			Rules:     []config.IngressRule{},
		},
		log: logger.Default(),
	}

	ctx := context.Background()
	err := module.Generate(ctx)

	if err == nil {
		t.Error("Expected error for ingress with no rules or services, got nil")
	}
}

func TestPrepareTCPConfigMap(t *testing.T) {
	tests := []struct {
		name        string
		tcpServices []config.TCPService
		namespace   string
		wantNil     bool
		wantData    map[string]string
	}{
		{
			name:        "no TCP services",
			tcpServices: []config.TCPService{},
			namespace:   "default",
			wantNil:     true,
		},
		{
			name: "single TCP service",
			tcpServices: []config.TCPService{
				{
					Port:        5432,
					ServiceName: "postgres",
					ServicePort: 5432,
				},
			},
			namespace: "infra",
			wantNil:   false,
			wantData: map[string]string{
				"5432": "infra/postgres:5432",
			},
		},
		{
			name: "multiple TCP services",
			tcpServices: []config.TCPService{
				{
					Port:        5432,
					ServiceName: "postgres",
					ServicePort: 5432,
				},
				{
					Port:        6379,
					ServiceName: "redis",
					ServicePort: 6379,
				},
			},
			namespace: "infra",
			wantNil:   false,
			wantData: map[string]string{
				"5432": "infra/postgres:5432",
				"6379": "infra/redis:6379",
			},
		},
		{
			name: "TCP service with custom namespace",
			tcpServices: []config.TCPService{
				{
					Port:        5432,
					ServiceName: "postgres",
					ServicePort: 5432,
					Namespace:   "database",
				},
			},
			namespace: "infra",
			wantNil:   false,
			wantData: map[string]string{
				"5432": "database/postgres:5432",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &IngressModule{
				IngressConfig: config.IngressConfig{
					Name:        "test-ingress",
					Namespace:   tt.namespace,
					TCPServices: tt.tcpServices,
				},
			}

			configMap := module.prepareTCPConfigMap()

			if tt.wantNil {
				if configMap != nil {
					t.Errorf("Expected nil ConfigMap, got %v", configMap)
				}
				return
			}

			if configMap == nil {
				t.Fatal("Expected non-nil ConfigMap")
			}

			if configMap.Name != "test-ingress-tcp" {
				t.Errorf("ConfigMap name = %s, want test-ingress-tcp", configMap.Name)
			}

			if configMap.Namespace != tt.namespace {
				t.Errorf("ConfigMap namespace = %s, want %s", configMap.Namespace, tt.namespace)
			}

			if len(configMap.Data) != len(tt.wantData) {
				t.Errorf("ConfigMap data length = %d, want %d", len(configMap.Data), len(tt.wantData))
			}

			for key, value := range tt.wantData {
				if configMap.Data[key] != value {
					t.Errorf("ConfigMap data[%s] = %s, want %s", key, configMap.Data[key], value)
				}
			}
		})
	}
}

func TestPrepareUDPConfigMap(t *testing.T) {
	tests := []struct {
		name        string
		udpServices []config.UDPService
		namespace   string
		wantNil     bool
		wantData    map[string]string
	}{
		{
			name:        "no UDP services",
			udpServices: []config.UDPService{},
			namespace:   "default",
			wantNil:     true,
		},
		{
			name: "single UDP service",
			udpServices: []config.UDPService{
				{
					Port:        53,
					ServiceName: "coredns",
					ServicePort: 53,
				},
			},
			namespace: "kube-system",
			wantNil:   false,
			wantData: map[string]string{
				"53": "kube-system/coredns:53",
			},
		},
		{
			name: "UDP service with custom namespace",
			udpServices: []config.UDPService{
				{
					Port:        53,
					ServiceName: "coredns",
					ServicePort: 53,
					Namespace:   "dns-system",
				},
			},
			namespace: "kube-system",
			wantNil:   false,
			wantData: map[string]string{
				"53": "dns-system/coredns:53",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &IngressModule{
				IngressConfig: config.IngressConfig{
					Name:        "test-ingress",
					Namespace:   tt.namespace,
					UDPServices: tt.udpServices,
				},
			}

			configMap := module.prepareUDPConfigMap()

			if tt.wantNil {
				if configMap != nil {
					t.Errorf("Expected nil ConfigMap, got %v", configMap)
				}
				return
			}

			if configMap == nil {
				t.Fatal("Expected non-nil ConfigMap")
			}

			if configMap.Name != "test-ingress-udp" {
				t.Errorf("ConfigMap name = %s, want test-ingress-udp", configMap.Name)
			}

			if configMap.Namespace != tt.namespace {
				t.Errorf("ConfigMap namespace = %s, want %s", configMap.Namespace, tt.namespace)
			}

			if len(configMap.Data) != len(tt.wantData) {
				t.Errorf("ConfigMap data length = %d, want %d", len(configMap.Data), len(tt.wantData))
			}

			for key, value := range tt.wantData {
				if configMap.Data[key] != value {
					t.Errorf("ConfigMap data[%s] = %s, want %s", key, configMap.Data[key], value)
				}
			}
		})
	}
}

//go:embed testdata/tcp-configmap.yaml
var expectedTCPConfigMapYAML string

//go:embed testdata/udp-configmap.yaml
var expectedUDPConfigMapYAML string

func TestGenerate_TCPUDPServices(t *testing.T) {
	// Create a temporary directory for output
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Change to temp directory so Generate creates files there
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Create module with TCP and UDP services
	module := &IngressModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		IngressConfig: config.IngressConfig{
			Name:      "test-ingress",
			Namespace: "default",
			TCPServices: []config.TCPService{
				{
					Port:        5432,
					ServiceName: "postgres",
					ServicePort: 5432,
					Namespace:   "infra",
				},
				{
					Port:        6379,
					ServiceName: "redis",
					ServicePort: 6379,
					Namespace:   "infra",
				},
			},
			UDPServices: []config.UDPService{
				{
					Port:        53,
					ServiceName: "coredns",
					ServicePort: 53,
					Namespace:   "kube-system",
				},
			},
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify TCP ConfigMap file exists
	tcpConfigMapPath := filepath.Join(tempDir, "configs/ingress/test-ingress/tcp-configmap.yaml")
	tcpContent, err := os.ReadFile(tcpConfigMapPath)
	if err != nil {
		t.Fatalf("failed to read TCP ConfigMap file: %v", err)
	}

	// Compare with expected
	if string(tcpContent) != expectedTCPConfigMapYAML {
		t.Errorf("Generated TCP ConfigMap YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s", string(tcpContent), expectedTCPConfigMapYAML)
	}

	// Verify UDP ConfigMap file exists
	udpConfigMapPath := filepath.Join(tempDir, "configs/ingress/test-ingress/udp-configmap.yaml")
	udpContent, err := os.ReadFile(udpConfigMapPath)
	if err != nil {
		t.Fatalf("failed to read UDP ConfigMap file: %v", err)
	}

	// Compare with expected
	if string(udpContent) != expectedUDPConfigMapYAML {
		t.Errorf("Generated UDP ConfigMap YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s", string(udpContent), expectedUDPConfigMapYAML)
	}
}
