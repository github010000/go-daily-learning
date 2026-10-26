> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `go: finding module for package github.com/stretchr/testify/assert`  
> `go: downloading github.com/stretchr/testify v1.12.1`  
> `go: found github.com/stretchr/testify/assert in github.com/stretchr/testify v1.12.1`  
> `go: downloading go.yaml.in/yaml/v3 v3.0.5`  
> `vet: ./main_test.go:39:10: undefined: fmt`  

# 타입 변환 · 섀도잉 · short variable declaration

## 한 줄 요약
Go 는 암묵적 타입 변환이 없고, `:=` 는 바깥 변수를 가리는 섀도잉을 일으킬 수 있다.

## 왜 필요한가
다른 언어(Python, JavaScript, Java)를 써 온 개발자는 숫자 타입을 자동으로 맞춰주는 것에 익숙하다. Python 에선 `int` 와 `float` 을 더하면 자동으로 `float` 이 되고, JavaScript 에선 숫자 + 문자열이 문자열 합치기로 바뀐다. Java 도 `double` 과 `int` 를 더할 때 자동으로 확대된다.

하지만 Go 는 다르다. 타입이 다르면 **무조건 명시적으로 변환**해야 한다. 이는 Go 설계자가 의도한 것이다. "어디서 어떤 타입으로 바뀌었는지" 코드를 읽는 순간 알 수 있어야 한다는 철학 때문이다. 암묵적 변환이 있으면 버그 원인을 찾느라 타입이 바뀐 곳을 일일이 추적해야 한다. Go 는 그 비용을 컴파일 타임에 던진다. "컴파일이 안 되면 타입이 맞는 것이다"라는 단순한 규칙으로.

`:=`(short variable declaration) 도 같은 맥락이다. Python 의 `=` 나 JavaScript 의 `let` 과 비슷해 보이지만, Go 의 `:=` 는 **변수를 새로 만들거나** **기존 변수에 재할당**하는 두 가지 동작을 모두 한다. 이 dual behavior 가 함정이다. `if` 블록 안에서 `err := doWork()` 를 쓰면, Go 는 `err` 를 `if` 블록 내 로컬 변수로 새로 만든다. 바깥의 `err` 는 그대로 두고, `if` 안의 `err` 만 바뀐다. Python 이나 JavaScript 에선 이런 일이 거의 없으니 Go 를 처음 쓰는 사람이 가장 많이 걸리는 함정 중 하나다.

## 기본 사용

### 명시적 타입 변환
```go
var age int = 30
var price float64 = 19.9

// age + price 는 컴파일 에러
sum := price + float64(age) // float64(age) 로 명시적 변환
```

### 문자열 ↔ 숫자 변환
```go
import "strconv"

num, err := strconv.Atoi("42")     // string → int
str := strconv.Itoa(42)            // int → string
float, err := strconv.ParseFloat("3.14", 64) // string → float64
```

### 섀도잉 피하기
```go
// 고친 버전: 먼저 선언, 나중에 할당
var err error
if err = doWork(); err != nil {
    // err 는 바깥 변수를 사용
}

// 실수 버전: 섀도잉 발생
if err := doWork(); err != nil {
    // 이 err 는 if 블록 내 로컬 변수, 바깥 err 와 무관
}
```

## 돌려보기
```bash
go vet ./...      # 정적 검사 — 타입 오류나 사용되지 않는 변수 확인
go run .          # 시연 실행 — 타입 변환과 섀도잉 출력 비교
go test -v ./...  # 테스트 — 고친 버전이 올바르게 동작하는지 검증
```

## 실전에서는 이렇게 쓴다
Go 코드에서 타입 변환은 주로 **API 경계**에서 일어난다. HTTP 핸들러에서 쿼리 파라미터를 `int` 로, 데이터베이스에서 나온 숫자를 `float64` 로 쓸 때 명시적 변환이 필요하다. `strconv` 패키지는 Go 에서 숫자 ↔ 문자열 변환의 표준이다.

섀도잉은 에러 처리 패턴에서 자주 문제 된다. Go 에서는 에러를 상위로 전파하는 것이 관례인데, `if err := ...; err != nil` 패턴을 남용하면 에러가 블록 안에서만 갱신되어 버린다. 이를 피하려면 `var err error` 를 먼저 선언하거나, 에러가 발생한 곳에서 바로 `return err` 로 처리하는 패턴을 쓴다.

## 흔한 실수

### 1. 타입을 명시하지 않고 연산하기

```go
// 틀린 코드
var a int = 10
var b float64 = 3.14
sum := a + b // 컴파일 에러: mismatched types int and float64
```

Go 컴파일러는 `"mismatched types int and float64"` 오류를 낸다. Python 이라면 자동으로 `float` 이 되겠지만, Go 는 그렇지 않다. 반드시 `float64(a) + b` 또는 `float64(a) + float64(b)` 와 같이 명시적으로 변환해야 한다.

### 2. if 블록 안에서 := 으로 에러 변수 만들기

```go
// 틀린 코드
var err error
if err := doWork(); err != nil {
    // 이 err 는 if 블록 내 로컬 변수
}
// 여기선 err 가 여전히 nil 일 수 있다
fmt.Println(err)
```

`if err := doWork()` 에서 `:=` 가 `err` 를 **새로 선언**한다. 바깥의 `err` 변수는 untouched 다. 이 버그는 컴파일 에러가 아니라 **런타임 논리 오류**라 훨씬 더 위험하다. 코드는 컴파일되고, 에러 처리 로직이 없거나 에러가 nil 인 경우만 테스트하면 걸리지 않는다.

### 3. string(int) 와 string("숫자") 의 차이 이해하기

```go
// 틀린 코드
fmt.Println(string(65))   // 'A' — 유니코드 코드포인트
fmt.Println(string("65")) // 컴파일 에러: cannot use "65" (type string) as type uint in conversion

// 올바른 이해
fmt.Println(string(65))   // 'A' (코드포인트 65)
fmt.Println(string('6'))  // '6' (코드포인트 54)
```

`string(int)` 는 유니코드 코드포인트를 문자로 변환한다. `string("문자열")` 은 컴파일 에러다. 숫자를 문자열로 바꾸려면 `strconv.Itoa(65)` 를 쓰거나 `fmt.Sprintf("%d", 65)` 를 써야 한다.

### 4. := 로 재할당하려는 변수가 없으면 새로 만들어진다

```go
// 틀린 코드
x := 1
if true {
    x := 2 // x 를 새로 만듦 (섀도잉)
    fmt.Println(x) // 2
}
fmt.Println(x) // 1 — 기대했던 2 가 아님
```

`if` 블록 안의 `:=` 는 `x` 를 새로 선언한다. 블록 밖의 `x` 는 변하지 않는다. Python 이나 JavaScript 에선 `x = 2` 로 작성했을 때 같은 결과가 나오지만, Go 의 `:=` 는 다름을 인지해야 한다.

## 주의할 점
- **Go 1.22 이전의 루프 변수 섀도잉**: `for i, v := range xs` 에서 `i`, `v` 가 루프 전체에서 공유되는 이전 Go 버전에서는 루프 내부에서 이 변수들을 goroutine 으로 전달하면 모든 goroutine 이 마지막 값을 사용한다. Go 1.22 이후에는 각 iteration 마다 새로 만들어지지만, 여전히 `for i := range` 에서 `i` 를 goroutine 에서 쓸 때는 복사해서 전달하는 것이 안전하다.
- **float64 정밀도**: `float64` 는 부동소수점이라 `0.1 + 0.2 != 0.3` 일 수 있다. 정확한 비교에는 `math.Abs(a-b) < epsilon` 패턴을 쓰거나 `decimal` 패키지를 사용한다.
- **strconv 오류 처리**: `strconv.Atoi` 는 실패 시 `nil` 이 아닌 에러를 반환한다. 에러를 무시하면 `"0"` 을 반환하는데, 이게 진짜 0 인지 실패한 건지 구분할 수 없다. 항상 에러를 확인한다.

## 코드로 확인하기
`go run .` 을 실행하면 두 섹션이 나온다.

**타입 변환 섹션**에서는:
- "실수 버전: age + price -> 컴파일 에러" 라인이 보여진다. 실제로 이 코드를 컴파일하면 `"mismatched types int and float64"` 오류가 생긴다.
- "고친 버전: 30 + 19.90 = 49.90" 이 보여진다. `float64(age)` 로 명시적 변환했기 때문에 정상 동작한다.
- `string(65)` 가 `'A'` 로 출력된다. 코드포인트 65 가 'A' 라는 것을 확인할 수 있다.

**섀도잉 섹션**에서는:
- 고친 버전: `err = nil` 이 출력된다. `var err error` 로 선언하고 `=` 로 할당했기 때문에 `if` 블록 밖에서도 에러가 전파된다.
- 실수 버전: "시뮬레이션 끝: 바깥 err = 바깥 에러 (섀도잉으로 인해 변경 없음!)" 가 출력된다. `if` 블록 안의 `err := doWork()` 가 바깥 `err` 를 가렸기 때문에 바깥 `err` 는 여전히 "바깥 에러"다.

`go test -v ./...` 는 `TestExplicitConversion` 과 `TestShadowingPattern` 을 실행한다. `TestExplicitConversion` 은 `float64(age) + price` 가 `49.9` 가 되는지, `strconv.Atoi("42")` 가 `42` 를 반환하는지 검증한다. `TestShadowingPattern` 은 고친 버전(`= 사용`)이 `err` 를 nil 로 유지하는지, 실수 버전(`:= 사용`)이 섀도잉으로 바깥 `err` 를 "외부 에러"로 남기는지 검증한다.

## 연습 문제
1. `strconv.ParseFloat` 로 문자열 `"3.14159"` 를 `float64` 로 변환하고, 소수점 둘째 자리까지 반올림해서 출력하는 코드를 작성하라. (`math.Round` 사용)
2. `for i := 0; i < 3; i++` 루프 안에서 `i` 를 goroutine 에 전달할 때, Go 1.22 이전과 이후의 차이를 확인하는 코드를 작성하라. (`go run -race` 와 함께)
3. `if err := doWork(); err != nil` 패턴이 실제로 문제를 일으키는 경우를 시뮬레이션하는 함수를 작성하라. 에러가 발생한 후 바깥 `err` 가 nil 인 채로 남아있는 상황을 보여라.

## 더 읽기
- [Go Tour: Type Conversions](https://go.dev/tour/basics/13) — 공식 튜토리얼
- [Effective Go: Conversions](https://go.dev/doc/effective_go#conversions) — Go 커뮤니티가 추천하는 타입 변환 가이드
- [Go Blog: Declaring Variables](https://go.dev/blog/declaring-variables) — `var`, `:=`, `const` 의 차이와 섀도잉에 대한 깊은 설명