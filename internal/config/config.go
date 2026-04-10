package config

import (
	"errors"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config 描述应用运行所需的最小配置。
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
	Job struct {
		APIKey string `yaml:"api_key"`
		Queue  string `yaml:"queue"`
	} `yaml:"job"`
	Worker struct {
		Concurrency int `yaml:"concurrency"`
	} `yaml:"worker"`
}

// Load 加载 YAML 与环境变量，并应用最小默认值。
func Load() (*Config, error) {
	cfg := &Config{}
	cfg.HTTP.Port = 8080
	cfg.Job.Queue = "default"
	cfg.Worker.Concurrency = 10

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
	if fromFile.Job.APIKey != "" {
		cfg.Job.APIKey = fromFile.Job.APIKey
	}
	if fromFile.Job.Queue != "" {
		cfg.Job.Queue = fromFile.Job.Queue
	}
	if fromFile.Worker.Concurrency != 0 {
		cfg.Worker.Concurrency = fromFile.Worker.Concurrency
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
	if value := os.Getenv("APP_JOB_API_KEY"); value != "" {
		cfg.Job.APIKey = value
	}
	if value := os.Getenv("APP_JOB_QUEUE"); value != "" {
		cfg.Job.Queue = value
	}
	if value := os.Getenv("APP_WORKER_CONCURRENCY"); value != "" {
		concurrency, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Worker.Concurrency = concurrency
	}

	return nil
}
