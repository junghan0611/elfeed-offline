// elfeed-offline Go 서버 테스트
// httptest 패키지를 사용하여 실제 네트워크 없이 단위 테스트 수행
package main

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestIsContentURI는 isContentURI 함수의 경로 판별 로직을 검증
// /elfeed/content/로 시작하는 경로만 true를 반환해야 함
func TestIsContentURI(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "콘텐츠 경로 - 정상",
			path:     "/elfeed/content/123",
			expected: true,
		},
		{
			name:     "콘텐츠 경로 - 슬래시로 끝남",
			path:     "/elfeed/content/",
			expected: true,
		},
		{
			name:     "콘텐츠 경로 - 중첩 경로",
			path:     "/elfeed/content/abc/def",
			expected: true,
		},
		{
			name:     "비콘텐츠 경로 - /elfeed/",
			path:     "/elfeed/",
			expected: false,
		},
		{
			name:     "비콘텐츠 경로 - /elfeed/search",
			path:     "/elfeed/search",
			expected: false,
		},
		{
			name:     "비콘텐츠 경로 - 루트",
			path:     "/",
			expected: false,
		},
		{
			name:     "비콘텐츠 경로 - 정적 파일",
			path:     "/index.html",
			expected: false,
		},
		{
			name:     "비콘텐츠 경로 - content 없는 elfeed",
			path:     "/elfeed/entries",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContentURI(tt.path)
			if result != tt.expected {
				t.Errorf("isContentURI(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestWrapHTML은 wrapHTML 함수가 콘텐츠를 올바르게 HTML로 감싸는지 검증
// 반환값은 반드시 <!doctype html>로 시작해야 함
func TestWrapHTML(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "간단한 텍스트",
			content: "Hello World",
		},
		{
			name:    "HTML 콘텐츠",
			content: "<p>This is a paragraph</p>",
		},
		{
			name:    "빈 콘텐츠",
			content: "",
		},
		{
			name:    "이미지 태그",
			content: `<img src="test.jpg" alt="test">`,
		},
		{
			name:    "특수 문자",
			content: "<script>alert('XSS')</script>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapHTML(tt.content)

			// 인바리언트 1: 반드시 <!doctype html>로 시작
			if !strings.HasPrefix(result, "<!doctype html>") {
				t.Errorf("wrapHTML() 결과가 <!doctype html>로 시작하지 않음")
			}

			// 원본 콘텐츠가 포함되어 있어야 함
			if !strings.Contains(result, tt.content) {
				t.Errorf("wrapHTML() 결과에 원본 콘텐츠가 포함되지 않음")
			}

			// <body> 태그가 있어야 함
			if !strings.Contains(result, "<body>") {
				t.Errorf("wrapHTML() 결과에 <body> 태그가 없음")
			}

			// color-scheme 스타일이 있어야 함 (다크모드 지원)
			if !strings.Contains(result, "color-scheme") {
				t.Errorf("wrapHTML() 결과에 color-scheme 스타일이 없음")
			}

			// 이미지 반응형 스타일이 있어야 함
			if !strings.Contains(result, "max-width: 100%") {
				t.Errorf("wrapHTML() 결과에 이미지 반응형 스타일이 없음")
			}
		})
	}
}

// TestContentWrapping은 /elfeed/content/* 경로의 응답이 HTML로 래핑되는지 검증
// 프록시를 통과한 콘텐츠 응답은 반드시 완전한 HTML 문서여야 함
func TestContentWrapping(t *testing.T) {
	// 모의 업스트림 서버 생성 - 피드 콘텐츠 반환
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<p>Feed content here</p>"))
	}))
	defer upstream.Close()

	// 프록시 생성
	proxy := NewElfeedProxy(upstream.URL)

	// 테스트 서버 생성 (프록시만 사용)
	testServer := httptest.NewServer(proxy)
	defer testServer.Close()

	// /elfeed/content/123 요청
	resp, err := http.Get(testServer.URL + "/elfeed/content/123")
	if err != nil {
		t.Fatalf("요청 실패: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("응답 읽기 실패: %v", err)
	}

	bodyStr := string(body)

	// 인바리언트 1: /elfeed/content/* 응답은 반드시 <!doctype html> 포함
	if !strings.HasPrefix(bodyStr, "<!doctype html>") {
		t.Errorf("콘텐츠 응답이 <!doctype html>로 시작하지 않음: %s", bodyStr[:min(100, len(bodyStr))])
	}

	// 원본 콘텐츠가 래핑 내에 포함되어야 함
	if !strings.Contains(bodyStr, "<p>Feed content here</p>") {
		t.Errorf("래핑된 응답에 원본 콘텐츠가 없음")
	}

	// color-scheme 스타일이 포함되어야 함
	if !strings.Contains(bodyStr, "color-scheme") {
		t.Errorf("래핑된 응답에 color-scheme 스타일이 없음")
	}
}

// TestBasicAuthRequired는 /elfeed/* 경로에 인증이 필수인지 검증
// 인증 없이 접근 시 401 Unauthorized를 반환해야 함
func TestBasicAuthRequired(t *testing.T) {
	// 환경변수 설정 (테스트용)
	os.Setenv("ELFEED_USERNAME", "testuser")
	os.Setenv("ELFEED_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("ELFEED_USERNAME")
		os.Unsetenv("ELFEED_PASSWORD")
	}()

	// 모의 핸들러 (인증 성공 시 호출됨)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Basic Auth 미들웨어 적용
	authHandler := BasicAuthMiddleware(handler)

	tests := []struct {
		name           string
		path           string
		auth           string
		expectedStatus int
	}{
		{
			name:           "인증 없이 /elfeed/ 접근 - 401 반환",
			path:           "/elfeed/",
			auth:           "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "잘못된 인증으로 /elfeed/ 접근 - 401 반환",
			path:           "/elfeed/entries",
			auth:           "Basic " + base64.StdEncoding.EncodeToString([]byte("wrong:wrong")),
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "올바른 인증으로 /elfeed/ 접근 - 200 반환",
			path:           "/elfeed/entries",
			auth:           "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "인증 없이 /elfeed/content/ 접근 - 401 반환",
			path:           "/elfeed/content/123",
			auth:           "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()

			authHandler.ServeHTTP(rec, req)

			// 인바리언트 2: 인증 없이 /elfeed/* 접근 시 401 반환
			if rec.Code != tt.expectedStatus {
				t.Errorf("상태 코드 = %d, expected %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

// TestBasicAuthBypass는 정적 파일 경로가 인증 없이 접근 가능한지 검증
// 정적 파일(index.html, CSS, JS)은 인증 없이 제공되어야 함
func TestBasicAuthBypass(t *testing.T) {
	// 환경변수 설정
	os.Setenv("ELFEED_USERNAME", "testuser")
	os.Setenv("ELFEED_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("ELFEED_USERNAME")
		os.Unsetenv("ELFEED_PASSWORD")
	}()

	// 모의 핸들러 (정적 파일 시뮬레이션)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Static content"))
	})

	// Basic Auth 미들웨어 적용
	authHandler := BasicAuthMiddleware(handler)

	// 인바리언트 4: 정적 파일 경로는 인증 없이 접근 가능
	tests := []struct {
		name string
		path string
	}{
		{name: "루트 경로", path: "/"},
		{name: "index.html", path: "/index.html"},
		{name: "CSS 파일", path: "/css/app.css"},
		{name: "JS 파일", path: "/js/app.js"},
		{name: "sw.js", path: "/sw.js"},
		{name: "manifest.json", path: "/manifest.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			// 인증 헤더 없음
			rec := httptest.NewRecorder()

			authHandler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("정적 파일 %s 접근 시 상태 코드 = %d, expected 200", tt.path, rec.Code)
			}
		})
	}
}

// TestStaticFileServing은 정적 파일이 올바르게 제공되는지 검증
// 실제 파일 시스템 없이 라우팅 로직만 테스트
func TestStaticFileServing(t *testing.T) {
	// 라우터 설정 (main.go와 동일한 패턴)
	mux := http.NewServeMux()

	// 루트 리다이렉트 테스트
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/index.html", http.StatusFound)
			return
		}
		// 실제 파일 서버 대신 테스트용 응답
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Static file: " + r.URL.Path))
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	// 루트 경로 접근 시 /index.html로 리다이렉트
	t.Run("루트 리다이렉트", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // 리다이렉트 따라가지 않음
			},
		}

		resp, err := client.Get(testServer.URL + "/")
		if err != nil {
			t.Fatalf("요청 실패: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusFound {
			t.Errorf("상태 코드 = %d, expected 302", resp.StatusCode)
		}

		location := resp.Header.Get("Location")
		if location != "/index.html" {
			t.Errorf("리다이렉트 위치 = %s, expected /index.html", location)
		}
	})

	// 정적 파일 경로 접근
	t.Run("정적 파일 경로", func(t *testing.T) {
		resp, err := http.Get(testServer.URL + "/css/app.css")
		if err != nil {
			t.Fatalf("요청 실패: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("상태 코드 = %d, expected 200", resp.StatusCode)
		}
	})
}

// TestUpstreamConnectionError는 업스트림 서버 연결 실패 시 503 에러를 반환하는지 검증
// 인바리언트 3: 업스트림 연결 실패 시 503 반환
func TestUpstreamConnectionError(t *testing.T) {
	// 존재하지 않는 업스트림 URL로 프록시 생성
	// 연결 불가능한 주소 사용
	proxy := NewElfeedProxy("http://127.0.0.1:59999")

	testServer := httptest.NewServer(proxy)
	defer testServer.Close()

	// /elfeed/entries 요청 (업스트림에 연결해야 함)
	resp, err := http.Get(testServer.URL + "/elfeed/entries")
	if err != nil {
		t.Fatalf("요청 실패: %v", err)
	}
	defer resp.Body.Close()

	// 인바리언트 3: 업스트림 연결 실패 시 503 반환
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("상태 코드 = %d, expected 503 (Service Unavailable)", resp.StatusCode)
	}

	// 에러 페이지에 적절한 메시지가 있어야 함
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Connection Error") {
		t.Errorf("에러 응답에 'Connection Error' 메시지가 없음")
	}
}

// TestProxyNonContentPath는 /elfeed/content/ 이외의 경로가 래핑 없이 전달되는지 검증
func TestProxyNonContentPath(t *testing.T) {
	// 모의 업스트림 서버 - JSON 응답
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"entries": []}`))
	}))
	defer upstream.Close()

	proxy := NewElfeedProxy(upstream.URL)
	testServer := httptest.NewServer(proxy)
	defer testServer.Close()

	// /elfeed/entries 요청 (content 경로가 아님)
	resp, err := http.Get(testServer.URL + "/elfeed/entries")
	if err != nil {
		t.Fatalf("요청 실패: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// HTML로 래핑되지 않아야 함
	if strings.HasPrefix(bodyStr, "<!doctype html>") {
		t.Errorf("/elfeed/entries 응답이 HTML로 래핑됨 (래핑되지 않아야 함)")
	}

	// 원본 JSON 응답이어야 함
	if !strings.Contains(bodyStr, `{"entries": []}`) {
		t.Errorf("원본 JSON 응답이 아님: %s", bodyStr)
	}
}

// TestContentErrorPage는 콘텐츠 요청 시 업스트림 에러가 발생하면
// 사용자 친화적 에러 페이지를 반환하는지 검증
func TestContentErrorPage(t *testing.T) {
	// 모의 업스트림 서버 - 404 에러 반환
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	}))
	defer upstream.Close()

	proxy := NewElfeedProxy(upstream.URL)
	testServer := httptest.NewServer(proxy)
	defer testServer.Close()

	// /elfeed/content/123 요청 (존재하지 않는 콘텐츠)
	resp, err := http.Get(testServer.URL + "/elfeed/content/123")
	if err != nil {
		t.Fatalf("요청 실패: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// 에러 페이지도 HTML이어야 함
	if !strings.HasPrefix(bodyStr, "<!doctype html>") {
		t.Errorf("에러 페이지가 <!doctype html>로 시작하지 않음")
	}

	// 에러 메시지가 포함되어야 함
	if !strings.Contains(bodyStr, "Error") {
		t.Errorf("에러 페이지에 'Error' 메시지가 없음")
	}
}

// TestHeaderFiltering은 허용된 헤더만 업스트림으로 전달되는지 검증
// 참고: X-Forwarded-For는 httputil.ReverseProxy가 자동 추가하므로 필터링 대상에서 제외
func TestHeaderFiltering(t *testing.T) {
	var receivedHeaders http.Header

	// 모의 업스트림 서버 - 받은 헤더 저장
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	proxy := NewElfeedProxy(upstream.URL)
	testServer := httptest.NewServer(proxy)
	defer testServer.Close()

	// 여러 헤더를 포함한 요청
	req, _ := http.NewRequest("GET", testServer.URL+"/elfeed/entries", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic dGVzdDp0ZXN0") // test:test
	req.Header.Set("X-Custom-Header", "should-be-filtered")
	req.Header.Set("X-Secret-Token", "should-be-filtered")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("요청 실패: %v", err)
	}
	defer resp.Body.Close()

	// 허용된 헤더 확인
	allowedCheck := []string{"Content-Type", "Accept", "Authorization"}
	for _, h := range allowedCheck {
		if receivedHeaders.Get(h) == "" {
			t.Errorf("허용된 헤더 %s가 전달되지 않음", h)
		}
	}

	// 필터링된 헤더 확인
	// X-Forwarded-For는 httputil.ReverseProxy가 자동 추가하므로 제외
	filteredCheck := []string{"X-Custom-Header", "X-Secret-Token"}
	for _, h := range filteredCheck {
		if receivedHeaders.Get(h) != "" {
			t.Errorf("필터링되어야 할 헤더 %s가 전달됨", h)
		}
	}
}

// TestInvalidBasicAuth는 다양한 잘못된 인증 형식을 테스트
func TestInvalidBasicAuth(t *testing.T) {
	os.Setenv("ELFEED_USERNAME", "testuser")
	os.Setenv("ELFEED_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("ELFEED_USERNAME")
		os.Unsetenv("ELFEED_PASSWORD")
	}()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	authHandler := BasicAuthMiddleware(handler)

	tests := []struct {
		name string
		auth string
	}{
		{
			name: "Bearer 토큰 (Basic이 아님)",
			auth: "Bearer sometoken",
		},
		{
			name: "잘못된 Base64",
			auth: "Basic not-valid-base64!!!",
		},
		{
			name: "콜론 없는 디코딩",
			auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("nocolon")),
		},
		{
			name: "빈 사용자명",
			auth: "Basic " + base64.StdEncoding.EncodeToString([]byte(":password")),
		},
		{
			name: "빈 비밀번호",
			auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("username:")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/elfeed/entries", nil)
			req.Header.Set("Authorization", tt.auth)
			rec := httptest.NewRecorder()

			authHandler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("상태 코드 = %d, expected 401 (Unauthorized)", rec.Code)
			}
		})
	}
}

// TestLoggerMiddleware는 로깅 미들웨어가 올바르게 동작하는지 검증
func TestLoggerMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	loggedHandler := LoggerMiddleware(handler)

	req := httptest.NewRequest("GET", "/test-path", nil)
	rec := httptest.NewRecorder()

	// 로깅 미들웨어가 에러 없이 실행되어야 함
	loggedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("상태 코드 = %d, expected 200", rec.Code)
	}
}

// TestResponseWriterWrapper는 responseWriter 래퍼가 상태 코드를 올바르게 캡처하는지 검증
func TestResponseWriterWrapper(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapper := &responseWriter{
		ResponseWriter: rec,
		status:         http.StatusOK,
	}

	// 상태 코드 변경
	wrapper.WriteHeader(http.StatusNotFound)

	if wrapper.status != http.StatusNotFound {
		t.Errorf("캡처된 상태 코드 = %d, expected 404", wrapper.status)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("실제 응답 상태 코드 = %d, expected 404", rec.Code)
	}
}

// min은 두 정수 중 작은 값을 반환 (Go 1.21 이전 호환용)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
