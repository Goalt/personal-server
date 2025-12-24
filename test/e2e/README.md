# E2E Tests for Personal Server

This directory contains end-to-end (e2e) tests for the personal-server application.

## Overview

The e2e tests validate the complete functionality of the personal-server CLI by running it against a real Kubernetes cluster. These tests go beyond unit tests by ensuring that the application works correctly in a realistic environment.

## Current Tests

### Namespace E2E Test

Tests the complete lifecycle of the `namespace` subcommand:

1. **Generate**: Creates Kubernetes namespace YAML configurations
2. **Apply**: Applies the configurations to the cluster
3. **Status**: Checks the status of deployed namespaces
4. **Clean**: Removes namespaces from the cluster

## Prerequisites

- Go 1.25.3 or later
- A running Kubernetes cluster (local or remote)
- `kubectl` configured to access the cluster
- The `personal-server` binary built (run `make build` from repository root)

### Setting up a local Kubernetes cluster with KinD

For local testing, you can use [KinD (Kubernetes in Docker)](https://kind.sigs.k8s.io/):

```bash
# Install KinD (if not already installed)
go install sigs.k8s.io/kind@latest

# Create a KinD cluster
kind create cluster --name e2e-test

# Verify cluster is running
kubectl cluster-info
kubectl get nodes
```

## Running E2E Tests

### Using Make

From the repository root:

```bash
# Build the binary and run e2e tests
make e2e-test
```

### Using Go directly

```bash
# From repository root
cd test/e2e
go test -v -timeout 10m -run TestNamespaceE2E
```

### Skipping E2E tests during regular test runs

E2E tests are automatically skipped when running tests with the `-short` flag:

```bash
go test -short ./...
```

## GitHub Actions

E2E tests run automatically on GitHub Actions for every push and pull request to the `main` branch. The workflow:

1. Sets up a KinD cluster using `helm/kind-action`
2. Builds the personal-server binary
3. Runs the e2e tests against the KinD cluster
4. Cleans up resources

See `.github/workflows/e2e-tests.yml` for the full workflow configuration.

## Configuration

The e2e tests use a test-specific configuration file (`test-config.yaml`) that defines:

- Test namespaces to create
- Domain configuration
- Other settings required for testing

## Cleanup

The e2e tests automatically clean up after themselves by:

1. Deleting any created Kubernetes namespaces
2. Removing generated configuration files

If a test fails or is interrupted, you may need to manually clean up:

```bash
# Delete test namespaces
kubectl delete namespace e2e-test-infra e2e-test-hobby

# Remove generated configs
rm -rf configs/namespace
```

## Troubleshooting

### Test fails with "binary not found"

Make sure to build the binary first:

```bash
make build
```

### Test fails with "failed to build kubeconfig"

Ensure you have a valid kubeconfig and `kubectl` is configured:

```bash
kubectl cluster-info
```

### Test fails with "namespace already exists"

Clean up from a previous test run:

```bash
kubectl delete namespace e2e-test-infra e2e-test-hobby
```

## Adding New E2E Tests

To add new e2e tests:

1. Create a new test file in this directory (e.g., `module_name_test.go`)
2. Follow the pattern used in `namespace_test.go`:
   - Skip if running in short mode
   - Create Kubernetes client
   - Set up cleanup with `defer`
   - Run test scenarios
3. Update this README with information about the new test
4. Consider adding the test to the GitHub Actions workflow if appropriate

## Best Practices

- Always use unique resource names to avoid conflicts
- Clean up resources in a `defer` statement
- Use timeouts to prevent tests from hanging indefinitely
- Log important steps for easier debugging
- Verify both success and failure scenarios where appropriate
