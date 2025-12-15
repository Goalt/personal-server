# 🚀 Personal Server

A comprehensive Go application for managing Kubernetes-based personal infrastructure on MicroK8s. This tool provides a unified CLI interface to deploy, manage, backup, and restore self-hosted services.

## ✨ Features

- **Modular Architecture**: Manage multiple services through a unified interface
- **Kubernetes Integration**: Generate and apply Kubernetes configurations for various services
- **Backup & Restore**: Automated backup and restore capabilities for critical services
- **Configuration Management**: YAML-based configuration for easy customization
- **Scheduled Backups**: Support for automated backups with configurable cron schedules
- **Encrypted Backups**: GPG encryption support for secure backup storage

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
# Run all tests
make test

# Run tests with coverage report
make coverage

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

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
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
| `test` | Run tests |
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
