#!/bin/bash
# Oracle VM ARM 서버용 실행 스크립트

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# SSL 인증서 경로
SSL_DIR="$SCRIPT_DIR/../ssl"

# SSL 인증서 확인
if [ ! -f "$SSL_DIR/server.pem" ] || [ ! -f "$SSL_DIR/server.key" ]; then
    echo "SSL certificates not found. Please run: bash ../scripts/make-cert.sh"
    exit 1
fi

# ARM64 바이너리 확인
if [ ! -f "./elfeed-offline-linux-arm64" ]; then
    echo "ARM64 binary not found. Building..."
    GOOS=linux GOARCH=arm64 go build -o elfeed-offline-linux-arm64 .
fi

# 서버 실행
echo "Starting elfeed-offline ARM64 server..."
exec ./elfeed-offline-linux-arm64 \
    --no-auth \
    -ssl-cert "$SSL_DIR/server.pem" \
    -ssl-key "$SSL_DIR/server.key" \
    -p 9000 \
    "$@"
