## 한 줄 요약

`panic`은 복구 불가능한 심각한 오류에 사용하고, `recover`는 반드시 `defer` 안에서만 호출하여 panic을 잡을 수 있습니다.

## 왜 필요한가

Go는 다른 언어(Java의 예외, Python의 raise, JavaScript의 throw)와 달리 **오류(error)와 panic을 분리**했습니다. Java나 Python에서는 예외를 던지고 catch로 잡는 것이 일반적 에러 처리 패턴이지만, Go는 **error 값을 리턴**하는 것을 기본으로 합니다.

Go가 panic/recover를 따로 둔 이유는 **두 가지**입니다.

1. **복구 불가능한 상황**: 초기화 실패, nil 포인터 디레퍼런스, 불변조건 위반처럼 프로그램이 계속 실행되는 것이 의미가 없는 경우에 사용합니다. 이런 경우는 error 리턴으로 처리하기보다 program을 빠르게 종료시키는 것이 디버깅에 도움이 됩니다.
2. **defer와의 결합**: `defer`는 함수가 종료될 때 항상 실행되는 메커니즘인데, panic이 발생해도 defer는 실행됩니다. 즉, panic이 발생하면 defer 체인이 실행되고, 그 안에서 `recover()`를 호출하면 panic을 잡을 수 있습니다. 이는 자원 정리(파일 닫기, 잠금 해제)를 보장하는 강력한 패턴입니다.

반면에 일반적인 에러(파일 없음, 네트워크 오류 등)는 panic 대신 **error 리턴**을 사용해야 합니다. panic은 프로그램의 흐름을 급격히 바꾸고, 호출자가 처리하기 어려워지며, 성능 오버헤드도 있습니다.

## 기본 사용

### panic: 오류 신호 전달

```go
panic("처리할 수 없는 오류")
```

`panic`이 호출되면 그 순간 실행 중인 goroutine의 실행이 중단되고, defer 체인이 LIFO(후입선입) 순서로 실행됩니다. defer가 모두 실행되면 프로그램이 종료됩니다.

### recover: panic 잡기

```go
defer func() {
    if r := recover(); r != nil {
        // panic이 발생했을 때만 여기가 실행됩니다.
        fmt.Println("panic 복구:", r)
    }
}()
```

`recover()`는 현재 goroutine의 panic 체인을 검사합니다. panic이 defer 실행 중(스택이 내려오는 중)이면 값을 반환하고 panic 전파를 중단합니다.panic이 없으면 `nil`을 반환합니다.

**핵심**: `recover`는 **defer 블록 안에서만** panic을 잡을 수 있습니다. defer 밖에서 호출해도 nil을 반환할 뿐 아무것도 잡지 못합니다.

### goroutine과 panic

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            // 고루틴 내부에서만panic을 잡을 수 있습니다.
        }
    }()
    panic("고루틴panic")
}()
```

고루틴의 panic은 메인 고루틴의 recover로 잡히지 않습니다. **각 고루틴이 자체적으로 defer/recover를 가져야 합니다.** 고루틴 내부에 recover가 없으면 프로그램 전체가 종료됩니다.

## 돌려보기

아래 명령을 실행하여 panic과 recover의 동작을 직접 확인할 수 있습니다.

```bash
go vet ./...      # 정적 검사: 변수 미사용 등 확인
go run .          # 시연 실행: 실수/고친 버전 출력 비교
go test -v ./...  # 테스트: 모든 함수의 정상 동작 검증
```

**출력의 무엇을 봐야 할까요?**

1. `go run .` 실행 시, **[고친 버전] safeDivide** 섹션에서 `10 / 0`이panic 없이 `0`을 반환하는 것을 확인하세요. 이는 defer 안의 recover가 panic을 잡았기 때문입니다.
2. **[실수 버전] recover를 defer 밖에서 호출** 섹션에서 `recover()`가 `nil`을 반환하고, 이후panic이 main의 defer로 전파되어 복구되는 과정을 확인하세요.
3. **[고친 버전] guardedGoroutinePanics**에서 고루틴 내부 recover가 메인 고루틴을 살리는 것을 확인하세요.
4. `go test -v`에서 모든 테스트가 통과하는지 확인하세요. `TestRecoverInWrongPlace`는 panic이 전파되는 것을 검증합니다.

## 실전에서는 이렇게 쓴다

### 1. 서버 초기화 실패 시panic

```go
func main() {
    config, err := loadConfig("config.yaml")
    if err != nil {
        // 설정 파일이 없으면 서버가 시작될 수 없으므로panic
        panic(fmt.Sprintf("설정 로딩 실패: %v", err))
    }
    srv := NewServer(config)
    srv.Start()
}
```

### 2.defer를 이용한 자원 정리

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer func() {
        if r := recover(); r != nil {
            //panic이 발생해도 무조건 파일을 닫습니다.
            fmt.Println("panic 발생, 파일 닫음:", r)
        }
        f.Close()
    }()
    // ... 파일 처리 ...
    return nil
}
```

### 3. 고루틴 내부panic 처리

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("고루틴panic 복구: %v", r)
        }
    }()
    // ... 고루틴 작업 ...
}()
```

## 흔한 실수

### 1. recover를 defer 밖에서 호출하기

**틀린 코드:**

```go
func badRecover() {
    result := recover() // defer 밖에서 호출
    fmt.Println("result:", result) // nil 출력
    panic("panic") // 이panic은 아무것도 잡지 못함
}
```

**어떤 일이 벌어지는지:**

`recover()`는 panic 체인이 없을 때 `nil`을 반환합니다. 따라서 `result`는 `nil`이 되고, 이후 `panic("panic")`이 발생하면 이panic은 전파되어 프로그램을 종료시킵니다.

**왜 그런지:**

`recover`는 panic이 **defer 실행 중**(스택이 내려오는 중)일 때만 동작합니다. defer 블록 밖에서는 panic 체인이 없으므로 아무것도 잡지 못합니다.

**고친 코드:**

```go
func goodRecover() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("panic 복구:", r)
        }
    }()
    panic("panic") // defer가 이panic을 잡습니다.
}
```

### 2. 고루틴panic을 메인 고루틴에서 잡으려 하기

**틀린 코드:**

```go
func main() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("메인 recover:", r)
        }
    }()
    go func() {
        panic("고루틴panic") // 메인 recover로 못 잡음
    }()
    time.Sleep(100 * time.Millisecond)
}
```

**어떤 일이 벌어지는지:**

고루틴 내부에 recover가 없으므로, 고루틴의panic은 메인 고루틴의 defer로 전파되지 않습니다. 대신 프로그램 전체가 종료됩니다.

**왜 그런지:**

panic은 **같은 goroutine 내에서만** recover로 잡을 수 있습니다. 다른 goroutine의panic은 격리되어 있습니다.

**고친 코드:**

```go
func main() {
    done := make(chan string, 1)
    go func() {
        defer func() {
            if r := recover(); r != nil {
                done <- fmt.Sprintf("고루틴recover: %v", r)
            }
        }()
        panic("고루틴panic")
    }()
    fmt.Println(<-done) // 고루틴 내부 recover 결과 출력
    fmt.Println("메인 계속 실행")
}
```

### 3. 일반적인 에러에panic 사용하기

**틀린 코드:**

```go
func divide(a, b int) int {
    if b == 0 {
        panic("0으로 나눌 수 없습니다") // 일반적인 에러에panic
    }
    return a / b
}

func main() {
    result := divide(10, 0) // 호출자가panic을 처리하지 않으면 프로그램 종료
}
```

**어떤 일이 벌어지는지:**

호출자가panic을 recover하지 않으면 프로그램이 종료됩니다. 또한 panic은 디버깅을 어렵게 하고 성능 오버헤드가 있습니다.

**왜 그런지:**

0으로 나누기는 **예상 가능한 에러**입니다. 호출자가 에러를 처리할 수 있어야 하므로, error 리턴을 사용해야 합니다.

**고친 코드:**

```go
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, fmt.Errorf("0으로 나눌 수 없습니다: %d / %d", a, b)
    }
    return a / b, nil
}

func main() {
    result, err := divide(10, 0)
    if err != nil {
        fmt.Println("에러 처리:", err)
    } else {
        fmt.Println("결과:", result)
    }
}
```

### 4. defer의 LIFO 순서 오해하기

**틀린 코드:**

```go
func main() {
    defer fmt.Println("1번 defer")
    defer fmt.Println("2번 defer")
    panic("panic")
    // 출력 순서: 2번 defer, 1번 defer (LIFO)
}
```

**어떤 일이 벌어지는지:**

defer는 LIFO(후입선입) 순서로 실행됩니다. 즉, 나중에 등록한 defer가 먼저 실행됩니다.

**왜 그런지:**

defer는 **스택**처럼 동작합니다.panic이 발생하면 스택을 거꾸로 돌며 defer를 실행합니다.

**고친 코드:**

```go
func main() {
    defer fmt.Println("1번 defer") // 먼저 등록
    defer fmt.Println("2번 defer") // 나중에 등록
    panic("panic")
    // 출력: 2번 defer, 1번 defer (LIFO 순서)
}
```

## 주의할 점

### 1. panic은 성능 오버헤드가 있습니다

panic은 스택 트레이스를 수집하고 defer를 실행하는 등 비교적 무거운 작업입니다. hot path(자주 실행되는 경로)에서panic을 사용하면 성능에 영향을 미칩니다.

### 2. defer는 panic이 발생해도 실행됩니다

이는 장점이기도 하지만, defer 안에서panic이 발생하면 **새로운panic**이 발생합니다. 이panic은 기존panic을 덮어쓰고 전파됩니다.

```go
func main() {
    defer func() {
        panic("defer 내panic") // 이panic이 메인panic을 덮어씀
    }()
    panic("메인panic")
}
// 출력: defer 내panic
```

### 3. Go 1.22부터 루프 변수 스코프 변경

Go 1.22부터 `for` 루프의 변수가 각 iteration마다 새로 생성됩니다. 이전 버전에서는 모든 iteration이 동일한 변수를 공유했습니다. 이는panic/recover와 직접적인 연관은 없지만, defer를 루프 안에서 사용할 때 혼란을 줄 수 있습니다.

```go
// Go 1.22 이전
for _, v := range []int{1, 2, 3} {
    defer func() { fmt.Println(v) }() // 모든 defer가 마지막 v(3)를 출력
}
// Go 1.22 이후
for _, v := range []int{1, 2, 3} {
    defer func() { fmt.Println(v) }() // 각 defer가 해당 v를 출력
}
```

## 코드로 확인하기

`go run .`을 실행하면 다음 출력을 볼 수 있습니다.

1. **[고친 버전] safeDivide**에서 `10 / 0`이 `0`을 반환합니다. 이는 defer 안의 recover가panic을 잡았기 때문입니다.
2. **[실수 버전] recover를 defer 밖에서 호출**에서 `recover()`가 `nil`을 반환하고, 이후panic이 main의 defer로 전파되어 복구됩니다.
3. **[고친 버전] guardedGoroutinePanics**에서 고루틴 내부 recover가 메인 고루틴을 살립니다.
4. **[실수 버전] unguardedGoroutinePanics**를 설명으로만 남겼습니다(실제 호출 시 프로그램 종료).

`main_test.go`의 테스트는 다음과 같습니다.

- `TestSafeDivide`: 0으로 나누는panic이 정상 복구됨을 검증
- `TestRecoverInWrongPlace`: defer 밖 recover가panic을 못 잡음 검증
- `TestGuardedGoroutinePanics`: 고루틴 내부 recover가 메인 고루틴 살림 검증
- `TestNewConfig`: 필수 설정 없으면panic 발생 검증
- `TestReadFile`: 파일 없으면error 리턴 검증
- `TestDivideErrorVersion`: 일반적인 에러에panic 쓰는 것이 잘못된 패턴임을 검증

## 연습 문제

1. **쉬움**: `safeDivide`를 수정하여 `b`가 음수일 때도panic을 일으키도록 하세요. (힌트: `if b <= 0` 조건 추가)
2. **중간**: `guardedGoroutinePanics`를 수정하여panic 메시지에 고루틴 ID를 포함시키세요. (힌트: `runtime.GoID()` 사용)
3. **어려움**: `newConfig`를 수정하여 panic 대신 error를 리턴하도록 변경하고, 호출부를 수정하세요. (힌트: `error` 반환 타입 추가, 호출부에서 error 처리)

## 더 읽기

- [Go Blog: Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover) - 공식 문서
- [Go Effective Go: Panics](https://go.dev/doc/effective_go#panics) - Go 관용적 패턴
- [Go Tour: Panics](https://go.dev/tour/basics/9) - 인터랙티브 튜토리얼