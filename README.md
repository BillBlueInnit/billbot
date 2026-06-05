# BillBot

License: LGPL-3.0-only

BillBot is a QQ bridge prototype written in Go. It runs as an interactive terminal CLI and connects external NapCat OneBot endpoints to the external Hermes CLI.

```text
NapCat OneBot WS -> BillBot bridge -> Hermes CLI -> NapCat OneBot HTTP
```

BillBot does not bundle, download, install, or patch NapCatQQ, NTQQ, QQ, Hermes, model SDKs, API keys, QR codes, login caches, or third-party binaries.

## Build

Ubuntu/Linux:

```bash
go test ./...
go build -o ./bin/billbot ./cmd/billbot
```

Windows:

```powershell
D:\golang\go\bin\go.exe test ./...
D:\golang\go\bin\go.exe build -o .\bin\billbot.exe .\cmd\billbot
```

## Run

Start NapCat separately, preferably with Docker for headless Linux:

```bash
docker compose -f ./deploy/napcat-compose.yml up -d
```

Then run BillBot in a terminal:

```bash
./bin/billbot
```

`--config` is optional. Without it, BillBot uses a config file next to the executable:

1. Use existing `config.toml` if present.
2. Otherwise use existing `config.yaml` if present.
3. Otherwise create `config.toml` automatically on first start.

```bash
./bin/billbot
```

Both TOML and YAML are supported. The format is selected by file extension.

For long-running servers, use `screen` or `tmux`:

```bash
tmux new -s billbot './bin/billbot'
```

The program starts directly in interactive CLI mode. There is no dashboard and no autostart manager.

## CLI

Useful commands:

```text
start
status
diag
route
route off
stop
logs
clear
set qq <bot QQ number>
set admin <admin QQ number>
set token <onebot token>
set http_token <http token>
set ws_token <ws token>
qq <bot QQ number>
admin <admin QQ number>
token <onebot token>
set models.base_provider <provider>
set models.base_model <model>
set models.strong_provider <provider>
set models.strong_model <model>
quit
```

In an interactive terminal, the CLI supports Up/Down command history. Pasting uses the terminal shortcut, usually `Ctrl+Shift+V` or right click.

## External Dependencies

NapCat defaults:

```yaml
napcat:
  http: http://127.0.0.1:3000
  ws: ws://127.0.0.1:3001
  access_token: ""
  http_token: ""
  ws_token: ""
processes:
  napcat:
    auto_start: false
```

BillBot auto-detects reachable NapCat OneBot endpoints before starting the bridge. It first checks the configured endpoint, then common local defaults:

```text
http://127.0.0.1:3000 + ws://127.0.0.1:3001
http://localhost:3000 + ws://localhost:3001
```

This works for Docker, CLI, or manually managed NapCat as long as OneBot HTTP/WS is exposed to the host.

If diagnostics show `http 403`, NapCat HTTP is reachable but requires OneBot authentication. If HTTP and WS use the same token, set it once:

```text
set token <your-onebot-token>
diag
```

If NapCat has separate HTTP and WS tokens, set both:

```text
set http_token <your-http-token>
set ws_token <your-ws-token>
diag
```

Hermes defaults to the system command:

```yaml
hermes:
  command: hermes
  persistent: true
```

This must work in the same shell:

```bash
hermes status
hermes chat -Q -q "Reply OK"
```

With `hermes.persistent = true`, BillBot keeps one `hermes acp` child process running and forwards QQ messages to it serially. This avoids starting a new Hermes CLI process for every message. If ACP is not available on a host, set:

```text
set hermes.persistent false
```

## QQ Commands

These commands are handled by BillBot directly and do not go through the normal AI chat path:

```text
/help
/identity
/style
```

Admin-only commands use the QQ `user_id` from NapCat metadata. Text claims such as "I am owner 123" are not trusted.

```text
/identity <description>
/identity add <description>
/style <description>
/style add <description>
/sandbox
/full
/shell <command>
```

`/identity <description>` and `/style <description>` ask Hermes to rewrite the description into a concise English prompt, then save the rewritten prompt into the config.

Set the admin QQ number from the local BillBot CLI first:

```text
set admin <your QQ number>
```

## AI Routing

BillBot can route between Hermes providers/models. Configure:

```yaml
models:
  base_provider: ""
  base_model: ""
  strong_provider: ""
  strong_model: ""
  routing_timeout_sec: 30
```

When both base and strong routes are set, BillBot first calls Hermes with the base provider/model and a router prompt. If the request is simple, the base model answers directly. If it needs stronger reasoning, the router returns `BILLBOT_ROUTE_STRONG`, and BillBot calls Hermes again with the strong provider/model.

Routing is disabled by default. To force BillBot back to the Hermes default model and clear all model overrides:

```text
route off
```

Hermes receives these as CLI flags:

```text
hermes chat -Q -q <prompt> --provider <provider> -m <model>
```

## Docs

- `docs/SECURITY.md`
- `docs/LICENSING.md`
- `THIRD_PARTY_NOTICES.md`
