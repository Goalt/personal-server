package app

import (
	"fmt"

	"github.com/Goalt/personal-server/internal/config"
)

func (a *App) handleConfigEditCommand(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: %s config edit <module> image <value>", Name)
	}

	moduleName := args[0]
	key := args[1]
	value := args[2]

	if key != "image" {
		return fmt.Errorf("only the 'image' field can be edited, got: %s", key)
	}

	if err := cfg.SetModuleImage(moduleName, value); err != nil {
		return fmt.Errorf("editing config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	a.logger.Success("Updated module '%s': set image to '%s'\n", moduleName, value)
	return nil
}
