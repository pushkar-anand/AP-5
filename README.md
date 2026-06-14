# AP-5

AP-5 is a background email agent — part of the household agent suite alongside [JN-66](https://github.com/pushkar-anand/JN-66). It monitors Gmail inboxes, detects credit card spend notification emails using a local LLM, and posts the extracted transactions to JN-66 automatically.

Named after the [Imperial inventory droid](https://starwars.fandom.com/wiki/AP-5) from Star Wars Rebels.

## How it works

1. **Poll** — every configured interval, AP-5 calls the Gmail `history.list` API for each watched account and fetches all new messages (no server-side filter).
2. **Route** — a fast LLM call classifies each email into a type (e.g. `credit_card_transaction`) or `other`.
3. **Extract** — a second, more accurate LLM call pulls structured transaction fields from the email body.
4. **Import** — the transaction is posted to JN-66 (`POST /api/import`). The JN-66 account is auto-discovered or created using `(institution, last_four)` — no manual card configuration needed.

## Prerequisites

| Requirement | Notes |
|---|---|
| Go 1.26+ | Only needed for local builds |
| [Ollama](https://ollama.com) | Runs locally; pull the models listed in your config |
| Gmail OAuth2 credentials | Desktop OAuth app from Google Cloud Console |
| JN-66 instance | Running and reachable at `jn66.base_url` |

## Setup

### 1. Google Cloud Console

1. Create a project and enable the **Gmail API**.
2. Create an **OAuth 2.0 Client ID** (Desktop app type).
3. Add your redirect URI(s) to the allowed list:
   - `http://localhost:8080/auth/callback` (local dev)
   - `https://ap-5.yourdomain.dev/auth/callback` (production)

### 2. Config file

Copy the example config and fill in your values:

```bash
mkdir -p ~/.config/ap5
cp config.example.yaml ~/.config/ap5/config.yaml
$EDITOR ~/.config/ap5/config.yaml
```

`config.yaml` is gitignored — never commit it.

### 3. Pull Ollama models

```bash
ollama pull qwen3:14b   # extractor (accuracy-critical)
ollama pull qwen3:4b    # optional: use for router_model to save VRAM
```

### 4. Store secrets

```bash
# Gmail OAuth credentials (client ID + secret from Google Cloud Console)
ap5 auth set-gmail-credentials
```

JN-66 bearer tokens live in `config.yaml` under each account entry — no separate command needed.

### 5. Run

```bash
ap5 serve
```

On first run, AP-5 logs an OAuth URL for each account with no saved token. Open the URL in a browser, complete consent, and AP-5 begins polling.

## Docker

```bash
# 1. Fill in your config
cp config.example.yaml config.yaml
$EDITOR config.yaml   # set secrets.backend: file, secrets.encryption_key: <64-char hex>

# 2. Start
docker compose up -d

# 3. Store Gmail OAuth credentials (first run only)
docker exec -it ap5 ap5 auth set-gmail-credentials --config /config.yaml --data /data

# 4. Authorize Gmail accounts — copy the URL from docker logs and open in browser
docker logs ap5
```

Generate an encryption key:

```bash
openssl rand -hex 32
```

## Configuration reference

```yaml
server:
  port: 8080
  base_url: "http://localhost:8080"   # used to build OAuth redirect_uri

jn66:
  base_url: "http://jn66:8080"

ollama:
  base_url: "http://localhost:11434/v1"
  router_model: "qwen3:4b"      # classification — smaller/faster is fine
  extractor_model: "qwen3:14b"  # extraction — larger = more accurate

accounts:
  personal:
    email: "your-email@gmail.com"
    jn66_token: "your-jn66-bearer-token"
  partner:
    email: "partner-email@gmail.com"
    jn66_token: "partner-jn66-bearer-token"

gmail:
  poll_interval: 60s

log:
  level: "info"    # debug | info | warn | error
  format: "text"   # text | json

secrets:
  backend: "keyring"       # keyring (desktop/Linux) | file (Docker/headless)
  encryption_key: ""       # required when backend=file; 64-char hex
```

Any config value can be overridden with an `AP5_` environment variable using `__` as the dot separator:

```bash
AP5_SERVER__PORT=9090
AP5_SECRETS__ENCRYPTION_KEY=abc...
AP5_ACCOUNTS__PERSONAL__JN66_TOKEN=your-token
```

## Project structure

```
cmd/ap5/          # CLI entry point (serve, auth subcommands)
internal/
  config/         # koanf YAML + env config loading
  secrets/        # SecretStore interface: keyring (desktop) or encrypted file (Docker)
  state/          # persists Gmail historyId per account
  gmail/          # OAuth2, Gmail API client, history.list poller
  llm/            # Ollama client (Classifier + Extractor interfaces)
  router/         # classifies emails, dispatches to registered handlers
  handlers/
    creditcard/   # extracts transaction, resolves JN-66 account, posts import
  jn66/           # JN-66 HTTP client + in-memory account cache
  server/         # embedded HTTP server (OAuth callback + healthz)
```

## Adding a new email type

1. Create `internal/handlers/<type>/handler.go` implementing `router.Handler`.
2. Register it in `cmd/ap5/serve.go`: `r.Register("<type_string>", handler)`.
3. Add `<type_string>` to the router prompt's category list in `internal/llm/client.go`.

No changes to the router or any other package are needed.

## License

MIT
