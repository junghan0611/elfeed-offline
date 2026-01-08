# elfeed-offline Go Server

OCaml Dream 서버의 Go 재구현 버전입니다.

## 빌드

```bash
go build -o elfeed-offline-go .
```

## 실행

```bash
# 기본 실행 (SSL 인증서 자동 생성)
./run.sh

# 또는 직접 실행
./elfeed-offline-go \
    -ssl-cert ../ssl/server.pem \
    -ssl-key ../ssl/server.key

# 인증 비활성화
./elfeed-offline-go --no-auth -ssl-cert ../ssl/server.pem -ssl-key ../ssl/server.key

# 커스텀 설정
./elfeed-offline-go \
    -w http://localhost:8080 \
    -p 9000 \
    -i 0.0.0.0 \
    -ssl-cert ../ssl/server.pem \
    -ssl-key ../ssl/server.key
```

## CLI 옵션

| 플래그 | 기본값 | 설명 |
|--------|--------|------|
| `-w` | `http://127.0.0.1:8080` | Emacs elfeed-web 서버 URL |
| `-p` | `9000` | 리스닝 포트 |
| `-i` | `0.0.0.0` | 바인딩할 네트워크 인터페이스 |
| `-ssl-cert` | `ssl/server.pem` | SSL 인증서 경로 |
| `-ssl-key` | `ssl/server.key` | SSL 개인키 경로 |
| `--no-auth` | `false` | 인증 비활성화 |

## 환경변수

| 변수 | 설명 | 기본값 |
|------|------|--------|
| `ELFEED_WEB_URL` | elfeed-web 서버 URL | http://127.0.0.1:8080 |
| `PORT` | Go 서버 포트 | 9000 |
| `SSL_CERT` | SSL 인증서 경로 | ../ssl/server.pem |
| `SSL_KEY` | SSL 키 경로 | ../ssl/server.key |
| `ELFEED_USERNAME` | Basic Auth 사용자명 | - |
| `ELFEED_PASSWORD` | Basic Auth 비밀번호 | - |

## elfeed-web 연동

### Emacs에서 elfeed-web 시작

```elisp
;; Emacs에서 실행
M-x elfeed-web-start

;; 또는 init.el에 추가
(require 'elfeed-web)
(elfeed-web-start)
```

기본적으로 elfeed-web은 `http://localhost:8080`에서 실행됩니다.

### 연동 테스트

```bash
# elfeed-web 상태 확인
curl http://localhost:8080/elfeed/update

# 피드 검색
curl "http://localhost:8080/elfeed/search?q=@30-days-ago+unread"

# Go 서버 시작
cd go-server
ELFEED_WEB_URL=http://localhost:8080 ./run.sh

# 브라우저에서 https://localhost:9000 접속
```

### 모바일 접속

같은 네트워크에서 모바일 디바이스로 접속하려면:

```bash
# 호스트명 확인
hostname

# 모바일 브라우저에서 접속
# https://<hostname>.local:9000
```

## 아키텍처

```
[Emacs elfeed-web]     [Go 서버]          [브라우저]
  localhost:8080  <-->  HTTPS :9000  <-->  Web UI
                             |
                     ReverseProxy
                     + HTML 래핑
```

### 파일 구조

- `main.go` - CLI 파싱, 라우팅, HTTPS 서버 시작
- `proxy.go` - 리버스 프록시, 응답 래핑
- `middleware.go` - Basic Auth, 로깅 미들웨어

## Nix 개발 환경

프로젝트 루트에서:

```bash
# 개발 환경 진입
nix develop

# 빌드
nix build
```
