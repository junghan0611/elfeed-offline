# Elfeed-offline AGENT 가이드

## 프로젝트 개요

- **리포지토리**: junghan0611/elfeed-offline (fork of punchagan/elfeed-offline)
- **목적**: Emacs elfeed RSS 리더의 오프라인 모드 + 모바일 웹 인터페이스
- **언어**: OCaml (서버) + js_of_ocaml (프론트엔드)
- **프레임워크**: Dream (웹서버), Brr (브라우저 바인딩)

## 디렉터리 구조

```
elfeed-offline/
├── bin/
│   └── main.ml           # 서버 진입점 (Dream 프레임워크)
│
├── lib/
│   └── proxy.ml          # 리버스 프록시 (Emacs elfeed-web → HTTPS)
│
├── web/
│   ├── index.html        # 메인 HTML
│   ├── css/app.css       # 스타일시트
│   ├── sw.ml             # Service Worker (오프라인 캐싱)
│   └── js/               # 프론트엔드 (OCaml → JS)
│       ├── app.ml        # 앱 메인 로직
│       ├── state.ml      # 상태 관리
│       ├── entry.ml      # 피드 엔트리 렌더링
│       ├── nav.ml        # 네비게이션
│       ├── heartbeat.ml  # 서버 연결 상태
│       └── ...
│
├── shared/               # 서버/클라이언트 공유 타입
│   └── elfeed_message.ml # 메시지 타입 정의
│
├── scripts/
│   ├── make-cert.sh      # SSL 인증서 생성 (mkcert)
│   └── generate.ml       # 코드 생성
│
├── ssl/                  # 생성된 SSL 인증서 (gitignore)
├── run.sh                # 빌드 & 실행 스크립트
└── dune-project          # Dune 빌드 설정
```

## 아키텍처

```
[Emacs elfeed-web]     [elfeed-offline 서버]     [브라우저]
  localhost:8080  <-->  HTTPS Proxy :9000   <-->  Web UI
                              |
                       Service Worker
                       (오프라인 캐시)
```

### 핵심 컴포넌트

| 컴포넌트 | 파일 | 역할 |
|---------|------|------|
| **서버** | `bin/main.ml` | Dream HTTPS 서버, 정적 파일 제공 |
| **프록시** | `lib/proxy.ml` | elfeed-web API 포워딩, Basic Auth |
| **프론트엔드** | `web/js/*.ml` | 반응형 UI (Brr + Lwd) |
| **SW** | `web/sw.ml` | 캐시 전략, 오프라인 동기화 |

## 빌드 & 실행

```bash
# 빌드
dune build

# SSL 인증서 생성 (최초 1회)
bash scripts/make-cert.sh

# 실행
./run.sh
# 또는
dune exec -- elfeed-offline

# 접속
# https://localhost:9000
# https://<hostname>.local:9000 (모바일)
```

### 의존성

- OCaml 4.14+
- dune 3.0+
- mkcert (SSL 인증서)
- Emacs + elfeed + elfeed-web (백엔드)

## 개발 가이드

### Web UI 수정

**HTML/CSS만 수정** (빌드 불필요):
- `web/index.html`
- `web/css/app.css`

**로직 수정** (dune build 필요):
- `web/js/*.ml` → `dune build` → `_build/default/web/js/app.js`
- `web/sw.ml` → `dune build` → `_build/default/web/sw.js`

### 서버 수정

- `bin/main.ml` - 라우팅, 미들웨어
- `lib/proxy.ml` - 프록시 로직

빌드 후 재시작 필요.

### js_of_ocaml 코드 패턴

```ocaml
open Brr                          (* 브라우저 API *)
module Fetch = Brr_io.Fetch       (* Fetch API *)

(* DOM 조작 *)
let el = Document.find_el_by_id G.document (Jstr.v "my-id")

(* 이벤트 핸들링 *)
Ev.listen Ev.click handler (El.as_target button)

(* 반응형 UI (Lwd) *)
let doc = Lwd.map ~f:render state
```

## 환경 변수

| 변수 | 설명 |
|------|------|
| `ELFEED_USERNAME` | Basic Auth 사용자명 (선택) |
| `ELFEED_PASSWORD` | Basic Auth 비밀번호 (선택) |

## Git 커밋 가이드

### 커밋 메시지 스타일
```
feat: add dark mode toggle

- CSS custom properties for theme
- LocalStorage persistence
```

**중요**: "Generated with Claude" 또는 "Co-Authored-By" 제외!

## Issue Tracking

This project uses **br (beads_rust)** for issue tracking.

**Note:** `br` is non-invasive and never executes git commands. After `br sync --flush-only`, you must manually run `git add .beads/ && git commit`.

## Quick Reference

```bash
br ready              # Find available work
br show <id>          # View issue details
br update <id> --status in_progress  # Claim work
br close <id>         # Complete work
br sync --flush-only  # Export JSONL (no git)
git add .beads/
git commit -m "sync beads"
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - `dune build`
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   br sync --flush-only
   git add .beads/
   git commit -m "sync beads"
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

## Go 서버 (신규)

OCaml Dream 서버의 Go 재구현. 표준 라이브러리 기반.

```
go-server/
├── main.go          # CLI + HTTPS 서버
├── proxy.go         # ReverseProxy + HTML 래핑
├── middleware.go    # BasicAuth + Logger
├── main_test.go     # 통합 테스트
└── go.mod
```

### 빌드 & 실행

```bash
cd go-server
go build -o elfeed-offline-go .
./elfeed-offline-go --no-auth --ssl-cert ../ssl/server.pem --ssl-key ../ssl/server.key
```

### 인바리언트

1. `/elfeed/*` 요청 → 반드시 업스트림 프록시
2. `/elfeed/content/*` 응답 → 반드시 HTML 래핑
3. `--no-auth` 없으면 `/elfeed/*` 인증 필수
4. SSL 인증서 없으면 서버 시작 불가

## 테스트 명세

### Go 서버 테스트 (main_test.go)

```go
// 테스트 케이스
func TestProxyForwardsToUpstream(t *testing.T)     // 프록시 동작
func TestContentWrapping(t *testing.T)             // HTML 래핑
func TestBasicAuthRequired(t *testing.T)           // 인증 필수 (/elfeed/*)
func TestBasicAuthBypass(t *testing.T)             // 정적파일 인증 불필요
func TestStaticFileServing(t *testing.T)           // 정적 파일 제공
func TestSSLCertValidation(t *testing.T)           // SSL 인증서 검증
func TestUpstreamConnectionError(t *testing.T)     // 503 에러 처리
```

### 테스트 실행

```bash
cd go-server
go test -v ./...
```

### 수동 테스트 체크리스트

- [ ] `curl -k https://localhost:9000/` → 302 /index.html
- [ ] `curl -k https://localhost:9000/index.html` → HTML
- [ ] `curl -k https://localhost:9000/elfeed/search?q=test` → JSON (elfeed-web 필요)
- [ ] `curl -k https://localhost:9000/elfeed/content/abc` → 래핑된 HTML

## 향후 계획

- [x] Go 서버로 포팅
- [ ] TypeScript 프론트엔드로 재작성 (선택적)
- [ ] UI 개선 (다크모드, 반응형 등)
