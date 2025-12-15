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
