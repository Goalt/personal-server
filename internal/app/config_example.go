package app

import "github.com/Goalt/personal-server/internal/configexample"

func (a *App) handleConfigExampleCommand() error {
	a.logger.Print("%s", configexample.Content)
	return nil
}
