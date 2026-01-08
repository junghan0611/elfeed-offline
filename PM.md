# PM Dashboard - elfeed-offline

## 완료된 스프린트 (2026-01-08)

### ✅ Go 서버 재구현 (elfeed-offline-ano)

**완료 항목:**
- [x] go-server/ 디렉터리 + go.mod
- [x] main.go: CLI + HTTPS 서버
- [x] proxy.go: ReverseProxy + HTML 래핑
- [x] middleware.go: BasicAuth + Logger (타이밍 공격 방지)
- [x] main_test.go: 13개 테스트 (PASS)
- [x] flake.nix: Nix 개발환경
- [x] run.sh: 환경변수 + 자동 SSL

### ✅ CI/CD (elfeed-offline-529)

- [x] .github/workflows/build.yml
- [x] linux/amd64, linux/arm64 크로스 컴파일

### ✅ HTML/JS 프론트엔드 (elfeed-offline-meb)

- [x] index.html, app.css, app.js, sw.js
- [x] 검색, 읽기, 태그 토글
- [x] 다크모드, 모바일 반응형
- [ ] 오프라인 검색 필터링 (미구현)

## 현재 상태

| 기준 | 상태 |
|------|------|
| go build | ✅ |
| go test (13개) | ✅ |
| 프론트엔드 UI | ✅ |
| elfeed-web 연동 | ⏳ 테스트 필요 |

---

## 백로그

| Issue | 상태 | 우선순위 |
|-------|------|----------|
| elfeed-offline-ano | in_progress | P1 |
| elfeed-offline-k9j | open | P2 |
| elfeed-offline-lvt | open | P3 |

---

## 결정사항

- 2025-01-08: Go 서버 먼저, 프론트엔드는 기존 유지
- 2025-01-08: 별도 문서 대신 bd + 코드 주석

---

## 에이전트 위임 가이드

Go 서버 구현 위임 시:
```
Task(subagent_type=general-purpose)
- go-server/ 디렉터리 생성
- 참조: bin/main.ml, lib/proxy.ml
- 스펙: bd show elfeed-offline-ano
```
