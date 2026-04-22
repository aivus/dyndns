package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	UpdateToken string          `yaml:"update_token"`
	Cloudflare  CloudflareConfig `yaml:"cloudflare"`
	Records     []RecordConfig  `yaml:"records"`
}

type CloudflareConfig struct {
	APIToken string `yaml:"api_token"`
}

type RecordConfig struct {
	ZoneID string `yaml:"zone_id"`
	Name   string `yaml:"name"`
	Suffix string `yaml:"suffix"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if token := os.Getenv("UPDATE_TOKEN"); token != "" {
		cfg.UpdateToken = token
	}
	if token := os.Getenv("CLOUDFLARE_API_TOKEN"); token != "" {
		cfg.Cloudflare.APIToken = token
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.UpdateToken == "" {
		return fmt.Errorf("update_token is required")
	}
	if c.Cloudflare.APIToken == "" {
		return fmt.Errorf("cloudflare.api_token is required (or set CLOUDFLARE_API_TOKEN env var)")
	}
	if len(c.Records) == 0 {
		return fmt.Errorf("at least one record must be configured under records:")
	}
	for i, r := range c.Records {
		if r.ZoneID == "" {
			return fmt.Errorf("records[%d]: zone_id is required", i)
		}
		if r.Name == "" {
			return fmt.Errorf("records[%d]: name is required", i)
		}
	}
	return nil
}
