package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Module represents a module configuration
type Module struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Secrets   map[string]string `yaml:"secrets"`
}

// ServicePort represents a service port configuration
type ServicePort struct {
	Name       string `yaml:"name"`
	Port       int32  `yaml:"port"`
	TargetPort int32  `yaml:"targetPort"`
}

// ServiceConfig represents service configuration for a pet project
type ServiceConfig struct {
	Ports []ServicePort `yaml:"ports"`
}

// PetProject represents a pet project configuration
type PetProject struct {
	Name            string            `yaml:"name"`
	Namespace       string            `yaml:"namespace"`
	Image           string            `yaml:"image"`
	ImagePullSecret string            `yaml:"imagePullSecret,omitempty"`
	Environment     map[string]string `yaml:"environment"`
	Service         *ServiceConfig    `yaml:"service,omitempty"`
}

type GeneralConfig struct {
	Domain     string   `yaml:"domain"`
	Namespaces []string `yaml:"namespaces"`
}

// BackupConfig represents the backup configuration
type BackupConfig struct {
	WebdavHost     string `yaml:"webdav_host"`
	WebdavUsername string `yaml:"webdav_username"`
	WebdavPassword string `yaml:"webdav_password"`
	SentryDSN      string `yaml:"sentry_dsn"`
	Cron           string `yaml:"cron"`
	Passphrase     string `yaml:"passphrase"`
}

// Config represents the application configuration
type Config struct {
	Path        string        `yaml:"-"`
	General     GeneralConfig `yaml:"general"`
	Backup      BackupConfig  `yaml:"backup"`
	Modules     []Module      `yaml:"modules"`
	PetProjects []PetProject  `yaml:"pet-projects"`
}

// LoadConfig loads and parses the configuration file
func LoadConfig(configFile string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configFile)
	}

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Parse YAML
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML config: %v", err)
	}

	config.Path = configFile

	return &config, nil
}

// GetModule retrieves a module by name
func (c *Config) GetModule(name string) (Module, error) {
	for _, module := range c.Modules {
		if module.Name == name {
			return module, nil
		}
	}
	return Module{}, fmt.Errorf("module not found: %s", name)
}

// GetPetProject retrieves a pet project by name
func (c *Config) GetPetProject(name string) (PetProject, error) {
	for _, project := range c.PetProjects {
		if project.Name == name {
			return project, nil
		}
	}
	return PetProject{}, fmt.Errorf("pet project not found: %s", name)
}
