package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/configexample"
	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules"
)

func TestHandleConfigExampleCommand_PrintsExampleConfig(t *testing.T) {
	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	app := &App{logger: log}

	err := app.handleConfigExampleCommand()
	if err != nil {
		t.Fatalf("handleConfigExampleCommand() returned unexpected error: %v", err)
	}

	output := logBuf.String()
	if output != configexample.Content {
		t.Errorf("handleConfigExampleCommand() output mismatch.\nGot:\n%s\nWant:\n%s", output, configexample.Content)
	}
}

func TestRunConfigExample_DoesNotLoadConfig(t *testing.T) {
	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)
	registry := modules.NewRegistry(log)

	configLoaderCalled := false
	app := New(
		WithLogger(log),
		WithRegistry(registry),
		WithConfigLoader(func(path string) (*config.Config, error) {
			configLoaderCalled = true
			return nil, nil
		}),
	)

	if err := app.Run(context.Background(), []string{"config", "example"}); err != nil {
		t.Fatalf("Run(config example) returned error: %v", err)
	}
	if configLoaderCalled {
		t.Fatal("expected 'config example' command to avoid loading config")
	}

	output := logBuf.String()
	if !strings.Contains(output, "general:") {
		t.Errorf("expected output to contain 'general:', got:\n%s", output)
	}
	if !strings.Contains(output, "domain:") {
		t.Errorf("expected output to contain 'domain:', got:\n%s", output)
	}
}
