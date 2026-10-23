> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `main.go:99:2: fmt.Println call has possible Printf formatting directive %s`  

# Go 도구와 첫 프로그램

## 한 줄 요약
`go mod init`, `go run`, `go build`, `go fmt`, `go vet` 등 Go의 표준 도구를 사용하여 프로젝트를 설정하고 빌드하며 코드를 정리하는 방법을 배웁니다.

## 왜 필요한가

Go는 다른 언어와 달리 **도구 체계가 표준**입니다. Python은 `black`, `flake8`, `pytest` 등 여러 도구를 별도로 설치해야 하지만, Go는 `gofmt`, `go vet`, `go test` 등이 모두 표준 도구에 포함되어 있습니다. 이는 **모든 환경에서 동일한 결과를 보장**하고, **팀 협업 시 스타일 논의를 없애는** 장점이 있습니다.

특히 `go mod`는 Python의 `requirements.txt`나 Node.js의 `package.json`과 유사하지만 **표준 도구에 통합**되어 있어 별도 패키지 매니저가 필요 없습니다. 또한 `package main`과 `func main`은 Go가 **실행 가능한 프로그램을 명시적으로 정의**하는 방식입니다. Java의 `public static void main(String[] args)`와 달리, Go는 패키지와 함수를 분리하여 더 명확한 구조를 가집니다.

## 기본 사용

### 프로젝트 초기화
```bash
go mod init example.com/myproject
```
`go.mod` 파일을 생성하여 모듈 이름을 지정합니다. 이 이름은 의존성 관리의 기준이 됩니다.

### 코드 실행
```bash
go run main.go
```
컴파일과 실행을 한 번에 처리합니다. 개발 중 빠르게 코드를 테스트할 때 사용합니다.

### 바이너리 생성
```bash
go build -o myapp
```
`myapp`이라는 이름의 실행 파일을 생성합니다. 배포용으로 사용할 때 사용합니다.

### 코드 포맷팅
```bash
go fmt ./...
```
모든 Go 파일을 표준 스타일로 정리합니다. 한 번 실행하면 모든 파일이 동일한 포맷을 따릅니다.

### 정적 분석
```bash
go vet ./...
```
잠재적 버그를 검색합니다. `fmt.Printf`의 포맷 문자열과 인수 불일치 등을 감지합니다.

### 테스트 실행
```bash
go test ./...
```
모든 테스트 파일을 찾아 실행합니다.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

- `go vet`: 코드에 잠재적 버그가 없음을 확인
- `go run`: 프로그램이 정상 실행되는지 확인
- `go test -v`: 테스트가 통과하고 상세 출력이 생성되는지 확인

## 실전에서는 이렇게 쓴다

1. **새 프로젝트 시작 시**: `go mod init`으로 모듈을 만들고, `go run .`으로 빠르게 테스트합니다.
2. **배포 전**: `go build -o app`으로 바이너리를 만들고, `go vet ./...`으로 버그를 점검합니다.

## 흔한 실수

### 1. `go.mod` 없이 코드 실행 시도

```go
// 틀린 코드
package main

import "fmt"

func main() {
    fmt.Println("Hello")
}
```
```bash
go run main.go
# error: go.mod file not found in current directory or any parent directory
```
Go 1.16부터는 `go.mod`가 있는 디렉토리에서만 코드를 실행해야 합니다. `go mod init`으로 모듈을 먼저 생성하세요.

### 2. `fmt.Printf` 포맷 문자열과 인수 불일치

```go
// 틀린 코드
func printName(name string) {
    fmt.Printf("안녕, %s\n") // 인수가 부족
}
```
```bash
go vet .
# ./main.go:5: printf: fmt.Printf format %s has no argument
```
`fmt.Printf`의 포맷 문자열과 인수를 일치시켜야 합니다.

### 3. `package main` 없이 `main` 함수 정의

```go
// 틀린 코드
package mylib

func main() {
    fmt.Println("Hello")
}
```
```bash
go run .
# error: main function must be in package main
```
실행 가능한 프로그램은 반드시 `package main`이어야 합니다.

### 4. import 순서 오류

```go
// 틀린 코드 (go fmt가 자동으로 정리)
package main

import "fmt"
import "os"

func main() {
    fmt.Println("Hello")
    os.Exit(0)
}
```
```bash
go fmt .
```
`go fmt`가 import 블록을 자동으로 정리합니다.

## 주의할 점

1. **`go.mod` 필수**: Go 1.16부터는 `go.mod`가 없는 디렉토리에서 코드를 실행할 수 없습니다.
2. **`go fmt`는 강제**: 스타일 논의 대신 도구가 강제합니다. 커밋 전에 `go fmt`를 실행하세요.
3. **`go vet`는 컴파일 오류가 아님**: `go vet`는 경고만 출력하고 빌드를 막지 않습니다. 하지만 잠재적 버그를 찾아주므로 반드시 실행하세요.
4. **Windows 경로 구분자**: Windows에서는 `filepath.Join`을 사용하여 경로 생성 시 OS 독립적으로 처리하세요.

## 코드로 확인하기

`main.go`를 실행하면 다음 출력이 표시됩니다:

1. **도구체계 개요**: 주요 명령어 설명
2. **`go fmt` 예시**: 포맷 전/후 코드 비교
3. **`go vet` 예시**: 버그 감지 시연
4. **`go build` vs `go run`**: 차이점 설명
5. **`go mod` 중요성**: 모듈 관리 설명
6. **`package main`**: 실행 가능한 프로그램 구조 설명

`main_test.go`는 다음을 검증합니다:
- `go fmt`가 코드를 표준 스타일로 포맷팅하는지
- `go build`가 실행 가능한 바이너리를 생성하는지
- `go test`가 테스트를 실행하는지
- `go vet`가 버그를 감지하는지
- `go.mod`에 필수 문구가 있는지

## 연습 문제

1. **쉬움**: `go mod init`으로 새 모듈을 만들고, `go run`으로 "Hello, World!"를 출력하세요.
2. **중간**: `fmt.Printf`의 포맷 문자열과 인수를 잘못 맞춰서 `go vet`가 감지하도록 만든 후, 고치세요.
3. **어려움**: `go build`로 바이너리를 생성하고, `go test`로 테스트를 실행하는 스크립트를 작성하세요.

## 더 읽기

- [Go 문서 - 튜토리얼](https://go.dev/doc/tutorial/getting-started)
- [Go 명령어 참조](https://go.dev/cmd/)
- [Go 모듈 문서](https://go.dev/doc/modules/managing-dependencies)