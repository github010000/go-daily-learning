package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// main 함수는 모듈 관리의 핵심 개념을 코드 구조와 출력으로 설명합니다.
func main() {
	// 1. go.mod 파일 생성 예시
	fmt.Println("=== 1. go.mod 파일 생성 (go mod init) ===")
	modContent := `module example.com/myapp

go 1.21

require (
	github.com/pkg/errors v0.9.1
)
`
	fmt.Println("go.mod 파일 내용 예시:")
	fmt.Println(modContent)
	fmt.Println("go mod init example.com/myapp 명령어는 위와 같은 파일을 생성합니다.")
	fmt.Println()

	// 2. go.sum 파일 설명
	fmt.Println("=== 2. go.sum 파일 (의존성 무결성 검증) ===")
	fmt.Println("go mod tidy 실행 시 생성됨")
	fmt.Println("형식: module/hash v1.2.3 h1:hash_value\nmodule/hash v1.2.3 h1:hash_value")
	fmt.Println("목적: 의존성 다운로드 시 해시값 비교하여 파일 변조 방지")
	fmt.Println()

	// 3. 내부 패키지 (internal) 개념
	fmt.Println("=== 3. 내부 패키지 (internal) ===")
	internalPath := "example.com/myapp/internal/database"
	fmt.Printf("패키지 경로 예: %s\n", internalPath)
	fmt.Println("특징: 외부 모듈에서 import 할 수 없음")
	fmt.Println("장점: 모듈 내부 API 안정화, 리팩토링 시 외부 영향 최소화")
	fmt.Println()

	// 4. replace 지시어 사용 예
	fmt.Println("=== 4. Replace 지시어 (의존성 교체) ===")
	fmt.Println("go.mod 파일에 추가:")
	replaceContent := `replace golang.org/x/text => golang.org/x/text v0.3.0-local
replace ./lib/old => ./lib/new`
	fmt.Println(replaceContent)
	fmt.Println("효과: 원격 패키지 버전을 로컬 경로나 특정 버전으로 대체")
	fmt.Println("사용 사례: 의존성 버그 핫픽트, 내부 라이브러리 마이그레이션")
	fmt.Println()

	// 5. 패키지 네이밍 관례
	fmt.Println("=== 5. 패키지 네이밍 관례 ===")
	fmt.Println("1. 소문자 사용: example.com/myapp/user (not user_data)")
	fmt.Println("2. 간결성: user, not user_management")
	fmt.Println("3. 중복 제거: module/user/user -> module/users")
	fmt.Println("4. internal 경로 일치: 외부에서 import 불가 확인")
	fmt.Println()

	// 6. vendor 디렉토리 설명
	fmt.Println("=== 6. vendor 디렉토리 (go mod vendor) ===")
	fmt.Println("명령어: go mod vendor")
	fmt.Println("효과: 의존성 패키지를 프로젝트 내 vendor 폴더로 복사")
	fmt.Println("장점: 인터넷 연결 없이 빌드 가능, CI 환경 안정화")
	fmt.Println("단점: 저장소 크기 증가, 의존성 동기화 수동 관리 필요")
	fmt.Println()

	// 7. 모듈 경로의 중요성
	fmt.Println("=== 7. 모듈 경로 중요성 ===")
	modulePath := "example.com/myapp"
	fmt.Printf("모듈 경로: %s\n", modulePath)
	fmt.Println("패키지 임포트 시 모듈 경로 포함: import \"%s/utils\"", modulePath)
	fmt.Println("모듈 경로가 다르면 완전히 다른 패키지로 간주됨")
}