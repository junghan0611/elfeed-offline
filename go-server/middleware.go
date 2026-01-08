// HTTP 미들웨어 구현
// - Basic Auth: /elfeed/* 경로 인증
// - Logger: 요청 로깅
package main

import (
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// BasicAuthMiddleware는 /elfeed/* 경로에 Basic Auth 인증을 적용
// 환경변수 ELFEED_USERNAME, ELFEED_PASSWORD에서 자격증명 읽기
func BasicAuthMiddleware(next http.Handler) http.Handler {
	// 환경변수에서 사용자명과 비밀번호 읽기
	username := os.Getenv("ELFEED_USERNAME")
	password := os.Getenv("ELFEED_PASSWORD")

	// 환경변수가 설정되지 않으면 에러
	if username == "" || password == "" {
		log.Fatal("Authentication is enabled but ELFEED_USERNAME and/or ELFEED_PASSWORD environment variables are not set. Either set both variables or run with --no-auth to disable authentication.")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /elfeed/ 경로가 아니면 인증 없이 통과
		// 정적 파일(index.html, CSS, JS)은 인증 없이 제공
		if !strings.HasPrefix(r.URL.Path, "/elfeed/") {
			next.ServeHTTP(w, r)
			return
		}

		// Authorization 헤더 확인
		auth := r.Header.Get("Authorization")
		if auth == "" {
			unauthorized(w)
			return
		}

		// "Basic <base64>" 형식 파싱
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Basic" {
			unauthorized(w)
			return
		}

		// Base64 디코딩
		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			unauthorized(w)
			return
		}

		// "username:password" 형식 파싱
		credentials := strings.SplitN(string(decoded), ":", 2)
		if len(credentials) != 2 {
			unauthorized(w)
			return
		}

		// 자격증명 검증 (타이밍 공격 방지를 위해 constant-time 비교 사용)
		usernameMatch := subtle.ConstantTimeCompare([]byte(credentials[0]), []byte(username))
		passwordMatch := subtle.ConstantTimeCompare([]byte(credentials[1]), []byte(password))
		if usernameMatch != 1 || passwordMatch != 1 {
			unauthorized(w)
			return
		}

		// 인증 성공: 다음 핸들러로 전달
		next.ServeHTTP(w, r)
	})
}

// unauthorized는 401 Unauthorized 응답을 보냄
// WWW-Authenticate 헤더로 Basic Auth 방식임을 알림
func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Elfeed Proxy"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("Unauthorized"))
}

// LoggerMiddleware는 모든 HTTP 요청을 로깅
// 형식: [시간] 메서드 경로 - 상태코드 (소요시간)
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 응답 상태 코드를 캡처하기 위한 ResponseWriter 래퍼
		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// 다음 핸들러 호출
		next.ServeHTTP(wrapped, r)

		// 요청 로깅
		duration := time.Since(start)
		log.Printf("%s %s - %d (%v)",
			r.Method,
			r.URL.Path,
			wrapped.status,
			duration,
		)
	})
}

// responseWriter는 응답 상태 코드를 캡처하기 위한 래퍼
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader는 상태 코드를 저장하고 원본 WriteHeader 호출
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
