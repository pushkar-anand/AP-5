# AGENT.md — AP-5 agent profile

AP-5 is an autonomous background agent in the household agent suite. This document describes its role, capabilities, and integration points for other agents and operators.

## Role

AP-5 monitors Gmail inboxes and processes transactional emails on behalf of household members. Phase 1 focus: credit card spend notifications → transaction records in JN-66.

AP-5 does not expose a task API or take requests from other agents. It is an event-driven daemon: Gmail events flow in, JN-66 import calls flow out.

## Inputs

| Source | Mechanism | Notes |
|---|---|---|
| Gmail inboxes | `history.list` polling (every `poll_interval`) | All new messages, no server-side filter |
| Local LLM (Ollama) | OpenAI-compatible REST API | Classification + extraction; privacy-first, runs on-device |

## Outputs

| Destination | API call | Triggered by |
|---|---|---|
| JN-66 | `POST /api/import` | Classified credit card transaction |
| JN-66 | `POST /api/accounts` | First transaction from an unknown card |

## Integration with JN-66

AP-5 is a write-only client of JN-66. It uses two JN-66 endpoints:

**Account resolution** (`GET /api/accounts`, `POST /api/accounts`)

AP-5 identifies a credit card by `(institution, last_four)` extracted from the email. It looks up the matching JN-66 account by `external_account_id = last_four`. If no account exists, it creates one:

```json
{
  "institution": "HDFC Bank",
  "name": "HDFC Bank ••••1234",
  "account_type": "credit_card",
  "external_account_id": "1234"
}
```

**Transaction import** (`POST /api/import`)

```json
{
  "account_id": "<jn66-account-uuid>",
  "transactions": [
    {
      "amount_paise": 125000,
      "merchant": "Swiggy",
      "date": "2026-05-25",
      "direction": "debit"
    }
  ]
}
```

JN-66 handles duplicate detection — AP-5 logs the result and moves on.

## LLM pipeline

Two sequential calls per email:

1. **Router** (`router_model`, e.g. `qwen3:4b`) — classifies email type. Returns `{"type": "credit_card_transaction"}` or `{"type": "other"}`. Cheap and fast.
2. **Extractor** (`extractor_model`, e.g. `qwen3:14b`) — called only for matched types. Returns structured transaction fields.

Both models run locally via Ollama. No data leaves the network.

## HTTP endpoints (embedded server)

AP-5 runs an HTTP server (default port 8080) used only for the Gmail OAuth2 callback flow.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Liveness check — returns `200 OK` |
| `GET` | `/auth/callback` | Receives OAuth2 code from Google, exchanges for token, begins polling |

The callback URL is `<server.base_url>/auth/callback`. For Docker deployments, port 8080 must be reachable from the browser used for OAuth consent (either via port forwarding or a reverse proxy).

## Error behaviour

| Error | AP-5 behaviour |
|---|---|
| LLM parse failure | Log and skip; next poll retries the same message |
| JN-66 API error | Log with subject line; skip import |
| Gmail transient error | Log; historyId not advanced; retried next poll |
| Missing OAuth token | Log URL; skip that account; continues polling other accounts |
| Missing JN-66 token | Log error; skip that account |

## Adding handlers (extensibility)

AP-5 supports additional email types via a handler registry. To add a new type:

1. Implement `router.Handler` in a new `internal/handlers/<type>/handler.go`
2. Register in `serve.go`: `r.Register("<type_string>", handler)`
3. Add the type string to the LLM router prompt categories in `internal/llm/client.go`

The router, poller, and all other packages remain unchanged.

## Deployment

| Mode | Secret backend | Notes |
|---|---|---|
| Desktop | `keyring` | OS keyring (libsecret on Linux, Keychain on macOS) |
| Docker / headless | `file` | AES-256-GCM encrypted file; key in `config.yaml` (gitignored) |

State (Gmail `historyId` per account) is persisted to `$dataDir/state.json`.
