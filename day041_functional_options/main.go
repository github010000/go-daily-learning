package main

import (
	"fmt"
	"time"
)

// Server 구조체는 애플리케이션 서버의 설정 정보를 담고 있습니다.
type Server struct {
	Host     string
	Port     int
	Timeout  time.Duration
	Protocol string
}

// Option 함수 타입은 Server 구조체 포인터를 받아 수정하는 함수의 타입입니다.
// 이것이 함수형 옵션 패턴의 핵심입니다.
type Option func(*Server)

// --- 옵션 생성 함수들 (WithXxx) ---

// WithHost는 서버의 호스트 주소를 설정하는 옵션을 반환합니다.
func WithHost(host string) Option {
	return func(s *Server) {
		s.Host = host
	}
}

// WithPort는 서버의 포트 번호를 설정하는 옵션을 반환합니다.
func WithPort(port int) Option {
	return func(s *Server) {
		s.Port = port
	}
}

// WithTimeout은 요청 처리 시간을 설정하는 옵션을 반환합니다.
func WithTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.Timeout = timeout
	}
}

// WithProtocol은 통신 프로토콜을 설정하는 옵션을 반환합니다.
func WithProtocol(proto string) Option {
	return func(s *Server) {
		s.Protocol = proto
	}
}

// NewServer는 서버 인스턴스를 생성하고, 전달된 옵션들을 순차적으로 적용합니다.
// ...Option 가변 인자를 사용하여 호출 시 옵션을 선택적으로 지정할 수 있습니다.
func NewServer(options ...Option) (*Server, error) {
	// 1. 기본값(Default) 설정
	srv := &Server{
		Host:     "localhost",
		Port:     8080,
		Timeout:  30 * time.Second,
		Protocol: "HTTP",
	}

	// 2. 전달된 옵션 함수들을 순차적으로 실행하여 srv 수정
	for _, opt := range options {
		opt(srv)
	}

	// 3. (선택사항) 유효성 검사 등 추가 로직
	if srv.Timeout <= 0 {
		return nil, fmt.Errorf("timeout은 0보다 커야 합니다")
	}

	return srv, nil
}

func main() {
	fmt.Println("=== 서버 설정 예제 ===")

	// 1. 옵션을 전혀 지정하지 않으면 기본값만 적용됩니다.
	s1, _ := NewServer()
	fmt.Printf("1. 기본 서버: Host=%s, Port=%d, Timeout=%v\n", s1.Host, s1.Port, s1.Timeout)

	// 2. 필요한 옵션만 선택적으로 전달합니다.
	// 포트만 9090으로 변경, 나머지는 기본값 유지
	s2, _ := NewServer(
		WithPort(9090),
		WithTimeout(5*time.Second), // 타임아웃도 변경
	)
	fmt.Printf("2. 커스텀 서버: Host=%s, Port=%d, Timeout=%v\n", s2.Host, s2.Port, s2.Timeout)

	// 3. 다양한 옵션을 조합하여 복잡한 설정 생성
	s3, _ := NewServer(
		WithHost("api.example.com"),
		WithPort(443),
		WithProtocol("HTTPS"),
		WithTimeout(10*time.Second),
	)
	fmt.Printf("3. 고급 설정: Host=%s, Port=%d, Proto=%s, Timeout=%v\n",
		s3.Host, s3.Port, s3.Protocol, s3.Timeout)
}