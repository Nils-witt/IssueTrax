## Go development

Before any Go coding, review, debugging, troubleshooting, or setup task, load the `samber/cc-skills-golang@golang-how-to` skill first — it routes to whichever other Go skills the task needs.

## Project

IssueTrax: Go backend (`nilswitt.dev/issuetrax`) serving a JSON API under `/api` (documented in `docs/openapi.yaml`) plus a Vite + React + TypeScript SPA (React Router, TanStack Query, i18next en/de) embedded via `//go:embed` (`frontend/embed.go`). Design decisions: `base_architecture.md`.

Architecture is hexagonal (ports & adapters), wired by manual constructor injection in `cmd/issuetrax/main.go`:

- `internal/core/domain` — entities and rules (projects, issues, tags, users, sessions, coded errors, pagination); no adapter imports.
- `internal/core/port` — driving (`*Service`, e.g. `IssueService`, `AuthService`) and driven (`*Repository`, `TxManager`, `IdentityProvider`) interfaces.
- `internal/core/service` — use-case implementations of driving ports.
- `internal/adapter/httpapi` — inbound HTTP adapter (JSON DTOs live here, not in domain): `/api` JSON, `/auth` browser sign-in (signed login cookie, session cookie, CSRF via `http.CrossOriginProtection`), SPA fallback.
- `internal/adapter/oidc` — OpenID Connect client for Authentik (code flow + PKCE, ID token verification).
- `internal/adapter/storage/postgres` — PostgreSQL repositories (pgx + sqlc; goose migrations embedded in `migrations/`, queries in `queries/`, generated code in `sqlc/` — never edit by hand, run `make generate`).
- `internal/adapter/storage/memory` — in-memory repositories for service unit tests.
- `internal/adapter/storage/storagetest` — repository contract suite every storage adapter must pass.

Dependencies point inward: adapters → port/service → domain.

## Commands

- `make build` — builds `frontend/dist` then `bin/issuetrax`. `frontend/dist` must exist before any Go build, vet, test or lint (`make frontend`).
- `make test` runs everything incl. Postgres integration tests (`-tags integration`, testcontainers, needs Docker); `make test-unit` skips them. `make lint` (golangci-lint + oxlint) / `make audit` (govulncheck).
- `make generate` regenerates sqlc code after changing `queries/*.sql` or migrations.
- Dev: `make dev-db` (Postgres on localhost:55432 and Authentik on localhost:9000 via `deploy/compose.yaml`; the Authentik app/test user `ada`/`ada` come from `deploy/authentik/blueprints/`), `make dev-backend` (port 8080, dev settings from the Makefile) and `make dev-frontend` (Vite HMR on :5173, proxies `/api` and `/auth` to the backend; open the app there).
