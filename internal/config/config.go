package config

import (
	"errors"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP struct {
		Port int `yaml:"port"`
	} `yaml:"http"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Redis struct {
		Addr string `yaml:"addr"`
	} `yaml:"redis"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	cfg.HTTP.Port = 8080

	if err := loadFromYAML(cfg); err != nil {
		return nil, err
	}
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadFromYAML(cfg *Config) error {
	data, err := os.ReadFile("configs/config.yaml")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	fromFile := &Config{}
	if err := yaml.Unmarshal(data, fromFile); err != nil {
		return err
	}

	if fromFile.HTTP.Port != 0 {
		cfg.HTTP.Port = fromFile.HTTP.Port
	}
	if fromFile.Database.DSN != "" {
		cfg.Database.DSN = fromFile.Database.DSN
	}
	if fromFile.Redis.Addr != "" {
		cfg.Redis.Addr = fromFile.Redis.Addr
	}

	return nil
}

func applyEnvOverrides(cfg *Config) error {
	if value := os.Getenv("APP_HTTP_PORT"); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.HTTP.Port = port
	}

	if value := os.Getenv("APP_DATABASE_DSN"); value != "" {
		cfg.Database.DSN = value
	}
	if value := os.Getenv("APP_REDIS_ADDR"); value != "" {
		cfg.Redis.Addr = value
	}

	return nil
}
