## 한 줄 요약

defer는 함수가 반환될 때 "예약된 순서(LIFO)로" 실행되지만, **인수는 defer 등록 시점에 즉시 평가**됩니다.

## 왜 필요한가

Go는 다른 언어와 달리 try/finally 블록을 명시적으로 작성하지 않습니다. 대신 **defer** 키워드로 "이 코드는 함수가 끝날 때 무조건 실행해라"라고 선언합니다. C++의 소멸자나 Java의 try-with-resources와 유사한 목적이지만, Go에서는 **어디서든, 여러 개 중첩해서, 심지어 panic 상황에서도** 실행 보장이라는 점이 다릅니다.

Python의 `with` 문이나 JavaScript의 `finally` 블록은 코드 블록 단위로 동작하지만, Go defer는 **함수 레벨**에서 작동합니다. 그래서 루프 안에서 defer를 사용하면 의도치 않게 자원이 함수 전체 종료 시까지 보류되는 함정에 빠지기 쉽습니다. 이걸 피하려면 **즉시 실행되는 익명 함수 안에서 defer**를 등록하는 관용구를 쓰게 됩니다.

또 다른 Go 특유의 차이점은 **인수 평가 시점**입니다. 다른 언어의 finally 블록은 보통 "블록이 실행될 때" 현재 값을 참조하지만, Go defer는 "defer가 등록되는 순간" 인수를 복사해서 저장합니다. 이 차이를 모르면 파일 경로나 에러 메시지를 defer에 전달했을 때 예상과 다른 값이 나오는 버그를 마주합니다.

## 기본 사용

```go
func doWork() {
    f, err := os.Open("file.txt")
    if err != nil {
        return // 여기서 defer가 자동으로 실행되어 f가 닫힙니다
    }
    defer f.Close() // 함수가 반환될 때 f.Close()가 실행됩니다

    // ... 파일 처리 ...
}
```

핵심은 세 가지입니다. **1) 함수가 반환될 때 실행**, **2) 등록된 순서와 반대(LIFO)로 실행**, **3) 인수는 등록 시점에 즉시 평가**됩니다.

```go
// 인수가 즉시 평가되는 예
func showArgEval() {
    path := "/tmp/a.txt"
    defer fmt.Println("cleanup:", path) // path는 여기서 "/tmp/a.txt"로 고정
    path = "/tmp/b.txt"
    fmt.Println("working:", path)       // "/tmp/b.txt"
    // cleanup: /tmp/a.txt  (변경 전 값이 출력됨)
}
```

## 돌려보기

```bash
go vet ./...      # 정적 검사 — fmt.Println arg list ends with redundant newline 경고 확인
go run .          # 시연 실행 — defer 인수 평가 순서, LIFO, 루프 동작 비교
go test -v ./...  # 테스트 — LIFO 반환값과 파일 닫힘 검증
```

`go run .` 실행 시 출력에서 다음을 확인하세요.

- **인수 평가 시점**: 실수 버전은 `cleanup: /tmp/foo.txt` (변경 전), 고친 버전은 `cleanup: /tmp/bar.txt` (변경 후)
- **LIFO**: `LIFO demo 반환값: 11` — 두 defer 중 나중에 등록된 것이 먼저 실행됨
- **루프**: 실수 버전은 `loop finished — defer are pending until here`, 고친 버전은 각 step에서 즉시 닫힘

## 실전에서는 이렇게 쓴다

**1) 파일/커넥션 정리 (표준 라이브러리 관용구)**

```go
func readConfig(path string) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close() // panic이 터져도 반드시 닫힘
    return io.ReadAll(f)
}
```

**2) 잠금 해제도 defer로**

```go
func safeUpdate(mu *sync.Mutex, data []byte) {
    mu.Lock()
    defer mu.Unlock() // Lock/Unlock이 항상 쌍을 이룸
    // ... mutex 보호 영역 ...
}
```

**3) 루프 안에서 defer 사용할 때는 즉시 실행 wrapper 사용**

```go
for _, url := range urls {
    func() {
        resp, err := http.Get(url)
        if err != nil {
            return
        }
        defer resp.Body.Close() // 이 함수 scope에서만 유효
        // ... 응답 처리 ...
    }()
}
```

## 흔한 실수

### 실수 1: defer 인수 평가 시점 오해

```go
// 틀린 코드
func badDeferArg() {
    path := "/tmp/foo.txt"
    defer fmt.Println("cleanup:", path) // path가 즉시 복사됨
    path = "/tmp/bar.txt"
}
// 출력: cleanup: /tmp/foo.txt (원하지 않는 값)
```

**무슨 일이 벌어지나**: `defer fmt.Println("cleanup:", path)`가 실행되는 순간 `path` 값이 복사됩니다. 이후 `path`를 바꿔도 defer에 기록된 값은 변하지 않습니다. Go는 " defer 실행 시 인수를 다시 읽는다"는 보장을 하지 않습니다.

**왜 그런지**: Go spec에 명시된 대로 defer 구문의 인수는 **defer문이 실행되는 시점**이 아니라 **defer가 등록되는 시점**에 평가됩니다. 이는 defer가 "함수 종료 시 실행되는 콜백"이 아니라 "함수 반환 시 실행되는 명령어 목록"이기 때문입니다.

**고친 코드**:

```go
func goodDeferArg() {
    path := "/tmp/foo.txt"
    defer func() {
        fmt.Println("cleanup:", path) // path를 함수 리터럴 내부에서 읽음
    }()
    path = "/tmp/bar.txt"
}
// 출력: cleanup: /tmp/bar.txt (예상한 값)
```

### 실수 2: 루프 안에서 defer로 자원을 열어서 닫지 않음

```go
// 틀린 코드
func badDeferLoop() {
    for i := 0; i < 1000; i++ {
        f, _ := os.CreateTemp("", "*.txt")
        defer f.Close() // 1000개가 main() 종료까지 보류됨
        fmt.Fprintf(f, "%d", i)
    }
}
```

**무슨 일이 벌어지나**: 루프가 1000회 반복되면 defer 스택에 `f.Close()`가 1000개 쌓입니다. 이 함수(`main`)가 끝날 때까지 파일 디스크립터가 열려 있습니다. long-running 서비스라면 파일 디스크립터 고갈로 이어집니다.

**왜 그런지**: defer는 **함수 레벨**에서 동작합니다. 루프 안에서 defer를 선언해도 그 defer는 **함수가 반환될 때** 한꺼번에 실행됩니다. 루프 iteration마다 즉시 정리하려면 defer를 **즉시 실행되는 함수 scope** 안에서 등록해야 합니다.

**고친 코드**:

```go
func goodDeferLoop() {
    for i := 0; i < 1000; i++ {
        func() {
            f, _ := os.CreateTemp("", "*.txt")
            defer f.Close() // 이 익명 함수가 끝나면 바로 실행
            fmt.Fprintf(f, "%d", i)
        }()
    }
}
```

### 실수 3: LIFO 순서를 모르고 defer를 중복 등록

```go
// 틀린 코드
func badLIFO() {
    result := 0
    defer func() { result++ }()
    defer func() { result *= 10 }()
    fmt.Println(result) // 0을 기대했지만?
}
```

**무슨 일이 벌어지나**: `result`는 0으로 출력되지 않고, defer가 실행된 후의 값이 반영됩니다. 두 defer가 LIFO 순서로 실행되므로 `result *= 10`이 먼저, `result++`이 나중에 실행됩니다. 즉 `0 * 10 = 0`, `0 + 1 = 1`이 됩니다.

**왜 그런지**: defer는 **등록된 순서와 반대**로 실행됩니다. 먼저 등록한 것이 나중에, 나중에 등록한 것이 먼저 실행됩니다. 이걸 모르면 defer 내부의 상태 변경 순서를 잘못 예상합니다.

**고친 코드** (의도를 명확히):

```go
func goodLIFO() {
    result := 0
    defer func() { result *= 10 }()  // 나중에 등록 → 먼저 실행
    defer func() { result++ }()     // 먼저 등록 → 나중에 실행
    // result: 0 * 10 = 0, 0 + 1 = 1
    fmt.Println("result:", result)
}
```

## 주의할 점

**1) defer는 panic 상황에서도 실행됩니다.** Go의 `defer`는 `try/finally`보다 강력합니다. panic이 터져도 defer는 반드시 실행되므로, 자원 정리용으로는 `defer`가 가장 안전합니다. 하지만 `recover()`를 쓰지 않으면 panic은 계속 전파됩니다.

**2) defer 스택이 너무 커지면 성능에 영향.** defer는 스택에 푸시되므로, 수천 개를 등록하면 메모리 사용량이 늘어납니다. 루프 안에서 defer를 남용하지 마세요.

**3) Go 1.22 이후 루프 변수 동작이 바뀌었습니다.** 이전에는 루프 변수가 공유되지만, Go 1.22부터는 iteration마다 새로 생성됩니다. defer를 루프 안에서 쓸 때 이 차이를 고려해야 합니다.

**4) 다른 언어 습관이 통하지 않는 곳.** Python의 `finally` 블록은 항상 현재 스코프의 변수를 참조하지만, Go defer는 인수를 **복사**합니다. JavaScript의 `finally`도 현재 값 참조이지만, Go는 다릅니다. 이 차이를 명확히 알아두세요.

## 코드로 확인하기

`go run .` 실행 결과를 읽는 방법:

1. **인수 평가 시점**: 실수 버전에서 `cleanup:` 뒤에 `/tmp/foo.txt`가 나오는 것은 defer가 등록될 때 `path`가 복사되었기 때문입니다. 고친 버전에서 `/tmp/bar.txt`가 나오는 것은 함수 리터럴 내부에서 `path`를 읽었기 때문입니다.

2. **LIFO**: `LIFO demo 반환값: 11` — `result`는 1로 초기화되고, defer 두 개가 등록됩니다. LIFO 순서로 나중에 등록된 `result *= 10`이 먼저 실행되어 `1 * 10 = 10`, 그 다음 `result++`이 실행되어 `10 + 1 = 11`이 됩니다.

3. **루프**: 실수 버전에서 `loop finished` 메시지가 나오는 것은 defer가 아직 보류 중이기 때문입니다. 고친 버전에서는 각 step에서 `created and closed` 메시지가 나오는 것은 defer가 즉시 실행되었기 때문입니다.

테스트는 `demoLIFOAndNamedReturn()`이 11을 반환하는지, `demoDeferInLoopFixed()`가 파일을 올바르게 닫는지 검증합니다.

## 연습 문제

1. **쉬움**: `defer`를 사용해 함수가 반환될 때 항상 "function end"를 출력하는 함수를 작성하세요.
2. **중간**: 루프 안에서 10개의 임시 파일을 만들고, 각 파일을 즉시 닫는 코드를 `defer`와 익명 함수로 작성하세요.
3. **어려움**: `defer`와 `recover()`를 함께 사용해 panic이 터져도 프로그램이 crash되지 않고 에러 메시지를 출력하는 함수를 작성하세요.

## 더 읽기

- [Go Blog: Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover) — defer의 panic 처리 동작을 공식적으로 설명합니다
- [Effective Go: Defer](https://go.dev/doc/effective_go#defer) — defer의 실제 활용 예시와 주의점을 다룹니다