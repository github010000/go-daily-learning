> ⚠️ **자동 검증 미통과 — `readme` 단계.** 본문 설명은 그대로 유효하지만, README 의 코드 조각 일부가 그대로는 컴파일되지 않습니다(main.go·테스트는 통과).
>
> `README 코드 조각 #16 (절: 주의할 점) — 컴파일 오류:`  
> `./main.go:6:3: no new variables on left side of :=`  

## 한 줄 요약

Go 에서 변수는 `var` 로 선언하거나 `:=` 로 단축 선언하며, 초기화하지 않아도 제로값이 자동 주입되고, `string` 은 한 번 만들면 절대 바뀌지 않는다.

## 왜 필요한가

Go 는 "명시적임이 암시적임보다 낫다"는 철학으로 변수와 타입을 다룬다.

다른 언어(Python, JavaScript)에서는 변수 선언에 `var`, `let`, `const` 가 있고, 타입 추론이 유연하다. Java 는 `int i = 0;`처럼 타입을 항상 명시한다. Go 는 이 중간쯤에 위치한다: `var i int` 로 타입을 명시할 수도 있고, `i := 10` 으로 추론도 허용한다. 하지만 **초기화하지 않으면 무조건 제로값**이 들어간다. `undefined`(JavaScript) 나 `nil pointer` 로 인한 런타임 오류를 사전에 차단하려는 의도다.

또한 Go 는 **"선언한 변수를 쓰지 않으면 컴파일 오류"**다. Python 은 경고만 하고, Java/Go 는 오류다. 이는 "쓰지 않는 코드는 버그의 씨앗"이라는 Go 팀의 신념 때문이다. 초보자에게는 "왜 안 되지?"라는 질문을 자주 일으키는 부분이지만, 장기적으로 코드 품질을 높여준다.

`string` 도 마찬가지다. Python/JavaScript 의 `str` 은 mutable 이지만, Go 의 `string` 은 immutable 다. "왜 바꿀 수 없지?"라는 의문을 자주 일으키는데, 이는 메모리 효율성과 문자열 리터럴 공유(gofmt 가 권장하는 패턴)를 위해 설계된 선택이다.

## 기본 사용

`var` 와 `:=` 는 선언 방식이 다르다.

```go
var age int = 30          // 타입 명시적, 초기값 생략 시 0
name := "고고"            // 타입 추론, string
var score float64         // 타입 명시, 초기값 생략 시 0.0
```

기본 타입의 제로값:

| 타입 | 제로값 |
|------|--------|
| `int` | `0` |
| `float64` | `0.0` |
| `bool` | `false` |
| `string` | `""` (빈 문자열) |

`string` 은 인덱스로 읽을 수 있지만, **대입은 불가**하다:

```go
s := "hello"
fmt.Println(s[0]) // 'h' 읽기는 가능
// s[0] = 'H'     // 컴파일 오류: cannot assign to s[0]
```

쓰지 않는 변수를 막으려면 `_` 를 사용한다:

```go
var _ = 42 // 무시하겠다는 의도 표시
```

## 돌려보기

```bash
go vet ./...      # 정적 검사 — 변수 사용, 타입 문제 등 확인
go run .          # 시연 실행 — var/:= 차이, 제로값, string 불변성 출력
go test -v ./...  # 테스트 — 제로값, string 불변성 검증
```

## 실전에서는 이렇게 쓴다

**1) CLI 도구에서 설정 값 초기화**

```go
var config struct {
	host string
	port int
}
// config.host 는 "" (빈 문자열), config.port 는 0
// 사용자가 플래그로 지정하지 않았을 때 제로값으로 안전하게 시작
flag.StringVar(&config.host, "host", "localhost", "서버 호스트")
flag.IntVar(&config.port, "port", 8080, "서버 포트")
```

**2) HTTP 핸들러에서 요청 파싱**

```go
func handler(w http.ResponseWriter, r *http.Request) {
	// q := r.URL.Query().Get("q") — 단축 선언으로 타입 추론
	// 만약 q 를 나중에 쓰지 않으면 컴파일 오류가 나므로, 의도적으로 쓰지 않는 코드를 방지
	_ = q // 쓰지 않겠다는 명시적 의도
	// ...
}
```

## 흔한 실수

### 1. `:=` 로 재선언해서 오류

```go
// 틀린 코드
name := "고고"
name := "바보" // 컴파일 오류: no new variable on left side of :=
```

`:=` 는 **동일 스코프에서 같은 변수를 재선언하지 못한다**. `name` 이 이미 선언되었으므로 두 번째 `:=` 는 오류다.

```go
// 고친 코드
name := "고고"
name = "바보" // 재할당은 = 만으로
```

### 2. 초기화하지 않은 변수를 기대해서 panic

```go
// 틀린 코드
var name string
fmt.Println(len(name)) // 0 — 하지만 nil pointer 기대하고 쓰면?
// map 을 사용하는데 nil map 은 panic: assignment to entry in nil map
m := map[string]int{} // nil map 이 아니다, 빈 map 이다
// var m map[string]int // nil map — 여기에 쓰면 panic
m["a"] = 1 // 정상
```

Go 는 변수를 선언하면 무조건 제로값을 준다. `map`, `slice`, `channel`, `pointer` 등은 제로값이 `nil` 이다. `nil` map 에 쓰면 panic 한다.

```go
// 고친 코드
var m map[string]int       // nil map — 여기서 쓰면 panic
m = make(map[string]int)   // make 로 할당 후 사용
m["a"] = 1 // 이제 안전
```

### 3. `string` 을 인덱스로 대입해서 오류

```go
// 틀린 코드
s := "hello"
s[0] = 'H' // 컴파일 오류: cannot assign to s[0]
```

`string` 은 **읽기 전용**이다. 바이트 단위로 접근은 가능하지만 대입은 불가하다.

```go
// 고친 코드
s := "hello"
b := []byte(s) // []byte 로 변환
b[0] = 'H'
newS := string(b) // 다시 string 으로
fmt.Println(newS) // "Hello"
fmt.Println(s)    // "hello" — 원본은 불변
```

### 4. `var` 와 `:=` 혼동해서 타입이 기대와 다름

```go
// 틀린 코드
var f = 3.14       // float64
g := 3.14          // float64 (같음)
h := 3             // int
var i float64 = 3  // 3.0 — 명시적 타입 변환 필요
```

`:=` 는 우변 타입을 추론한다. `3` 은 `int`, `3.14` 는 `float64` 다. `var` 로 선언하면 타입을 명시해야 한다.

```go
// 고친 코드
var f float64 = 3.14
var i float64 = float64(3) // 명시적 변환
```

### 5. 쓰지 않는 변수를 두고 컴파일 오류

```go
// 틀린 코드
var unused int
fmt.Println("이 아래는 컴파일 오류")
```

Go 는 **"선언한 변수를 쓰지 않으면 컴파일 오류"**다. Python 은 경고만 하고, Java/Go 는 오류다.

```go
// 고친 코드
var _ = 42 // _ 에 버리면 허용
// 또는 실제로 사용
var used int = 42
fmt.Println(used)
```

## 주의할 점

**1. `:=` 는 같은 패키지 내 재선언 불가**

```go
x := 1
x := 2 // 오류: no new variables on left side
```

하지만 **다른 스코프**에서는 가능하다:

```go
{
	x := 1
}
x := 2 // 허용 — 다른 스코프
```

**2. `string` 은 UTF-8 바이트 시퀀스**

```go
s := "안"
fmt.Println(len(s)) // 3 — 바이트 단위 길이
fmt.Println(len([]rune(s))) // 1 — 문자 단위 길이
```

`len(string)` 은 **바이트 수**다. 한글 같은 다바이트 문자는 `[]rune` 으로 변환해야 문자 단위로 다룰 수 있다.

**3. 제로값이 `nil` 인 타입은 주의**

`map`, `slice`, `channel`, `pointer`, `interface` 는 제로값이 `nil` 이다. `nil` map 에 쓰면 panic, `nil` slice 는 안전하지만 `nil` map 은 아니다.

```go
var m map[string]int // nil — 여기서 쓰면 panic
m = make(map[string]int)
m["a"] = 1 // 이제 안전
```

## 코드로 확인하기

`go run .` 을 실행하면 다음과 같은 출력이 나온다:

- `var vs :=` 섹션: `:=` 는 타입을 추론하고, `var` 는 타입을 명시하며, 초기값을 생략하면 제로값이 들어감을 보여준다.
- `제로값과 쓰지 않는 변수`: `int` 는 `0`, `bool` 은 `false`, `string` 은 `""`, `float64` 는 `0.0` 인 것을 확인한다.
- `string 불변성`: `s[0]` 으로 읽기는 가능하지만, 대입은 불가능하고, `[]byte` 로 변환해야 수정 가능함을 보여준다.

`go test -v` 는 제로값, string 불변성을 실제로 검증한다:

- `TestVarAndAssign`: `var` 와 `:=` 가 올바른 값과 타입을 가지는지
- `TestZeroValues`: 초기화하지 않은 변수의 제로값이 맞는지
- `TestStringImmutable`: string 이 불변이고, `[]byte` 로 변환해야 수정 가능한지

## 연습 문제

**1. (쉽다) `var` 와 `:=` 차이 이해하기**

아래 코드를 작성하고 실행해 보라. `x` 와 `y` 의 타입과 값이 무엇인지 예측하고 확인하라.

```go
x := 10
var y int = 20
x = 30
// y = 40.0 — 오류가 날까?
```

힌트: `var` 로 선언한 변수는 타입이 고정되어 있다.

**2. (중간) string 을 반대로 만들기**

`"hello"` 를 `[]byte` 로 변환해서 `"olleh"` 로 만든 뒤, 원본 `"hello"` 가 그대로인 것을 확인하는 프로그램을 작성하라.

힌트: `[]byte(s)` 로 변환한 뒤 인덱스로 대입하거나 `reverse` 로직을 구현하라.

**3. (어려움) nil map vs 빈 map 구별하기**

`var m map[string]int` 와 `m := make(map[string]int)` 의 차이를 실험해 보라. 둘 다 `len(m) == 0` 인데, 어떤 상황에서panic 이 발생하는지 확인하라.

힌트: `nil` map 에 쓰면 panic, 빈 map 에 쓰면 안전하다. `fmt.Printf("%v\n", m == nil)` 로 구별 가능.

## 더 읽기

- [Go Tour: Variables](https://go.dev/tour/basics/8) — 공식 튜토리얼
- [Effective Go: Variables](https://go.dev/doc/effective_go#variables) — 변수 선언 관용구
- [Go Blog: Go Slices: Usage and Internals](https://go.dev/blog/slices-intro) — slice 와 string 의 관계 이해에 도움