# 인터페이스와 nil 함정

## 한 줄 요약
nil 포인터를 담은 인터페이스는 nil이 아니다. `error`를 반환할 때 가장 흔한 버그다.

## 왜 필요한가
Go에서 `error`는 인터페이스다. 인터페이스는 내부적으로 `(타입, 값)` 두 칸으로 이루어져 있다. 이 구조를 모르면 nil 체크가 의도한 대로 작동하지 않는 버그를 만난다.

다른 언어(Python·JavaScript·Java)에서는 `null`이나 `nil`이 특별한 값으로 취급된다. 하지만 Go의 인터페이스는 다르면 다르다. **타입이 있으면 값이 nil이어도 인터페이스는 nil이 아니다.**

Go가 이렇게 설계한 이유는 명확하다. 인터페이스는 메서드 호출이 가능한 객체다. 타입 정보가 있으면 런타임에 메서드를 찾을 수 있다. 값이 nil이어도 타입이 살아있으면 메서드 호출이 가능하다(panic할 뿐). 따라서 Go는 "타입이 있다 = 인터페이스가 있다"로 판단한다.

이 동작은 초보자에게 매우 직관적이지 않다. Python이면 `None`은 `None`이고, Java이면 `null`은 `null`이다. Go에서는 `(*MyError)(nil)`과 `nil`이 다르다. 이 차이를 이해하지 못하면 에러 처리 로직이 엉망이 된다.

## 기본 사용
```go
type MyError struct{ Msg string }
func (e *MyError) Error() string { return e.Msg }

func badFunc() error {
    var e *MyError // 타입: *MyError, 값: nil
    return e       // 반환값: ( *MyError, nil ) → nil이 아니다!
}

func fixedFunc() error {
    return nil     // 반환값: ( nil, nil ) → nil이다!
}
```

인터페이스가 nil이 되려면 **타입과 값 둘 다 nil**이어야 한다.

## 돌려보기
이 디렉토리에서 그대로 실행할 수 있는 명령.

```bash
go vet ./...      # 정적 검사 (사용되지 않는 import 체크)
go run .          # 시연 실행 - nil 체크 결과 확인
go test -v ./...  # 테스트
```

`go run .` 실행 시 출력의 핵심:
- `okFunc: nil입니다.` — 정상적인 nil 반환
- `badFunc: nil이 아닙니다. 에러 메시지:` — nil 포인터를 담은 인터페이스 (실수)
- `fixedFunc: nil입니다.` — 올바른 nil 반환 (고침)

## 실전에서는 이렇게 쓴다

**패턴 1: 함수가 실패했을 때만 에러 반환**
```go
func Open(name string) (*File, error) {
    f, err := os.Open(name)
    if err != nil {
        return nil, err // err는 이미 error 인터페이스다
    }
    return f, nil // 성공 시 nil 반환
}
```

**패턴 2: nil 포인터를 반환하지 않기**
```go
func findUser(id int) (*User, error) {
    user := db.Query(id)
    if user == nil {
        return nil, ErrNotFound // 명시적인 nil 반환
    }
    return user, nil
}
```

표준 라이브러리에서도 `os.Open`, `json.Unmarshal` 등 에러를 반환하는 함수들은 항상 `nil` 인터페이스나 실제 에러 값을 반환한다. nil 포인터를 반환하지 않는다.

## 흔한 실수

### 1. nil 포인터를 error로 반환하기

```go
// 틀린 코드
func badFunc() error {
    var e *MyError // 타입: *MyError, 값: nil
    return e       // ( *MyError, nil ) → nil이 아니다!
}
```

**어떤 일이 벌어지는가:**
```
badFunc: nil이 아닙니다. 에러 메시지: 
```

에러 메시지가 비어있는 것을 볼 수 있다. 타입은 있지만 값이 nil인 상황이다. `err.Error()`를 호출하면 빈 문자열을 반환한다.

**왜 그런가:**
인터페이스는 `(타입, 값)` 두 칸이다. 타입이 `*MyError`이므로 인터페이스는 nil이 아니다. 값이 nil이어도 타입 정보만 있으면 인터페이스는 "살아있다".

**고친 코드:**
```go
func fixedFunc() error {
    return nil // ( nil, nil ) → nil이다!
}
```

### 2. 에러 처리 로직에서 nil 체크 실패

```go
// 틀린 코드
func process() error {
    err := badFunc() // nil 포인터를 반환하는 함수
    if err == nil {  // 이 체크가 실패한다!
        return nil
    }
    return err
}
```

**어떤 일이 벌어지는가:**
`err == nil`이 `false`가 된다. 에러 처리 로직이 실패한다.

**왜 그런가:**
`badFunc()`가 반환한 값은 `( *MyError, nil )`이다. 타입이 있으므로 nil이 아니다.

**고친 코드:**
```go
func process() error {
    err := fixedFunc() // nil 인터페이스를 반환하는 함수
    if err == nil {    // 이 체크가 성공한다!
        return nil
    }
    return err
}
```

### 3. nil 인터페이스와 nil 포인터 혼동

```go
// 틀린 코드
var err error = (*MyError)(nil)
if err == nil {
    fmt.Println("nil") // 출력되지 않는다!
} else {
    fmt.Println("not nil") // 이쪽이 출력된다
}
```

**어떤 일이 벌어지는가:**
"not nil"이 출력된다. `(*MyError)(nil)`은 nil 포인터이지만, 인터페이스로 담으면 `( *MyError, nil )`이 되어 nil이 아니다.

**왜 그런가:**
인터페이스에 값을 넣으면 그 값의 타입 정보가 인터페이스에 기록된다. nil 포인터도 타입이 `*MyError`이므로 인터페이스는 nil이 아니다.

**고친 코드:**
```go
var err error = nil
if err == nil {
    fmt.Println("nil") // 이쪽이 출력된다
}
```

## 주의할 점

**1. nil 인터페이스 vs nil 포인터**
```go
var i interface{} = nil       // (nil, nil) → nil
var p *int = nil
i = p                         // (*int, nil) → nil이 아니다!
```

포인터를 인터페이스에 할당하면 타입 정보가 기록된다. 값이 nil이어도 인터페이스는 nil이 아니다.

**2. nil 채널 vs nil 슬라이스 vs nil 맵**
- `var ch chan int` → nil 채널 (닫힌 채널과 다름)
- `var sl []int` → nil 슬라이스 (길이 0, 용량 0)
- `var m map[string]int` → nil 맵 (쓰기 불가)

이들은 모두 `nil`이지만 서로 다른 동작을 한다. 특히 nil 채널은 블록킹 동작을 한다.

**3. error 인터페이스의 특별한 경우**
`errors.New("")`는 빈 문자열 에러를 만든다. 이는 nil이 아니다.
```go
err := errors.New("")
if err == nil { /* false */ }
```

빈 문자열 에러도 nil 체크를 통과하지 못한다.

## 코드로 확인하기

`go run .` 실행 결과:

```
--- 1. 정상적인 nil 반환 (okFunc) ---
okFunc: nil입니다.

--- 2. 타입은 있지만 값이 nil인 에러 반환 (badFunc - 실수) ---
badFunc: nil이 아닙니다. 에러 메시지: 

--- 3. 고친 버전: nil 포인터 대신 nil 인터페이스 반환 (fixedFunc) ---
fixedFunc: nil입니다.

--- 4. 에러 처리 시 nil 체크 예시 ---
okFunc 결과: 성공
badFunc 결과 (실수): 에러 발생: 
fixedFunc 결과: 성공
```

**핵심 관찰:**
- `okFunc`: `nil` 반환 → `nil입니다.` 출력
- `badFunc`: `(*MyError)(nil)` 반환 → `nil이 아닙니다.` 출력 (에러 메시지 없음)
- `fixedFunc`: `nil` 반환 → `nil입니다.` 출력

`badFunc`의 에러 메시지가 비어있는 이유는 값이 nil이기 때문이다. 타입만 있고 값이 없는 상황.

`main_test.go`는 다음을 검증한다:
- `TestNilErrorTrap`: 세 함수의 nil 여부를 테이블 주도 테스트로 검증
- `TestErrorHandling`: 에러 처리 로직에서 nil 체크가 올바르게 작동하는지 검증

## 연습 문제

1. `*MyError` 대신 `MyError` (값 타입)을 사용한다면 nil 포인터 함정에 빠지지 않을까? 직접 코드를 써서 확인해 보라.
2. `error` 인터페이스를 구현하는 타입을 여러 개 만들고, nil 체크가 어떻게 작동하는지 출력으로 확인해 보라.
3. nil 채널, nil 슬라이스, nil 맵을 만들어 각각의 `== nil` 체크 결과를 비교해 보라.

## 더 읽기

- [Go FAQ: nil error](https://go.dev/doc/faq#nil_error) — 공식 문서의 nil error 설명
- [Effective Go: Interfaces](https://go.dev/doc/effective_go#interfaces) — 인터페이스 사용 가이드
- [Go Blog: Go Slices: usage and internals](https://go.dev/blog/slices-intro) — 슬라이스와 인터페이스의 관계 이해에 도움