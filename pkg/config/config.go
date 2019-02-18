package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// https://github.com/docker/compose/blob/master/compose/config/config_schema_v2.1.json
type Config struct {
	ComposeYaml struct{
		Version  string `yaml:"version"`
		Services map[string]struct{
			Build struct{
				Context string `yaml:"context"`
				Dockerfile string `yaml:"dockerfile"`
			} `yaml:"build"`
			Image string `yaml:"image"`
			Volumes []string `yaml:"volumes"`
			WorkingDir string `yaml:"working_dir"`
		} `yaml:"services"`
		Volumes map[string]interface{} `yaml:"volumes"`
	}
}

func New() (*Config, error) {
	cfg := &Config{}

	data, err := ioutil.ReadFile("docker-compose.yml")
	if err != nil {
		return nil, err
	}
	// "^[a-zA-Z0-9._-]+$"
	err = yaml.Unmarshal(data, &cfg.ComposeYaml)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
