## Go development

Before any Go coding, review, debugging, troubleshooting, or setup task, load the `samber/cc-skills-golang@golang-how-to` skill first — it routes to whichever other Go skills the task needs.

## Project

IssueTrax: Go backend (`nilswitt.dev/issuetrax`) serving a JSON API under `/api` plus a Vite + React + TypeScript SPA embedded via `//go:embed` (`frontend/embed.go`).

Architecture is hexagonal (ports & adapters), wired by manual constructor injection in `cmd/issuetrax/main.go`:

- `internal/core/domain` — entities and rules; no adapter imports.
- `internal/core/port` — driving (`IssueService`) and driven (`IssueRepository`) interfaces.
- `internal/core/service` — use-case implementations of driving ports.
- `internal/adapter/httpapi` — inbound HTTP adapter (JSON DTOs live here, not in domain) + SPA fallback.
- `internal/adapter/storage/memory` — in-memory repository (dev/tests).

Dependencies point inward: adapters → port/service → domain.

## Commands

- `make build` — builds `frontend/dist` then `bin/issuetrax`. `frontend/dist` must exist before any Go build, vet, test or lint (`make frontend`).
- `make test` / `make lint` (golangci-lint + oxlint) / `make audit` (govulncheck).
- Dev: `make dev-backend` (port 8080) and `make dev-frontend` (Vite HMR, proxies `/api` to the backend).
