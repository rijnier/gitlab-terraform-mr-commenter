package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	GitlabToken    string `envconfig:"GITLAB_TOKEN" required:"true"`
	GitlabURL      string `envconfig:"GITLAB_URL" default:"https://gitlab.com"`
	ProjectID      string `envconfig:"GITLAB_PROJECT_ID" required:"true"`
	MergeRequestID int    `envconfig:"GITLAB_MR_ID" required:"true"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
