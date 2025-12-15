package app

import (
	"io"

	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules"
)

// Option configures the App
type Option func(*App)

// WithRegistry sets a custom module registry
func WithRegistry(r *modules.Registry) Option {
	return func(a *App) {
		a.registry = r
	}
}

// WithLogger sets a custom logger
func WithLogger(log logger.Logger) Option {
	return func(a *App) {
		a.logger = log
		a.registry = modules.DefaultRegistry(log)
	}
}

// WithConfigLoader sets a custom config loader
func WithConfigLoader(loader ConfigLoader) Option {
	return func(a *App) {
		a.configLoader = loader
	}
}

// WithStdout sets a custom stdout writer
func WithStdout(w io.Writer) Option {
	return func(a *App) {
		a.stdout = w
	}
}

// WithStderr sets a custom stderr writer
func WithStderr(w io.Writer) Option {
	return func(a *App) {
		a.stderr = w
	}
}
