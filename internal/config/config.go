package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for kz-bridge.
type Config struct {
	Karakeep    KarakeepConfig    `yaml:"karakeep"`
	Zotero      ZoteroConfig      `yaml:"zotero"`
	Translator  TranslatorConfig  `yaml:"translator"`
	DryRun      bool              `yaml:"dry_run"`
	Since       string            `yaml:"since"`
	Collection  string            `yaml:"collection"`
	IncludeAITags bool            `yaml:"include_ai_tags"`
}

type KarakeepConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type ZoteroConfig struct {
	UserID string `yaml:"user_id"`
	APIKey string `yaml:"api_key"`
}

type TranslatorConfig struct {
	URL string `yaml:"url"`
}

// Load reads config from the given path, or the default location.
func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("finding home dir: %w", err)
		}
		path = filepath.Join(home, ".config", "kz-bridge", "config.yaml")
	}

	cfg := &Config{
		IncludeAITags: true,
		Translator: TranslatorConfig{
			URL: "http://localhost:1969",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
