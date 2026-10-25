> ⚠️ **자동 검증 미통과 — `readme` 단계.** 본문 설명은 그대로 유효하지만, README 의 코드 조각 일부가 그대로는 컴파일되지 않습니다(main.go·테스트는 통과).
>
> `README 코드 조각 #2 (절: iota 와 0 값을 비워두는 이유) — 컴파일 오류:`  
> `./main.go:8:2: syntax error: unexpected ..., expected name`  

## 한 줄 요약

iota 는 `const` 블록 안에서 0, 1, 2 … 으로 올라가는 자동 증가 상수이고, Go 의 리터럴은 타입이 없어서(`int`나 `string` 등 타입 없는 상수) 변수에 할당하는 순간 타입이 결정됩니다.

## 왜 필요한가

### 타입 없는 상수가 왜 편한가

Python · JavaScript · Java 에서는 숫자 리터럴이 모두 해당 타입으로 고정됩니다. Python 에는 `int`와 `float`가 다르고, Java 는 `int` 리터럴을 `byte` 변수에 바로 넣으려면 캐스트가 필요합니다.

Go 는 리터럴을 **'타입 없는 상수'**로 다룹니다. `100`이라는 리터럴은 `int`도 아니고 `byte`도 아닙니다.

```go
const limit = 100
var b byte = byte(limit)
var i int = limit
```

`limit`이 변수에 할당되는 순간 타입이 정해집니다. `byte`에 쓰면 `byte`, `int`에 쓰면 `int`가 됩니다. 이 때문에:

- 한 상수를 다양한 타입에 재사용할 수 있습니다
- `byte + int` 같은 타입 불일치 문제가 발생하지 않습니다
- 리터럴 그대로 연산에 넣어도 타입 추론이 작동합니다

이것이 Go 가 설계한 방식의 핵심입니다. **'타입 없는 상수'는 컴파일 시점에 타입이 정해지는 값**이지, 런타임에 타입이 바뀌는 변수가 아닙니다.

### iota 와 0 값을 비워두는 이유

C 나 C++ 에서는 열거형 값이 구현에 따라 달라질 수 있습니다. Go 는 `iota`를 통해 값이 명확하게 정해집니다.

하지만 `iota`를 쓸 때 중요한 점은 **0 값을 어떻게 사용하느냐**입니다.

```go
const (
	Sunday    DayOfWeek = iota // 0
	Monday                     // 1
	...
)
```

이렇게 0 을 `Sunday`로 두면, `var d DayOfWeek` 나 `new(DayOfWeek)` 로 만든 값이 0 이 되어도 문제가 없습니다 — 0 은 이미 'Sunday'이니까요.

반면에:

```go
const (
	BadMonday    BadDayOfWeek = iota // 0
	BadTuesday                         // 1
	...
)
```

처럼 0 을 첫 번째 실체 값에 두면, `var d BadDayOfWeek` 는 `BadMonday` 가 됩니다. 이것은 **의도하지 않은 초기값**입니다.

Go 에서 `var` 나 `new()` 로 만든 값은 항상 영값(zero value)을 가집니다. 정수 타입이면 0 이고, 포인터이면 nil 이고, 슬라이스는 nil 입니다.

그래서 열거형에서 0 을 **'알 수 없음'**이나 **'미정의 상태'**로 비워두는 것이 관례입니다.

```go
const (
	Unknown DayOfWeek = iota // 0 — 미정의
	Sunday                   // 1
	Monday                   // 2
	...
)
```

이렇게 하면 `var d DayOfWeek` 는 0(`Unknown`) 이 되고, 이것은 '아직 결정되지 않은 요일'이라는 의미로 일관되게 해석됩니다.

## 기본 사용

### 타입 없는 상수

```go
const limit = 100

var b byte = byte(limit) // byte(100)
var i int = limit        // int(100)
var u uint = uint(limit) // uint(100)

// 타입 없는 상수는 리터럴처럼 쓸 수 있습니다.
sum := limit + 50 // 타입 없는 상수 + 정수 리터럴 -> 타입 없는 상수
```

리터럴이 타입 없는 상수이므로 `limit + 50` 은 타입 없는 상수가 됩니다. 이것을 변수에 할당하면 변수 타입에 맞춰 저장됩니다.

### iota 로 열거형 만들기

```go
type DayOfWeek int

const (
	Sunday    DayOfWeek = iota // 0
	Monday                     // 1
	Tuesday                    // 2
	Wednesday                  // 3
	Thursday                   // 4
	Friday                     // 5
	Saturday                   // 6
)
```

`iota` 는 `const` 블록 안에서 0 에서 시작해 1, 2, 3 … 으로 올라갑니다. 각 줄에 `iota` 를 명시하지 않아도 이전 줄의 `iota` 값을 계속 사용합니다.

### iota 는 블록당 리셋

```go
const (
	a = iota // 0
	b        // 1
)

const (
	c = iota // 0 — 리셋!
	d        // 1
)
```

`iota` 는 전역 카운터가 아닙니다. 각 `const` 블록이 열릴 때마다 0 으로 돌아갑니다.

### 1 << iota 로 비트 플래그 만들기

```go
type Color int

const (
	Red   Color = 1 << iota // 1 (0b001)
	Green                   // 2 (0b010)
	Blue                    // 4 (0b100)
)

combined := Red | Green // 3 (0b011)
hasRed   := combined & Red != 0    // true
hasBlue  := combined & Blue != 0   // false
```

`1 << iota` 는 1, 2, 4, 8, 16 … 으로 올라가서 각 비트가 독립적인 플래그가 됩니다. OR 로 합치고 AND 로 확인할 수 있습니다.

### String() 메서드로 열거형 문자열 변환

```go
func (d DayOfWeek) String() string {
	switch d {
	case 0:
		return "일"
	case 1:
		return "월"
	// ...
	default:
		return "알 수 없음"
	}
}
```

`fmt.Println` 은 `String() string` 메서드가 있으면 자동으로 호출합니다. `strconv.Itoa` 같은 함수를 쓸 필요가 없습니다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령입니다. 각 명령에서 무엇을 봐야 하는지 아래에 적었습니다.

```bash
go vet ./...      # 정적 검사 — import 오류 없이 통과해야 합니다
go run .          # 시연 실행 — 출력 결과로 타입 없는 상수와 iota 동작을 확인합니다
go test -v ./...  # 테스트 — 모든 테스트가 통과해야 합니다
```

`go run .` 출력에서 특히 확인할 것:

1. **"타입 없는 상수 유연성"** 섹션에서 `limit=100` 이 `byte`, `int`, `uint` 에 각각 올바르게 저장됩니다
2. **"iota: 블록당 리셋"** 섹션에서 `a=0, b=1, c=0, d=1` 로 나옵니다 — `iota` 가 블록마다 리셋됩니다
3. **"0 을 '월요일'로 잘못 쓴 예"** 에서 `var BadDayOfWeek{}` 가 `0 (BadMonday)` 로 나옵니다 — 의도하지 않은 초기값
4. **"0 을 '알 수 없음'으로 비워둔 예"** 에서 `var DayOfWeek{}` 가 `0 (Sunday=알 수 없음)` 으로 나옵니다 — 안전한 시작점

동시성 주제가 아니므로 `-race` 는 필요하지 않습니다.

```bash
go test -race ./...  # race detector — 정상적으로 통과합니다
```

## 실전에서는 이렇게 쓴다

### 1. HTTP 상태 코드 정의

표준 라이브러리 `net/http` 에서도 같은 패턴을 씁니다.

```go
type StatusCode int

const (
	StatusOK           StatusCode = 200
	StatusNotFound     StatusCode = 404
	StatusInternalServerError StatusCode = 500
)
```

HTTP 상태 코드는 0 이 '알 수 없음'이 아니라 실제 코드와 겹치지 않도록 0 을 비워두는 것이 안전합니다.

### 2. 파일 권한 모드

```go
type FileMode int

const (
	ModeRead   FileMode = 1 << iota // 1
	ModeWrite                       // 2
	ModeExecute                     // 4
)
```

`1 << iota` 패턴은 시스템 프로그래밍에서 매우 흔합니다. POSIX 파일 권한, 네트워크 플래그, 데이터베이스 쿼리 옵션 등 모든 곳에서 볼 수 있습니다.

### 3. 상태 머신 상태값

```go
type State int

const (
	StateIdle      State = iota // 0
	StateRunning                // 1
	StatePaused                 // 2
	StateDone                   // 3
)
```

`StateIdle` 를 0 으로 두면 `var s State` 가 자동으로 `StateIdle` 이 되어 상태 초기화가 필요합니다 없습니다.

## 흔한 실수

### 1. import 를 쓰지 않으면 컴파일 오류가 납니다

Python 은 쓰지 않는 import 를 경고만 하고 지나갑니다. Go 는 **컴파일 오류**입니다.

```go
// 틀린 코드
package main

import (
	"fmt"
	"strconv"
)

func main() {
	fmt.Println("hello")
}
```

`strconv` 를 쓰지 않았는데 import 했습니다. Go 는 이렇게 합니다:

```
./main.go:5:2: strconv imported and not used
```

`go vet` 도 같은 오류를 잡습니다.

**고친 코드:**

```go
package main

import "fmt"

func main() {
	fmt.Println("hello")
}
```

쓰지 않는 import 는 지우세요. `go vet` 가 알려주면 바로 고칠 수 있습니다.

### 2. iota 를 const 블록 바깥에서 씁니다

```go
// 틀린 코드
package main

import "fmt"

func main() {
	iota := 1 // iota 는 예약어이므로 변수 이름으로 쓸 수 없습니다
	fmt.Println(iota)
}
```

`iota` 는 예약어로, 변수 · 상수 · 타입 이름으로 쓸 수 없습니다.

```
./main.go:5:2: cannot use iota as value: untyped int constant needs type
```

또는:

```
./main.go:5:2: illegal use of iota
```

**고친 코드:**

```go
package main

import "fmt"

const a = iota // const 블록 안에서만 사용합니다
const b = iota // 각 블록마다 리셋

func main() {
	fmt.Println(a, b) // 0 0
}
```

`iota` 는 `const` 블록 안에서만 의미가 있습니다. 블록이 끝나면 사라집니다.

### 3. 0 을 실체 값에 두면 var 로 만든 값이 의도치 않게 초기화됩니다

```go
// 틀린 코드
type Status int

const (
	StatusActive   Status = iota // 0
	StatusPending                // 1
	StatusDone                   // 2
)
```

```go
var s Status // 0 — StatusActive 가 됩니다!
```

`var s Status` 는 0 인데, 0 은 `StatusActive` 입니다. 이것은 의도한 동작일 수도 있고 아닐 수도 있습니다. 만약 0 을 '미정의 상태'로 두고 싶었다면 문제가 됩니다.

```
var s Status = 0 (StatusActive)
```

**고친 코드:**

```go
type Status int

const (
	StatusUnknown Status = iota // 0 — 미정의 상태
	StatusActive                // 1
	StatusPending               // 2
	StatusDone                  // 3
)

var s Status // 0 (StatusUnknown) — 안전한 시작점
```

0 을 '알 수 없음'으로 두면 `var` 나 `new()` 로 만든 값이 항상 유효한 상태로 시작됩니다.

### 4. 타입 없는 상수를 타입이 붙은 변수에 무작정 넣습니다

```go
// 틀린 코드
var b byte = 300 // 300 은 byte 범위를 벗어납니다
```

타입 없는 상수 300 을 `byte` 변수에 할당할 수 없습니다. `byte` 는 0~255 입니다.

```
./main.go:5:16: constant 300 overflows byte
```

**고친 코드:**

```go
var b byte = 100 // byte 범위 내에서
var i int = 300  // int 는 300 을 저장할 수 있습니다
```

타입 없는 상수는 범위도 체크됩니다. 타입에 맞는 값만 할당할 수 있습니다.

### 5. iota 와 비트 시프트를 쓸 때 괄호를 생략합니다

```go
// 틀린 코드
const (
	Bit0 = 1 << iota // 1
	Bit1             // 2
	Bit2             // 4
)
```

이것은 사실 괜찮은 코드입니다. 하지만:

```go
// 틀린 코드 — 의도한 것과 다르게 동작할 수 있습니다
const (
	A = iota << 2 // 0, 4, 8 ... ?
	B             // ?
	C             // ?
)
```

`iota << 2` 를 쓰면 `iota` 가 먼저 평가된 후 시프트됩니다. `1 << iota` 를 쓰는 것이 더 일반적입니다:

```go
const (
	A = 1 << iota // 1, 2, 4 ...
	B             // 2
	C             // 4
)
```

`1 << iota` 는 `iota` 가 0, 1, 2 일 때 각각 1, 2, 4 가 됩니다. `iota << 2` 는 0, 4, 8 이 됩니다.

**고친 코드:**

```go
const (
	A = 1 << iota // 1
	B             // 2
	C             // 4
)
```

## 주의할 점

### 타입 없는 상수는 런타임에 타입이 바뀌지 않습니다

타입 없는 상수는 컴파일 시점에 타입이 결정됩니다. 한번 할당된 타입은 바뀝니다.

```go
const x = 100
var b byte = byte(x) // b 는 byte(100)
var i int = x        // i 는 int(100)
```

`x` 자체가 `byte` 나 `int` 가 되는 것이 아니라, `x` 를 변수에 할당할 때 변수의 타입에 맞춰 저장됩니다.

### iota 는 const 블록 안에서만 동작합니다

`const` 블록 바깥에서 `iota` 를 사용하면 컴파일 오류가 납니다.

```go
const (
	a = iota // 0
)
// iota = 1 // 오류: illegal use of iota
```

### Go 1.22 이전과 이후의 루프 변수 동작

Go 1.22 이전에는 `for` 루프의 변수가 반복마다 같은 메모리 위치를 가리켰습니다. 1.22 이후에는 각 반복마다 새 변수가 생성됩니다.

```go
// Go 1.22 이전: 모든 goroutine 이 같은 마지막 값을 공유할 수 있습니다
var funcs []func()
for i := 0; i < 3; i++ {
	funcs = append(funcs, func() { fmt.Println(i) })
}

// Go 1.22 이후: 각 goroutine 이 자신의 i 를 가집니다
for i := 0; i < 3; i++ {
	funcs = append(funcs, func() { fmt.Println(i) })
}
```

이것은 상수 · iota 와는 직접적인 관련은 없지만, Go 를 배우는 과정에서 자주 마주치는 변화입니다.

## 코드로 확인하기

`go run .` 을 실행하면 다음과 같은 출력이 나옵니다.

**타입 없는 상수 유연성:**
```
=== 타입 없는 상수 유연성 ===
limit=100 -> byte: 100, int: 100, uint: 100
limit + 50 = 150 (타입 없는 상수)
```
`limit=100` 이 `byte`, `int`, `uint` 에 모두 올바르게 저장됩니다. 타입 없는 상수는 변수에 할당되는 순간 타입이 정해집니다.

**iota: 블록당 리셋:**
```
=== iota: 블록당 리셋 ===
a=0, b=1, c=0, d=1
iota 는 const 블록 안에서만 0, 1, 2... 로 올라갑니다.
```
두 개의 `const` 블록이 있고, 각 블록에서 `iota` 가 0 에서 시작합니다. `iota` 는 전역 카운터가 아닙니다.

**0 을 '월요일'로 잘못 쓴 예:**
```
=== 0 을 '월요일'로 잘못 쓴 예 ===
var BadDayOfWeek{} = 0 (BadMonday)
0 = BadMonday 인데, new() 로 만들면 월요일이 됩니다.
의도하지 않은 초기값이 생깁니다.
```
`var BadDayOfWeek{}` 는 0 인데, 0 은 `BadMonday` 입니다. `new()` 나 `var` 로 만든 값이 의도하지 않은 초기값을 가집니다.

**0 을 '알 수 없음'으로 비워둔 예:**
```
=== 0 을 '알 수 없음'으로 비워둔 예 ===
var DayOfWeek{} = 0 (Sunday=알 수 없음)
0 은 '미정의 상태'. new() 로 만들어도 안전한 시작점입니다.
Monday=1, Wednesday=3, Saturday=6
Monday.String()=월, Saturday.String()=토
```
`var DayOfWeek{}` 는 0 인데, 0 은 `Sunday`(알 수 없음) 입니다. 이것은 안전한 시작점입니다.

**비트 플래그:**
```
=== 1 << iota 로 비트 플래그 만들기 ===
Red   = 1 (1)
Green = 2 (2)
Blue  = 4 (4)
Red|Green = 3 (11)
Red|Green has Red? true
Red|Green has Green? true
Red|Green has Blue? false
```
`1 << iota` 패턴으로 비트 플래그를 만들면 OR 로 합치고 AND 로 확인할 수 있습니다.

`main_test.go` 는 다음을 검증합니다:

- `TestDayOfWeekIotaValues`: 요일 열거형의 iota 값이 0~6 인지
- `TestDayOfWeekZeroValueIsSafe`: `var DayOfWeek{}` 가 0 인지
- `TestDayOfWeekStringMethod`: `String()` 메서드가 올바른 이름을 반환하는지
- `TestUntypedConstantTypeResolution`: 타입 없는 상수가 다양한 타입에 할당되는지
- `TestIotaResetsPerBlock`: `iota` 가 블록당 리셋되는지
- `TestBitFlagsCombine`: 비트 플래그가 OR 조합으로 동작하는지

## 연습 문제

### 문제 1: 색 조합 열거형 만들기

`Red`, `Green`, `Blue` 를 비트 플래그로 정의하고, `Yellow`(`Red | Green`), `Cyan`(`Green | Blue`), `Magenta`(`Red | Blue`), `White`(`Red | Green | Blue`) 를 추가하세요. `hasColor(combined, Red)` 같은 함수를 만들어 어떤 색이 포함되어 있는지 확인하는 코드를 작성하세요.

힌트: `combined & Red != 0` 으로 비트를 확인할 수 있습니다.

### 문제 2: 타입 없는 상수를 활용한 유틸리티

타입 없는 상수 `PI = 3.141592653589793` 를 정의하고, 이를 `float32`, `float64`, `int`(정수 부분만) 변수에 할당하는 코드를 작성하세요. 각 변수의 값을 출력하고, `int` 로 변환할 때 소수점이 잘리는 것을 확인하세요.

힌트: `int(PI)` 로 정수 부분을 추출할 수 있습니다.

### 문제 3: 상태 머신 구현

`State` 열거형(`Idle`, `Running`, `Paused`, `Done`) 을 `iota` 로 정의하고, `String()` 메서드를 구현하세요. `var s State` 를 만들고 초기값이 `Idle` 인 것을 확인한 후, `s = Running`, `s = Paused`, `s = Done` 으로 바꾸면서 `String()` 출력이 어떻게 변하는지 확인하는 코드를 작성하세요.

힌트: `s.String()` 으로 현재 상태를 문자열로 확인할 수 있습니다.

## 더 읽기

- [Go Specification: Constants](https://go.dev/ref/spec#Constants) — 타입 없는 상수와 iota 에 대한 공식 문서
- [Go Blog: Go Constants](https://go.dev/blog/constants) — 타입 없는 상수에 대한 깊이 있는 설명
- [Effective Go: Constants](https://go.dev/doc/effective_go#constants) — 상수를 효과적으로 쓰는 방법