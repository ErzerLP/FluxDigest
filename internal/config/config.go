package config

import (
    "os"
    "strconv"
)

type Config struct {
    HTTP     struct{ Port int }
    Database struct{ DSN string }
    Redis    struct{ Addr string }
}

func Load() (*Config, error) {
    cfg := &Config{}

    cfg.HTTP.Port = 8080
    if value := os.Getenv("APP_HTTP_PORT"); value != "" {
        port, err := strconv.Atoi(value)
        if err != nil {
            return nil, err
        }
        cfg.HTTP.Port = port
    }

    cfg.Database.DSN = os.Getenv("DATABASE_DSN")
    cfg.Redis.Addr = os.Getenv("REDIS_ADDR")

    return cfg, nil
}