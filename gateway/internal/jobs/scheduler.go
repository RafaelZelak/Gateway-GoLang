package jobs

import (
	"os"

	"gopkg.in/yaml.v3"
)

// JobConfig represents a single job entry in jobs.yml
type JobConfig struct {
	Job    string `yaml:"job"`
	Target string `yaml:"target"`
	Cron   string `yaml:"cron"`
}

// JobFile aggregates all jobs from jobs.yml
type JobFile struct {
	Jobs []JobConfig `yaml:"jobs"`
}

// LoadJobConfig reads and unmarshals the jobs YAML file
func LoadJobConfig(path string) ([]JobConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg JobFile
	err = yaml.Unmarshal(data, &cfg)
	return cfg.Jobs, err
}
