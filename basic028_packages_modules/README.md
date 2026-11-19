> ⚠️ **자동 검증 미통과 — `test` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `--- FAIL: TestInternalPackageAccess (0.12s)`  
> `    main_test.go:75: internal 패키지 import가 허용되면 안 됨`  
> `--- FAIL: TestCircularImport (0.02s)`  
> `    main_test.go:138: 순환 import 오류가 아니함: package testmodule`  
> `--- FAIL: TestInternalPackageAccess (0.12s)`  
> `    main_test.go:75: internal 패키지 import가 허용되면 안 됨`  
> `--- FAIL: TestCircularImport (0.02s)`  
> `    main_test.go:138: 순환 import 오류가 아니함: package testmodule`  

# 패키지 구조와 모듈 시스템: Go의 조직화 철학

## 한 줄 요약
Go는 `internal` 디렉토리로 내부 구현을 감싸고, `go.mod`로 의존성을 명시적·결정적으로 관리하며, 순환 import를 아예 금지해 설계 품질을 강제합니다.

## 왜 필요한가
다른 언어(Python의 `__init__.py`, JavaScript의 `node_modules`, Java의 Maven/Gradle)는 대부분 파일 이름이나 설정 파일로 패키징 경계를 암시적으로 정합니다. Go는 **디렉토리 구조 자체가 버전·보안·컴파일 단위를 정의**합니다. `internal/` 아래에 둔 코드는 모듈 외부에서 아예 import 문 자체가 컴파일 오류가 되도록 막습니다. 이는 "이건 우리 것"이라는 설계 의사를 언어 차원으로 강제하는 선택입니다.

Go Modules(`go.mod`·`go.sum`)은 Python의 `requirements.txt`, Node의 `package.json`과 달리 **버전 잠금·무결성 검증을 표준 도구에 내장**했습니다. `go.sum`에 각 의존성의 해시가 기록되므로, 같은 `go.mod`라도 중간에 파일이 바뀌면 `go build` 단계에서 바로 잡힙니다. "의존성 트러스트"를 프로세스가 아닌 데이터로 검증하는 구조입니다.

순환 import 금지도 비슷한 철학입니다. Java·Python·TypeScript는 모두 순환 참조를 허용하지만, Go는 **컴파일 자체가 실패**하게 합니다. 순환이 생겼다는 것은 설계가 이미 뒤틀렸다는 신호로 보고, 해결책으로 "공통 추상 패키지"를 도입하도록 유도합니다. 편리함보다 구조 건전성을 먼저 지키는 선택입니다.

## 기본 사용
```go
// go.mod 파일
module example.com/myapp

go 1.21
```
```bash
# 모듈 초기화 (첫 번째 명령만 치면 됩니다)
go mod init example.com/myapp
```
```go
// api/public.go — 대문자로 시작하면 외부 패키지에서 import 가능
package api

func Greet(name string) string {
	return "Hello, " + name
}

// 내부 헬퍼 — 소문자라 api 패키지만 접근 가능
func hash(s string) string { return s }
```
```go
// internal/auth/auth.go — internal/ 아래는 모듈 내부에서만 import 가능
package auth

func Validate(token string) bool {
	return len(token) > 0
}
```
```go
// main.go — 내부 패키지도 import 가능 (같은 모듈 내부이므로)
package main

import (
	"fmt"
	"example.com/myapp/api"
	"example.com/myapp/internal/auth"
)

func main() {
	fmt.Println(api.Greet("Go"))
	fmt.Println(auth.Validate("abc"))
}
```
```bash
go run .
```

`internal/` 아래에 둔 코드는 `example.com/other/internal/...` 같은 경로에서 아예 import가 막힙니다. `go build` 시점에 **"cannot find module providing package …/internal/…: go.mod not found"** 같은 메시지가 뜹니다.

`go.mod`는 모듈의 "주소"와 Go 버전을 적어두고, `go get github.com/foo/bar@v1.2.3`하면 해당 버전의 해시가 `go.sum`에 추가됩니다. **의존성 변경 없이 빌드 환경만 바꿔도 `go.sum`이 다르면 경고**가 뜹니다 — 이것이 Go Modules의 핵심 가치입니다.

## 돌려보기
이 디렉토리에서 그대로 실행할 수 있는 명령입니다.

```bash
go vet ./...      # 정적 검사: import 순서, 사용되지 않는 변수 등
go run .          # 시연 실행 — internal 패키지 접근, 공개 규칙 확인
go test -v ./...  # 테스트 — 컴파일 규칙, 내부 패키지 접근 금지, 순환 import 금지 검증
```
동시성 주제는 아니므로 `-race`는 생략합니다. `go test`는 **컴파일 오류가 나는 케이스를 의도적으로 검증**합니다(테스트가 `exec.Command`로 빌드를 시도하고 실패 코드를 체크합니다).

## 실전에서는 이렇게 쓴다
**1. 클린 아키텍처 스타일**
```
myapp/
  cmd/server/main.go   # 진입점
  internal/
    auth/              # 인증 로직 (외부 공개 불가)
    db/                # 데이터베이스 추상화
  pkg/                 # 재사용 가능한 유틸 (외부 공개 가능)
  api/                 # HTTP 핸들러 (공개 API 정의)
  go.mod
```
`internal/`는 "이건 우리 서비스 안에서만 쓰는 거야"라고 선언하는 것입니다. 외부 라이브러리가 `myapp/internal/db`를 import하려 해도 Go 컴파일러가 막습니다.

**2. 다중 모듈 구조**
```
monorepo/
  mod-a/go.mod         # example.com/a
  mod-b/go.mod         # example.com/b, requires example.com/a v1.0.0
  mod-c/go.mod         # example.com/c, requires example.com/a v1.0.0
```
`mod-b`와 `mod-c`가 같은 `mod-a`를 의존할 때, `go.work`로 함께 빌드하거나 `replace` 지시어로 로컬 경로를 가리킵니다. 이는 Python의 `pip install -e`나 Node의 `workspace`와 유사하지만, Go는 **컴파일 단위**로 분리한다는 점이 다릅니다.

## 흔한 실수

### 1. internal 디렉토리 위치를 잘못 둔다

**무엇을 했는지**
```go
// 틀린 코드
// myapp/
//   internal/      ← 모듈 루트 바로 아래에 두면 됨
//   src/
//     internal/    ← 이렇게 두면 외부에서 접근 가능해짐 (보안 실효성 떨어짐)
```
**어떤 일이 벌어지는지**
`src/internal/`는 `src/` 패키지가 속한 서브모듈이 아니면 외부에서 import할 수 있습니다. Go의 `internal` 규칙은 **가장 가까운 `go.mod` 파일 기준으로** 동작합니다. 모듈 루트가 `myapp/`이면 `myapp/internal/`만 내부 패키지가 되고, `myapp/src/internal/`는 `src/`가 독립 모듈이 아닌 한 외부에서 접근 가능합니다.

**왜 그런지**
Go 컴파일러가 `internal`을 찾을 때 **상위 디렉토리부터 `go.mod`를 역추적**하기 때문입니다. `myapp/src/internal/`의 상위에는 `myapp/go.mod`가 있지만, `src/internal`은 `myapp/internal`이 아니므로 규칙이 적용되지 않습니다.

**고친 코드**
```
myapp/
  go.mod
  internal/        # ✅ 모듈 루트 바로 아래
    auth/
  cmd/
    server/
      main.go
```
`internal/`는 **항상 모듈 루트와 동급**에 둡니다. `cmd/`, `internal/`, `pkg/`를 모두 `go.mod`가 있는 디렉토리 바로 아래에 배치하세요.

### 2. 순환 import를 "패키지 분리"로 해결하려 한다

**무엇을 했는지**
```go
// package a — 틀린 코드
package a

import "example.com/b"

func Foo() { b.Bar() }
```
```go
// package b — 틀린 코드
package b

import "example.com/a"

func Bar() { a.Foo() }
```
**어떤 일이 벌어지는지**
```
import cycle not allowed:
  example.com/a imports example.com/b imports example.com/a
```
Go 컴파일러가 **컴파일 시점**에 순환을 감지해 바로 오류를 내고 종료합니다. 실행 시점이 아니라 빌드 단계에서 막힙니다.

**왜 그런지**
패키지 A를 컴파일하려면 패키지 B의 타입 정보와 시그니처가 필요한데, B를 컴파일하려면 다시 A가 필요합니다. 순환이 있으면 어떤 순서로 컴파일해도 순환 참조가 해결되지 않습니다. Go는 이 문제를 **설계 단계에서 막음**으로써, 개발자가 "왜 순환이 생겼는지"를 생각하도록 강제합니다.

**고친 코드**
```go
// package shared — 올바른 해결책
package shared

type Handler interface {
	Handle() string
}
```
```go
// package a
package a

import "example.com/shared"

func New() shared.Handler { return &aImpl{} }
type aImpl struct{}
func (aImpl) Handle() string { return "a" }
```
```go
// package b
package b

import "example.com/shared"

func Use(h shared.Handler) { h.Handle() }
```
`a`와 `b`가 서로를 직접 import하지 않고, **공통 추상 인터페이스**인 `shared` 패키지를 통해 연결됩니다. 이는 Go의 "의존성 역전 원칙"을 언어 차원으로 구현한 사례입니다.

### 3. go.mod 없이 go get을 실행한다

**무엇을 했는지**
```bash
# 틀린 코드
go get github.com/gin-gonic/gin
# go.mod가 없는 디렉토리에서 실행
```
**어떤 일이 벌어지는지**
```
go: go.mod file not found in current directory or any parent directory;
	go get is no longer supported without a go.mod
```
Go 1.16부터 `go get`은 **반드시 `go.mod`가 있는 디렉토리에서만 동작**합니다. 이전에는 `$GOPATH/src`에 무작위로 파일을 만들어 넣었지만, 이제는 모듈 단위로 관리해야 합니다.

**왜 그런지**
GOPATH 기반의 "전역 패키지 저장소" 모델은 버전 충돌과 의존성 불일치를 유발했습니다. Go Modules는 **프로젝트 단위**로 의존성을 격리시켜, 프로젝트 A가 gin v1.0을, 프로젝트 B가 gin v2.0을 쓰더라도 충돌 없이 공존할 수 있게 합니다.

**고친 코드**
```bash
# 1. 모듈 초기화
go mod init example.com/myapp

# 2. 의존성 추가
go get github.com/gin-gonic/gin

# 3. go.sum도 같이 생성됨 (무결성 검증용)
ls go.mod go.sum
```
`go get`은 `go.mod`에 의존성을 추가하고, `go.sum`에 해당 버전의 해시를 기록합니다. **두 파일은 항상 함께 관리**해야 합니다.

### 4. internal 패키지를 외부에 노출하려는 유혹

**무엇을 했는지**
```go
// internal/auth/auth.go
package auth

func GetSecret() string { return "secret" }
```
```go
// main.go — 틀린 코드 (외부 모듈에서 시도)
package main

import "example.com/myapp/internal/auth"  // 컴파일 오류

func main() {
    println(auth.GetSecret())
}
```
**어떤 일이 벌어지는지**
```
imports example.com/myapp/internal/auth:
	use of internal package not allowed
```
**왜 그런지**
Go 컴파일러가 import 경로를 확인해 `internal/`가 모듈 루트보다 아래에 있는지, 그리고 import하는 측이 **같은 모듈 내부**에 있는지를 검사합니다. 외부 모듈(예: `example.com/other`)이 `example.com/myapp/internal/...`을 import하면 무조건 거부됩니다.

**고친 코드**
```go
// api/auth.go — 대신 공개 API로 노출
package api

func Validate(token string) bool {
    // 내부 로직을 공개 함수로 감싸서 제공
    return internalValidate(token)
}

// internal/auth/auth.go — 실제 구현은 내부에 숨김
package auth

func internalValidate(token string) bool {
    return len(token) > 0
}
```
`internal/`에 구현을 두고, **공개 API 패키지를 통해 필요한 기능만 노출**합니다. 이는 "구현 세부사항은 숨기고 인터페이스만 공개한다"는 Go의 설계 철학을 따릅니다.

## 주의할 점
- **internal은 "같은 모듈 내"만 허용합니다.** 같은 레포지토리여도 `go.mod`가 다르면 다른 모듈로 취급됩니다. monorepo에서 `internal`을 쓰려면 모든 서브모듈이 같은 루트 `go.mod`를 공유하거나, `go.work`로 연결해야 합니다.
- **go.mod를 커밋할 때 go.sum도 함께 커밋합니다.** `go.sum`은 의존성의 "무결성 지문"이므로, 이 파일이 없으면 빌드 환경이 달라진다고 판단할 수 없습니다.
- **순환 import는 런타임이 아닌 컴파일 시간에 막힙니다.** "일단 돌아가면 된다"는 마인드셋으로 순환을 두면, 나중에 대규모 리팩터링 시 컴파일이 안 되는 상황이 벌어집니다. 순환이 생긴 순간 디자인을 재검토하세요.
- **대문자/소문자 규칙은 "패키지 외부" 기준입니다.** 같은 패키지 안에서는 소문자 함수도 호출 가능합니다. `package main` 안의 함수들은 서로 공개·비공개 구분이 없습니다.
- **go.mod의 `replace` 지시어는 개발 전용입니다.** `replace example.com/foo v1.0.0 => ../foo`는 로컬 개발 시 유용하지만, 이 지시어를 포함한 `go.mod`를 배포하면 다른 사람이 빌드할 때 문제가 생길 수 있습니다.

## 코드로 확인하기
`main.go`를 실행하면 다음 출력이 나옵니다.

```
=== 1. 대문자 공개 규칙 ===
  - 대문자로 시작: 다른 패키지에서 사용 가능 (공개)
  - 소문자로 시작: 같은 패키지 내부에서만 사용 가능 (비공개)

=== 2. internal 디렉토리 ===
  - internal/ 패키지: 모듈 내부에서만 접근 가능
  - 외부 라이브러리에서 internal 패키지를 import하면 컴파일 오류 발생

=== 3. go.mod와 go.sum ===
  - go.mod: 모듈 경로, Go 버전, 의존성 목록
  - go.sum: 의존성의 해시값으로 무결성 검증

=== 4. 순환 import ===
  - 패키지 A가 B를, B가 A를 import하면 컴파일 오류
  - 해결책: 공통 패키지 C를 만들어 A, B가 모두 C를 사용

=== 테스트: 올바른 모듈 구조 ===
  - api: 공개 함수 노출 (대문자)
  - internal/auth: 내부 인증 로직 (소문자/내부)
  - main: 모듈 진입점
  - go.mod: 모듈 선언

  [성공] 모든 규칙을 따르는 구조는 정상 컴파일됨
```

이 출력에서 확인할 수 있는 것은:
1. `internal/auth` 패키지는 `main.go`에서 import 가능 (같은 모듈 내부)
2. `api` 패키지의 대문자 함수 `PublicFunc()`는 외부에서 호출 가능
3. `go.mod`가 모듈의 경계선을 정의함

`main_test.go`의 테스트는 세 가지를 검증합니다:
- `TestModuleStructure`: 올바른 구조의 모듈이 컴파일되는지
- `TestInternalPackageAccess`: `internal/` 밖에서 `internal` 패키지 import 시 컴파일 오류가 나는지
- `TestCircularImport`: 순환 import 시 컴파일 오류가 나는지

테스트가 **컴파일 오류를 기대하는 케이스**를 포함한다는 점이 중요합니다. `exec.Command`로 `go build`를 실행하고, 실패 코드(`err != nil`)를 검증합니다.

## 연습 문제
1. `myapp/` 아래에 `internal/db/`, `api/handler.go`, `cmd/server/main.go` 구조를 만들고, `internal/db`를 `api`에서 사용하도록 코드를 작성하세요. (힌트: `internal/`는 같은 모듈 내 어디서나 import 가능)
2. 순환 import가 발생하는 두 패키지를 만들고, `go build`로 오류를 확인한 뒤, 공통 패키지 `shared/`를 만들어 순환을 해소하세요. (힌트: 인터페이스를 `shared/`에 두고 `a`, `b`가 각각 구현하게 하세요)
3. `go.mod` 없이 `go get`을 실행했을 때의 오류 메시지를 확인하고, `go mod init` 후 다시 실행하여 `go.sum`이 생성되는 과정을 관찰하세요. (힌트: `ls -la go.*`로 파일 생성 확인)

## 더 읽기
- [Go Modules 공식 문서](https://go.dev/doc/modules/intro) — 모듈 시스템의 철학과 사용법
- [Go Module Layout](https://go.dev/doc/modules/layout) — `internal/`, `cmd/`, `pkg/` 등의 표준 디렉토리 구조
- [Effective Go - Exported Names](https://go.dev/doc/effective_go#mixed-caps) — 대문자/소문자 공개 규칙의 설계 의도
- [Go Blog: Go Modules](https://go.dev/blog/module-support) — Modules 시스템의 역사와 도입 배경