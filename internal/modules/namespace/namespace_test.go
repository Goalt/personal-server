package namespace

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestNamespaceModule_Name(t *testing.T) {
	module := &NamespaceModule{}
	if module.Name() != "namespace" {
		t.Errorf("Name() = %s, want namespace", module.Name())
	}
}

func TestNamespaceModule_Prepare(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		wantCount  int
	}{
		{
			name:       "single namespace",
			namespaces: []string{"infra"},
			wantCount:  1,
		},
		{
			name:       "multiple namespaces",
			namespaces: []string{"infra", "hobby", "monitoring"},
			wantCount:  3,
		},
		{
			name:       "empty namespaces",
			namespaces: []string{},
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &NamespaceModule{
				GeneralConfig: config.GeneralConfig{
					Domain:     "example.com",
					Namespaces: tt.namespaces,
				},
			}

			namespaces := module.prepare()

			if len(namespaces) != tt.wantCount {
				t.Errorf("prepare() returned %d namespaces, want %d", len(namespaces), tt.wantCount)
			}
		})
	}
}

func TestNamespaceModule_PrepareNamespaceNames(t *testing.T) {
	expectedNamespaces := []string{"infra", "hobby", "monitoring"}
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: expectedNamespaces,
		},
	}

	namespaces := module.prepare()

	for i, ns := range namespaces {
		if ns.Name != expectedNamespaces[i] {
			t.Errorf("Namespace[%d] name = %s, want %s", i, ns.Name, expectedNamespaces[i])
		}
	}
}

func TestNamespaceModule_PrepareNamespaceLabels(t *testing.T) {
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: []string{"infra", "hobby"},
		},
	}

	namespaces := module.prepare()

	for _, ns := range namespaces {
		// Check managed-by label
		if ns.Labels["managed-by"] != "personal-server" {
			t.Errorf("Namespace %s label managed-by = %s, want personal-server", ns.Name, ns.Labels["managed-by"])
		}
	}
}

func TestNamespaceModule_PreparePreservesOrder(t *testing.T) {
	expectedNamespaces := []string{"alpha", "beta", "gamma", "delta"}
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: expectedNamespaces,
		},
	}

	namespaces := module.prepare()

	if len(namespaces) != len(expectedNamespaces) {
		t.Fatalf("prepare() returned %d namespaces, want %d", len(namespaces), len(expectedNamespaces))
	}

	for i, ns := range namespaces {
		if ns.Name != expectedNamespaces[i] {
			t.Errorf("Namespace order mismatch at index %d: got %s, want %s", i, ns.Name, expectedNamespaces[i])
		}
	}
}

func TestNamespaceModule_PrepareEmptyNamespaces(t *testing.T) {
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: []string{},
		},
	}

	namespaces := module.prepare()

	if len(namespaces) != 0 {
		t.Errorf("prepare() returned %d namespaces for empty config, want 0", len(namespaces))
	}
}

func TestNamespaceModule_PrepareNilNamespaces(t *testing.T) {
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: nil,
		},
	}

	namespaces := module.prepare()

	if len(namespaces) != 0 {
		t.Errorf("prepare() returned %d namespaces for nil config, want 0", len(namespaces))
	}
}

func TestNamespaceModule_PrepareObjectMetaComplete(t *testing.T) {
	namespaceName := "test-namespace"
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: []string{namespaceName},
		},
	}

	namespaces := module.prepare()

	if len(namespaces) != 1 {
		t.Fatalf("prepare() returned %d namespaces, want 1", len(namespaces))
	}

	ns := namespaces[0]

	// Verify ObjectMeta
	if ns.ObjectMeta.Name != namespaceName {
		t.Errorf("Namespace ObjectMeta.Name = %s, want %s", ns.ObjectMeta.Name, namespaceName)
	}

	// Verify Labels exist
	if ns.ObjectMeta.Labels == nil {
		t.Error("Namespace ObjectMeta.Labels is nil")
	}

	// Verify managed-by label
	if ns.ObjectMeta.Labels["managed-by"] != "personal-server" {
		t.Errorf("Namespace ObjectMeta.Labels[managed-by] = %s, want personal-server", ns.ObjectMeta.Labels["managed-by"])
	}
}

//go:embed testdata/infra.yaml
var expectedInfraYAML string

//go:embed testdata/hobby.yaml
var expectedHobbyYAML string

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
	module := &NamespaceModule{
		GeneralConfig: config.GeneralConfig{
			Domain:     "example.com",
			Namespaces: []string{"infra", "hobby"},
		},
		log: logger.Default(),
	}

	// Run Generate
	ctx := context.Background()
	if err := module.Generate(ctx); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify generated files exist and match expected content
	testCases := []struct {
		name     string
		filename string
		expected string
	}{
		{"infra namespace", "configs/namespace/infra.yaml", expectedInfraYAML},
		{"hobby namespace", "configs/namespace/hobby.yaml", expectedHobbyYAML},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read generated file
			generatedPath := filepath.Join(tempDir, tc.filename)
			generatedContent, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("failed to read generated file %s: %v", tc.filename, err)
			}

			// Compare with expected
			if string(generatedContent) != tc.expected {
				t.Errorf("Generated YAML does not match expected.\nGenerated:\n%s\n\nExpected:\n%s", string(generatedContent), tc.expected)
			}
		})
	}
}
