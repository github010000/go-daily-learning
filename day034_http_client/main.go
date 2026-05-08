package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func main() {
	// 1. 커스텀 HTTP Client 생성 (타임아웃 설정 포함)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// JSONPlaceholder API 엔드포인트 (실제 테스트용 공개 API)
	userURL := "https://jsonplaceholder.typicode.com/users"

	// 2. http.NewRequest를 사용한 정교한 GET 요청
	// 요청 객체를 직접 생성하여 메서드, URL, 바디, 헤더 등을 세밀하게 제어할 수 있습니다.
	req, err := http.NewRequest("GET", userURL, nil)
	if err != nil {
		fmt.Printf("요청 객체 생성 실패: %v\n", err)
		return
	}

	// 3. 헤더 설정
	// 서버가 클라이언트의 종류를 알 수 있도록 User-Agent를 설정합니다.
	req.Header.Set("User-Agent", "Go-HTTP-Client-Day34")
	req.Header.Set("Accept", "application/json")

	fmt.Println("=== 1. http.NewRequest & 헤더 설정 ===")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("GET 요청 실패: %v\n", err)
	} else {
		// 응답 바디는 반드시 닫아줘야 합니다.
		defer resp.Body.Close()

		// 상태 코드 확인
		fmt.Printf("상태 코드: %d\n", resp.StatusCode)

		// 응답 바디 전체 읽기
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("바디 읽기 실패: %v\n", err)
		} else {
			// 버전 호환성을 위해 슬라이스 길이 안전 처리
			printLen := 50
			if len(body) < printLen {
				printLen = len(body)
			}
			fmt.Printf("응답 바디 크기: %d bytes\n", len(body))
			fmt.Printf("응답 바디 시작 부분: %s\n", string(body[:printLen]))
		}
	}

	// 4. http.Get를 사용한 간단한 GET 요청
	fmt.Println("\n=== 2. http.Get를 사용한 간단한 요청 ===")
	resp, err = http.Get(userURL)
	if err != nil {
		fmt.Printf("http.Get 실패: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("http.Get 상태 코드: %d\n", resp.StatusCode)
	}

	// 5. http.Post를 사용한 POST 요청
	fmt.Println("\n=== 3. http.Post를 사용한 데이터 전송 ===")
	postURL := "https://jsonplaceholder.typicode.com/posts"
	payload := strings.NewReader(`{"title": "Go 학습", "body": "net/http 패키지 실습", "userId": 1}`)

	// Content-Type 헤더를 명시적으로 전달합니다.
	resp, err = http.Post(postURL, "application/json", payload)
	if err != nil {
		fmt.Printf("POST 요청 실패: %v\n", err)
	} else {
		defer resp.Body.Close()
		fmt.Printf("POST 상태 코드: %d\n", resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("POST 바디 읽기 실패: %v\n", err)
		} else {
			printLen := 80
			if len(body) < printLen {
				printLen = len(body)
			}
			fmt.Printf("POST 응답 바디 시작 부분: %s\n", string(body[:printLen]))
		}
	}
}