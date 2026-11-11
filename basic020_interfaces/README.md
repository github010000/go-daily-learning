> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main.go:43:6: declared and not used: r`  

## 한 줄 요약

Go의 인터페이스는 타입이 명시적으로 구현 선언을 하지 않아도 메서드만 일치하면 자동으로 만족된다.

## 왜 필요한가

다른 언어에서는 인터페이스 구현을 명시해야 하지만, Go에서는 타입이 필요한 메서드를 가지고 있기만 하면 암묵적으로 인터페이스를 구현합니다. 이는 Go가 'what it does'에 더 관심이 있기 때문입니다. 예를 들어, Python의 `abc` 모듈이나 Java의 `implements` 키워드처럼 명시적으로 선언하지 않아도 됩니다.

Go의 설계 철학은 작고 단순한 인터페이스를 조합하여 복잡한 기능을 만드는 것입니다. `io.Reader`나 `io.Writer` 같은 작은 인터페이스는 다양한 타입에서 자연스럽게 구현될 수 있도록 설계되었습니다.

## 기본 사용

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

이 인터페이스를 구현하려면 `Read` 메서드가 있는 타입이면 됩니다. 명시적인 선언 없이도 됩니다.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

## 실전에서는 이렇게 쓴다

1. 함수 매개변수로 인터페이스를 받아 다양한 타입을 처리:
```go
func Copy(dst Writer, src Reader) (written int64, err error)
```

2. 테스트 시 인터페이스를 이용한 mocking:
```go
type DB interface {
    Query(query string, args ...interface{}) (*Rows, error)
}
```

## 흔한 실수

### 1. concrete type의 필드에 접근하려 함

```go
var r io.Reader = &smallReader{data: "안녕"}
fmt.Println(r.data) // 컴파일 오류: r.data undefined
```

인터페이스는 메서드만 노출합니다. 필드 접근은 컴파일 오류가 납니다.

### 2. nil 인터페이스 메서드 호출

```go
var nilReader io.Reader
nilReader.Read([]byte{0}) // panic
```

nil 인터페이스는 타입과 값 모두 nil입니다. 메서드 호출 시 panic이 발생합니다.

### 3. nil concrete pointer를 인터페이스에 할당

```go
var ptr *smallReader = nil
var ifaceReader io.Reader = ptr
// ifaceReader는 nil이 아님!
ifaceReader.Read([]byte{0}) // panic
```

nil 포인터를 인터페이스에 할당하면 인터페이스는 nil이 아닙니다. 타입 정보는 있지만 값이 nil인 상태입니다.

## 주의할 점

- 인터페이스는 메서드 세트만 일치하면 됩니다
- 빈 인터페이스 `any`(또는 `interface{}`)는 모든 타입을 받을 수 있습니다
- 타입 어설션 시 `ok` 체크를 반드시 해야 합니다

## 코드로 확인하기

`mistakeVersion()`은 다음과 같은 출력을 보여줍니다:

```
--- 실수 버전 (주의: panic 시연 포함) ---
패닉 발생 (예상됨): interface conversion: interface {} is int, not string
nil 인터페이스 호출 시도...
nil 인터페이스 호출 panic: runtime error: invalid memory address or nil pointer dereference
```

`correctVersion()`은 안전하게 값을 처리합니다:

```
--- 고친 버전 ---
고친 버전에서 얻은 문자열: world
읽은 내용: Go is great
작성된 내용: Hello, Interface!
값: 42 (동적 타입: int)
값: 3.14 (동적 타입: float64)
값: true (동적 타입: bool)
값: [1 2 3] (동적 타입: []int)
```

테스트는 인터페이스 구현의 correctness를 검증합니다.

## 연습 문제

1. `io.Closer` 인터페이스를 구현하는 타입을 만들어 보세요
2. `any` 슬라이스에서 특정 타입만 필터링하는 함수를 작성하세요
3. `io.Reader`와 `io.Writer`를 모두 구현하는 타입을 만들어 보세요

## 더 읽기

- [Go Tour - Interfaces](https://go.dev/tour/methods/9)
- [Effective Go - Interfaces](https://go.dev/doc/effective_go#interfaces)