# 🚀 Personal Server

A comprehensive Go application for managing Kubernetes-based personal infrastructure on MicroK8s. This tool provides a unified CLI interface to deploy, manage, backup, and restore self-hosted services.

## ✨ Features

- **Modular Architecture**: Manage multiple services through a unified interface
- **Pet Projects Support**: Deploy custom containerized applications with simple configuration
- **Kubernetes Integration**: Generate and apply Kubernetes configurations for various services
- **Backup & Restore**: Automated backup and restore capabilities for critical services
- **Configuration Management**: YAML-based configuration for easy customization
- **Scheduled Backups**: Support for automated backups with configurable cron schedules
- **Encrypted Backups**: GPG encryption support for secure backup storage

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

Create a `config.yaml` file in the project root:

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
    imagePullSecret: my-registry-secret
    registryCredentials:
      server: https://registry.example.com
      username: myuser
      password: mypassword
    environment:
      PORT: "8080"
      ENV: "production"
  
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
- **monitoring**: Monitoring stack
- **postgres**: PostgreSQL database
- **pgadmin**: PostgreSQL administration interface
- **ssh-login-notifier**: SSH login notification service
- **ingress**: HTTP routing and ingress management with TLS support

### Pet Projects

Pet projects allow you to deploy custom containerized applications easily. Just define them in the `pet-projects` section of your config:

```yaml
pet-projects:
  - name: myapp
    namespace: hobby
    image: nginx:latest
    imagePullSecret: my-registry-secret # Optional: reference an existing registry secret
    registryCredentials:              # Optional: create the secret automatically
      server: https://registry.example.com
      username: myuser
      password: mypassword
    environment:
      PORT: "8080"
      ENV: "production"
    service:                # Optional: Create a Kubernetes Service
      ports:
        - name: http
          port: 80
          targetPort: 8080
```

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
│       ├── hobbypod/
│       ├── monitoring/
│       ├── namespace/
│       ├── pgadmin/
│       ├── postgres/
│       ├── sshlogin/
│       ├── webdav/
│       └── workpod/
├── docs/                  # Documentation
├── config.yaml            # Configuration file
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

Contributions are welcome! Please ensure:

1. Code follows Go best practices
2. All tests pass (`make test`)
3. Code is formatted (`make fmt`)
4. No linting errors (`make vet`)

## 📄 Version

Current version: **1.0.0**

## 🔗 Dependencies

Key dependencies include:

- **Kubernetes Client**: `k8s.io/client-go` for Kubernetes API interactions
- **WebDAV**: `github.com/emersion/go-webdav` for backup storage
- **Sentry**: `github.com/getsentry/sentry-go` for error tracking
- **YAML**: `gopkg.in/yaml.v3` for configuration parsing

See `go.mod` for the complete list of dependencies.
