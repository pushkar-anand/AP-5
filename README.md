# AP-5

AP-5 is a background email agent — part of the household agent suite alongside [JN-66](https://github.com/pushkar-anand/JN-66). It monitors Gmail inboxes, classifies emails with a local LLM, and progressively learns how to handle new email types through a web UI review-and-teach loop. Credit card transaction emails are handled out of the box; any other financial email type can be taught to the agent via the browser.

Named after the [Imperial inventory droid](https://starwars.fandom.com/wiki/AP-5) from Star Wars Rebels.

## How it works

1. **Poll** — every configured interval, AP-5 calls the Gmail `history.list` API for each watched account and fetches all new messages (no server-side filter).
2. **Route** — a fast LLM call classifies each email into a descriptive category (e.g. `credit_card_transaction`, `bank_account_transaction`) using a dynamic category list that grows as you teach the agent.
3. **Handle** — if a handler is registered for that category, it runs a second, more accurate LLM call to extract structured fields and posts the result to JN-66. If no handler exists, the email is saved to a **review queue**.
4. **Learn** — open `/review` in a browser to see queued emails. For each one you can:
   - **Reclassify + Reprocess** — the LLM got the category wrong; pick the correct handler and reprocess now.
   - **Teach** — write an extraction prompt, choose an action (`import_transaction` or `log_only`), and the agent handles all future emails of that type automatically.
   - **Ignore** — discard from the queue.

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
  review/         # review queue: persists unhandled emails to review_queue.json
  rules/          # learned rules: persists user-taught handlers to learned_rules.json
  handlers/
    creditcard/   # built-in: extracts CC transaction, resolves JN-66 account, imports
    learned/      # generic: runs user-defined extraction prompt, same import pipeline
  jn66/           # JN-66 HTTP client + in-memory account cache
  server/         # HTTP server: OAuth callback, healthz, review UI (/review, /rules)
```

## Teaching the agent a new email type

No code changes needed. Just let an email of the new type arrive:

1. AP-5 classifies it (e.g. `bank_account_transaction`) and adds it to the review queue.
2. Open `http://localhost:8080/review` in a browser.
3. Click the email, write an extraction prompt like:

   > Extract bank transaction details. Return JSON with fields:
   > - `institution`: bank name (e.g. "SBI")
   > - `last_four`: last 4 digits of the account number
   > - `amount_paise`: amount in paise as an integer
   > - `merchant`: payee name
   > - `date`: YYYY-MM-DD
   > - `direction`: "debit" or "credit"
   >
   > If you cannot extract all fields, return `{"error": "reason"}`.

4. Choose **Import transaction to JN-66** and click **Teach & Process Now**.

The agent registers the rule immediately (no restart), processes the queued email, and handles all future emails of the same type automatically. Rules survive restarts — they're persisted in `~/.config/ap5/learned_rules.json`.

## Adding a new built-in handler (for developers)

To ship a handler as code rather than a learned rule:

1. Create `internal/handlers/<type>/handler.go` implementing `router.Handler`.
2. Register it in `cmd/ap5/serve.go`: `r.Register("<type_string>", handler)`.

The LLM classifier already recognises common financial categories (`bank_account_transaction`, `bill_payment`, `investment_transaction`) as hints — no prompt changes needed unless you introduce an entirely new vocabulary term.

## License

MIT
