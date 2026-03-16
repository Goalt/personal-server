package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules"
)

// TestPrintUsageContainsAllModules verifies that the help output dynamically
// lists all modules registered in the default registry.
func TestPrintUsageContainsAllModules(t *testing.T) {
	var buf strings.Builder
	log := logger.NewStdLogger(&buf)

	app := New(WithLogger(log))
	app.printUsage()

	output := buf.String()

	expectedModules := []string{
		"namespace",
		"cloudflare",
		"bitwarden",
		"webdav",
		"hobby-pod",
		"work-pod",
		"drone",
		"gitea",
		"grafana",
		"monitoring",
		"postgres",
		"postgres-exporter",
		"pgadmin",
		"redis",
		"prometheus",
		"ssh-login-notifier",
		"openclaw",
	}

	for _, mod := range expectedModules {
		if !strings.Contains(output, mod) {
			t.Errorf("Expected help output to contain module %q, but it was missing.\nOutput:\n%s", mod, output)
		}
	}
}

// TestPrintUsageContainsBaseSubcommands verifies that the help output shows
// the base subcommands for every module.
func TestPrintUsageContainsBaseSubcommands(t *testing.T) {
	var buf strings.Builder
	log := logger.NewStdLogger(&buf)

	app := New(WithLogger(log))
	app.printUsage()

	output := buf.String()

	baseSubcmds := []string{"generate", "apply", "clean", "status"}
	for _, sub := range baseSubcmds {
		if !strings.Contains(output, sub) {
			t.Errorf("Expected help output to contain subcommand %q, but it was missing.\nOutput:\n%s", sub, output)
		}
	}
}

// TestPrintUsageContainsOptionalSubcommands verifies that optional subcommands
// appear in the help output for modules that support them.
func TestPrintUsageContainsOptionalSubcommands(t *testing.T) {
	var buf strings.Builder
	log := logger.NewStdLogger(&buf)

	app := New(WithLogger(log))
	app.printUsage()

	output := buf.String()

	// backup/restore should appear (bitwarden, gitea, etc. support them)
	for _, sub := range []string{"backup", "restore", "add-db", "remove-db", "notify", "test", "rollout", "code-serve-web"} {
		if !strings.Contains(output, sub) {
			t.Errorf("Expected help output to contain optional subcommand %q, but it was missing.\nOutput:\n%s", sub, output)
		}
	}
}

// TestPrintUsageGlobalCommands verifies that global (non-module) commands
// are still present in the help output.
func TestPrintUsageGlobalCommands(t *testing.T) {
	var buf strings.Builder
	log := logger.NewStdLogger(&buf)

	app := New(WithLogger(log))
	app.printUsage()

	output := buf.String()

	for _, cmd := range []string{"help", "update", "config", "backup"} {
		if !strings.Contains(output, cmd) {
			t.Errorf("Expected help output to contain global command %q, but it was missing.\nOutput:\n%s", cmd, output)
		}
	}
}

// TestHelpCommandThroughRun verifies that running the "help" command (as a CLI
// argument) produces help output.
func TestHelpCommandThroughRun(t *testing.T) {
	var buf strings.Builder
	log := logger.NewStdLogger(&buf)

	app := New(WithLogger(log))
	err := app.Run(context.Background(), []string{"help"})
	if err != nil {
		t.Fatalf("Run(help) returned unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, Name) {
		t.Errorf("Expected help output to contain app name %q.\nOutput:\n%s", Name, output)
	}
	if !strings.Contains(output, "Module Commands") {
		t.Errorf("Expected help output to contain 'Module Commands' section.\nOutput:\n%s", output)
	}
}

// TestSupportedSubcommands verifies the SupportedSubcommands helper for
// a minimal mock module and a module that implements optional interfaces.
func TestSupportedSubcommands_BaseOnly(t *testing.T) {
	m := &mockModule{}
	got := modules.SupportedSubcommands(m)
	want := []string{"generate", "apply", "clean", "status"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SupportedSubcommands() = %v, want %v", got, want)
	}
}

func TestSupportedSubcommands_WithBackupRestore(t *testing.T) {
	m := &mockModuleWithBackup{}
	got := modules.SupportedSubcommands(m)
	if !contains(got, "backup") {
		t.Errorf("Expected subcommands to contain 'backup', got %v", got)
	}
	if !contains(got, "restore") {
		t.Errorf("Expected subcommands to contain 'restore', got %v", got)
	}
}

// TestRegistryGetAllWithEmptyConfig verifies that all built-in modules can be
// instantiated with empty configs and that their command names are non-empty.
func TestRegistryGetAllWithEmptyConfig(t *testing.T) {
	log := logger.NewNopLogger()
	r := modules.DefaultRegistry(log)

	entries := r.GetAllWithEmptyConfig()
	if len(entries) == 0 {
		t.Fatal("GetAllWithEmptyConfig() returned no entries")
	}

	for _, e := range entries {
		if e.CommandName == "" {
			t.Errorf("Entry has empty CommandName")
		}
		if e.Module == nil {
			t.Errorf("Entry for %q has nil Module", e.CommandName)
		}
	}

	// Verify entries are returned in sorted order
	for i := 1; i < len(entries); i++ {
		if entries[i-1].CommandName > entries[i].CommandName {
			t.Errorf("Entries not in sorted order: %q > %q", entries[i-1].CommandName, entries[i].CommandName)
		}
	}
}

// TestRegistryDescriptions verifies that descriptions are registered for all
// built-in modules.
func TestRegistryDescriptions(t *testing.T) {
	log := logger.NewNopLogger()
	r := modules.DefaultRegistry(log)

	for _, name := range r.Commands() {
		if r.GetDescription(name) == "" {
			t.Errorf("Module %q has no description registered", name)
		}
	}
}

// helpers

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// mockModule implements only the base Module interface.
type mockModule struct{}

func (m *mockModule) Name() string                          { return "mock" }
func (m *mockModule) Generate(_ context.Context) error     { return nil }
func (m *mockModule) Apply(_ context.Context) error        { return nil }
func (m *mockModule) Clean(_ context.Context) error        { return nil }
func (m *mockModule) Status(_ context.Context) error       { return nil }

// mockModuleWithBackup also implements Backuper and Restorer.
type mockModuleWithBackup struct{ mockModule }

func (m *mockModuleWithBackup) Backup(_ context.Context, _ string) error         { return nil }
func (m *mockModuleWithBackup) Restore(_ context.Context, _ []string) error      { return nil }
