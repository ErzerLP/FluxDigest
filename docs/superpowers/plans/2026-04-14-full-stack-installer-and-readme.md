# Full Docker Stack Installer and README Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Linux-first Docker Compose one-command installer for FluxDigest with random credentials, Miniflux/Halo auto-wiring, and a standard open-source README/doc set that matches the real installer.

**Architecture:** Keep application bootstrap changes small in Go, then add a dedicated `deploy/stack/` Bash installer with render/bootstrap/healthcheck helpers and shell smoke tests. Validate the generated stack on a real Linux host before rewriting README and deployment docs around the verified Docker workflow.

**Tech Stack:** Go, GORM, Bash, Docker Compose, PostgreSQL, Redis, Miniflux, Halo, React WebUI, Markdown docs

---

## File Map

### Create
- `deploy/stack/install.sh` — Docker stack installer entrypoint.
- `deploy/stack/stack.env.tpl` — generated stack `.env` template.
- `deploy/stack/docker-compose.yml.tpl` — generated compose template.
- `deploy/stack/initdb/01-init-app-dbs.sql.tpl` — PostgreSQL init SQL for FluxDigest / Miniflux / Halo DBs.
- `deploy/stack/scripts/common.sh` — logging, arg helpers, profile helpers, random credential helpers.
- `deploy/stack/scripts/render.sh` — render `.env`, compose, SQL, and install summary.
- `deploy/stack/scripts/healthcheck.sh` — wait loops for Postgres / Redis / HTTP endpoints.
- `deploy/stack/scripts/bootstrap_miniflux.sh` — create and verify Miniflux API key.
- `deploy/stack/scripts/bootstrap_halo.sh` — create and verify Halo PAT.
- `deploy/stack/tests/render_smoke.sh` — shell smoke test for rendered files.
- `deploy/stack/tests/install_mocked_smoke.sh` — shell smoke test for installer orchestration with mocked docker.
- `deploy/stack/tests/bootstrap_miniflux_smoke.sh` — mock-server test for Miniflux bootstrap.
- `deploy/stack/tests/bootstrap_halo_smoke.sh` — mock-server test for Halo bootstrap.
- `docs/deployment/docker-stack.md` — Docker stack install / reinstall / upgrade / uninstall guide.
- `docs/deployment/runtime-configuration.md` — WebUI-driven Miniflux / Halo / LLM / prompts configuration guide.

### Modify
- `internal/config/config.go` — support admin bootstrap username/password config.
- `internal/config/config_test.go` — config load coverage for bootstrap credentials.
- `internal/service/admin_user_service.go` — bootstrap admin from env-backed config with fallback.
- `internal/service/admin_user_service_test.go` — service behavior coverage for bootstrap credentials and fallback.
- `cmd/rss-api/main.go` — pass bootstrap config into runtime bootstrap service.
- `cmd/rss-api/main_test.go` — bootstrap wiring coverage if constructor signatures change.
- `README.md` — standard project overview + quick start around Docker installer.

### Validation Commands Used Across Tasks
- `go test ./internal/config ./internal/service ./cmd/rss-api`
- `bash deploy/stack/tests/render_smoke.sh`
- `bash deploy/stack/tests/install_mocked_smoke.sh`
- `bash deploy/stack/tests/bootstrap_miniflux_smoke.sh`
- `bash deploy/stack/tests/bootstrap_halo_smoke.sh`
- `go test ./...`
- `npm --prefix web test -- --run`

### Task 1: Bootstrap admin credentials from config/env

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/service/admin_user_service.go`
- Modify: `internal/service/admin_user_service_test.go`
- Modify: `cmd/rss-api/main.go`
- Modify: `cmd/rss-api/main_test.go`

- [ ] **Step 1: Write the failing config and service tests**

```go
func TestLoadReadsAdminBootstrapEnvValues(t *testing.T) {
	t.Setenv("APP_ADMIN_BOOTSTRAP_USERNAME", "fd-admin")
	t.Setenv("APP_ADMIN_BOOTSTRAP_PASSWORD", "fd-secret")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Admin.BootstrapUsername != "fd-admin" {
		t.Fatalf("want fd-admin got %q", cfg.Admin.BootstrapUsername)
	}
	if cfg.Admin.BootstrapPassword != "fd-secret" {
		t.Fatalf("want fd-secret got %q", cfg.Admin.BootstrapPassword)
	}
}

func TestAdminUserServiceSeedDefaultsUsesBootstrapCredentials(t *testing.T) {
	repo := &adminUserRepoStub{findErr: admin.ErrNotFound}
	svc := service.NewAdminUserService(repo, service.AdminBootstrapConfig{
		Username: "fd-admin",
		Password: "fd-secret",
	})

	if err := svc.SeedDefaults(context.Background()); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	if repo.lastFindUsername != "fd-admin" {
		t.Fatalf("want lookup username fd-admin got %q", repo.lastFindUsername)
	}
	if repo.createdUser.Username != "fd-admin" {
		t.Fatalf("want created username fd-admin got %q", repo.createdUser.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.createdUser.PasswordHash), []byte("fd-secret")); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
}
```

- [ ] **Step 2: Run the focused Go tests to verify they fail**

Run: `go test ./internal/config ./internal/service ./cmd/rss-api`
Expected: FAIL with unknown fields such as `BootstrapUsername`, `BootstrapPassword`, or constructor arity mismatch for `NewAdminUserService`.

- [ ] **Step 3: Implement config loading and admin bootstrap wiring**

```go
type Config struct {
	Admin struct {
		SessionSecret     string `yaml:"session_secret"`
		BootstrapUsername string `yaml:"bootstrap_username"`
		BootstrapPassword string `yaml:"bootstrap_password"`
	} `yaml:"admin"`
}

func applyEnvOverrides(cfg *Config) error {
	if value := os.Getenv("APP_ADMIN_BOOTSTRAP_USERNAME"); value != "" {
		cfg.Admin.BootstrapUsername = value
	}
	if value := os.Getenv("APP_ADMIN_BOOTSTRAP_PASSWORD"); value != "" {
		cfg.Admin.BootstrapPassword = value
	}
	return nil
}

type AdminBootstrapConfig struct {
	Username string
	Password string
}

func NewAdminUserService(repo AdminUserRepository, bootstrap AdminBootstrapConfig) *AdminUserService {
	if strings.TrimSpace(bootstrap.Username) == "" {
		bootstrap.Username = defaultAdminUsername
	}
	if strings.TrimSpace(bootstrap.Password) == "" {
		bootstrap.Password = defaultAdminPassword
	}
	return &AdminUserService{repo: repo, bootstrap: bootstrap}
}
```

- [ ] **Step 4: Update runtime bootstrap wiring and stubs**

```go
bootstrap := service.NewRuntimeBootstrapService(
	postgresrepo.NewMigrator(sqlDB, migrationsDir),
	service.NewProfileService(postgresrepo.NewProfileRepository(db)),
	service.NewAdminUserService(postgresrepo.NewAdminUserRepository(db), service.AdminBootstrapConfig{
		Username: cfg.Admin.BootstrapUsername,
		Password: cfg.Admin.BootstrapPassword,
	}),
)

type adminUserRepoStub struct {
	findUser         admin.User
	findErr          error
	findCalls        int
	lastFindUsername string
	createCalls      int
	createdUser      admin.User
	createErr        error
}
```

- [ ] **Step 5: Run the focused Go tests again**

Run: `go test ./internal/config ./internal/service ./cmd/rss-api`
Expected: PASS

- [ ] **Step 6: Commit the bootstrap plumbing**

```bash
git add internal/config/config.go internal/config/config_test.go internal/service/admin_user_service.go internal/service/admin_user_service_test.go cmd/rss-api/main.go cmd/rss-api/main_test.go
git commit -m "feat: support bootstrap admin credentials"
```

### Task 2: Add renderable Docker stack templates and shell smoke coverage

**Files:**
- Create: `deploy/stack/scripts/common.sh`
- Create: `deploy/stack/scripts/render.sh`
- Create: `deploy/stack/stack.env.tpl`
- Create: `deploy/stack/docker-compose.yml.tpl`
- Create: `deploy/stack/initdb/01-init-app-dbs.sql.tpl`
- Create: `deploy/stack/tests/render_smoke.sh`

- [ ] **Step 1: Write the failing render smoke test**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# shellcheck disable=SC1091
source "${ROOT}/deploy/stack/scripts/common.sh"
# shellcheck disable=SC1091
source "${ROOT}/deploy/stack/scripts/render.sh"

STACK_PROFILE="fluxdigest-only"
STACK_DIR="${TMP_DIR}/stack"
STACK_TEMPLATE_DIR="${ROOT}/deploy/stack"
APP_HTTP_PORT="18088"
POSTGRES_DB_NAME="fluxdigest"
POSTGRES_DB_USER="fd_db"
POSTGRES_DB_PASSWORD="fd_db_secret"
FLUXDIGEST_ADMIN_USERNAME="fd-admin"
FLUXDIGEST_ADMIN_PASSWORD="fd-secret"
export STACK_PROFILE STACK_DIR STACK_TEMPLATE_DIR APP_HTTP_PORT POSTGRES_DB_NAME POSTGRES_DB_USER POSTGRES_DB_PASSWORD FLUXDIGEST_ADMIN_USERNAME FLUXDIGEST_ADMIN_PASSWORD

render_stack_files

grep -q 'fluxdigest-api:' "${STACK_DIR}/docker-compose.yml"
! grep -q 'miniflux:' "${STACK_DIR}/docker-compose.yml"
! grep -q 'halo:' "${STACK_DIR}/docker-compose.yml"
grep -q 'APP_ADMIN_BOOTSTRAP_USERNAME=fd-admin' "${STACK_DIR}/.env"
grep -q 'CREATE DATABASE fluxdigest;' "${STACK_DIR}/initdb/01-init-app-dbs.sql"
docker compose -f "${STACK_DIR}/docker-compose.yml" --env-file "${STACK_DIR}/.env" config >/dev/null
```

- [ ] **Step 2: Run the render smoke test to verify it fails**

Run: `bash deploy/stack/tests/render_smoke.sh`
Expected: FAIL with `render_stack_files: command not found` or missing template/script files.
- [ ] **Step 3: Implement shared helper and render functions**

```bash
#!/usr/bin/env bash
set -euo pipefail

log() {
  printf '==> %s\n' "$*"
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

profile_has_service() {
  local profile="$1"
  local service="$2"
  case "${profile}:${service}" in
    full:postgres|full:redis|full:fluxdigest|full:miniflux|full:halo) return 0 ;;
    fluxdigest-miniflux:postgres|fluxdigest-miniflux:redis|fluxdigest-miniflux:fluxdigest|fluxdigest-miniflux:miniflux) return 0 ;;
    fluxdigest-halo:postgres|fluxdigest-halo:redis|fluxdigest-halo:fluxdigest|fluxdigest-halo:halo) return 0 ;;
    fluxdigest-only:postgres|fluxdigest-only:redis|fluxdigest-only:fluxdigest) return 0 ;;
    *) return 1 ;;
  esac
}

random_token() {
  openssl rand -hex "${1:-16}"
}

render_stack_files() {
  mkdir -p "${STACK_DIR}" "${STACK_DIR}/initdb"
  sed \
    -e "s|__APP_HTTP_PORT__|${APP_HTTP_PORT}|g" \
    -e "s|__FLUXDIGEST_ADMIN_USERNAME__|${FLUXDIGEST_ADMIN_USERNAME}|g" \
    -e "s|__FLUXDIGEST_ADMIN_PASSWORD__|${FLUXDIGEST_ADMIN_PASSWORD}|g" \
    "${STACK_TEMPLATE_DIR}/stack.env.tpl" > "${STACK_DIR}/.env"
  sed \
    -e "s|__MINIFLUX_SERVICE_BLOCK__|${MINIFLUX_SERVICE_BLOCK}|g" \
    -e "s|__HALO_SERVICE_BLOCK__|${HALO_SERVICE_BLOCK}|g" \
    "${STACK_TEMPLATE_DIR}/docker-compose.yml.tpl" > "${STACK_DIR}/docker-compose.yml"
  sed \
    -e "s|__POSTGRES_DB_NAME__|${POSTGRES_DB_NAME}|g" \
    -e "s|__POSTGRES_DB_USER__|${POSTGRES_DB_USER}|g" \
    -e "s|__POSTGRES_DB_PASSWORD__|${POSTGRES_DB_PASSWORD}|g" \
    "${STACK_TEMPLATE_DIR}/initdb/01-init-app-dbs.sql.tpl" > "${STACK_DIR}/initdb/01-init-app-dbs.sql"
}
```

- [ ] **Step 4: Add the first-pass env, compose, and SQL templates**

```dotenv
APP_HTTP_PORT=__APP_HTTP_PORT__
APP_DATABASE_DSN=postgres://__POSTGRES_DB_USER__:__POSTGRES_DB_PASSWORD__@postgres:5432/__POSTGRES_DB_NAME__?sslmode=disable
APP_REDIS_ADDR=redis:6379
APP_ADMIN_BOOTSTRAP_USERNAME=__FLUXDIGEST_ADMIN_USERNAME__
APP_ADMIN_BOOTSTRAP_PASSWORD=__FLUXDIGEST_ADMIN_PASSWORD__
APP_LLM_BASE_URL=
APP_LLM_API_KEY=
APP_LLM_MODEL=
APP_LLM_TIMEOUT_MS=30000
```

```yaml
services:
  postgres:
    image: postgres:17
    env_file:
      - .env
    ports:
      - "35432:5432"
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
      - ./initdb:/docker-entrypoint-initdb.d

  redis:
    image: redis:7
    ports:
      - "36379:6379"
    volumes:
      - ./data/redis:/data

  fluxdigest-api:
    build:
      context: ../..
      dockerfile: deployments/docker/api.Dockerfile
    env_file:
      - .env
    ports:
      - "18088:18088"

__MINIFLUX_SERVICE_BLOCK__
__HALO_SERVICE_BLOCK__
```

```sql
CREATE USER __POSTGRES_DB_USER__ WITH PASSWORD '__POSTGRES_DB_PASSWORD__';
CREATE DATABASE __POSTGRES_DB_NAME__ OWNER __POSTGRES_DB_USER__;
```

- [ ] **Step 5: Run the render smoke test again**

Run: `bash deploy/stack/tests/render_smoke.sh`
Expected: PASS

- [ ] **Step 6: Commit the renderer foundation**

```bash
git add deploy/stack/scripts/common.sh deploy/stack/scripts/render.sh deploy/stack/stack.env.tpl deploy/stack/docker-compose.yml.tpl deploy/stack/initdb/01-init-app-dbs.sql.tpl deploy/stack/tests/render_smoke.sh
git commit -m "feat: add docker stack render foundation"
```

### Task 3: Implement installer orchestration, profile support, and install summary

**Files:**
- Create: `deploy/stack/install.sh`
- Create: `deploy/stack/scripts/healthcheck.sh`
- Create: `deploy/stack/tests/install_mocked_smoke.sh`
- Modify: `deploy/stack/scripts/common.sh`
- Modify: `deploy/stack/scripts/render.sh`
- Modify: `deploy/stack/stack.env.tpl`
- Modify: `deploy/stack/docker-compose.yml.tpl`

- [ ] **Step 1: Write the failing installer orchestration smoke test**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT
mkdir -p "${TMP_DIR}/bin"

cat > "${TMP_DIR}/bin/docker" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == "compose" ]]; then
  echo "docker compose $*" >> "__LOG_FILE__"
  exit 0
fi
if [[ "$1" == "version" ]]; then
  exit 0
fi
EOF
chmod +x "${TMP_DIR}/bin/docker"

cat > "${TMP_DIR}/bin/openssl" <<'EOF'
#!/usr/bin/env bash
printf 'feedfacefeedface\n'
EOF
chmod +x "${TMP_DIR}/bin/openssl"

export PATH="${TMP_DIR}/bin:${PATH}"
export INSTALL_LOG_FILE="${TMP_DIR}/docker.log"

# shellcheck disable=SC1091
source "${ROOT}/deploy/stack/install.sh"

build_fluxdigest_images() { :; }
wait_for_http_ok() { :; }
bootstrap_miniflux() { :; }
bootstrap_halo() { :; }

main --profile fluxdigest-only --stack-dir "${TMP_DIR}/stack" --force

grep -q 'fluxdigest-only' "${TMP_DIR}/stack/install-summary.txt"
grep -q 'FluxDigest WebUI / API: http://<host>:18088' "${TMP_DIR}/stack/install-summary.txt"
grep -q 'docker compose up -d postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler' "${INSTALL_LOG_FILE}"
```

- [ ] **Step 2: Run the installer orchestration smoke test to verify it fails**

Run: `bash deploy/stack/tests/install_mocked_smoke.sh`
Expected: FAIL because `install.sh` is missing or `main` is undefined.

- [ ] **Step 3: Implement installer entrypoint with source-safe `main` and arg parsing**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STACK_PROFILE="full"
STACK_DIR="/opt/fluxdigest-stack"
FORCE_OVERWRITE=0

source "${ROOT_DIR}/deploy/stack/scripts/common.sh"
source "${ROOT_DIR}/deploy/stack/scripts/render.sh"
source "${ROOT_DIR}/deploy/stack/scripts/healthcheck.sh"

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --profile) STACK_PROFILE="$2"; shift 2 ;;
      --stack-dir) STACK_DIR="$2"; shift 2 ;;
      --force) FORCE_OVERWRITE=1; shift ;;
      -h|--help) print_usage; return 0 ;;
      *) die "unknown argument: $1" ;;
    esac
  done
}

main() {
  parse_args "$@"
  ensure_linux
  ensure_required_commands
  prepare_stack_dir
  generate_credentials
  render_stack_files
  build_fluxdigest_images
  start_selected_services
  write_install_summary
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
```
- [ ] **Step 4: Add health checks, generated credentials, profile-specific compose startup, and summary rendering**

```bash
ensure_required_commands() {
  require_cmd docker
  require_cmd openssl
  require_cmd sed
  require_cmd awk
  require_cmd curl
  docker compose version >/dev/null
}

generate_credentials() {
  FLUXDIGEST_ADMIN_USERNAME="fd-$(random_token 4)"
  FLUXDIGEST_ADMIN_PASSWORD="$(random_token 16)"
  POSTGRES_DB_NAME="fluxdigest"
  POSTGRES_DB_USER="fd_db_$(random_token 4)"
  POSTGRES_DB_PASSWORD="$(random_token 16)"
  APP_ADMIN_SESSION_SECRET="$(random_token 32)"
  APP_SECRET_KEY="$(random_token 32)"
  APP_JOB_API_KEY="$(random_token 24)"
  export FLUXDIGEST_ADMIN_USERNAME FLUXDIGEST_ADMIN_PASSWORD POSTGRES_DB_NAME POSTGRES_DB_USER POSTGRES_DB_PASSWORD APP_ADMIN_SESSION_SECRET APP_SECRET_KEY APP_JOB_API_KEY
}

start_selected_services() {
  local services=(postgres redis fluxdigest-api fluxdigest-worker fluxdigest-scheduler)
  profile_has_service "${STACK_PROFILE}" miniflux && services+=(miniflux)
  profile_has_service "${STACK_PROFILE}" halo && services+=(halo)
  docker compose --env-file "${STACK_DIR}/.env" -f "${STACK_DIR}/docker-compose.yml" up -d "${services[@]}"
}

write_install_summary() {
  cat > "${STACK_DIR}/install-summary.txt" <<EOF
Profile: ${STACK_PROFILE}
FluxDigest WebUI / API: http://<host>:18088
FluxDigest admin username: ${FLUXDIGEST_ADMIN_USERNAME}
FluxDigest admin password: ${FLUXDIGEST_ADMIN_PASSWORD}
PostgreSQL database: ${POSTGRES_DB_NAME}
PostgreSQL username: ${POSTGRES_DB_USER}
PostgreSQL password: ${POSTGRES_DB_PASSWORD}
LLM: configure later in FluxDigest WebUI
EOF
}
```

- [ ] **Step 5: Run syntax checks and the mocked orchestration smoke test**

Run: `bash -n deploy/stack/install.sh deploy/stack/scripts/common.sh deploy/stack/scripts/render.sh deploy/stack/scripts/healthcheck.sh && bash deploy/stack/tests/install_mocked_smoke.sh`
Expected: PASS

- [ ] **Step 6: Commit the installer orchestration**

```bash
git add deploy/stack/install.sh deploy/stack/scripts/healthcheck.sh deploy/stack/scripts/common.sh deploy/stack/scripts/render.sh deploy/stack/stack.env.tpl deploy/stack/docker-compose.yml.tpl deploy/stack/tests/install_mocked_smoke.sh
git commit -m "feat: add docker stack installer orchestration"
```

### Task 4: Bootstrap Miniflux and wire its API token into FluxDigest

**Files:**
- Create: `deploy/stack/scripts/bootstrap_miniflux.sh`
- Create: `deploy/stack/tests/bootstrap_miniflux_smoke.sh`
- Modify: `deploy/stack/install.sh`
- Modify: `deploy/stack/stack.env.tpl`
- Modify: `deploy/stack/docker-compose.yml.tpl`

- [ ] **Step 1: Write the failing Miniflux bootstrap smoke test**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT
PORT=39181

cat > "${TMP_DIR}/server.py" <<'PY'
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/v1/api-keys":
            body = json.dumps({"token": "miniflux-generated-token"}).encode()
            self.send_response(201)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(404)

    def do_GET(self):
        if self.path == "/v1/me" and self.headers.get("X-Auth-Token") == "miniflux-generated-token":
            body = json.dumps({"username": "admin"}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(401)

    def log_message(self, *_args):
        return

HTTPServer(("127.0.0.1", 39181), Handler).serve_forever()
PY

python "${TMP_DIR}/server.py" &
SERVER_PID=$!
trap 'kill "${SERVER_PID}" 2>/dev/null || true; rm -rf "${TMP_DIR}"' EXIT

source "${ROOT}/deploy/stack/scripts/bootstrap_miniflux.sh"

MINIFLUX_BASE_URL="http://127.0.0.1:${PORT}"
MINIFLUX_ADMIN_USERNAME="admin"
MINIFLUX_ADMIN_PASSWORD="secret"
MINIFLUX_API_KEY_DESCRIPTION="FluxDigest Installer"
export MINIFLUX_BASE_URL MINIFLUX_ADMIN_USERNAME MINIFLUX_ADMIN_PASSWORD MINIFLUX_API_KEY_DESCRIPTION

token="$(bootstrap_miniflux)"
[[ "${token}" == "miniflux-generated-token" ]]
```

- [ ] **Step 2: Run the Miniflux smoke test to verify it fails**

Run: `bash deploy/stack/tests/bootstrap_miniflux_smoke.sh`
Expected: FAIL because `bootstrap_miniflux` is undefined.

- [ ] **Step 3: Implement Miniflux bootstrap helper using Basic Auth + API key creation**

```bash
#!/usr/bin/env bash
set -euo pipefail

bootstrap_miniflux() {
  local response token
  response="$(curl -fsS \
    -u "${MINIFLUX_ADMIN_USERNAME}:${MINIFLUX_ADMIN_PASSWORD}" \
    -H 'Content-Type: application/json' \
    -X POST \
    "${MINIFLUX_BASE_URL%/}/v1/api-keys" \
    -d "{\"description\":\"${MINIFLUX_API_KEY_DESCRIPTION}\"}")"
  token="$(python -c 'import json,sys; print(json.load(sys.stdin)["token"])' <<<"${response}")"
  curl -fsS -H "X-Auth-Token: ${token}" "${MINIFLUX_BASE_URL%/}/v1/me" >/dev/null
  printf '%s\n' "${token}"
}
```

- [ ] **Step 4: Wire Miniflux container defaults and installer integration**

```yaml
  miniflux:
    image: miniflux/miniflux:latest
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://${MINIFLUX_DB_USER}:${MINIFLUX_DB_PASSWORD}@postgres:5432/${MINIFLUX_DB_NAME}?sslmode=disable
      RUN_MIGRATIONS: "1"
      CREATE_ADMIN: "1"
      ADMIN_USERNAME: ${MINIFLUX_ADMIN_USERNAME}
      ADMIN_PASSWORD: ${MINIFLUX_ADMIN_PASSWORD}
    ports:
      - "28082:8080"
```

```bash
if profile_has_service "${STACK_PROFILE}" miniflux; then
  wait_for_http_ok "http://127.0.0.1:28082/healthz" 60 2
  APP_MINIFLUX_BASE_URL="http://miniflux:8080"
  APP_MINIFLUX_AUTH_TOKEN="$(bootstrap_miniflux)"
  export APP_MINIFLUX_BASE_URL APP_MINIFLUX_AUTH_TOKEN
  render_stack_files
fi
```

- [ ] **Step 5: Run the Miniflux smoke test and the render smoke test**

Run: `bash deploy/stack/tests/bootstrap_miniflux_smoke.sh && bash deploy/stack/tests/render_smoke.sh`
Expected: PASS

- [ ] **Step 6: Commit the Miniflux bootstrap flow**

```bash
git add deploy/stack/scripts/bootstrap_miniflux.sh deploy/stack/tests/bootstrap_miniflux_smoke.sh deploy/stack/install.sh deploy/stack/stack.env.tpl deploy/stack/docker-compose.yml.tpl
git commit -m "feat: auto bootstrap miniflux for docker stack"
```
### Task 5: Bootstrap Halo PAT and wire publishing defaults

**Files:**
- Create: `deploy/stack/scripts/bootstrap_halo.sh`
- Create: `deploy/stack/tests/bootstrap_halo_smoke.sh`
- Modify: `deploy/stack/install.sh`
- Modify: `deploy/stack/stack.env.tpl`
- Modify: `deploy/stack/docker-compose.yml.tpl`

- [ ] **Step 1: Write the failing Halo bootstrap smoke test**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT
PORT=39282

cat > "${TMP_DIR}/server.py" <<'PY'
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == "/apis/uc.api.security.halo.run/v1alpha1/personalaccesstokens":
            body = json.dumps({
                "metadata": {"name": "fluxdigest-pat"},
                "spec": {
                    "name": "FluxDigest Publisher",
                    "tokenId": "pat_generated_token",
                    "username": "halo-admin",
                    "scopes": ["*"],
                    "roles": ["superadmin"]
                }
            }).encode()
            self.send_response(201)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(404)

    def do_GET(self):
        if self.path.startswith('/apis/api.console.halo.run/v1alpha1/posts') and self.headers.get('Authorization') == 'Bearer pat_generated_token':
            body = json.dumps({"items": []}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(401)

    def log_message(self, *_args):
        return

HTTPServer(("127.0.0.1", 39282), Handler).serve_forever()
PY

python "${TMP_DIR}/server.py" &
SERVER_PID=$!
trap 'kill "${SERVER_PID}" 2>/dev/null || true; rm -rf "${TMP_DIR}"' EXIT

source "${ROOT}/deploy/stack/scripts/bootstrap_halo.sh"

HALO_BASE_URL="http://127.0.0.1:${PORT}"
HALO_ADMIN_USERNAME="halo-admin"
HALO_ADMIN_PASSWORD="halo-secret"
HALO_PAT_NAME="FluxDigest Publisher"
export HALO_BASE_URL HALO_ADMIN_USERNAME HALO_ADMIN_PASSWORD HALO_PAT_NAME

token="$(bootstrap_halo)"
[[ "${token}" == "pat_generated_token" ]]
```

- [ ] **Step 2: Run the Halo smoke test to verify it fails**

Run: `bash deploy/stack/tests/bootstrap_halo_smoke.sh`
Expected: FAIL because `bootstrap_halo` is undefined.

- [ ] **Step 3: Implement Halo bootstrap helper using the User-center PAT endpoint**

```bash
#!/usr/bin/env bash
set -euo pipefail

bootstrap_halo() {
  local payload response token
  payload="$(cat <<JSON
{
  \"apiVersion\": \"security.halo.run/v1alpha1\",
  \"kind\": \"PersonalAccessToken\",
  \"metadata\": {
    \"generateName\": \"fluxdigest-pat-\"
  },
  \"spec\": {
    \"name\": \"${HALO_PAT_NAME}\",
    \"description\": \"FluxDigest publisher token\",
    \"username\": \"${HALO_ADMIN_USERNAME}\",
    \"roles\": [\"superadmin\"],
    \"scopes\": [\"*\"],
    \"tokenId\": \"\"
  }
}
JSON
)"
  response="$(curl -fsS \
    -u "${HALO_ADMIN_USERNAME}:${HALO_ADMIN_PASSWORD}" \
    -H 'Content-Type: application/json' \
    -X POST \
    "${HALO_BASE_URL%/}/apis/uc.api.security.halo.run/v1alpha1/personalaccesstokens" \
    -d "${payload}")"
  token="$(python -c 'import json,sys; print(json.load(sys.stdin)["spec"]["tokenId"])' <<<"${response}")"
  curl -fsS -H "Authorization: Bearer ${token}" "${HALO_BASE_URL%/}/apis/api.console.halo.run/v1alpha1/posts?page=1&size=1" >/dev/null
  printf '%s\n' "${token}"
}
```

- [ ] **Step 4: Wire Halo container defaults and FluxDigest publish envs**

```yaml
  halo:
    image: registry.fit2cloud.com/halo/halo:2.23
    env_file:
      - .env
    command:
      - --spring.r2dbc.url=r2dbc:pool:postgresql://postgres:5432/${HALO_DB_NAME}
      - --spring.r2dbc.username=${HALO_DB_USER}
      - --spring.r2dbc.password=${HALO_DB_PASSWORD}
      - --spring.sql.init.platform=postgresql
      - --halo.external-url=${HALO_EXTERNAL_URL}
      - --halo.security.initializer.superadminusername=${HALO_ADMIN_USERNAME}
      - --halo.security.initializer.superadminpassword=${HALO_ADMIN_PASSWORD}
      - --halo.security.basic-auth.disabled=false
    ports:
      - "28090:8090"
```

```bash
if profile_has_service "${STACK_PROFILE}" halo; then
  wait_for_http_ok "http://127.0.0.1:28090/actuator/health" 90 3
  APP_PUBLISH_CHANNEL="halo"
  APP_PUBLISH_HALO_BASE_URL="http://halo:8090"
  APP_PUBLISH_HALO_TOKEN="$(bootstrap_halo)"
  export APP_PUBLISH_CHANNEL APP_PUBLISH_HALO_BASE_URL APP_PUBLISH_HALO_TOKEN
  render_stack_files
else
  APP_PUBLISH_CHANNEL="markdown_export"
  APP_PUBLISH_HALO_BASE_URL=""
  APP_PUBLISH_HALO_TOKEN=""
  export APP_PUBLISH_CHANNEL APP_PUBLISH_HALO_BASE_URL APP_PUBLISH_HALO_TOKEN
fi
```

- [ ] **Step 5: Run the Halo smoke test plus the full shell validation set**

Run: `bash deploy/stack/tests/bootstrap_halo_smoke.sh && bash deploy/stack/tests/bootstrap_miniflux_smoke.sh && bash deploy/stack/tests/install_mocked_smoke.sh && bash deploy/stack/tests/render_smoke.sh`
Expected: PASS

- [ ] **Step 6: Commit the Halo bootstrap flow**

```bash
git add deploy/stack/scripts/bootstrap_halo.sh deploy/stack/tests/bootstrap_halo_smoke.sh deploy/stack/install.sh deploy/stack/stack.env.tpl deploy/stack/docker-compose.yml.tpl
git commit -m "feat: auto bootstrap halo publisher for docker stack"
```

### Task 6: Validate the installer on a real Linux host and capture the final behavior

**Files:**
- Modify: `deploy/stack/install.sh`
- Modify: `deploy/stack/stack.env.tpl`
- Modify: `deploy/stack/docker-compose.yml.tpl`
- Modify: `deploy/stack/scripts/bootstrap_miniflux.sh`
- Modify: `deploy/stack/scripts/bootstrap_halo.sh`
- Modify: `deploy/stack/scripts/healthcheck.sh`

- [ ] **Step 1: Run the full automated test suite locally before touching the Linux host**

Run: `go test ./... && npm --prefix web test -- --run && bash deploy/stack/tests/render_smoke.sh && bash deploy/stack/tests/install_mocked_smoke.sh && bash deploy/stack/tests/bootstrap_miniflux_smoke.sh && bash deploy/stack/tests/bootstrap_halo_smoke.sh`
Expected: PASS

- [ ] **Step 2: Push the branch and execute a real full-profile install on the Linux test host**

```bash
git push origin codex/full-stack-installer
ssh test-server 'cd ~/FluxDigest && git fetch origin && git checkout codex/full-stack-installer && git pull --ff-only && sudo bash deploy/stack/install.sh --profile full --force'
```

Expected: installer finishes successfully and writes `/opt/fluxdigest-stack/install-summary.txt`.

- [ ] **Step 3: Verify the generated stack from the Linux host exactly as the docs will describe it**

```bash
ssh test-server '
  cd /opt/fluxdigest-stack && \
  docker compose --env-file .env -f docker-compose.yml ps && \
  cat install-summary.txt && \
  curl -fsS http://127.0.0.1:18088/healthz && \
  curl -fsS http://127.0.0.1:28082/healthz && \
  curl -fsS http://127.0.0.1:28090/actuator/health
'
```

Expected: all services are `Up`, the summary includes generated credentials, and each health endpoint returns success.
- [ ] **Step 4: Fix any contract mismatches discovered on Linux before editing docs**

```bash
go test ./... && bash deploy/stack/tests/render_smoke.sh && bash deploy/stack/tests/install_mocked_smoke.sh && bash deploy/stack/tests/bootstrap_miniflux_smoke.sh && bash deploy/stack/tests/bootstrap_halo_smoke.sh
```

Expected: PASS again after any Linux-driven corrections.

- [ ] **Step 5: Commit the Linux validation fixes**

```bash
git add deploy/stack/install.sh deploy/stack/stack.env.tpl deploy/stack/docker-compose.yml.tpl deploy/stack/scripts/bootstrap_miniflux.sh deploy/stack/scripts/bootstrap_halo.sh deploy/stack/scripts/healthcheck.sh
git commit -m "fix: align docker stack installer with linux validation"
```

### Task 7: Rewrite README and deployment docs around the verified Docker installer

**Files:**
- Modify: `README.md`
- Create: `docs/deployment/docker-stack.md`
- Create: `docs/deployment/runtime-configuration.md`

- [ ] **Step 1: Write the failing documentation acceptance checklist in the commit message scratchpad or notes**

```text
README must:
- lead with one-sentence value proposition
- show one real quick start command path
- list default access URLs
- point users to /opt/fluxdigest-stack/install-summary.txt
- tell users that LLM is configured later in WebUI

Docker docs must:
- explain profiles
- explain generated credentials
- explain reinstall / upgrade / uninstall
- explain troubleshooting commands

Runtime config docs must:
- cover Miniflux, Halo, LLM, prompts, publish strategy
- point users to FluxDigest WebUI as the primary configuration path
```

- [ ] **Step 2: Replace `README.md` with a standard open-source project structure**

````markdown
# FluxDigest

> AI-powered RSS digest pipeline for Miniflux users who want translated article dossiers and one published daily digest.

## Overview
FluxDigest pulls articles from Miniflux, translates and analyzes each article with your configured LLM, produces one daily digest, and optionally publishes the result to Halo while keeping single-article dossiers available through APIs.

## Features
- Miniflux-backed RSS ingestion
- Per-article translation and analysis dossiers
- One daily digest per run
- Halo publishing and open API access
- React WebUI for runtime configuration

## Quick Start
```bash
git clone https://github.com/ErzerLP/FluxDigest.git
cd FluxDigest
sudo bash deploy/stack/install.sh --profile full
cat /opt/fluxdigest-stack/install-summary.txt
```

After installation:
- FluxDigest WebUI: `http://<host>:18088`
- Miniflux: `http://<host>:28082`
- Halo: `http://<host>:28090`
- Configure your LLM in the FluxDigest WebUI before the first real digest run.
````

- [ ] **Step 3: Write the verified Docker stack deployment guide**

````markdown
# Docker Stack Deployment

## Profiles
- `full`
- `fluxdigest-miniflux`
- `fluxdigest-halo`
- `fluxdigest-only`

## Install
```bash
sudo bash deploy/stack/install.sh --profile full
```

## Generated files
- `/opt/fluxdigest-stack/.env`
- `/opt/fluxdigest-stack/docker-compose.yml`
- `/opt/fluxdigest-stack/install-summary.txt`

## Troubleshooting
```bash
cd /opt/fluxdigest-stack
docker compose --env-file .env -f docker-compose.yml ps
docker compose --env-file .env -f docker-compose.yml logs -f fluxdigest-api
```
````

- [ ] **Step 4: Write the runtime configuration guide around WebUI flows**

```markdown
# Runtime Configuration

## FluxDigest WebUI
Open `http://<host>:18088` and sign in with the generated admin credentials from `/opt/fluxdigest-stack/install-summary.txt`.

## Configure LLM
Fill in:
- Base URL
- API Key
- Model
- Timeout

## Configure Miniflux
When the installer profile already includes Miniflux, the base URL and API token are pre-wired. Use this page to replace them only if you are switching to an external Miniflux instance.

## Configure Halo publishing
When the installer profile already includes Halo, the publish channel is pre-wired to `halo`. Use this page to change target site details or switch away from Halo.

## Prompt management
Use the prompts page to edit digest, translation, and analysis prompts stored by FluxDigest runtime config.
```

- [ ] **Step 5: Run the full verification pass one last time and confirm docs match the real installer**

Run: `go test ./... && npm --prefix web test -- --run && bash deploy/stack/tests/render_smoke.sh && bash deploy/stack/tests/install_mocked_smoke.sh && bash deploy/stack/tests/bootstrap_miniflux_smoke.sh && bash deploy/stack/tests/bootstrap_halo_smoke.sh`
Expected: PASS, and the Linux host still reports the same URLs and generated-file locations documented above.

- [ ] **Step 6: Commit the README and docs rewrite**

```bash
git add README.md docs/deployment/docker-stack.md docs/deployment/runtime-configuration.md
git commit -m "docs: rewrite readme around docker stack installer"
```

