#!/usr/bin/env bash
set -euo pipefail

# Check if SSL certificates exist
if [[ ! -f ssl/server.pem ]] || [[ ! -f ssl/server.key ]]; then
    echo "SSL certificates not found. Generating..."
    bash scripts/make-cert.sh
fi

# Build and run
dune build
dune exec -- elfeed-offline
