.PHONY: test build run-api run-worker run-scheduler

test:
	go test ./... -v

build:
	go build ./cmd/...

run-api:
	go run ./cmd/rss-api

run-worker:
	go run ./cmd/rss-worker

run-scheduler:
	go run ./cmd/rss-scheduler