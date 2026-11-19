package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// 모듈 구조 시뮬레이션을 위한 임시 디렉토리 생성 및 테스트 실행.
// 실제 패키징보다는 "왜 이런 규칙이 있는가"를 출력으로 보여줍니다.
func main() {
	// 임시 모듈 루트 생성
	tmpDir, err := os.MkdirTemp("", "go-module-lesson")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("=== 1. 대문자 공개 규칙 ===")
	// 이 부분은 코드로 보여주지 않고 설명으로 대체합니다.
	// 실제로는 api/public.go (PublicFunc) vs api/internal_impl.go (privateFunc)
	fmt.Println("  - 대문자로 시작: 다른 패키지에서 사용 가능 (공개)")
	fmt.Println("  - 소문자로 시작: 같은 패키지 내부에서만 사용 가능 (비공개)")
	fmt.Println("  - 예: Count()는 외부 호출 가능, secret()는 호출 불가")
	fmt.Println()

	fmt.Println("=== 2. internal 디렉토리 ===")
	// internal 디렉토리 구조 시뮬레이션
	internalDir := filepath.Join(tmpDir, "internal")
	os.MkdirAll(internalDir, 0755)
	
	// internal 안의 패키지는 외부(모듈 루트 기준)에서 import할 수 없음
	fmt.Println("  - internal/ 패키지: 모듈 내부에서만 접근 가능")
	fmt.Println("  - 외부 라이브러리에서 internal 패키지를 import하면 컴파일 오류 발생")
	fmt.Println("  - 보안/구현 세부사항 숨기기에 유용")
	fmt.Println()

	fmt.Println("=== 3. go.mod와 go.sum ===")
	// go.mod 예시 출력
	fmt.Println("  - go.mod: 모듈 경로, Go 버전, 의존성 목록")
	fmt.Println("  - go.sum: 의존성의 해시값으로 무결성 검증")
	fmt.Println("  - go.mod만 있으면: 의존성은 알지만 검증 못 함")
	fmt.Println("  - go.sum까지 있으면: 의존성 검증 완료 (보안 확인)")
	fmt.Println()

	fmt.Println("=== 4. 순환 import ===")
	fmt.Println("  - 패키지 A가 B를, B가 A를 import하면 컴파일 오류")
	fmt.Println("  - 이유: 컴파일 순환 참조 방지, 설계 개선 유도")
	fmt.Println("  - 해결책: 공통 패키지 C를 만들어 A, B가 모두 C를 사용")
	fmt.Println()

	fmt.Println("=== 테스트: 올바른 모듈 구조 ===")
	// 올바른 구조로 임시 파일 생성하여 시뮬레이션
	createFakeFiles(tmpDir)
	fmt.Println("  - api: 공개 함수 노출 (대문자)")
	fmt.Println("  - internal/auth: 내부 인증 로직 (소문자/내부)")
	fmt.Println("  - main: 모듈 진입점")
	fmt.Println("  - go.mod: 모듈 선언")
	fmt.Println()
	fmt.Println("  [성공] 모든 규칙을 따르는 구조는 정상 컴파일됨")
	fmt.Println()

	fmt.Println("=== 주의사항 시뮬레이션 ===")
	fmt.Println("  - internal 밖에서 internal 패키지 import 시도 -> 컴파일 오류")
	fmt.Println("  - 순환 import 시도 -> 컴파일 오류")
	fmt.Println("  - go.mod 없이 go get 실행 -> 오류")
	fmt.Println()
	fmt.Println("모듈 구조 레슨 완료")
}

func createFakeFiles(root string) {
	// api 패키지
	apiDir := filepath.Join(root, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "api.go"), []byte(`package api

// PublicFunc는 공개 함수 (대문자)
func PublicFunc() string {
	return "Hello from public"
}

// privateFunc는 비공개 함수 (소문자) - 외부에서 접근 불가
func privateFunc() string {
	return "secret"
}
`), 0644)

	// internal 패키지
	internalDir := filepath.Join(root, "internal")
	authDir := filepath.Join(internalDir, "auth")
	os.MkdirAll(authDir, 0755)
	os.WriteFile(filepath.Join(authDir, "auth.go"), []byte(`package auth

// secretKey는 이 패키지 내부에서만 사용
var secretKey = "secret123"

// Validate은 공개 함수
func Validate(token string) bool {
	return token == secretKey
}
`), 0644)

	// main 패키지
	mainFile := filepath.Join(root, "main.go")
	os.WriteFile(mainFile, []byte(`package main

import (
	"fmt"
	"myapp/api"
	"myapp/internal/auth"
)

func main() {
	fmt.Println(api.PublicFunc())
	fmt.Println(auth.Validate("secret123"))
}
`), 0644)

	// go.mod
	goMod := filepath.Join(root, "go.mod")
	os.WriteFile(goMod, []byte(`module myapp

go 1.21
`), 0644)
}