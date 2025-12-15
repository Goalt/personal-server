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
	Path    string        `yaml:"-"`
	General GeneralConfig `yaml:"general"`
	Backup  BackupConfig  `yaml:"backup"`
	Modules []Module      `yaml:"modules"`
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
