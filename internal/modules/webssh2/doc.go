// Package webssh2 implements a module for deploying WebSSH2, a web-based SSH client.
//
// WebSSH2 provides an HTML5 web-based terminal emulator and SSH client that runs in the browser.
// It uses the billchurch/webssh2 Docker image from GitHub Container Registry.
//
// Features:
//   - Web-based SSH client accessible through a browser
//   - Multiple authentication methods (password, publickey, keyboard-interactive)
//   - Configurable header text for branding
//   - Optional default SSH host configuration
//   - Responsive design that works on desktop and mobile devices
//
// Configuration Example:
//
//	modules:
//	  - name: webssh2
//	    namespace: infra
//	    secrets:
//	      header_text: "My WebSSH2"                        # Custom header text
//	      ssh_host: "ssh.example.com"                      # Default SSH host
//	      auth_allowed: "password,publickey"               # Allowed auth methods
//	      image: "ghcr.io/billchurch/webssh2:latest"      # Docker image
//
// The module creates the following Kubernetes resources:
//   - ConfigMap: webssh2-config (environment variables)
//   - Service: webssh2 (ClusterIP on port 2222)
//   - Deployment: webssh2 (1 replica with health probes)
//
// Access WebSSH2 through an ingress or port-forward to the service:
//
//	kubectl port-forward -n infra svc/webssh2 2222:2222
//
// Then navigate to: http://localhost:2222/ssh
//
// For more information about WebSSH2, visit: https://github.com/billchurch/webssh2
package webssh2
