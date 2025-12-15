package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules"
	"gopkg.in/yaml.v2"
)

const (
	Name        = "personal-server"
	Description = "A personal server application for Kubernetes infrastructure management"
)

// Version is the version of the application, set at build time via ldflags
var Version = "dev"

// ConfigLoader loads configuration from a file path
type ConfigLoader func(path string) (*config.Config, error)

// App is the main application with injectable dependencies
type App struct {
	registry     *modules.Registry
	configLoader ConfigLoader
	stdout       io.Writer
	stderr       io.Writer
	logger       logger.Logger
}

// New creates a new App with default dependencies
func New(opts ...Option) *App {
	log := logger.Default()
	app := &App{
		registry:     modules.DefaultRegistry(log),
		configLoader: config.LoadConfig,
		stdout:       os.Stdout,
		stderr:       os.Stderr,
		logger:       log,
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}

// Run executes the CLI application
func (a *App) Run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet(Name, flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	// Config file flag
	var configFile string
	fs.StringVar(&configFile, "config", "config.yaml", "Path to configuration file")
	fs.StringVar(&configFile, "c", "config.yaml", "Path to configuration file (shorthand)")

	var (
		help    = fs.Bool("help", false, "Show help information")
		h       = fs.Bool("h", false, "Show help information (shorthand)")
		version = fs.Bool("version", false, "Show version information")
		v       = fs.Bool("v", false, "Show version information (shorthand)")
	)

	fs.Usage = func() { a.printUsage() }
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Handle help flags
	if *help || *h {
		a.printUsage()
		return nil
	}

	// Handle version flags
	if *version || *v {
		a.printVersion()
		return nil
	}

	// Check for help command
	cmdArgs := fs.Args()
	if len(cmdArgs) > 0 && cmdArgs[0] == "help" {
		a.printUsage()
		return nil
	}

	// Main application logic
	if len(cmdArgs) == 0 {
		a.printUsage()
		return nil
	}

	cfg, err := a.configLoader(configFile)
	if err != nil {
		return fmt.Errorf("loading config %s: %w", configFile, err)
	}

	// Handle commands
	cmd := cmdArgs[0]

	// Handle config command separately (not a module)
	if cmd == "config" {
		return a.handleConfigCommand(cfg)
	}

	// Handle global backup command
	if cmd == "backup" {
		// potential subcommand
		if len(cmdArgs) > 1 && cmdArgs[1] == "schedule" {
			// Check for "clear" subcommand
			if len(cmdArgs) > 2 && cmdArgs[2] == "clear" {
				return a.handleBackupScheduleClear(ctx)
			}

			return a.handleBackupSchedule(ctx, cfg)
		}

		// Handle "backup download <file>" subcommand
		if len(cmdArgs) > 1 && cmdArgs[1] == "download" {
			if len(cmdArgs) < 3 {
				return fmt.Errorf("download requires a file name argument")
			}
			return a.handleBackupDownload(ctx, cfg, cmdArgs[2])
		}

		backupCmd := flag.NewFlagSet("backup", flag.ExitOnError)
		passphrase := backupCmd.String("passphrase", "", "Passphrase for GPG decryption")
		decrypt := backupCmd.String("decrypt", "", "Path to archive to decrypt")

		if err := backupCmd.Parse(cmdArgs[1:]); err != nil {
			return err
		}

		if *decrypt != "" {
			if *passphrase == "" {
				return fmt.Errorf("passphrase is required for decryption")
			}
			return a.handleGlobalDecryptCommand(ctx, *decrypt, *passphrase)
		}

		return a.handleGlobalBackupCommand(ctx, cfg)

	}

	// Use registry for module commands
	module, err := a.registry.Get(cmd, cfg)
	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}

	return a.handleModuleCommand(ctx, cmdArgs[1:], module)
}

func (a *App) handleModuleCommand(ctx context.Context, args []string, module modules.Module) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s <subcommand>\nAvailable subcommands: generate, apply, clean, status, backup, restore, add-db, remove-db, notify, test", module.Name())
	}

	subcommand := args[0]

	switch subcommand {
	case "generate":
		return module.Generate(ctx)
	case "apply":
		return module.Apply(ctx)
	case "clean":
		return module.Clean(ctx)
	case "status":
		return module.Status(ctx)
	case "backup":
		if backuper, ok := module.(modules.Backuper); ok {
			return backuper.Backup(ctx, "")
		}
		return fmt.Errorf("module '%s' does not support backup", module.Name())
	case "restore":
		if restorer, ok := module.(modules.Restorer); ok {
			return restorer.Restore(ctx, args[1:])
		}
		return fmt.Errorf("module '%s' does not support restore", module.Name())
	case "add-db":
		if dbManager, ok := module.(modules.DatabaseManager); ok {
			return dbManager.AddDB(ctx, args[1:])
		}
		return fmt.Errorf("module '%s' does not support add-db", module.Name())
	case "remove-db":
		if dbManager, ok := module.(modules.DatabaseManager); ok {
			return dbManager.RemoveDB(ctx, args[1:])
		}
		return fmt.Errorf("module '%s' does not support remove-db", module.Name())
	case "notify":
		// Special case for ssh-login-notifier notify command
		// Expected args: [user, ip, ssh_connection]
		if len(args) < 4 {
			return fmt.Errorf("notify requires 3 arguments: user, ip, ssh_connection")
		}
		if notifier, ok := module.(modules.Notifier); ok {
			return notifier.Notify(ctx, args[1], args[2], args[3])
		}
		return fmt.Errorf("module '%s' does not support notify", module.Name())
	case "test":
		if tester, ok := module.(modules.Tester); ok {
			return tester.Test(ctx)
		}
		return fmt.Errorf("module '%s' does not support test", module.Name())
	default:
		return fmt.Errorf("unknown subcommand: %s\nAvailable subcommands: generate, apply, clean, status, backup, restore, add-db, remove-db, notify, test", subcommand)
	}
}

func (a *App) printUsage() {
	a.logger.Info("%s - %s\n\n", Name, Description)
	a.logger.Info("Version: %s\n\n", Version)
	a.logger.Println("Usage:")
	a.logger.Info("  %s [OPTIONS] <command> [subcommand]\n\n", Name)

	a.logger.Println("Options:")
	a.logger.Println("  -c, --config   Path to configuration file (default: config.yaml)")
	a.logger.Println("  -h, --help     Show this help message")
	a.logger.Println("  -v, --version  Show version information")

	a.logger.Println("\nCommands:")
	a.logger.Println("  help                          Show help information")
	a.logger.Println("  config                        Parse and print loaded configuration")
	a.logger.Println("  backup                        Trigger a global backup including all modules")
	a.logger.Println("  backup download <file>        Download a backup archive from WebDAV")
	a.logger.Println("  namespace <subcommand>        Manage Kubernetes namespace configurations")
	a.logger.Println("  cloudflare <subcommand>       Manage Cloudflare tunnel configurations")
	a.logger.Println("  bitwarden <subcommand>        Manage Bitwarden configurations")
	a.logger.Println("  webdav <subcommand>           Manage WebDAV configurations")
	a.logger.Println("  hobby-pod <subcommand>        Manage hobby-pod configurations")
	a.logger.Println("  work-pod <subcommand>         Generate work-pod configurations to configs/work-pod/")
	a.logger.Println("  drone <subcommand>            Manage Drone configurations")
	a.logger.Println("  gitea <subcommand>            Manage Gitea configurations")
	a.logger.Println("  monitoring <subcommand>       Manage Monitoring configurations")
	a.logger.Println("  postgres <subcommand>         Manage Postgres configurations")
	a.logger.Println("  pgadmin <subcommand>          Manage pgadmin configurations")
	a.logger.Println("  ssh-login-notifier <subcommand>  Manage SSH login notification configurations")
}

func (a *App) printVersion() {
	a.logger.Info("%s version %s\n", Name, Version)
}

func (a *App) handleConfigCommand(cfg *config.Config) error {
	a.logger.Info("Configuration loaded from: %s\n\n", cfg.Path)
	a.logger.Info("Domain: %s\n", cfg.General.Domain)
	a.logger.Info("Namespaces: %v\n", cfg.General.Namespaces)

	a.logger.Println("\nFull configuration:")
	a.logger.Println("---")
	output, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config to YAML: %w", err)
	}
	a.logger.Print("%s", string(output))
	return nil
}
