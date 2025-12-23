package sshlogin

import (
	"context"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

func TestSSHLoginModule_Name(t *testing.T) {
	module := &SSHLoginModule{}
	if module.Name() != "ssh-login-notifier" {
		t.Errorf("Name() = %s, want ssh-login-notifier", module.Name())
	}
}

func TestGenerate(t *testing.T) {
	// SSH Login Notifier doesn't generate Kubernetes manifests
	// It installs directly on the host
	module := &SSHLoginModule{
		GeneralConfig: config.GeneralConfig{
			Domain: "example.com",
		},
		ModuleConfig: config.Module{
			Name:      "ssh-login-notifier",
			Namespace: "infra",
		},
		log: logger.Default(),
	}

	ctx := context.Background()
	err := module.Generate(ctx)

	// Generate should succeed but not create any files
	if err != nil {
		t.Errorf("Generate() unexpected error: %v", err)
	}
}
