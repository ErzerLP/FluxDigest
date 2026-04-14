version: "3.9"

services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: {{APP_DATABASE_NAME}}
      POSTGRES_USER: {{APP_DATABASE_USER}}
      POSTGRES_PASSWORD: {{APP_DATABASE_PASSWORD}}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U {{APP_DATABASE_USER}} -d {{APP_DATABASE_NAME}}"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  fluxdigest-api:
    image: fluxdigest/api:latest
    env_file:
      - .env
    ports:
      - "{{APP_HTTP_PORT}}:{{APP_HTTP_PORT}}"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  __MINIFLUX_SERVICE_BLOCK__
  __HALO_SERVICE_BLOCK__

volumes:
  postgres-data:
  redis-data:
