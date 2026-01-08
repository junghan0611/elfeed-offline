#!/bin/bash
# elfeed-offline 실행 스크립트

set -e

# 스크립트 위치 기준으로 경로 설정
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 기본값 (환경변수로 오버라이드 가능)
ELFEED_WEB_URL="${ELFEED_WEB_URL:-http://127.0.0.1:8080}"
PORT="${PORT:-9000}"
SSL_CERT="${SSL_CERT:-$SCRIPT_DIR/../ssl/server.pem}"
SSL_KEY="${SSL_KEY:-$SCRIPT_DIR/../ssl/server.key}"

# SSL 인증서 확인/생성
if [ ! -f "$SSL_CERT" ] || [ ! -f "$SSL_KEY" ]; then
    echo "SSL 인증서가 없습니다. 생성합니다..."
    mkdir -p "$(dirname "$SSL_CERT")"
    if command -v mkcert &> /dev/null; then
        mkcert -key-file "$SSL_KEY" -cert-file "$SSL_CERT" localhost 127.0.0.1
    else
        echo "mkcert가 없습니다. openssl로 자체 서명 인증서를 생성합니다..."
        openssl req -x509 -newkey rsa:4096 -keyout "$SSL_KEY" -out "$SSL_CERT" \
            -sha256 -days 365 -nodes -subj "/CN=localhost" \
            -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
    fi
fi

# 빌드
echo "빌드 중..."
go build -o elfeed-offline-go .

# 실행 정보 출력
echo ""
echo "================================"
echo "elfeed-offline Go 서버 시작"
echo "================================"
echo "서버 URL:    https://localhost:$PORT"
echo "elfeed-web:  $ELFEED_WEB_URL"
echo ""
echo "환경변수 설정 예시:"
echo "  ELFEED_WEB_URL=http://host:port PORT=9000 ./run.sh"
echo ""

exec ./elfeed-offline-go \
    --no-auth \
    -w "$ELFEED_WEB_URL" \
    -p "$PORT" \
    -ssl-cert "$SSL_CERT" \
    -ssl-key "$SSL_KEY"
