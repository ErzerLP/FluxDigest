services:
  postgres:
    image: ${POSTGRES_IMAGE}
    restart: unless-stopped
    env_file:
      - .env
    environment:
      POSTGRES_DB: ${POSTGRES_DEFAULT_DB}
      POSTGRES_USER: ${POSTGRES_ROOT_USER}
      POSTGRES_PASSWORD: ${POSTGRES_ROOT_PASSWORD}
    ports:
      - "35432:5432"
    volumes:
      - {{STACK_POSTGRES_DATA_DIR}}:/var/lib/postgresql/data
      - {{STACK_INITDB_DIR}}:/docker-entrypoint-initdb.d:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_ROOT_USER} -d ${POSTGRES_DEFAULT_DB}"]
      interval: 10s
      timeout: 5s
      retries: 12

  redis:
    image: ${REDIS_IMAGE}
    restart: unless-stopped
    ports:
      - "36379:6379"
    volumes:
      - {{STACK_REDIS_DATA_DIR}}:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 12

  fluxdigest-api:
    build:
      context: {{STACK_SOURCE_ROOT}}
      dockerfile: {{STACK_SOURCE_ROOT}}/deployments/docker/api.Dockerfile
      network: host
      args:
        http_proxy: ${http_proxy}
        https_proxy: ${https_proxy}
        HTTP_PROXY: ${HTTP_PROXY}
        HTTPS_PROXY: ${HTTPS_PROXY}
        GOPROXY: ${GOPROXY}
        GOSUMDB: ${GOSUMDB}
    image: ${FLUXDIGEST_API_IMAGE}
    restart: unless-stopped
    env_file:
      - .env
    ports:
      - "18088:18088"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  fluxdigest-worker:
    build:
      context: {{STACK_SOURCE_ROOT}}
      dockerfile: {{STACK_SOURCE_ROOT}}/deployments/docker/worker.Dockerfile
      network: host
      args:
        http_proxy: ${http_proxy}
        https_proxy: ${https_proxy}
        HTTP_PROXY: ${HTTP_PROXY}
        HTTPS_PROXY: ${HTTPS_PROXY}
        GOPROXY: ${GOPROXY}
        GOSUMDB: ${GOSUMDB}
    image: ${FLUXDIGEST_WORKER_IMAGE}
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - {{STACK_FLUXDIGEST_OUTPUT_DIR}}:/app/data/output
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  fluxdigest-scheduler:
    build:
      context: {{STACK_SOURCE_ROOT}}
      dockerfile: {{STACK_SOURCE_ROOT}}/deployments/docker/scheduler.Dockerfile
      network: host
      args:
        http_proxy: ${http_proxy}
        https_proxy: ${https_proxy}
        HTTP_PROXY: ${HTTP_PROXY}
        HTTPS_PROXY: ${HTTPS_PROXY}
        GOPROXY: ${GOPROXY}
        GOSUMDB: ${GOSUMDB}
    image: ${FLUXDIGEST_SCHEDULER_IMAGE}
    restart: unless-stopped
    env_file:
      - .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

__MINIFLUX_SERVICE_BLOCK__
__HALO_SERVICE_BLOCK__
