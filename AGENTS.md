# AGENTS.md — Guide for Writing New Modules

This document describes how to create a new module for personal-server. Follow every step
to ensure your module integrates correctly with the CLI, configuration system, tests, help
output, and README.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Module Interface](#2-module-interface)
3. [Directory Layout](#3-directory-layout)
4. [Step-by-Step: Implement a Module](#4-step-by-step-implement-a-module)
   - [4.1 Create the module package](#41-create-the-module-package)
   - [4.2 Implement the Module interface](#42-implement-the-module-interface)
   - [4.3 Implement optional interfaces](#43-implement-optional-interfaces)
   - [4.4 Register the module](#44-register-the-module)
   - [4.5 Add configuration](#45-add-configuration)
5. [Tests](#5-tests)
   - [5.1 Unit tests](#51-unit-tests)
   - [5.2 testdata YAML fixtures](#52-testdata-yaml-fixtures)
6. [Help & Usage](#6-help--usage)
7. [README Updates](#7-readme-updates)
8. [Checklist](#8-checklist)

---

## 1. Overview

A **module** is a self-contained Go package under `internal/modules/<module-name>/` that
manages one Kubernetes service (e.g. Redis, Gitea, Bitwarden). Every module:

- implements the `Module` interface (generate, apply, clean, status),
- optionally implements extra interfaces (backup, restore, test, …),
- is registered in `internal/modules/registry_default.go`,
- is configured through `config.yaml` in the `modules:` list,
- ships with unit tests and testdata YAML fixtures.

---

## 2. Module Interface

Defined in `internal/modules/module.go`:

```go
// Required — every module must implement this.
type Module interface {
    Name()     string
    Doc(ctx context.Context) error       // print module documentation
    Generate(ctx context.Context) error  // write YAML to configs/<name>/
    Apply(ctx context.Context) error     // kubectl-apply the generated YAML
    Clean(ctx context.Context) error     // remove all K8s resources
    Status(ctx context.Context) error    // print resource status
}

// Optional — implement whichever make sense for your service.
type Backuper  interface { Backup(ctx context.Context, destDir string) error }
type Restorer  interface { Restore(ctx context.Context, args []string) error }
type DatabaseManager interface {
    AddDB(ctx context.Context, args []string) error
    RemoveDB(ctx context.Context, args []string) error
}
type Tester    interface { Test(ctx context.Context) error }
type Notifier  interface { Notify(ctx context.Context, user, ip, sshConnection string) error }
type Rollouter interface { Rollout(ctx context.Context, args []string) error }
type CodeServeWebRunner interface { CodeServeWeb(ctx context.Context) error }
```

Each implemented optional interface automatically adds a corresponding CLI subcommand
(e.g. `backup`, `restore`, `test`, `rollout`, …) — no changes to `app.go` required.

---

## 3. Directory Layout

```
internal/modules/<module-name>/
├── <module-name>.go        # Module implementation
├── <module-name>_test.go   # Unit tests
└── testdata/               # Golden YAML fixtures used by tests
    ├── pvc.yaml
    ├── service.yaml
    └── deployment.yaml     # (add more as needed)
```

---

## 4. Step-by-Step: Implement a Module

### 4.1 Create the module package

```bash
mkdir -p internal/modules/myservice
touch internal/modules/myservice/myservice.go
touch internal/modules/myservice/myservice_test.go
mkdir -p internal/modules/myservice/testdata
```

### 4.2 Implement the Module interface

```go
package myservice

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/Goalt/personal-server/internal/config"
    "github.com/Goalt/personal-server/internal/k8s"
    "github.com/Goalt/personal-server/internal/logger"
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MyServiceModule holds all configuration for the module.
type MyServiceModule struct {
    GeneralConfig config.GeneralConfig
    ModuleConfig  config.Module
    log           logger.Logger
}

// New constructs a MyServiceModule.
func New(generalConfig config.GeneralConfig, moduleConfig config.Module, log logger.Logger) *MyServiceModule {
    return &MyServiceModule{
        GeneralConfig: generalConfig,
        ModuleConfig:  moduleConfig,
        log:           log,
    }
}

func (m *MyServiceModule) Name() string { return "myservice" }

// Doc prints human-readable documentation about the module.
// Use m.log.Info to describe: what the module deploys, required secrets, and available subcommands.
func (m *MyServiceModule) Doc(ctx context.Context) error {
    m.log.Info("Module: myservice\n\n")
    m.log.Info("Description:\n  Brief description of what this module deploys.\n\n")
    m.log.Info("Required configuration keys (modules[].secrets):\n  my_password   Password used by the service\n\n")
    m.log.Info("Subcommands:\n  generate   Write Kubernetes YAML to configs/myservice/\n  apply      Create/update resources in the cluster\n  clean      Delete all resources from the cluster\n  status     Print resource status\n  doc        Show this documentation\n")
    return nil
}

// prepare returns the Kubernetes objects for this module.
// Keep all object construction here so tests can call prepare() directly.
func (m *MyServiceModule) prepare() (*corev1.PersistentVolumeClaim, *corev1.Service, *appsv1.Deployment) {
    ns := m.ModuleConfig.Namespace
    labels := map[string]string{
        "managed-by": "personal-server",
        "app":        "myservice",
    }

    pvc := &corev1.PersistentVolumeClaim{
        // ... fill in spec
    }
    pvc.Namespace = ns
    pvc.Labels    = labels

    svc := &corev1.Service{/* ... */}
    svc.Namespace = ns

    dep := &appsv1.Deployment{/* ... */}
    dep.Namespace = ns

    return pvc, svc, dep
}

// Generate writes YAML files to configs/myservice/.
func (m *MyServiceModule) Generate(ctx context.Context) error {
    outputDir := filepath.Join("configs", "myservice")
    if err := os.MkdirAll(outputDir, 0755); err != nil {
        return fmt.Errorf("failed to create output directory '%s': %w", outputDir, err)
    }

    m.log.Info("Generating MyService Kubernetes configurations...\n")

    pvc, svc, dep := m.prepare()

    writeYAML := func(obj interface{}, name string) error {
        jsonBytes, err := json.Marshal(obj)
        if err != nil {
            return fmt.Errorf("failed to marshal %s: %w", name, err)
        }
        yamlContent, err := k8s.JSONToYAML(string(jsonBytes))
        if err != nil {
            return fmt.Errorf("failed to convert %s to YAML: %w", name, err)
        }
        filename := filepath.Join(outputDir, fmt.Sprintf("%s.yaml", name))
        if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
            return fmt.Errorf("failed to write %s: %w", name, err)
        }
        m.log.Success("Generated: %s\n", filename)
        return nil
    }

    if err := writeYAML(pvc, "pvc");        err != nil { return err }
    if err := writeYAML(svc, "service");    err != nil { return err }
    if err := writeYAML(dep, "deployment"); err != nil { return err }

    return nil
}

// Apply kubectl-applies all generated YAML files.
func (m *MyServiceModule) Apply(ctx context.Context) error {
    client, err := k8s.NewClient()
    if err != nil {
        return fmt.Errorf("failed to create k8s client: %w", err)
    }

    ns := m.ModuleConfig.Namespace
    pvc, svc, dep := m.prepare()

    // Apply PVC
    _, err = client.CoreV1().PersistentVolumeClaims(ns).Create(ctx, pvc, metav1.CreateOptions{})
    if err != nil && !errors.IsAlreadyExists(err) {
        return fmt.Errorf("failed to create PVC: %w", err)
    }

    // Apply Service
    _, err = client.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
    if err != nil && !errors.IsAlreadyExists(err) {
        return fmt.Errorf("failed to create Service: %w", err)
    }

    // Apply Deployment
    _, err = client.AppsV1().Deployments(ns).Create(ctx, dep, metav1.CreateOptions{})
    if err != nil && !errors.IsAlreadyExists(err) {
        return fmt.Errorf("failed to create Deployment: %w", err)
    }

    m.log.Success("MyService applied successfully\n")
    return nil
}

// Clean removes all Kubernetes resources for this module.
func (m *MyServiceModule) Clean(ctx context.Context) error {
    client, err := k8s.NewClient()
    if err != nil {
        return fmt.Errorf("failed to create k8s client: %w", err)
    }

    ns := m.ModuleConfig.Namespace
    del := metav1.DeleteOptions{}

    if err := client.AppsV1().Deployments(ns).Delete(ctx, "myservice", del); err != nil && !errors.IsNotFound(err) {
        return fmt.Errorf("failed to delete Deployment: %w", err)
    }
    if err := client.CoreV1().Services(ns).Delete(ctx, "myservice", del); err != nil && !errors.IsNotFound(err) {
        return fmt.Errorf("failed to delete Service: %w", err)
    }
    if err := client.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, "myservice-data", del); err != nil && !errors.IsNotFound(err) {
        return fmt.Errorf("failed to delete PVC: %w", err)
    }

    m.log.Success("MyService cleaned successfully\n")
    return nil
}

// Status prints the current state of all Kubernetes resources.
func (m *MyServiceModule) Status(ctx context.Context) error {
    client, err := k8s.NewClient()
    if err != nil {
        return fmt.Errorf("failed to create k8s client: %w", err)
    }

    ns := m.ModuleConfig.Namespace

    dep, err := client.AppsV1().Deployments(ns).Get(ctx, "myservice", metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get Deployment: %w", err)
    }

    m.log.Info("Deployment: %s  Ready: %d/%d\n",
        dep.Name,
        dep.Status.ReadyReplicas,
        dep.Status.Replicas,
    )
    return nil
}
```

### 4.3 Implement optional interfaces

Add optional methods to the same struct in `myservice.go` (or a separate file):

```go
// Backup implements modules.Backuper — runs `kubectl exec tar` to archive the PVC.
func (m *MyServiceModule) Backup(ctx context.Context, destDir string) error {
    // Use kubectl exec to tar the volume contents.
    // See bitwarden.go or openclaw.go for a reference implementation.
    return nil
}

// Restore implements modules.Restorer.
func (m *MyServiceModule) Restore(ctx context.Context, args []string) error {
    // args[0] is typically the backup timestamp or "latest".
    return nil
}

// Test implements modules.Tester — sends a smoke-test request to the running service.
func (m *MyServiceModule) Test(ctx context.Context) error {
    m.log.Info("Running smoke test for MyService...\n")
    // Perform a health-check or send a test payload.
    return nil
}
```

### 4.4 Register the module

Open `internal/modules/registry_default.go` and add one line inside `DefaultRegistry`:

```go
// For modules that need module-specific config (most modules):
r.Register("myservice", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
    return myservice.New(g, m, log)
})

// For modules that only need general config (e.g. namespace):
r.RegisterSimple("myservice", func(g config.GeneralConfig, log logger.Logger) Module {
    return myservice.New(g, log)
})
```

Also add the import at the top of the file:

```go
"github.com/Goalt/personal-server/internal/modules/myservice"
```

### 4.5 Add configuration

Add an entry to `config.example.yaml` so users know how to configure the module:

```yaml
modules:
  - name: myservice
    namespace: infra
    secrets:
      my_password: secret_password   # document every required secret key
```

If the module requires secrets, list each key with a comment explaining its purpose.

---

## 5. Tests

### 5.1 Unit tests

Create `internal/modules/myservice/myservice_test.go`. A complete test file includes:

| Test function | What it validates |
|---|---|
| `TestMyServiceModule_Name` | `Name()` returns the correct string |
| `TestMyServiceModule_Doc` | `Doc()` returns nil without error |
| `TestMyServiceModule_Prepare` | `prepare()` returns non-nil objects for multiple namespaces |
| `TestMyServiceModule_PreparePVC` | PVC name, labels, access mode, storage size |
| `TestMyServiceModule_PrepareService` | Service name, labels, annotations, ports, selector |
| `TestMyServiceModule_PrepareDeployment` | Deployment name, labels, replicas, selector, restart policy |
| `TestMyServiceModule_PrepareDeploymentContainer` | Container name, image, env vars, ports, volume mounts |
| `TestMyServiceModule_PrepareDeploymentVolumes` | Volume name and PVC claim name |
| `TestGenerate` | `Generate()` produces YAML files matching `testdata/` fixtures |

Example test skeleton:

```go
package myservice

import (
    "context"
    _ "embed"
    "os"
    "path/filepath"
    "testing"

    "github.com/Goalt/personal-server/internal/config"
    "github.com/Goalt/personal-server/internal/logger"
)

func TestMyServiceModule_Name(t *testing.T) {
    m := &MyServiceModule{}
    if m.Name() != "myservice" {
        t.Errorf("Name() = %s, want myservice", m.Name())
    }
}

func TestMyServiceModule_Doc(t *testing.T) {
    m := &MyServiceModule{
        GeneralConfig: config.GeneralConfig{Domain: "example.com"},
        ModuleConfig:  config.Module{Name: "myservice", Namespace: "infra"},
        log:           logger.Default(),
    }
    if err := m.Doc(context.Background()); err != nil {
        t.Errorf("Doc() returned unexpected error: %v", err)
    }
}

func TestMyServiceModule_Prepare(t *testing.T) {
    tests := []struct {
        name      string
        namespace string
    }{
        {"default namespace", "infra"},
        {"custom namespace", "myservice-ns"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := &MyServiceModule{
                GeneralConfig: config.GeneralConfig{Domain: "example.com"},
                ModuleConfig:  config.Module{Name: "myservice", Namespace: tt.namespace},
            }
            pvc, svc, dep := m.prepare()
            if pvc == nil { t.Fatal("prepare() returned nil PVC") }
            if svc == nil { t.Fatal("prepare() returned nil Service") }
            if dep == nil { t.Fatal("prepare() returned nil Deployment") }
            if pvc.Namespace != tt.namespace { t.Errorf("PVC namespace = %s, want %s", pvc.Namespace, tt.namespace) }
            if svc.Namespace != tt.namespace { t.Errorf("Service namespace = %s, want %s", svc.Namespace, tt.namespace) }
            if dep.Namespace != tt.namespace { t.Errorf("Deployment namespace = %s, want %s", dep.Namespace, tt.namespace) }
        })
    }
}

//go:embed testdata/pvc.yaml
var expectedPvcYAML string

//go:embed testdata/service.yaml
var expectedServiceYAML string

//go:embed testdata/deployment.yaml
var expectedDeploymentYAML string

func TestGenerate(t *testing.T) {
    tempDir := t.TempDir()
    origWd, _ := os.Getwd()
    if err := os.Chdir(tempDir); err != nil {
        t.Fatalf("failed to chdir: %v", err)
    }
    defer os.Chdir(origWd)

    m := &MyServiceModule{
        GeneralConfig: config.GeneralConfig{Domain: "example.com"},
        ModuleConfig:  config.Module{Name: "myservice", Namespace: "infra"},
        log:           logger.Default(),
    }

    if err := m.Generate(context.Background()); err != nil {
        t.Fatalf("Generate() failed: %v", err)
    }

    cases := []struct{ name, file, want string }{
        {"pvc",        "configs/myservice/pvc.yaml",        expectedPvcYAML},
        {"service",    "configs/myservice/service.yaml",    expectedServiceYAML},
        {"deployment", "configs/myservice/deployment.yaml", expectedDeploymentYAML},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, err := os.ReadFile(filepath.Join(tempDir, tc.file))
            if err != nil {
                t.Fatalf("failed to read %s: %v", tc.file, err)
            }
            if string(got) != tc.want {
                t.Errorf("Generated YAML does not match expected.\nGot:\n%s\n\nWant:\n%s", string(got), tc.want)
            }
        })
    }
}
```

Run the tests with:

```bash
go test ./internal/modules/myservice/...

# Or run the full test suite
make test
```

### 5.2 testdata YAML fixtures

The `testdata/` directory contains the expected YAML output that `Generate()` must produce.

**Workflow to create or update fixtures:**

1. Run `Generate()` once against a temporary directory.
2. Copy the produced YAML files into `testdata/`.
3. Verify the files look correct.
4. The `TestGenerate` test will fail if `Generate()` output ever drifts from these fixtures, catching regressions.

---

## 6. Help & Usage

The CLI help text is built automatically from the module registry — no manual edits are
needed to `app.go`. However there are two things you must do:

1. **Return the correct name from `Name()`.**
   The value returned by `Name()` is used as the CLI command name. It must be unique across
   all modules. Use lowercase and hyphens (e.g. `"my-service"`).

2. **Register the module** (see §4.4).
   Once registered, `personal-server help` and the error message shown when no subcommand
   is provided will automatically include the module and its subcommands.

To verify:

```bash
make build
./bin/personal-server help          # should list myservice
./bin/personal-server myservice     # should show available subcommands
```

---

## 7. README Updates

After implementing the module, make the following updates to `README.md`:

### 7.1 Available Modules list

Add a bullet to the **Available Modules** section (alphabetical order within the list):

```markdown
- **myservice**: Brief one-line description of what this module deploys
```

### 7.2 Configuration example

If the module requires non-obvious secrets or has optional configuration, add an example
snippet to the **Configuration** → **Modules** section:

```markdown
```yaml
modules:
  - name: myservice
    namespace: infra
    secrets:
      my_password: secret_password   # required: password for the service
      optional_key: value            # optional: controls feature X
```
```

### 7.3 Adding a New Module (developer note)

The **Adding a New Module** bullet list in the **Development** section should remain
accurate — update it if you introduce a new step to the process.

---

## 8. Checklist

Use this checklist when creating a new module:

- [ ] `internal/modules/<name>/<name>.go` created
- [ ] `Name()` returns the correct, unique CLI command name
- [ ] `Doc()` prints module description, required secrets, and available subcommands
- [ ] `Generate()` writes YAML to `configs/<name>/`
- [ ] `Apply()` creates all Kubernetes resources
- [ ] `Clean()` removes all Kubernetes resources
- [ ] `Status()` prints resource status
- [ ] Optional interfaces implemented as appropriate (`Backuper`, `Restorer`, `Tester`, …)
- [ ] Module registered in `internal/modules/registry_default.go`
- [ ] `config.example.yaml` updated with a sample entry including all required `secrets` keys
- [ ] `internal/modules/<name>/<name>_test.go` created
  - [ ] `TestXxx_Name` passes
  - [ ] `TestXxx_Doc` passes (returns nil)
  - [ ] `TestXxx_Prepare` covers multiple namespaces
  - [ ] `TestXxx_PrepareXxx` tests for each Kubernetes object (PVC, Service, Deployment, …)
  - [ ] `TestGenerate` compares output against `testdata/` fixtures
- [ ] `internal/modules/<name>/testdata/*.yaml` fixtures populated
- [ ] `make test` passes with no new failures
- [ ] `make build` succeeds
- [ ] `README.md` — **Available Modules** list updated
- [ ] `README.md` — **Configuration** example updated (if the module has required secrets)
