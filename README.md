# BillBot

License: LGPL-3.0-only

BillBot is a cross-platform QQ bridge prototype written in Go. It provides a local dashboard and CLI to manage a NapCat OneBot connector, Hermes runner, prompt settings, owner policy, slash commands, diagnostics, and runtime state.

## Principles

- Do not commit personal owner QQ IDs, persona prompts, model choices, API keys, runtime sessions, QR codes, or login caches.
- NapCatQQ is an external connector dependency. BillBot must not vendor, bundle, patch-package, or redistribute NapCatQQ or NTQQ binaries.
- Hermes is an external CLI runner.
- The Go backend should run on Windows, Ubuntu/Linux, and macOS.

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

Dashboard mode:

```bash
./bin/billbot --port 2006
```

Open:

```text
http://127.0.0.1:2006
```

CLI mode:

```bash
./bin/billbot --cli --port 2006
```

OS autostart:

```bash
./bin/billbot --cli
billbot> autostart
billbot> autostart-enable
```

Windows autostart writes a current-user Registry `Run` entry. Linux autostart writes and enables a systemd user service. These controls start BillBot itself; NapCat remains external and is managed through `processes.napcat`.
Autostart always launches dashboard mode, even when configured from CLI, so it can run unattended after login.

Use a config file:

```bash
./bin/billbot --config ./config.example.yaml --port 2006
```

## External Dependencies

NapCat OneBot defaults:

```yaml
napcat:
  http: http://127.0.0.1:3000
  ws: ws://127.0.0.1:3001
```

Bridge start can launch a user-configured local NapCat command through `processes.napcat`, but NapCat remains external and separately licensed.

Hermes defaults to the `hermes` command on `PATH`.

## Install Helpers

Linux helper scripts live under `scripts/`. They only download from upstream at runtime or print setup instructions. They do not include third-party binaries in this repository.

```bash
bash scripts/install-napcat.sh --mode external
bash scripts/install-hermes.sh --mode external
```

## Docs

- `docs/AGENT_TASKS.md`
- `docs/SECURITY.md`
- `docs/LICENSING.md`
- `THIRD_PARTY_NOTICES.md`
