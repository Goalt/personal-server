# 🚀 Personal Server

[![Go Version](https://img.shields.io/github/go-mod/go-version/Goalt/personal-server)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/Goalt/personal-server)](https://github.com/Goalt/personal-server/releases/latest)
[![License](https://img.shields.io/github/license/Goalt/personal-server)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Goalt/personal-server)](https://goreportcard.com/report/github.com/Goalt/personal-server)

A comprehensive Go application for managing Kubernetes-based personal infrastructure on MicroK8s. This tool provides a unified CLI interface to deploy, manage, backup, and restore self-hosted services.

## 📑 Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [System Setup](#️-system-setup)
- [Prerequisites](#-prerequisites)
- [Installation](#-installation)
- [Configuration](#️-configuration)
- [Usage](#-usage)
- [Development](#-development)
- [Testing](#-testing)
- [Contributing](#-contributing)
- [Troubleshooting](#-troubleshooting)
- [FAQ](#-faq)
- [Support](#-support)
- [License](#-license)

> [!NOTE]
> **For AI agents and contributors writing new modules**, see the step-by-step authoring guide in [AGENTS.md](AGENTS.md).

## ✨ Features

- **Modular Architecture**: Manage multiple services through a unified interface
- **Pet Projects Support**: Deploy custom containerized applications with simple configuration
- **Kubernetes Integration**: Generate and apply Kubernetes configurations for various services
- **Backup & Restore**: Automated backup and restore capabilities for critical services
- **Configuration Management**: YAML-based configuration for easy customization
- **Scheduled Backups**: Support for automated backups with configurable cron schedules
- **Encrypted Backups**: GPG encryption support for secure backup storage

## 🚀 Quick Start

Get started with Personal Server in 5 minutes:

```bash
# 1. Download the latest release for your platform
curl -L -o personal-server https://github.com/Goalt/personal-server/releases/latest/download/personal-server-linux-amd64
chmod +x personal-server
sudo mv personal-server /usr/local/bin/

# 2. Create a configuration file
cp config.example.yaml config.yaml
# Edit config.yaml with your settings (domain, modules, secrets)

# 3. Verify your configuration
personal-server config

# 4. Deploy your first service (example: namespace)
personal-server namespace generate
personal-server namespace apply

# 5. Check the status
personal-server namespace status
```

**Next Steps:**
- Review the [Configuration](#️-configuration) section to customize your setup
- Explore [Usage](#-usage) to learn about available commands
- Check [Available Modules](#available-modules) to see what you can deploy

## 🖥️ System Setup

This section covers the initial setup of a fresh Ubuntu system for running Personal Server with MicroK8s.

### 1. Initial System Configuration

```bash
# Disable swap (required for Kubernetes)
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# Update system and install snapd
sudo apt update && sudo apt upgrade -y
sudo apt install snapd git gpg make -y
```

### 2. Install MicroK8s

```bash
# Install MicroK8s
sudo snap install microk8s --classic

# Add user to microk8s group
sudo usermod -a -G microk8s $USER
sudo chown -f -R $USER ~/.kube
newgrp microk8s

# Enable required add-ons
microk8s enable dns dashboard storage ingress registry:size=20Gi

# Create kubectl alias for convenience
echo 'alias kubectl="microk8s kubectl"' >> ~/.bashrc
source ~/.bashrc
```

### 3. Install VS Code CLI (Remote Tunnels)

The VS Code CLI allows you to connect to your server remotely via secure tunnels. Follow the installation steps from the [official documentation](https://code.visualstudio.com/docs/remote/tunnels):

```bash
# Download and install the VS Code CLI
curl -L 'https://code.visualstudio.com/sha/download?build=stable&os=cli-linux-x64' --output vscode_cli.tar.gz
tar -xf vscode_cli.tar.gz

# Move to a directory in your PATH
sudo mv code /usr/local/bin/

# Start a tunnel (you'll need to authenticate with GitHub)
code tunnel
```

### 4. Configure Security (Optional but Recommended)

```bash
# Configure firewall (only SSH access)
# WARNING: These commands will briefly reset firewall rules. 
# Ensure you have console access in case SSH is interrupted.
sudo ufw --force reset
sudo iptables -F && sudo iptables -X
sudo ufw allow 22
sudo ufw --force enable
```

## 📋 Prerequisites

- Go 1.25.3 or later
- Kubernetes cluster (MicroK8s recommended)
- kubectl configured to access your cluster
- WebDAV server (for backup storage)

## 🔧 Installation

### Download Pre-built Binaries

The easiest way to install Personal Server is to download a pre-built binary from the [GitHub Releases](https://github.com/Goalt/personal-server/releases/latest) page.

#### Linux

```bash
# Download the binary for your architecture
# For AMD64 (x86_64)
curl -L -o personal-server https://github.com/Goalt/personal-server/releases/latest/download/personal-server-linux-amd64

# For ARM64
curl -L -o personal-server https://github.com/Goalt/personal-server/releases/latest/download/personal-server-linux-arm64

# Make it executable
chmod +x personal-server

# Optionally, move to a directory in your PATH
sudo mv personal-server /usr/local/bin/

# Verify the installation
personal-server --version
```

#### macOS

```bash
# Download the binary for your architecture
# For Intel Macs (AMD64)
curl -L -o personal-server https://github.com/Goalt/personal-server/releases/latest/download/personal-server-darwin-amd64

# For Apple Silicon Macs (ARM64)
curl -L -o personal-server https://github.com/Goalt/personal-server/releases/latest/download/personal-server-darwin-arm64

# Make it executable
chmod +x personal-server

# Move to a directory in your PATH
sudo mv personal-server /usr/local/bin/

# Verify the installation
personal-server --version
```

#### Windows

```powershell
# Download using PowerShell
Invoke-WebRequest -Uri "https://github.com/Goalt/personal-server/releases/latest/download/personal-server-windows-amd64.exe" -OutFile "personal-server.exe"

# Move to a directory in your PATH (optional)
# Or run directly from the current directory
.\personal-server.exe --version
```

#### Verify Download (Optional)

Each release includes SHA256 checksums. To verify your download:

```bash
# Download the checksum file
curl -L -O https://github.com/Goalt/personal-server/releases/latest/download/personal-server-linux-amd64.sha256

# Verify the checksum (Linux/macOS)
sha256sum -c personal-server-linux-amd64.sha256

# Or manually compare
sha256sum personal-server-linux-amd64
cat personal-server-linux-amd64.sha256
```

### From Source

```bash
# Clone the repository
git clone https://github.com/Goalt/personal-server.git
cd personal-server

# Download dependencies
make deps

# Build the binary
make build

# The binary will be available in ./bin/personal-server
```

### Using Make

```bash
# Install directly to $GOPATH/bin
make install
```

## ⚙️ Configuration

Create a `config.yaml` file based on `config.example.yaml`:

```yaml
general:
  domain: example.com
  namespaces: [infra, hobby]

backup:
  webdav_host: https://webdav.example.com
  webdav_username: username
  webdav_password: password
  sentry_dsn: your_sentry_dsn
  cron: "*/30 * * * *"  # Every 30 minutes
  passphrase: your_gpg_passphrase

# Optional: define named registry credentials used by pet-projects
registries:
  my-registry:
    server: https://registry.example.com
    username: myuser
    password: mypassword
    namespace: hobby  # Kubernetes namespace where the secret is created

modules:
  - name: cloudflare
    namespace: infra
    secrets:
      cloudflare_api_token: your_token

  - name: bitwarden
    namespace: infra

  - name: webdav
    namespace: infra
    secrets:
      webdav_username: username
      webdav_password: password

  - name: gitea
    namespace: infra
    secrets:
      gitea_db_user: gitea
      gitea_db_password: gitea

  - name: postgres
    namespace: infra
    secrets:
      admin_postgres_user: postgres
      admin_postgres_password: postgres

  - name: pgadmin
    namespace: infra
    secrets:
      pgadmin_default_email: admin@example.com
      pgadmin_admin_password: password

  - name: drone
    namespace: infra
    secrets:
      drone_gitea_client_id: client_id
      drone_gitea_client_secret: client_secret
      drone_rpc_secret: rpc_secret
      drone_server_proto: https

  - name: grafana
    namespace: infra
    secrets:
      grafana_admin_user: admin
      grafana_admin_password: password

  - name: monitoring
    namespace: infra
    secrets:
      sentry_dsn: your_sentry_dsn

  - name: ssh-login-notifier
    namespace: infra
    secrets:
      sentry_dsn: your_sentry_dsn

pet-projects:
  - name: myapp
    namespace: hobby
    image: nginx:latest
    registry: my-registry  # Reference a key from the top-level registries section
    environment:
      PORT: "8080"
      ENV: "production"
    prometheusPort: 8080  # Port for Prometheus scraping (default: 8080)
    service:
      ports:
        - name: http
          port: 80
          targetPort: 8080

  - name: api-service
    namespace: hobby
    image: node:18-alpine
    environment:
      NODE_ENV: "development"
      API_PORT: "3000"
```

## 🚀 Usage

### Basic Commands

```bash
# Show help
personal-server --help

# Show version
personal-server --version

# Validate configuration
personal-server config
```

### Module Operations

Each module supports the following subcommands:

```bash
# Generate Kubernetes configurations
personal-server <module> generate

# Apply configurations to cluster
personal-server <module> apply

# Check module status
personal-server <module> status

# Clean up module resources
personal-server <module> clean

# Backup module data (if supported)
personal-server <module> backup

# Restore module data (if supported)
personal-server <module> restore <backup-file>

# Rollout operations (if supported)
personal-server <module> rollout <restart|status|history|undo>
```

### Available Modules

- **namespace**: Manage Kubernetes namespace configurations
- **cloudflare**: Cloudflare tunnel management
- **bitwarden**: Password manager deployment
- **webdav**: WebDAV server management
- **hobby-pod**: Personal hobby development pod
- **work-pod**: Work development pod
- **drone**: CI/CD server (Drone CI)
- **gitea**: Git hosting server
- **grafana**: Grafana observability dashboard
- **monitoring**: Monitoring stack
- **postgres**: PostgreSQL database
- **postgres-exporter**: PostgreSQL metrics exporter for Prometheus
- **pgadmin**: PostgreSQL administration interface
- **redis**: Redis in-memory data store
- **prometheus**: Prometheus monitoring and metrics collection
- **openclaw**: OpenClaw application deployment
- **ssh-login-notifier**: SSH login notification service
- **registry**: Kubernetes docker-registry secret management for configured registries
- **ingress**: HTTP routing and ingress management with TLS support, plus TCP/UDP service exposure

### Pet Projects

Pet projects allow you to deploy custom containerized applications easily. Just define them in the `pet-projects` section of your config:

```yaml
# Optional: define named registry credentials at the top level
registries:
  my-registry:
    server: https://registry.example.com
    username: myuser
    password: mypassword
    namespace: hobby  # Kubernetes namespace where the secret is created

pet-projects:
  - name: myapp
    namespace: hobby
    image: nginx:latest
    registry: my-registry  # Optional: reference a key from the top-level registries section
    environment:
      PORT: "8080"
      ENV: "production"
    prometheusPort: 8080    # Optional: port for Prometheus scraping (default: 8080)
    service:                # Optional: Create a Kubernetes Service
      ports:
        - name: http
          port: 80
          targetPort: 8080
```

The `registry` field references a named entry from the top-level `registries` section. The corresponding Kubernetes docker-registry secret is created by the `registry` command.

The `service` attribute is optional. When provided, it creates a Kubernetes Service that exposes your application on the specified ports. Each port requires:
- `name`: A descriptive name for the port (e.g., "http", "https")
- `port`: The port on which the service will be exposed
- `targetPort`: The port your container is listening on

Then use the same commands as modules:

```bash
# Generate Kubernetes configurations
personal-server myapp generate

# Deploy the pet project
personal-server myapp apply

# Check status
personal-server myapp status

# Rollout operations
personal-server myapp rollout restart  # Restart the deployment
personal-server myapp rollout status   # Check rollout status
personal-server myapp rollout history  # View rollout history
personal-server myapp rollout undo     # Undo last rollout

# Clean up
personal-server myapp clean
```

### Ingress & HTTP Routing

The ingress module allows you to configure HTTP routing rules to expose your services externally with optional TLS/HTTPS support.

#### Configuration

Define ingress rules in your `config.yaml`:

```yaml
ingresses:
  - name: web-ingress
    namespace: infra
    rules:
      - host: gitea.example.com
        path: /
        pathType: Prefix        # Prefix, Exact, or ImplementationSpecific
        serviceName: gitea
        servicePort: 3000
      - host: bitwarden.example.com
        path: /
        pathType: Prefix
        serviceName: bitwarden
        servicePort: 80
    tls: true                   # Enable TLS/HTTPS
```

#### Path Types

- **Prefix**: Matches the beginning of the path (default, most common)
- **Exact**: Matches the exact path only
- **ImplementationSpecific**: Implementation-specific matching (depends on ingress controller)

#### Multiple Paths on Same Host

You can define multiple paths for the same hostname:

```yaml
ingresses:
  - name: api-ingress
    namespace: infra
    rules:
      - host: api.example.com
        path: /v1
        pathType: Prefix
        serviceName: api-service-v1
        servicePort: 8080
      - host: api.example.com
        path: /v2
        pathType: Prefix
        serviceName: api-service-v2
        servicePort: 8080
```

#### Commands

```bash
# Generate ingress configuration
personal-server web-ingress generate

# Apply ingress to cluster
personal-server web-ingress apply

# Check ingress status
personal-server web-ingress status

# Clean up ingress
personal-server web-ingress clean
```

#### TCP and UDP Services

For non-HTTP protocols (like databases, SSH, VPN), you can expose TCP and UDP services through the ingress controller using ConfigMaps:

```yaml
ingresses:
  - name: tcp-udp-services
    namespace: infra
    tcpServices:
      - port: 5432           # External port exposed by ingress
        serviceName: postgres
        servicePort: 5432    # Internal service port
      - port: 6379
        serviceName: redis
        servicePort: 6379
      - port: 2222           # Custom SSH port
        serviceName: gitea
        servicePort: 22
        namespace: infra     # Optional: defaults to ingress namespace
    udpServices:
      - port: 1194           # OpenVPN UDP port
        serviceName: openvpn
        servicePort: 1194
```

**Note:** TCP/UDP services are exposed via ConfigMaps that configure the ingress controller. Make sure your ingress controller (e.g., nginx-ingress) is configured to watch these ConfigMaps. For nginx-ingress, you may need to configure the controller with:
- `--tcp-services-configmap=<namespace>/<configmap-name>-tcp`
- `--udp-services-configmap=<namespace>/<configmap-name>-udp`

#### Setting Up TLS/HTTPS

To enable HTTPS for your services, you need to set up TLS certificates. There are two main approaches:

##### Option 1: Using cert-manager (Recommended)

cert-manager automates certificate management and renewal using Let's Encrypt or other certificate authorities.

**1. Install cert-manager on MicroK8s:**

```bash
# Enable cert-manager addon
microk8s enable cert-manager

# Or install manually with kubectl
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

**2. Create a ClusterIssuer for Let's Encrypt:**

```bash
# Create a production Let's Encrypt issuer
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: public
EOF
```

**3. Annotate your Ingress to use cert-manager:**

After generating your ingress configuration with `personal-server <ingress-name> generate`, edit the generated YAML file to add cert-manager annotations:

```bash
# Edit the generated ingress file
nano configs/ingress/web-ingress/ingress.yaml
```

Add these annotations under `metadata`:

```yaml
metadata:
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  name: web-ingress
  namespace: infra
```

Then apply the ingress:

```bash
kubectl apply -f configs/ingress/web-ingress/ingress.yaml
```

cert-manager will automatically create and manage the TLS secret (`web-ingress-tls` in this example).

**4. Verify certificate creation:**

```bash
# Check certificate status
kubectl get certificate -n infra

# Check the certificate details
kubectl describe certificate web-ingress-tls -n infra
```

##### Option 2: Manual TLS Secret Creation

If you have your own certificates, create a TLS secret manually:

```bash
# Create TLS secret from certificate files
kubectl create secret tls web-ingress-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n infra

# Or from a combined PEM file
kubectl create secret tls web-ingress-tls \
  --cert=path/to/fullchain.pem \
  --key=path/to/privkey.pem \
  -n infra
```

The secret name must match the one referenced in your ingress configuration (e.g., `web-ingress-tls` for an ingress named `web-ingress`).

##### Option 3: Using Cloudflare for TLS

If you're using Cloudflare, you can enable "Full (strict)" SSL/TLS mode and use Cloudflare Origin Certificates:

1. Generate an origin certificate in the Cloudflare dashboard
2. Create a Kubernetes secret with the certificate:

```bash
kubectl create secret tls web-ingress-tls \
  --cert=origin.crt \
  --key=origin.key \
  -n infra
```

3. Ensure your Cloudflare SSL/TLS mode is set to "Full (strict)"

#### Troubleshooting TLS

**Certificate not issued:**
```bash
# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Check certificate request status
kubectl get certificaterequest -n infra
kubectl describe certificaterequest <request-name> -n infra
```

**DNS not resolving:**
- Ensure your DNS records point to your cluster's ingress controller IP
- For MicroK8s: `kubectl get svc -n ingress` to find the ingress controller service
- Update your DNS A/AAAA records to point to this IP

**Port 80/443 not accessible:**
- Check firewall rules: `sudo ufw status`
- Allow HTTP/HTTPS: `sudo ufw allow 80/tcp && sudo ufw allow 443/tcp`

### Examples

```bash
# Deploy Bitwarden
personal-server bitwarden generate
personal-server bitwarden apply
personal-server bitwarden status

# Deploy Prometheus monitoring
personal-server prometheus generate
personal-server prometheus apply
personal-server prometheus status

# Backup Gitea data
personal-server gitea backup

# Manage PostgreSQL databases
personal-server postgres add-db myapp
personal-server postgres remove-db myapp

# Global backup (all modules)
personal-server backup

# Schedule automated backups
personal-server backup schedule

# Clear backup schedule
personal-server backup schedule clear

# Decrypt a backup
personal-server backup --decrypt backup.tar.gz.gpg --passphrase your_passphrase
```

### Prometheus Monitoring

The Prometheus module deploys a complete Prometheus monitoring stack for your Kubernetes cluster.

#### Features

- **Service Discovery**: Automatically discovers Kubernetes services, pods, and nodes
- **Metrics Collection**: Collects metrics from Kubernetes API server, nodes, pods, and services
- **Persistent Storage**: Configurable persistent volume for metric data (default: 10Gi)
- **RBAC**: Proper service account and cluster role for Kubernetes API access
- **Health Checks**: Liveness and readiness probes for reliability
- **Configurable**: Customize Prometheus version and storage size

#### Quick Start

```bash
# Add prometheus to your config.yaml
modules:
  - name: prometheus
    namespace: infra
    # Optional: Customize settings
    # secrets:
    #   prometheus_image: prom/prometheus:v2.48.0  # Customize version
    #   storage_size: 20Gi                         # Customize storage

# Generate and apply Prometheus
personal-server prometheus generate
personal-server prometheus apply

# Check status
personal-server prometheus status

# Restart deployment (e.g., after config changes)
personal-server prometheus rollout restart
```

#### Accessing Prometheus UI

The Prometheus UI is exposed on port 9090 via a ClusterIP service. To access it:

**Option 1: Port Forward**
```bash
kubectl port-forward -n infra svc/prometheus 9090:9090
```
Then open http://localhost:9090 in your browser.

**Option 2: Ingress**
Add an ingress rule to expose Prometheus externally:
```yaml
ingresses:
  - name: monitoring-ingress
    namespace: infra
    rules:
      - host: prometheus.example.com
        path: /
        pathType: Prefix
        serviceName: prometheus
        servicePort: 9090
    tls: true
```

#### Scraping Custom Metrics

To expose metrics from your services for Prometheus to scrape:

**For Pods:**
Add these annotations to your pod:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

**For Services:**
Add these annotations to your service:
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

#### Default Scrape Configs

The Prometheus deployment includes these scrape configurations:
- **prometheus**: Self-monitoring
- **kubernetes-apiservers**: Kubernetes API server metrics
- **kubernetes-nodes**: Node metrics (kubelet)
- **kubernetes-pods**: Pod metrics (with prometheus.io/scrape annotation)
- **kubernetes-service-endpoints**: Service endpoint metrics

#### Configuration

The Prometheus configuration is stored in a ConfigMap (`prometheus-config`). To customize:

1. Generate the configuration: `personal-server prometheus generate`
2. Edit `configs/prometheus/configmap.yaml`
3. Apply the updated config: `kubectl apply -f configs/prometheus/configmap.yaml`
4. Restart Prometheus: `personal-server prometheus rollout restart`

## 🔨 Development

### Building

```bash
# Format code
make fmt

# Run linter
make vet

# Run tests
make test

# Run tests with coverage
make coverage

# Full development cycle (format, vet, test, build)
make dev
```

### Project Structure

```
personal-server/
├── cmd/                    # Application entry point
│   └── main.go
├── internal/               # Internal packages
│   ├── app/               # Application logic and CLI
│   ├── config/            # Configuration management
│   ├── k8s/               # Kubernetes utilities
│   ├── logger/            # Logging utilities
│   └── modules/           # Service modules
│       ├── bitwarden/
│       ├── cloudflare/
│       ├── drone/
│       ├── gitea/
│       ├── grafana/
│       ├── hobbypod/
│       ├── ingress/
│       ├── monitoring/
│       ├── namespace/
│       ├── openclaw/
│       ├── petproject/
│       ├── pgadmin/
│       ├── postgres/
│       ├── postgresexporter/
│       ├── prometheus/
│       ├── redis/
│       ├── registrysecret/
│       ├── sshlogin/
│       ├── webdav/
│       └── workpod/
├── docs/                  # Documentation
├── test/                  # Test suites
│   └── e2e/              # End-to-end tests
├── config.example.yaml    # Example configuration file
├── Makefile              # Build automation
└── go.mod                # Go module definition
```

### Running Tests

```bash
# Run all unit tests
make test

# Run tests with coverage report
make coverage

# Run e2e tests (requires Kubernetes cluster)
make e2e-test

# Run specific module tests
go test ./internal/modules/gitea/...
```

### Adding a New Module

See **[AGENTS.md](AGENTS.md)** for a complete step-by-step guide with code examples, test patterns, and a checklist covering every integration point.

Quick summary:

1. Create a new directory in `internal/modules/<module-name>/`
2. Implement the `Module` interface
3. Optionally implement `Backuper`, `Restorer`, `DatabaseManager`, `Tester`, or `Notifier` interfaces
4. Register the module in `internal/modules/registry_default.go`
5. Add configuration to `config.yaml`
6. Write tests in `<module-name>_test.go`

## 🧪 Testing

The project includes both unit tests and end-to-end (e2e) tests.

### Unit Tests

```bash
# Run all unit tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### E2E Tests

E2E tests validate the complete functionality of the CLI against a real Kubernetes cluster.

```bash
# Run e2e tests (requires a running Kubernetes cluster)
make e2e-test

# Or run directly
cd test/e2e
go test -v -timeout 10m
```

For more information about e2e tests, see [test/e2e/README.md](test/e2e/README.md).

#### Setting up a local test cluster with KinD

```bash
# Install KinD
go install sigs.k8s.io/kind@latest

# Create a test cluster
kind create cluster --name e2e-test

# Run e2e tests
make e2e-test

# Clean up
kind delete cluster --name e2e-test
```

## 🔐 Security

- Backup data is encrypted using GPG with a configurable passphrase
- Secrets are stored in Kubernetes secrets
- Sentry integration for error monitoring and alerting
- SSH login notifications for security monitoring

## 📝 Makefile Targets

| Target | Description |
|--------|-------------|
| `all` | Clean, download deps, test, and build |
| `build` | Build the binary |
| `build-linux` | Cross-compile for Linux |
| `clean` | Remove build artifacts |
| `test` | Run unit tests |
| `e2e-test` | Run e2e tests (requires Kubernetes cluster) |
| `coverage` | Generate test coverage report |
| `deps` | Download and tidy dependencies |
| `fmt` | Format code |
| `vet` | Run go vet |
| `run` | Run the application |
| `install` | Build and install the binary |
| `dev` | Format, vet, test, and build |
| `help` | Show available targets |

## 🤝 Contributing

Contributions are welcome! We appreciate your help in making Personal Server better.

### How to Contribute

1. **Fork the repository** and create your branch from `main`
2. **Make your changes** following the project's coding standards
3. **Write or update tests** for your changes
4. **Run the test suite** to ensure everything passes
5. **Update documentation** if you're changing functionality
6. **Submit a pull request** with a clear description of your changes

### Development Guidelines

Before submitting a pull request, please ensure:

1. **Code Quality**:
   - Code follows Go best practices and idioms
   - All tests pass: `make test`
   - Code is formatted: `make fmt`
   - No linting errors: `make vet`
   - Run the full dev cycle: `make dev`

2. **Testing**:
   - Write unit tests for new functionality
   - Ensure test coverage doesn't decrease
   - Run e2e tests if applicable: `make e2e-test`

3. **Documentation**:
   - Update README.md for new features
   - Add inline code comments for complex logic
   - Update config.example.yaml if adding new configuration options

4. **Commit Messages**:
   - Use clear and descriptive commit messages
   - Reference issue numbers when applicable
   - Follow conventional commit format (optional but preferred)

### Reporting Issues

Found a bug or have a feature request? Please:

1. Check if the issue already exists in [GitHub Issues](https://github.com/Goalt/personal-server/issues)
2. If not, create a new issue with:
   - Clear title and description
   - Steps to reproduce (for bugs)
   - Expected vs actual behavior
   - Your environment details (OS, Go version, etc.)
   - Relevant logs or error messages

### Code of Conduct

Be respectful and constructive in all interactions. We're all here to learn and improve together.

## 📄 Version

Current version: **1.0.0**

## 🔗 Dependencies

Key dependencies include:

- **Kubernetes Client**: `k8s.io/client-go` for Kubernetes API interactions
- **WebDAV**: `github.com/emersion/go-webdav` for backup storage
- **Sentry**: `github.com/getsentry/sentry-go` for error tracking
- **YAML**: `gopkg.in/yaml.v3` for configuration parsing

See `go.mod` for the complete list of dependencies.

## 🔧 Troubleshooting

### Common Issues and Solutions

#### Build Errors

**Issue**: `go: cannot find main module`
```bash
# Solution: Ensure you're in the project directory
cd /path/to/personal-server
make build
```

**Issue**: Dependency download fails
```bash
# Solution: Clear module cache and retry
go clean -modcache
make deps
```

#### Kubernetes Connection Issues

**Issue**: `Unable to connect to Kubernetes cluster`
```bash
# Solution 1: Check if MicroK8s is running
microk8s status

# Solution 2: Verify kubectl configuration
kubectl cluster-info

# Solution 3: For MicroK8s, ensure you're in the microk8s group
sudo usermod -a -G microk8s $USER
newgrp microk8s
```

#### Module Deployment Issues

**Issue**: Module fails to deploy
```bash
# Check the module status and logs
personal-server <module-name> status
kubectl describe pod -n <namespace> -l app=<module-name>
kubectl logs -n <namespace> -l app=<module-name>

# Try regenerating and reapplying
personal-server <module-name> generate
personal-server <module-name> apply
```

**Issue**: Image pull errors
```bash
# Verify registry credentials are correct
kubectl get secret -n <namespace>
kubectl describe secret <secret-name> -n <namespace>

# For MicroK8s registry issues
microk8s enable registry:size=20Gi
```

#### Backup and Restore Issues

**Issue**: Backup fails with WebDAV errors
```bash
# Verify WebDAV credentials and connectivity
curl -u username:password https://webdav.example.com/

# Check the backup configuration in config.yaml
personal-server config
```

**Issue**: GPG decryption fails
```bash
# Ensure the correct passphrase is used
personal-server backup --decrypt backup.tar.gz.gpg --passphrase your_passphrase

# Check GPG installation
gpg --version
```

#### Permission Issues

**Issue**: Permission denied when accessing Kubernetes
```bash
# Ensure proper permissions on kubeconfig
chmod 600 ~/.kube/config

# For MicroK8s, check group membership
groups | grep microk8s
```

### Getting More Help

If you encounter an issue not listed here:
1. Check the [FAQ](#-faq) section below
2. Search [existing issues](https://github.com/Goalt/personal-server/issues)
3. Review Kubernetes pod logs: `kubectl logs -n <namespace> <pod-name>`
4. Enable verbose logging in your application
5. See [Support](#-support) section for ways to get help

## ❓ FAQ

### General Questions

**Q: What is Personal Server?**  
A: Personal Server is a Go-based CLI tool for managing self-hosted services on Kubernetes (MicroK8s). It simplifies deployment, backup, and management of personal infrastructure.

**Q: Do I need to know Kubernetes to use this?**  
A: Basic familiarity with Kubernetes concepts helps, but isn't required. The tool abstracts away most complexity. The [System Setup](#️-system-setup) section guides you through the prerequisites.

**Q: Can I use this with cloud Kubernetes services (EKS, GKE, AKS)?**  
A: While designed for MicroK8s, it should work with any Kubernetes cluster. You may need to adjust ingress and storage configurations for your specific environment.

**Q: Is this production-ready?**  
A: This tool is designed for personal infrastructure and hobby projects. For production workloads, consider enterprise-grade solutions with professional support.

### Configuration Questions

**Q: Where should I store my config.yaml file?**  
A: By default, Personal Server looks for `config.yaml` in the current directory. You can also specify a custom location using command-line flags (check `personal-server --help`).

**Q: How do I secure sensitive data in config.yaml?**  
A: Never commit config.yaml with real secrets to version control. Use environment variables or external secret management tools. The tool creates Kubernetes secrets from the configuration values.

**Q: Can I use multiple configuration files?**  
A: Currently, the tool uses a single config.yaml file. You can maintain different files for different environments and specify which to use when running commands.

### Deployment Questions

**Q: Which modules should I deploy first?**  
A: Start with `namespace` to create your Kubernetes namespaces, then deploy foundational services like `postgres` before deploying applications that depend on them.

**Q: How do I update a deployed module?**  
A: Update your config.yaml, regenerate the module configuration with `personal-server <module> generate`, then apply with `personal-server <module> apply`. Use `personal-server <module> rollout restart` to restart the deployment.

**Q: Can I customize the generated Kubernetes manifests?**  
A: Yes! After running `personal-server <module> generate`, you can edit the generated YAML files in the `configs/` directory before applying them.

### Backup Questions

**Q: What data is backed up?**  
A: Each module defines what data is backed up. Typically includes databases, configuration files, and persistent volumes. Check individual module documentation for details.

**Q: How do I restore from a backup?**  
A: Use `personal-server <module> restore <backup-file>` to restore a specific module, or follow the backup documentation for full system restoration.

**Q: Are backups encrypted by default?**  
A: Yes, when you configure a GPG passphrase in your config.yaml, backups are encrypted using GPG.

### Development Questions

**Q: How do I add a new module?**  
A: See **[AGENTS.md](AGENTS.md)** for a complete step-by-step guide with code examples, test patterns, and a checklist covering every integration point.

**Q: How do I run tests?**  
A: Run `make test` for unit tests or `make e2e-test` for end-to-end tests (requires a Kubernetes cluster). See the [Testing](#-testing) section for more details.

**Q: What Go version is required?**  
A: Go 1.25.3 or later. Check `go.mod` for the exact version requirement.

## 💬 Support

### Getting Help

If you need assistance:

1. **Documentation**: Start with this README and the [docs/](docs/) directory
2. **GitHub Issues**: Search or create an issue at [github.com/Goalt/personal-server/issues](https://github.com/Goalt/personal-server/issues)
3. **GitHub Discussions**: For questions and community support, use [GitHub Discussions](https://github.com/Goalt/personal-server/discussions) (if enabled)

### Reporting Security Vulnerabilities

If you discover a security vulnerability, please **do not** open a public issue. Instead:
- Email the maintainer directly (see GitHub profile)
- Provide detailed information about the vulnerability
- Allow reasonable time for a fix before public disclosure

### Community

- **Repository**: [github.com/Goalt/personal-server](https://github.com/Goalt/personal-server)
- **Author**: [@Goalt](https://github.com/Goalt)
- **Issues**: [github.com/Goalt/personal-server/issues](https://github.com/Goalt/personal-server/issues)

## 📄 License

This project is open source. Please check the repository for license information.

For the most up-to-date licensing terms, see the [LICENSE](LICENSE) file in the repository or visit [github.com/Goalt/personal-server](https://github.com/Goalt/personal-server).

---

**Made with ❤️ for the self-hosting community**
