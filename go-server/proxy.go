// 리버스 프록시 구현
// elfeed-web 서버로 요청을 전달하고 응답을 처리
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// 허용된 헤더 목록 (업스트림으로 전달할 헤더)
// 보안을 위해 화이트리스트 방식 사용
var allowedHeaders = map[string]bool{
	"Content-Type":  true,
	"Accept":        true,
	"Authorization": true,
	"Cookie":        true,
	"User-Agent":    true,
}

// HTML 래핑 템플릿
// /elfeed/content/* 응답을 감싸는 HTML
// - color-scheme: 다크모드 지원
// - img max-width: 이미지 반응형 처리
// - base target=_blank: 링크를 새 탭에서 열기
const wrapperTemplate = `<!doctype html>
<html>
    <head>
        <style type='text/css'>
        :root {
            color-scheme: light dark;
        }
        img {
            max-width: 100%%;
            height: auto;
            object-fit: contain;
        }
        </style>
        <meta charset='utf-8' />
        <base target='_blank' />
    </head>
    <body>
        %s
    </body>
</html>`

// 에러 HTML 템플릿
const errorTemplate = `<!doctype html>
<html>
    <head>
        <style type='text/css'>
        :root {
            color-scheme: light dark;
        }
        </style>
        <meta charset='utf-8' />
    </head>
    <body>
        <h1>%s</h1>
        <p>%s</p>
    </body>
</html>`

// ElfeedProxy는 elfeed-web 서버에 대한 리버스 프록시
type ElfeedProxy struct {
	upstream *url.URL
	proxy    *httputil.ReverseProxy
}

// NewElfeedProxy는 새 리버스 프록시 인스턴스를 생성
func NewElfeedProxy(upstreamURL string) *ElfeedProxy {
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		log.Fatalf("Invalid upstream URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)

	ep := &ElfeedProxy{
		upstream: upstream,
		proxy:    proxy,
	}

	// Director: 요청을 수정하는 함수
	// 허용된 헤더만 전달하도록 필터링
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		ep.filterHeaders(req)
	}

	// ModifyResponse: 응답을 수정하는 함수
	// /elfeed/content/* 응답을 HTML로 래핑
	proxy.ModifyResponse = ep.modifyResponse

	// ErrorHandler: 연결 실패 시 503 응답
	proxy.ErrorHandler = ep.errorHandler

	return ep
}

// filterHeaders는 허용된 헤더만 남기고 나머지를 제거
func (ep *ElfeedProxy) filterHeaders(req *http.Request) {
	filteredHeaders := http.Header{}
	for name, values := range req.Header {
		// 헤더 이름을 정규화하여 비교 (Content-Type 등)
		canonicalName := http.CanonicalHeaderKey(name)
		if allowedHeaders[canonicalName] {
			filteredHeaders[canonicalName] = values
		}
	}
	req.Header = filteredHeaders
}

// isContentURI는 URI가 /elfeed/content/로 시작하는지 확인
func isContentURI(path string) bool {
	return strings.HasPrefix(path, "/elfeed/content/")
}

// wrapHTML은 HTML 콘텐츠를 템플릿으로 감싸기
func wrapHTML(content string) string {
	return fmt.Sprintf(wrapperTemplate, content)
}

// modifyResponse는 프록시 응답을 수정
// - /elfeed/content/* 경로의 응답을 HTML 템플릿으로 래핑
// - 4xx/5xx 에러 시 사용자 친화적 에러 페이지 표시
func (ep *ElfeedProxy) modifyResponse(resp *http.Response) error {
	// /elfeed/content/ 경로가 아니면 원본 응답 그대로 반환
	if !isContentURI(resp.Request.URL.Path) {
		return nil
	}

	// 응답 본문 읽기
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	var newBody string

	// HTTP 에러 상태 코드 처리
	if resp.StatusCode >= 400 {
		// 에러 페이지 생성
		title := fmt.Sprintf("Error %d", resp.StatusCode)
		message := "Failed to load content from the Elfeed server. Is it offline?"
		newBody = fmt.Sprintf(errorTemplate, title, message)

		// Content-Type을 HTML로 변경
		resp.Header.Set("Content-Type", "text/html; charset=utf-8")

		// 5xx 에러를 404로 변환 (클라이언트에서 더 나은 처리를 위해)
		if resp.StatusCode >= 500 {
			resp.StatusCode = 404
			resp.Status = "404 Not Found"
		}
	} else {
		// 정상 응답: HTML 템플릿으로 래핑
		newBody = wrapHTML(string(body))
	}

	// 새 응답 본문 설정
	resp.Body = io.NopCloser(bytes.NewBufferString(newBody))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))

	return nil
}

// errorHandler는 프록시 연결 실패 시 호출됨
// 503 Service Unavailable 응답 반환
func (ep *ElfeedProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Proxy error for %s: %v", r.URL.Path, err)

	// 연결 실패 에러 페이지
	errorHTML := fmt.Sprintf(errorTemplate,
		"Connection Error",
		"Failed to connect to the Elfeed server. Is it offline?")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable) // 503
	w.Write([]byte(errorHTML))
}

// ServeHTTP는 http.Handler 인터페이스 구현
func (ep *ElfeedProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ep.proxy.ServeHTTP(w, r)
}
