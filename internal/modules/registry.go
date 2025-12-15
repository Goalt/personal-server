package modules

import (
	"fmt"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

// ModuleFactory creates a module from config
type ModuleFactory func(general config.GeneralConfig, modCfg config.Module, log logger.Logger) Module

// Registry holds module factories indexed by command name
type Registry struct {
	factories map[string]ModuleFactory
	// requiresModuleConfig tracks which modules need module-specific config
	requiresModuleConfig map[string]bool
	logger               logger.Logger
}

// NewRegistry creates a new module registry with a logger
func NewRegistry(log logger.Logger) *Registry {
	return &Registry{
		factories:            make(map[string]ModuleFactory),
		requiresModuleConfig: make(map[string]bool),
		logger:               log,
	}
}

// Register adds a module factory that requires module config
func (r *Registry) Register(name string, factory ModuleFactory) {
	r.factories[name] = factory
	r.requiresModuleConfig[name] = true
}

// RegisterSimple adds a module factory that only needs general config
func (r *Registry) RegisterSimple(name string, factory func(config.GeneralConfig, logger.Logger) Module) {
	r.factories[name] = func(general config.GeneralConfig, _ config.Module, log logger.Logger) Module {
		return factory(general, log)
	}
	r.requiresModuleConfig[name] = false
}

// Get creates a module by name
func (r *Registry) Get(name string, cfg *config.Config) (Module, error) {
	factory, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("unknown module: %s", name)
	}

	var modCfg config.Module
	if r.requiresModuleConfig[name] {
		var err error
		modCfg, err = cfg.GetModule(name)
		if err != nil {
			return nil, fmt.Errorf("retrieving %s config: %w", name, err)
		}
	}

	return factory(cfg.General, modCfg, r.logger), nil
}

// Commands returns all registered command names
func (r *Registry) Commands() []string {
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// Has checks if a command is registered
func (r *Registry) Has(name string) bool {
	_, ok := r.factories[name]
	return ok
}
