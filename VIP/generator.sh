#!/bin/bash
set -e


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

set -o allexport
source "$PROJECT_ROOT/.env"
set +o allexport

envsubst < "$SCRIPT_DIR/templates/keepalived.conf.template" > "$SCRIPT_DIR/keepalived.conf"

