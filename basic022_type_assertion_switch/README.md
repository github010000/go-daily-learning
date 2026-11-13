> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main_test.go:133:15: undefined: os`  

## 한 줄 요약
인터페이스가 실제로 무슨 타입인지 꺼내는 `v.(T)` 와 `v.(type)` 스위치를 다룹니다.

## 왜 필요한가
Go 는 정적 타입 언어지만, 인터페이스를 쓰면 실행 시간에 어떤 타입이 들어올지 모릅니다.
다른 언어는 `instanceof` 나 `typeof` 로 타입을 검사하고 캐스팅하지만, Go 는 **단언(Assertion)**이라는 독특한 문법을 씁니다.

Python의 `isinstance()`, JavaScript의 `typeof` 와 달리 Go는 **두 가지 형태**를 제공합니다:
- `v.(T)` — 타입이 다르면 **무조건 panic**. 빠르지만 위험합니다.
- `v.(T), ok := v.(T)` — 타입이 다르면 **panic 없이 false** 반환. 안전합니다.

이 두 가지 중 언제 어느 것을 써야 할지 모르면, 예외 처리를 놓치거나 불필요하게 느린 코드를 만들 수 있습니다.

## 기본 사용

```go
var val interface{} = StringVal{V: "Hello"}

// 안전한 방식: ok 체크
if sv, ok := val.(StringVal); ok {
    fmt.Println(sv.V)
} else {
    fmt.Println("StringVal이 아님")
}

// 위험한 방식: panic 발생 가능
iv := val.(IntVal) // val이 IntVal이 아니면 여기서 죽습니다
```

타입 스위치는 여러 타입을 한 번에 분기합니다:
```go
switch v := val.(type) {
case StringVal:
    fmt.Println(v.V)
case IntVal:
    fmt.Println(v.V)
default:
    fmt.Printf("모르는 타입: %T\n", v)
}
```

## 돌려보기

```bash
go vet ./...      # 정적 검사 — 에러 없이 통과해야 합니다
go run .          # 시연 실행 — 타입 단언과 스위치 동작을 출력으로 확인
go test -v ./...  # 테스트 — 모든 테스트가 통과해야 합니다
```

## 실전에서는 이렇게 쓴다
1. **설정 파싱**: `map[string]interface{}` 에서 특정 타입의 값을 꺼낼 때
2. **에러 처리**: `err` 인터페이스의 실제 타입(예: `*os.PathError`)에 따라 다르게 처리
3. **JSON 언마샬링**: `json.Unmarshal` 후 `interface{}` 를 실제 구조체로 변환

```go
// 실제 코드에서 자주 보는 패턴
if pathErr, ok := err.(*os.PathError); ok {
    log.Printf("경로 오류: %s", pathErr.Path)
}
```

## 흔한 실수

### 실수 1: ok 없이 단언해서 panic
```go
// 틀린 코드
val := getValue() // interface{} 반환
str := val.(string) // val이 string이 아니면 panic!
```
**무엇이 벌어지는가**: `panic: interface conversion: interface {} is int, not string`
**왜 그런가**: 타입 단언은 "이건 꼭 string이다"라고 확신할 때만 써야 합니다. 모를 때는 ok 체크를 해야 합니다.
**고친 코드**:
```go
if str, ok := val.(string); ok {
    // 사용
}
```

### 실수 2: 타입 스위치에서 `var` 대신 `:=` 안 쓰기
```go
// 틀린 코드
switch val.(type) {
case string:
    fmt.Println(var) // var은 선언되지 않음
}
```
**무엇이 벌어지는가**: `undefined: var` 컴파일 오류
**왜 그런가**: 타입 스위치에서 각 케이스의 변수는 자동으로 선언됩니다. `v := val.(type)` 형식으로 써야 합니다.
**고친 코드**:
```go
switch v := val.(type) {
case string:
    fmt.Println(v)
}
```

### 실수 3: nil 인터페이스를 단언하기
```go
// 틀린 코드
var val interface{} = nil
str := val.(string) // panic!
```
**무엇이 벌어지는가**: `panic: interface conversion: interface is nil, not string`
**왜 그런가**: `nil` 인터페이스는 타입 정보 자체가 없습니다. 단언할 타입이 없으니panic합니다.
**고친 코드**:
```go
if val != nil {
    if str, ok := val.(string); ok { ... }
}
```

### 실수 4: 타입 스위치에서 interface{} 케이스를 못 쓰는 것
```go
// 틀린 코드
switch v := val.(type) {
case interface{}: // 컴파일 오류: interface{} 케이스는 허용되지 않음
    fmt.Println(v)
}
```
**무엇이 벌어지는가**: `invalid case interface{} in type switch`
**왜 그런가**: Go 타입 스위치는 `interface{}` 케이스를 허용하지 않습니다. 모든 타입을 잡으려면 `default` 를 쓰세요.
**고친 코드**:
```go
switch v := val.(type) {
default:
    fmt.Println(v) // 모든 타입을 잡음
}
```

## 주의할 점
- `interface{} == nil` 은 값이 `nil` 이면서 타입이 `nil` 일 때만 true. 값이 `nil` 인 포인터는 false
- 타입 단언은 **메서드가 있는 타입**도 가능합니다. `val.(io.Reader)` 형태로
- Go 1.18+ 제네릭이 있지만, 타입 단언은 여전히 인터페이스 처리에 필수적입니다

## 코드로 확인하기
`go run .` 을 돌리면:
- **타입 단언 성공**: `StringVal` 을 꺼낼 때 내용 출력
- **타입 단언 실패**: `IntVal` 을 `StringVal` 로 단언하면 `ok=false` 출력
- **타입 스위치**: 각 타입별 출력 (`StringVal 타입:`, `IntVal 타입:`, `float64 타입:`, 등)
- **실수 시연**: `panic 발생: interface conversion: ...` 출력 후 recover로 종료

`main_test.go` 는:
- 올바른 단언이 성공하는지
- 잘못된 단언이 panic 없이 false를 반환하는지
- 타입 스위치가 올바르게 분기하는지
- panic이 실제로 발생하는지

## 연습 문제
1. `map[string]interface{}` 에서 string, int, bool을 각각 꺼내는 함수를 작성하세요.
2. 타입 스위치를 써서 `interface{}` 슬라이스를 순회하며 각 타입별 총합을 계산하세요.
3. `error` 인터페이스의 실제 타입에 따라 다른 메시지를 출력하는 함수를 만드세요.

## 더 읽기
- [Go Tour - Type Assertions](https://go.dev/tour/methods/15)
- [Effective Go - Interfaces](https://go.dev/doc/effective_go#interfaces)
- [Go Blog - Go Slices: usage and internals](https://go.dev/blog/slices-intro)