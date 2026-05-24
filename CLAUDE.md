# CLAUDE.md — AP-5 coding agent instructions

## Project overview

AP-5 is a Go background daemon that monitors Gmail inboxes, classifies emails with a local LLM (Ollama), and posts detected credit card transactions to JN-66 (household finance tracker). Module: `github.com/pushkar-anand/ap-5`, Go 1.26.3.

## Security — top priority

**This repo is public. Never commit personal data.**

- No real email addresses — use `your-email@gmail.com` or `user@example.com` as placeholders
- No real domain names — use `yourdomain.dev`, not any real personal domain
- No real tokens, API keys, or encryption keys
- `config.yaml` is gitignored — never commit it. Only `config.example.yaml` (with placeholders) is committed
- Before any commit, grep for personal-looking data: real emails, internal hostnames, bearer tokens

## Build and test

```bash
go build ./cmd/ap5          # build the binary
go test ./...               # run all unit tests (60 tests across 9 packages)
go vet ./...
```

All tests use only the standard library and `net/http/httptest` — no external services, no real Gmail, no real Ollama.

## Architecture principles

- **Interface-based DI everywhere.** Every external dependency (`llm.Classifier`, `llm.Extractor`, `accountResolver`, `transactionImporter`, `OAuthExchanger`, `secrets.Store`) is an interface so tests can inject fakes via `httptest.Server` or simple mock structs. Avoid concrete types in function signatures where a caller needs to be testable.
- **Two LLM calls per email**: router (cheap, `router_model`) + handler (accurate, `extractor_model`). The router only classifies; it never extracts. Keep these responsibilities separate.
- **Pluggable handler pattern.** New email type = new `internal/handlers/<type>/handler.go` + one `r.Register()` call in `serve.go` + one new category in the LLM router prompt. No other files change.
- **No server-side Gmail filter.** The poller reads all new messages and lets the LLM router decide relevance. Do not add Gmail search queries.
- **historyId is the resume cursor.** Always advance it after successful fetch, not after processing. If processing fails, the next poll will re-fetch the same messages — that's intentional (idempotent imports rely on JN-66's duplicate detection).

## Secret storage

Two backends behind `secrets.Store`:
- `keyring` — OS keyring via `go-keyring` (default, desktop/Linux with libsecret)
- `file` — AES-256-GCM encrypted JSON at `$dataDir/secrets/store.enc` (Docker/headless)

Backend and encryption key come from `config.yaml` (`secrets.backend`, `secrets.encryption_key`), not env vars. Config is gitignored. Env override still works: `AP5_SECRETS__ENCRYPTION_KEY`.

## Config

Loaded by `koanf/v2`: YAML file first, then `AP5_` env vars override (`__` = `.`).

```
AP5_SERVER__PORT          → server.port
AP5_OLLAMA__ROUTER_MODEL  → ollama.router_model
```

## Key secret keys

| Secret | Store key |
|---|---|
| Gmail OAuth token (per account) | `ap5/gmail/<email>` |
| JN-66 bearer token (per account) | `ap5/jn66/<email>` |
| Gmail OAuth client ID | `ap5/gmail/client_id` |
| Gmail OAuth client secret | `ap5/gmail/client_secret` |

## Testing conventions

- Package `_test` (external test packages) everywhere — tests the exported API, not internals
- `gmail/client_test.go` is the exception: package `gmail_test` but imports internal helpers via a `_test.go` file in the same package for `extractBody`/`headerValue` unit tests
- External HTTP services (JN-66, Ollama) are mocked with `httptest.NewServer`
- No filesystem side effects in tests — use `t.TempDir()` for state and file-backend tests
- `t.Setenv()` for env var tests (auto-cleanup)

## What not to do

- Don't add card configuration to `config.yaml` — accounts are auto-discovered via JN-66's `GET /api/accounts` and created with `POST /api/accounts`
- Don't add Gmail server-side search filters — let the LLM classify
- Don't retry LLM failures — log and skip; the next poll will retry naturally
- Don't store secrets in `config.example.yaml` or any committed file
- Don't use `go-keyring` directly outside `internal/secrets/keyring.go`
