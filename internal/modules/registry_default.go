package modules

import (
	"github.com/Goalt/personal-server/internal/config"
	"github.com/Goalt/personal-server/internal/logger"
	"github.com/Goalt/personal-server/internal/modules/bitwarden"
	"github.com/Goalt/personal-server/internal/modules/cloudflare"
	"github.com/Goalt/personal-server/internal/modules/drone"
	"github.com/Goalt/personal-server/internal/modules/gitea"
	"github.com/Goalt/personal-server/internal/modules/hobbypod"
	"github.com/Goalt/personal-server/internal/modules/monitoring"
	"github.com/Goalt/personal-server/internal/modules/namespace"
	"github.com/Goalt/personal-server/internal/modules/petproject"
	"github.com/Goalt/personal-server/internal/modules/pgadmin"
	"github.com/Goalt/personal-server/internal/modules/postgres"
	"github.com/Goalt/personal-server/internal/modules/sshlogin"
	"github.com/Goalt/personal-server/internal/modules/webdav"
	"github.com/Goalt/personal-server/internal/modules/workpod"
)

// DefaultRegistry returns a registry with all built-in modules
func DefaultRegistry(log logger.Logger) *Registry {
	r := NewRegistry(log)

	// Modules that only need general config
	r.RegisterSimple("namespace", func(g config.GeneralConfig, log logger.Logger) Module {
		return namespace.New(g, log)
	})

	// Modules that need module-specific config
	r.Register("cloudflare", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return cloudflare.New(g, m, log)
	})
	r.Register("bitwarden", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return bitwarden.New(g, m, log)
	})
	r.Register("webdav", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return webdav.New(g, m, log)
	})
	r.Register("hobby-pod", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return hobbypod.New(g, m, log)
	})
	r.Register("work-pod", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return workpod.New(g, m, log)
	})
	r.Register("drone", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return drone.New(g, m, log)
	})
	r.Register("gitea", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return gitea.New(g, m, log)
	})
	r.Register("monitoring", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return monitoring.New(g, m, log)
	})
	r.Register("postgres", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return postgres.New(g, m, log)
	})
	r.Register("pgadmin", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return pgadmin.New(g, m, log)
	})
	r.Register("ssh-login-notifier", func(g config.GeneralConfig, m config.Module, log logger.Logger) Module {
		return sshlogin.New(g, m, log)
	})

	// Register default pet project factory
	r.RegisterPetProject("_default", func(g config.GeneralConfig, p config.PetProject, log logger.Logger) Module {
		return petproject.New(g, p, log)
	})

	return r
}
