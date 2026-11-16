> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `main.go:137:2: fmt.Println call has possible Printf formatting directive %v`  
> `main_test.go:50:26: (*testing.common).Errorf format %v reads arg #1, but call has 0 args`  

## 한 줄 요약

Go 1.13 이후 에러는 `fmt.Errorf`의 `%w`로 감싸고, `errors.Is`/`errors.As`로 검사해야 한다.

## 왜 필요한가

에러 처리는 소프트웨어의 신뢰성을 결정하는 핵심 기능입니다. Python이나 JavaScript에서는 예외(exception)를 던지고 `try-catch`로 잡는 방식이 일반적입니다. Java도 `throw`와 `catch`로 에러를 전달하지요. 그런데 Go는 다릅니다. Go는 에러를 "던지지" 않고 **값으로 반환**합니다.

이게 왜 중요한가? 에러를 값으로 다루면 에러를 다른 값처럼 다룰 수 있습니다. 함수의 반환값으로 에러를 전달하고, 에러를 감싸서 새로운 컨텍스트를 추가할 수 있습니다. 하지만 이때 중요한 문제가 하나 생깁니다. **에러를 감싸면 원본 에러를 잃어버릴까?**

Go 1.13 이전에는 에러를 감쌀 때 `fmt.Errorf("에러 메시지: %v", err)`를 썼습니다. 이건 에러를 텍스트로 합치는 거라, 나중에 `errors.Is`로 내부 원인을 찾을 수 없었습니다. Go 1.13에서 `%w`가 도입되면서 에러 체인(wrap chain)이 생겼고, `errors.Is`/`errors.As`로 체인을 거슬러 올라가며 에러를 찾을 수 있게 됐습니다.

또 다른 중요한 차이가 있습니다. **센티널 에러(sentinel error)**입니다. Python의 `ValueError`, JavaScript의 `TypeError`처럼 프로그램 전반에서 재사용하는 에러가 있습니다. Go에서는 `var ErrNotFound = errors.New("데이터를 찾지 못함")`처럼 변수로 정의합니다. 그런데 이 에러를 `==`로 비교하면 래핑된 에러에서는 실패합니다. `errors.Is`를 써야 합니다.

## 기본 사용

```go
// 1. 에러 래핑: %w를 사용하라
func readConfig(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        // %w: 에러 체인 생성
        return fmt.Errorf("설정 파일 읽기 실패: %w", err)
    }
    return nil
}

// 2. 에러 검사: errors.Is / errors.As
func handleConfigError(err error) {
    // 센티널 에러 검사
    if errors.Is(err, os.ErrNotExist) {
        fmt.Println("파일이 없습니다")
        return
    }
    
    // 구체 타입 추출
    var pathErr *os.PathError
    if errors.As(err, &pathErr) {
        fmt.Printf("경로 에러: %s\n", pathErr.Path)
    }
}
```

핵심: `%w`로 감싸면 `errors.Is`로 찾을 수 있고, `%v`로 감싸면 찾을 수 없습니다.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

- `go vet`: 에러 처리 관련 코드의 정적 검사를 수행합니다. 포맷Directive 오류 등을 잡아냅니다.
- `go run .`: 에러 래핑, 센티널 비교, 타입 추출, nil 안전성 시연을 실행합니다.
- `go test -v ./...`: 에러 처리 로직의 정확성을 검증합니다.

## 실전에서는 이렇게 쓴다

**1. HTTP 서버 에러 처리**

```go
func handleRequest(w http.ResponseWriter, r *http.Request) error {
    user, err := db.FindUser(r.URL.Query().Get("id"))
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            http.Error(w, "사용자를 찾을 수 없습니다", http.StatusNotFound)
            return nil
        }
        return fmt.Errorf("사용자 조회 실패: %w", err)
    }
    // ... 처리
}
```

**2. CLI 도구 에러 처리**

```go
func run() error {
    if err := initConfig(); err != nil {
        if errors.Is(err, ErrConfigMissing) {
            fmt.Fprintln(os.Stderr, "설정 파일이 없습니다")
            return err
        }
        return fmt.Errorf("설정 초기화 실패: %w", err)
    }
    return nil
}

func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "오류: %v\n", err)
        os.Exit(1)
    }
}
```

## 흔한 실수

### 실수 1: %v로 래핑하면 errors.Is가 동작하지 않는다

```go
// 틀린 코드
func openFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("파일 열기 실패: %v", err) // %v 사용
    }
    return nil
}

func handleError(err error) {
    if errors.Is(err, os.ErrNotExist) { // false 반환!
        // 이 코드는 실행되지 않는다
    }
}
```

**무엇이 일어나는가?** `%v`는 에러를 텍스트로 합칠 뿐, 에러 체인을 형성하지 않습니다. 따라서 `errors.Is`가 체인을 거슬러 올라가며 `os.ErrNotExist`를 찾을 수 없습니다.

**왜 그런가?** `%v`는 `fmt.Formatter` 인터페이스를 호출하지 않고, 단순히 `Error()` 메서드 결과를 문자열로 붙입니다. `%w`는 내부적으로 에러 객체를 저장하고 `errors.Unwrap()`을 통해 체인을 형성합니다.

**고친 코드:**

```go
func openFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("파일 열기 실패: %w", err) // %w 사용
    }
    return nil
}
```

### 실수 2: 센티널 에러를 ==로 비교한다

```go
// 틀린 코드
var ErrNotFound = errors.New("데이터를 찾지 못함")

func getUser(id int) error {
    if id == 999 {
        return ErrNotFound
    }
    return nil
}

func callGetUser(id int) error {
    err := getUser(id)
    if err != nil {
        return fmt.Errorf("사용자 조회 실패: %w", err) // 래핑
    }
    return nil
}

func handleError(err error) {
    if err == ErrNotFound { // false 반환!
        fmt.Println("찾지 못함")
    }
}
```

**무엇이 일어나는가?** `getUser(999)`는 `ErrNotFound`을 반환하지만, `callGetUser`가 `%w`로 래핑하면 에러가 새로운 객체가 됩니다. `==` 비교는 메모리 주소를 비교하므로 항상 `false`를 반환합니다.

**왜 그런가?** Go의 에러는 인터페이스 값입니다. `==`는 인터페이스의 동적 값을 비교하는데, 래핑된 에러는 원본 에러를 내부에 포함하는 **새로운 구조체**이므로 메모리 주소가 다릅니다.

**고친 코드:**

```go
func handleError(err error) {
    if errors.Is(err, ErrNotFound) { // true 반환!
        fmt.Println("찾지 못함")
    }
}
```

### 실수 3: errors.As에 포인터 포인터를 전달하지 않는다

```go
// 틀린 코드
type CustomError struct {
    Code int
}

func (e *CustomError) Error() string {
    return fmt.Sprintf("에러 코드: %d", e.Code)
}

func checkError(err error) {
    var customErr CustomError // 값 타입
    if errors.As(err, &customErr) { // 잘못된 사용!
        // customErr은 nil일 수 있고, 수정해도 원본 에러의 필드에 반영되지 않는다
    }
}
```

**무엇이 일어나는가?** `errors.As`는 두 번째 인수로 **포인터 포인터**(`**T`)를 받습니다. 값 타입을 전달하면 컴파일은 되지만, 에러가 해당 타입이 아닌 경우 `customErr`은 영변한 값으로 남고, 에러가 해당 타입인 경우에도 추출된 값을 수정할 수 없습니다.

**왜 그런가?** `errors.As`는 에러 체인에서 해당 타입을 찾아서 포인터에 할당합니다. 이 작업은 포인터 포인터를 통해 이루어집니다. 값 타입을 전달하면 포인터를 얻을 수 없고, 할당이 불가능합니다.

**고친 코드:**

```go
func checkError(err error) {
    var customErr *CustomError // 포인터 타입
    if errors.As(err, &customErr) {
        fmt.Printf("에러 코드: %d\n", customErr.Code)
    }
}
```

### 실수 4: nil 에러에서 errors.As를 잘못 사용한다

```go
// 틀린 코드
func safeCheck(err error) {
    if err != nil {
        var customErr *CustomError
        errors.As(err, &customErr) // nil이 아니면 호출
    }
}
```

**무엇이 일어나는가?** 이 코드는 기능적으로는 맞지만, 불필요한 nil 체크가 있습니다. `errors.As`는 내부적으로 nil 에러를 처리하므로 nil 체크 없이 호출해도 안전합니다.

**왜 그런가?** `errors.As`의 구현을 보면, 첫 번째 인수가 `nil`이면 바로 `false`를 반환합니다. 따라서 nil 체크가 필요 없습니다.

**고친 코드:**

```go
func safeCheck(err error) {
    var customErr *CustomError
    if errors.As(err, &customErr) {
        fmt.Printf("에러 코드: %d\n", customErr.Code)
    }
}
```

## 주의할 점

**1. 에러 체인은 무한히 깊어질 수 있다**

에러를 계속 래핑하면 체인이 길어집니다. `errors.Is`/`errors.As`는 체인을 거슬러 올라가지만, 너무 깊으면 성능에 영향을 줄 수 있습니다. 실제로는 3~5단계 정도가 적절합니다.

```go
func level1() error {
    return fmt.Errorf("level1: %w", level2())
}

func level2() error {
    return fmt.Errorf("level2: %w", level3())
}

func level3() error {
    return fmt.Errorf("level3: %w", os.ErrNotExist)
}
```

**2. 에러 메시지를 중복해서 만들지 말라**

`fmt.Errorf`로 에러를 래핑할 때, 메시지에 이미 에러 메시지가 포함되는 경우가 있습니다.

```go
// 중복 메시지
return fmt.Errorf("파일 열기 실패: %w", err) // err에 "파일 열기 실패"가 포함될 수 있다
```

에러 메시지가 이미 상세하다면 간단히 래핑하는 게 낫습니다.

**3. 에러를 숨기지 말라**

```go
// 틀린 코드: 에러를 무시한다
if err := doSomething(); err != nil {
    log.Println(err)
    return nil // 에러를 숨긴다
}
```

에러를 로깅했다고 해서 에러를 무시할 수 없습니다. 호출자에게 에러를 전달해야 합니다.

## 코드로 확인하기

`main.go`를 실행하면 다음과 같은 출력이 나옵니다.

```
=== 1. 에러 래핑: %v vs %w ===
  [v 래핑] ErrNotExist 직접 비교: false
  [v 래핑] 에러 메시지: 파일 열기 실패 (nonexistent.txt): open nonexistent.txt: No such file or directory
  [w 래핑] ErrNotExist 직접 비교: true
  [w 래핑] 에러 메시지: 파일 열기 실패 (nonexistent.txt): open nonexistent.txt: No such file or directory

=== 2. 센티널 에러: == vs errors.Is ===
  [잘못된 비교] ErrNotFound 미발견 (동등 연산자)
  [올바른 비교] errors.Is로 ErrNotFound 발견
  [잘못된 비교] ErrPermissionDenied 미발견 (동등 연산자)
  [올바른 비교] errors.Is로 ErrPermissionDenied 발견

=== 3. errors.As: 타입 추출 ===
  [CustomError] 발견! ID=-1, Msg="ID -1: 존재하지 않음"
  [CustomError] 사용자 찾음

=== 4. nil 안전성 ===
  [Safe] CustomError 아님 (nil 포함)
```

**출력을 읽는 법:**

1. `%v` 래핑은 `errors.Is`로 `os.ErrNotExist`를 찾지 못합니다 (`false`). `%w` 래핑은 찾습니다 (`true`).
2. `==` 비교는 래핑된 에러에서 항상 `false`를 반환합니다. `errors.Is`는 에러 체인을 탐색해 `true`를 반환합니다.
3. `errors.As`는 구체 에러 타입을 찾아 필드에 접근할 수 있습니다.
4. `errors.As`는 nil 에러에서도 panic하지 않고 안전하게 처리합니다.

`main_test.go`는 다음과 같은 사항을 검증합니다:
- `TestSentinelErrorComparison`: `errors.Is`가 센티널 에러를 올바르게 찾는지
- `TestWrappedErrorIs`: `%w` 래핑이 `errors.Is`로 찾아지는지
- `TestCustomErrorAs`: `errors.As`가 구체 타입을 찾고 값을 추출하는지
- `TestNilSafeAs`: nil 에러에서 `errors.As`가 안전하게 동작하는지

## 연습 문제

1. **에러 래핑 연습**: `os.ReadFile`를 호출하는 함수를 만들고, `%w`로 에러를 래핑하세요. `errors.Is`로 `os.ErrNotExist`를 검사하는 코드를 작성하세요.
2. **사용자 정의 에러 타입**: `TimeoutError`라는 구조체를 만들고, `Error()` 메서드를 구현하세요. `errors.As`로 이 에러를 추출하는 코드를 작성하세요.
3. **에러 체인 디버깅**: 3단계로 에러를 래핑하는 함수를 만들고, `errors.Is`와 `errors.As`로 내부 에러를 각각 찾세요.

## 더 읽기

- [Go 1.13 Errors 블로그](https://go.dev/blog/go1.13-errors) - 공식 문서
- [Go Errors 정리](https://go.dev/doc/error) - 공식 가이드
- [Effective Go - Errors](https://go.dev/doc/effective_go#errors) - 실전 팁