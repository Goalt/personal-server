package modules

import "context"

// Module defines the interface that all personal-server modules must implement
type Module interface {
	// Name returns the name of the module
	Name() string

	// Generate generates the Kubernetes configurations for the module
	Generate(ctx context.Context) error

	// Apply applies the Kubernetes configurations for the module
	Apply(ctx context.Context) error

	// Clean removes the Kubernetes resources associated with the module
	Clean(ctx context.Context) error

	// Status checks and prints the status of the module's resources
	Status(ctx context.Context) error
}

// Backuper defines the interface for modules that support backup
type Backuper interface {
	Backup(ctx context.Context, destDir string) error
}

// Restorer defines the interface for modules that support restore
type Restorer interface {
	Restore(ctx context.Context, args []string) error
}

// DatabaseManager defines the interface for modules that support database management
type DatabaseManager interface {
	AddDB(ctx context.Context, args []string) error
	RemoveDB(ctx context.Context, args []string) error
}

// Tester defines the interface for modules that support testing
type Tester interface {
	Test(ctx context.Context) error
}

// Notifier defines the interface for modules that support notification
type Notifier interface {
	Notify(ctx context.Context, user, ip, sshConnection string) error
}

// Rollouter defines the interface for modules that support rollout operations
type Rollouter interface {
	Rollout(ctx context.Context, args []string) error
}

// CodeServeWebRunner defines the interface for modules that support starting code serve-web
type CodeServeWebRunner interface {
	CodeServeWeb(ctx context.Context) error
}

// SupportedSubcommands returns the list of subcommands supported by a module.
// All modules support generate, apply, clean, and status. Additional subcommands
// depend on which optional interfaces the module implements.
func SupportedSubcommands(m Module) []string {
	cmds := []string{"generate", "apply", "clean", "status"}
	if _, ok := m.(Backuper); ok {
		cmds = append(cmds, "backup")
	}
	if _, ok := m.(Restorer); ok {
		cmds = append(cmds, "restore")
	}
	if _, ok := m.(DatabaseManager); ok {
		cmds = append(cmds, "add-db", "remove-db")
	}
	if _, ok := m.(Notifier); ok {
		cmds = append(cmds, "notify")
	}
	if _, ok := m.(Tester); ok {
		cmds = append(cmds, "test")
	}
	if _, ok := m.(Rollouter); ok {
		cmds = append(cmds, "rollout")
	}
	if _, ok := m.(CodeServeWebRunner); ok {
		cmds = append(cmds, "code-serve-web")
	}
	return cmds
}
