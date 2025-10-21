package config

import (
	"fmt"
	"strconv"

	"github.com/kelseyhightower/envconfig"
	"gitlab-terraform-mr-commenter/internal/constants"
)

type Config struct {
	GitlabToken    string `envconfig:"GITLAB_TOKEN" required:"true"`
	GitlabURL      string `envconfig:"GITLAB_URL" default:"https://gitlab.com"`
	ProjectID      string `envconfig:"GITLAB_PROJECT_ID" required:"true"`
	MergeRequestID string `envconfig:"GITLAB_MR_ID" required:"true"`
}

func (c *Config) ValidateMRID() error {
	_, err := strconv.Atoi(c.MergeRequestID)
	if err != nil {
		return fmt.Errorf("invalid MergeRequestID '%s': must be a valid integer", c.MergeRequestID)
	}
	return nil
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("gitlab_tfmr", &cfg); err != nil {
		return nil, fmt.Errorf(constants.ErrConfigLoad, err)
	}
	if err := cfg.ValidateMRID(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
