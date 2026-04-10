package main

import (
	"fmt"
	"log"

	"rss-platform/internal/app/api"
	"rss-platform/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	if err := api.NewRouter().Run(addr); err != nil {
		log.Fatalf("run api: %v", err)
	}
}
