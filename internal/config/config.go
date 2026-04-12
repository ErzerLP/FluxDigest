package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

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
	Miniflux struct {
		BaseURL   string `yaml:"base_url"`
		AuthToken string `yaml:"auth_token"`
	} `yaml:"miniflux"`
	LLM struct {
		BaseURL        string   `yaml:"base_url"`
		APIKey         string   `yaml:"api_key"`
		Model          string   `yaml:"model"`
		FallbackModels []string `yaml:"fallback_models"`
		TimeoutMS      int      `yaml:"timeout_ms"`
	} `yaml:"llm"`
	Publish struct {
		HoloEndpoint string `yaml:"holo_endpoint"`
		HoloToken    string `yaml:"holo_token"`
		Channel      string `yaml:"channel"`
		OutputDir    string `yaml:"output_dir"`
	} `yaml:"publish"`
}

// Load 加载 YAML 与环境变量，并应用最小默认值。
func Load() (*Config, error) {
	cfg := &Config{}
	cfg.HTTP.Port = 8080
	cfg.Job.Queue = "default"
	cfg.Worker.Concurrency = 10
	cfg.LLM.Model = "MiniMax-M2.7"
	cfg.LLM.FallbackModels = []string{"mimo-v2-pro"}
	cfg.LLM.TimeoutMS = 30000

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
	if fromFile.Miniflux.BaseURL != "" {
		cfg.Miniflux.BaseURL = fromFile.Miniflux.BaseURL
	}
	if fromFile.Miniflux.AuthToken != "" {
		cfg.Miniflux.AuthToken = fromFile.Miniflux.AuthToken
	}
	if fromFile.LLM.BaseURL != "" {
		cfg.LLM.BaseURL = fromFile.LLM.BaseURL
	}
	if fromFile.LLM.APIKey != "" {
		cfg.LLM.APIKey = fromFile.LLM.APIKey
	}
	if fromFile.LLM.Model != "" {
		cfg.LLM.Model = fromFile.LLM.Model
	}
	if len(fromFile.LLM.FallbackModels) > 0 {
		cfg.LLM.FallbackModels = copyStringSlice(fromFile.LLM.FallbackModels)
	}
	if fromFile.LLM.TimeoutMS > 0 {
		cfg.LLM.TimeoutMS = fromFile.LLM.TimeoutMS
	}
	if fromFile.Publish.HoloEndpoint != "" {
		cfg.Publish.HoloEndpoint = fromFile.Publish.HoloEndpoint
	}
	if fromFile.Publish.HoloToken != "" {
		cfg.Publish.HoloToken = fromFile.Publish.HoloToken
	}
	if fromFile.Publish.Channel != "" {
		cfg.Publish.Channel = fromFile.Publish.Channel
	}
	if fromFile.Publish.OutputDir != "" {
		cfg.Publish.OutputDir = fromFile.Publish.OutputDir
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
	if value := os.Getenv("APP_MINIFLUX_BASE_URL"); value != "" {
		cfg.Miniflux.BaseURL = value
	}
	if value := os.Getenv("APP_MINIFLUX_AUTH_TOKEN"); value != "" {
		cfg.Miniflux.AuthToken = value
	}
	if value := os.Getenv("APP_LLM_BASE_URL"); value != "" {
		cfg.LLM.BaseURL = value
	}
	if value := os.Getenv("APP_LLM_API_KEY"); value != "" {
		cfg.LLM.APIKey = value
	}
	if value := os.Getenv("APP_LLM_MODEL"); value != "" {
		cfg.LLM.Model = value
	}
	if value := os.Getenv("APP_LLM_FALLBACK_MODELS"); value != "" {
		cfg.LLM.FallbackModels = parseCSVStrings(value)
	}
	if value := os.Getenv("APP_LLM_TIMEOUT_MS"); value != "" {
		timeoutMS, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.LLM.TimeoutMS = timeoutMS
	}
	if value := os.Getenv("APP_PUBLISH_HOLO_ENDPOINT"); value != "" {
		cfg.Publish.HoloEndpoint = value
	}
	if value := os.Getenv("APP_PUBLISH_HOLO_TOKEN"); value != "" {
		cfg.Publish.HoloToken = value
	}
	if value := os.Getenv("APP_PUBLISH_CHANNEL"); value != "" {
		cfg.Publish.Channel = value
	}
	if value := os.Getenv("APP_PUBLISH_OUTPUT_DIR"); value != "" {
		cfg.Publish.OutputDir = value
	}

	return nil
}

func parseCSVStrings(value string) []string {
	items := strings.Split(value, ",")
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func copyStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
