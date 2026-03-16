package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules"
)

type helpTestModule struct {
	name string
}

func (m helpTestModule) Name() string                         { return m.name }
func (m helpTestModule) Generate(context.Context) error       { return nil }
func (m helpTestModule) Apply(context.Context) error          { return nil }
func (m helpTestModule) Clean(context.Context) error          { return nil }
func (m helpTestModule) Status(context.Context) error         { return nil }
func (m helpTestModule) Backup(context.Context, string) error { return nil }
func (m helpTestModule) Restore(context.Context, []string) error {
	return nil
}
func (m helpTestModule) Test(context.Context) error { return nil }

func TestPrintUsage_ListsRegisteredModulesAndSupportedSubcommands(t *testing.T) {
	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)

	registry := modules.NewRegistry(log)
	registry.Register("basic", func(g config.GeneralConfig, modCfg config.Module, log logger.Logger) modules.Module {
		return basicHelpTestModule{name: "basic"}
	})
	registry.Register("advanced", func(g config.GeneralConfig, modCfg config.Module, log logger.Logger) modules.Module {
		return helpTestModule{name: "advanced"}
	})

	app := New(WithLogger(log), WithRegistry(registry))

	app.printUsage()

	output := logBuf.String()
	if !strings.Contains(output, "  advanced <generate|apply|clean|status|backup|restore|test>") {
		t.Fatalf("expected advanced module line in help output, got:\n%s", output)
	}
	if !strings.Contains(output, "  basic <generate|apply|clean|status>") {
		t.Fatalf("expected basic module line in help output, got:\n%s", output)
	}
	if strings.Index(output, "  advanced <") > strings.Index(output, "  basic <") {
		t.Fatalf("expected module lines to be sorted alphabetically, got:\n%s", output)
	}
}

func TestRunHelp_DoesNotLoadConfig(t *testing.T) {
	var logBuf strings.Builder
	log := logger.NewStdLogger(&logBuf)

	registry := modules.NewRegistry(log)
	registry.Register("basic", func(g config.GeneralConfig, modCfg config.Module, log logger.Logger) modules.Module {
		return basicHelpTestModule{name: "basic"}
	})

	configLoaderCalled := false
	app := New(
		WithLogger(log),
		WithRegistry(registry),
		WithConfigLoader(func(path string) (*config.Config, error) {
			configLoaderCalled = true
			return nil, nil
		}),
	)

	if err := app.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatalf("Run(help) returned error: %v", err)
	}
	if configLoaderCalled {
		t.Fatal("expected help command to avoid loading config")
	}
	if !strings.Contains(logBuf.String(), "  basic <generate|apply|clean|status>") {
		t.Fatalf("expected help output to include module subcommands, got:\n%s", logBuf.String())
	}
}

func TestHandleModuleCommand_UsesModuleSupportedSubcommandsInErrors(t *testing.T) {
	app := &App{}
	module := helpTestModule{name: "advanced"}

	err := app.handleModuleCommand(context.Background(), nil, module)
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
	if !strings.Contains(err.Error(), "Available subcommands: generate, apply, clean, status, backup, restore, test") {
		t.Fatalf("expected supported subcommands in error, got: %v", err)
	}

	err = app.handleModuleCommand(context.Background(), []string{"unknown"}, module)
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "Available subcommands: generate, apply, clean, status, backup, restore, test") {
		t.Fatalf("expected supported subcommands in error, got: %v", err)
	}
}

type basicHelpTestModule struct {
	name string
}

func (m basicHelpTestModule) Name() string                   { return m.name }
func (m basicHelpTestModule) Generate(context.Context) error { return nil }
func (m basicHelpTestModule) Apply(context.Context) error    { return nil }
func (m basicHelpTestModule) Clean(context.Context) error    { return nil }
func (m basicHelpTestModule) Status(context.Context) error   { return nil }
