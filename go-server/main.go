// elfeed-offline Go 서버
// OCaml Dream 서버를 Go로 재구현한 버전
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// CLI 플래그 정의
	// -w: Emacs elfeed-web 서버 URL (업스트림)
	elfeedWebURL := flag.String("w", "http://127.0.0.1:8080", "Emacs Elfeed web server URL")

	// -p: 리스닝 포트
	port := flag.Int("p", 9000, "Port to listen on")

	// -i: 바인딩할 네트워크 인터페이스
	iface := flag.String("i", "0.0.0.0", "Network interface to bind to")

	// SSL 인증서 경로
	sslCert := flag.String("ssl-cert", "ssl/server.pem", "Path to SSL certificate file")
	sslKey := flag.String("ssl-key", "ssl/server.key", "Path to SSL private key file")

	// 인증 비활성화 플래그
	noAuth := flag.Bool("no-auth", false, "Disable authentication (allow unauthenticated access)")

	flag.Parse()

	// SSL 인증서 존재 여부 확인
	if _, err := os.Stat(*sslCert); os.IsNotExist(err) {
		log.Fatalf("SSL certificate file not found at '%s'. Please generate it by running scripts/make-cert.sh", *sslCert)
	}
	if _, err := os.Stat(*sslKey); os.IsNotExist(err) {
		log.Fatalf("SSL key file not found at '%s'. Please generate it by running scripts/make-cert.sh", *sslKey)
	}

	// 리버스 프록시 생성 (elfeed-web 서버로 요청 전달)
	proxy := NewElfeedProxy(*elfeedWebURL)

	// 라우터(mux) 설정
	mux := http.NewServeMux()

	// /elfeed/** 경로: 리버스 프록시로 전달
	// GET, PUT, POST 모두 처리
	mux.Handle("/elfeed/", proxy)

	// 정적 파일 서버 (./web 디렉터리)
	fileServer := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fileServer)

	// 미들웨어 체인 구성
	var handler http.Handler = mux

	// Basic Auth 미들웨어 (--no-auth 플래그가 없을 때만)
	if !*noAuth {
		handler = BasicAuthMiddleware(handler)
	}

	// 로깅 미들웨어
	handler = LoggerMiddleware(handler)

	// 서버 주소 구성
	addr := fmt.Sprintf("%s:%d", *iface, *port)

	log.Printf("Starting elfeed-offline server on https://%s", addr)
	log.Printf("Proxying /elfeed/* to %s", *elfeedWebURL)
	if *noAuth {
		log.Printf("Authentication is DISABLED")
	} else {
		log.Printf("Authentication is enabled (set ELFEED_USERNAME and ELFEED_PASSWORD)")
	}

	// HTTPS 서버 시작
	if err := http.ListenAndServeTLS(addr, *sslCert, *sslKey, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
