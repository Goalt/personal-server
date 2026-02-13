package app

import (
	"fmt"
	"strings"

	"github.com/Goalt/personal-server/internal/config"
)

func (a *App) handleConfigEditCommand(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: %s config edit <module> <key> <value>\nOnly image-related fields (e.g. image_tag, prometheus_image) can be edited", Name)
	}

	moduleName := args[0]
	key := args[1]
	value := args[2]

	if !strings.Contains(key, "image") {
		return fmt.Errorf("only image-related fields can be edited (key must contain 'image'), got: %s", key)
	}

	if err := cfg.SetModuleSecret(moduleName, key, value); err != nil {
		return fmt.Errorf("editing config: %w", err)
	}

	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	a.logger.Success("Updated module '%s': set '%s'\n", moduleName, key)
	return nil
}
