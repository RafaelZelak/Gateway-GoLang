package config

import (
	"fmt"
	"net/url"
	"os"

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

// LoadConfig reads, parses and validates the YAML configuration file
func LoadConfig(path string) (*Config, error) {
	// read file using os.ReadFile (deprecated ioutil.ReadFile removed)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// validate each service entry
	for i, svc := range cfg.Services {
		if svc.Route == "" {
			return nil, fmt.Errorf("service %d: route is required", i)
		}
		// require at least one of Target or TemplateDir
		if svc.Target == "" && svc.TemplateDir == "" {
			return nil, fmt.Errorf("service %q: either target or templateDir must be specified", svc.Route)
		}
		// if Target is set, ensure it's a valid URL
		if svc.Target != "" {
			if _, err := url.ParseRequestURI(svc.Target); err != nil {
				return nil, fmt.Errorf("service %q: invalid target URL %q: %v", svc.Route, svc.Target, err)
			}
		}
	}

	return &cfg, nil
}
