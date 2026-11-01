> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main_test.go:19:11: undefined: fmt`  

# 클로저와 루프 변수 스코프

## 한 줄 요약
클로저는 함수 바깥의 변수를 **참조**로 붙잡습니다. Go 1.22부터 루프 변수 스코프가 바뀌어, 고루틴에서 클로저를 쓸 때 버그가 생길 수 있습니다.

## 왜 필요한가
클로저는 함수 안에서 정의된 함수가 바깥 변수에 접근할 수 있게 해줍니다. Python의 `lambda`나 JavaScript의 `function`과 비슷해 보이지만, **무엇을 붙잡는지**가 다릅니다.

다른 언어에서는 루프 안에서 클로저를 만들 때 대개 각 반복마다 새로운 변수가 생성됩니다. 하지만 Go는 1.22까지 **루프 변수를 하나만 생성**했습니다. 그래서 다음과 같은 버그가 있었습니다:

```go
for i := 0; i < 5; i++ {
    go func() {
        fmt.Println(i) // 5를 5번 출력!
    }()
}
```

Go 1.22에서 이 문제가 해결됐지만, **구버전 코드**를 유지보수하거나 이해해야 할 때는 여전히 중요합니다.

## 기본 사용
클로저는 `func() ...` 형태로 변수에 저장할 수 있습니다:

```go
var fn func() int
fn = func() int {
    return 42
}
```

클로저가 변수를 **참조**로 붙잡는다는 것은, 변수가 바뀌면 클로저도 그 변화를 본다는 뜻입니다:

```go
x := 1
fn := func() {
    x++ // 이 x는 바깥의 x를 참조합니다.
}
fn()
fmt.Println(x) // 2
```

## 돌려보기
이 디렉토리에서 그대로 실행할 수 있는 명령입니다:

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

`go run .` 실행 시 출력에서:
- **1번**: 모든 인덱스가 `10`을 출력하는 것 확인 (공유 변수 버그)
- **2번**: 인덱스마다 `0~9`가 출력되는 것 확인 (수정된 버전)
- **3번**: 데이터 레이스 발생으로 counter가 불확실한 값 출력
- **4번**: mutex 보호로 counter가 정확히 5 출력

## 실전에서는 이렇게 쓴다

### 1. HTTP 핸들러 팩토리
URL 패턴에 따라 다른 핸들러를 생성할 때:

```go
func makeHandler(prefix string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from %s", prefix)
    }
}
```

### 2. 이벤트 리스너
각 이벤트에 고유한 데이터를 클로저에 담아 전달:

```go
for _, event := range events {
    go func(e Event) {
        process(e) // e는 각 반복마다 새로운 변수
    }(event)
}
```

## 흔한 실수

### 1. 루프 변수를 클로저에서 사용하면 모두 같은 값 출력

**틀린 코드**
```go
// Go 1.22 이전: 모든 클로저가 5를 출력
for i := 0; i < 5; i++ {
    go func() {
        fmt.Println(i)
    }()
}
```

**어떤 일이 벌어지는지**
`i`가 루프 전체에서 하나만 생성되고, 모든 클로저가 이 `i`를 참조합니다. 고루틴이 실행될 때는 루프가 이미 끝나 `i=5`가 된 상태라, **5를 5번 출력**합니다.

**왜 그런지**
클로저는 변수의 값을 복사하는 게 아니라, **메모리 주소(참조)**를 저장하기 때문입니다.

**고친 코드**
```go
// Go 1.22+: i가 각 반복마다 새로 생성되므로 정상 동작
for i := 0; i < 5; i++ {
    go func() {
        fmt.Println(i)
    }()
}

// 또는 구버전 호환: 매 반복마다 변수 복사
for i := 0; i < 5; i++ {
    i := i // 지역 변수로 복사
    go func() {
        fmt.Println(i)
    }()
}
```

### 2. 클로저에서 공유 변수를 고루틴으로 수정 (데이터 레이스)

**틀린 코드**
```go
counter := 0
var closures []func()
for i := 0; i < 5; i++ {
    closures = append(closures, func() {
        counter++ // 동시 수정 → 데이터 레이스
    })
}
```

**어떤 일이 벌어지는지**
`go test -race` 실행 시 다음과 같은 메시지가 출력됩니다:
```
WARNING: DATA RACE
Read by goroutine N:
  main.demonstrateDataRace()
Previous write by goroutine M:
  main.demonstrateDataRace()
```
실행 결과 `counter`가 5가 안 될 수 있고, 예측 불가능한 값을 가집니다.

**왜 그런지**
여러 고루틴이 동시에 같은 메모리 위치를 수정하면, 연산이 interleaving되어 값이 유실됩니다.

**고친 코드**
```go
var mu sync.Mutex
counter := 0
var closures []func()
for i := 0; i < 5; i++ {
    closures = append(closures, func() {
        mu.Lock()
        counter++
        mu.Unlock()
    })
}
```

### 3. 클로저가 루프 변수의 변경을 추적

**틀린 코드**
```go
values := []int{1, 2, 3}
var closures []func()
for i := range values {
    closures = append(closures, func() {
        fmt.Println(values[i])
    })
}
// values[0] = 100으로 바꾸면, closures[0]도 100을 출력
```

**어떤 일이 벌어지는지**
클로저가 `values` 슬라이스와 `i` 변수를 **참조**로 붙잡기 때문에, 바깥에서 값을 바꾸면 클로저도 그 변화를 봅니다. 이게 버그가 될 수도 있고, 의도한 동작일 수도 있습니다.

**왜 그런지**
Go 클로저는 **값이 아닌 참조**를 저장합니다. Python의 `lambda`와 달리, JavaScript의 `function`과 더 비슷한 동작입니다.

**고친 코드**
```go
// 클로저가 독립적인 값을 갖길 원하면:
for i := range values {
    v := values[i] // 지역 변수로 복사
    closures = append(closures, func() {
        fmt.Println(v) // v는 이 클로저만의 값
    })
}
```

## 주의할 점

### 1. Go 버전 확인
Go 1.22 이전에서는 루프 변수 스코프 버그가 있었습니다. **구버전 코드베이스**를 유지보수한다면 이 차이를 알아야 합니다. `go version`으로 확인하세요.

### 2. 성능 함정
클로저가 바깥 변수를 참조하면, 그 변수가 **히트캐시에서 벗어나지 않을 수** 있습니다. 빈번한 호출이 필요한 코드는 값 전달을 고려하세요.

### 3. 다른 언어 습관
Python/JavaScript와 달리 Go 클로저는 **값 복사**가 아닙니다. "이 클로저가 이 값을 복사했겠지"라고 생각하면 버그가 생깁니다. 항상 **무엇을 참조하는지** 확인하세요.

## 코드로 확인하기

`main.go` 실행 결과에서:

1. **실수 버전 (1번)**: `인덱스 0: 10`, `인덱스 1: 10` ... 모든 인덱스가 `10`을 출력합니다. 모든 클로저가 같은 `i`를 참조하기 때문입니다.
2. **고친 버전 (2번)**: `인덱스 0: 0`, `인덱스 1: 1` ... 인덱스마다 고유한 값을 출력합니다. 각 클로저가 독립적인 `i`를 갖기 때문입니다.
3. **데이터 레이스 (3번)**: `counter`가 5가 아닌 값을 출력할 수 있고, 실행할 때마다 값이 다를 수 있습니다.
4. **mutex 보호 (4번)**: `counter`가 항상 5로 출력됩니다.

`main_test.go`는:
- `TestLoopVariableScopeNewStyle`: 고친 버전이 0~9를 반환하는지 검증
- `TestFixDataRace`: mutex로 보호한 클로저가 counter를 정확히 5로 만드는지 검증
- `TestClosureReference`: 클로저가 변수 참조로 동작하는지 검증

## 연습 문제

1. **쉬움**: 3개의 클로저를 만들고, 각 클로저가 10, 20, 30을 반환하도록 하세요. (힌트: 루프 변수 복사)
2. **보통**: 5개의 고루틴이 공유 counter를 1씩 증가시키고, 최종값이 5가 되도록 하세요. (힌트: mutex 사용)
3. **어려움**: `http.HandlerFunc` 팩토리를 만들어, URL 패턴에 따라 다른 응답을 반환하는 핸들러를 생성하세요. (힌드: 클로저 활용)

## 더 읽기
- [Go Blog: Loop Variable Preview](https://go.dev/blog/loopvar-preview)
- [Effective Go: Functions](https://go.dev/doc/effective_go#functions)
- [Go Spec: Function literals](https://go.dev/ref/spec#Function_literals)