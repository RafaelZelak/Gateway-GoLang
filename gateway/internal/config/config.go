package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

// ServiceConfig represents each entry in config.yml
type ServiceConfig struct {
	Route          string            `yaml:"route"`
	Target         string            `yaml:"target,omitempty"`
	TemplateDir    string            `yaml:"templateDir,omitempty"`
	TemplateRoutes map[string]string `yaml:"templateRoutes,omitempty"`
	Log            string            `yaml:"log,omitempty"`
	Auth           string            `yaml:"auth,omitempty"`
}

// Config holds all service configurations
type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
