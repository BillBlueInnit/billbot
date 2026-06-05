#!/usr/bin/env bash
set -euo pipefail

MODE="external"
INSTALLER_URL="https://nclatest.znin.net/NapNeko/NapCat-Installer/main/script/install.sh"
INSTALL_DIR="${BILLBOT_NAPCAT_DIR:-}"

usage() {
  cat <<'USAGE'
Usage: install-napcat.sh [--mode external|installer|patch] [--install-dir DIR]

Modes:
  external   Do not download NapCat. Print setup instructions only.
  installer  Download the upstream NapCat installer after confirmation.
  patch      Prepare local BillBot config snippets for an existing NapCat install.

BillBot does not bundle NapCatQQ. NapCatQQ has its own license.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift 2 ;;
    --install-dir) INSTALL_DIR="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$MODE" in
  external)
    cat <<'TEXT'
External mode selected.
Install NapCatQQ separately, then set these BillBot config fields:
  napcat.http: http://127.0.0.1:3000
  napcat.ws: ws://127.0.0.1:3001
TEXT
    ;;
  installer)
    cat <<TEXT
BillBot will download NapCatQQ's upstream installer from:
  $INSTALLER_URL

NapCatQQ is an external project with its own license.
BillBot does not redistribute or modify it.
TEXT
    read -r -p "Continue with upstream download? [y/N] " answer
    case "$answer" in
      y|Y|yes|YES)
        tmp="$(mktemp -d)"
        curl -fsSL "$INSTALLER_URL" -o "$tmp/napcat-install.sh"
        bash "$tmp/napcat-install.sh"
        ;;
      *) echo "Cancelled" ;;
    esac
    ;;
  patch)
    cat <<'TEXT'
Patch mode selected.
Apply BillBot's OneBot endpoint settings to your existing local NapCat install.
No NapCat files are redistributed by BillBot in this mode.
TEXT
    ;;
  *) echo "Invalid mode: $MODE" >&2; usage; exit 2 ;;
esac
