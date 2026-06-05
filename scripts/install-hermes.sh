#!/usr/bin/env bash
set -euo pipefail

MODE="external"
INSTALLER_URL="https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh"

usage() {
  cat <<'USAGE'
Usage: install-hermes.sh [--mode external|installer]

Modes:
  external   Do not install Hermes. Print setup instructions only.
  installer  Run the upstream Hermes installer after confirmation.

Hermes Agent is an external project with its own license.
BillBot does not vendor or redistribute Hermes.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$MODE" in
  external)
    cat <<TEXT
External mode selected.
Install Hermes Agent separately, then make sure this command works:
  hermes status

Official Linux/macOS installer:
  curl -fsSL $INSTALLER_URL | bash

Then set BillBot config:
  hermes.command: hermes
TEXT
    ;;
  installer)
    cat <<TEXT
BillBot will run Hermes Agent's upstream installer from:
  $INSTALLER_URL

Hermes Agent is an external project with its own license.
BillBot does not redistribute or modify it.
TEXT
    read -r -p "Continue with upstream install? [y/N] " answer
    case "$answer" in
      y|Y|yes|YES)
        tmp="$(mktemp -d)"
        curl -fsSL "$INSTALLER_URL" -o "$tmp/hermes-install.sh"
        bash "$tmp/hermes-install.sh"
        ;;
      *) echo "Cancelled" ;;
    esac
    ;;
  *) echo "Invalid mode: $MODE" >&2; usage; exit 2 ;;
esac
