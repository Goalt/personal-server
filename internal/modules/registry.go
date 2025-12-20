package modules

import (
	"fmt"

	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
)

// ModuleFactory creates a module from config
type ModuleFactory func(general config.GeneralConfig, modCfg config.Module, log logger.Logger) Module

// PetProjectFactory creates a pet project module from config
type PetProjectFactory func(general config.GeneralConfig, projectCfg config.PetProject, log logger.Logger) Module

// Registry holds module factories indexed by command name
type Registry struct {
	factories            map[string]ModuleFactory
	petProjectFactories  map[string]PetProjectFactory
	// requiresModuleConfig tracks which modules need module-specific config
	requiresModuleConfig map[string]bool
	logger               logger.Logger
}

// NewRegistry creates a new module registry with a logger
func NewRegistry(log logger.Logger) *Registry {
	return &Registry{
		factories:            make(map[string]ModuleFactory),
		petProjectFactories:  make(map[string]PetProjectFactory),
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

// RegisterPetProject adds a pet project factory
func (r *Registry) RegisterPetProject(name string, factory PetProjectFactory) {
	r.petProjectFactories[name] = factory
}

// Get creates a module by name
func (r *Registry) Get(name string, cfg *config.Config) (Module, error) {
	factory, ok := r.factories[name]
	if !ok {
		// Check if it's a pet project
		return r.GetPetProject(name, cfg)
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

// GetPetProject creates a pet project module by name
func (r *Registry) GetPetProject(name string, cfg *config.Config) (Module, error) {
	// Try to find a pet project with this name
	projectCfg, err := cfg.GetPetProject(name)
	if err != nil {
		return nil, fmt.Errorf("unknown module or pet project: %s", name)
	}

	// Check if there's a specific factory registered for this pet project
	if factory, ok := r.petProjectFactories[name]; ok {
		return factory(cfg.General, projectCfg, r.logger), nil
	}

	// Use the default pet project factory if no specific factory is registered
	if defaultFactory, ok := r.petProjectFactories["_default"]; ok {
		return defaultFactory(cfg.General, projectCfg, r.logger), nil
	}

	return nil, fmt.Errorf("no pet project factory registered")
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
