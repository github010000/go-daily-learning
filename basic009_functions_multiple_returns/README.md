## 한 줄 요약
Go의 함수는 여러 값을 동시에 반환할 수 있으며, 특히 `(값, error)` 패턴이 표준 관례입니다. 이름 붙은 반환값은 코드를 간결하게 하지만, `defer`와 함께 쓰면 예상치 못한 결과를 초래할 수 있으므로 신중해야 합니다.

## 왜 필요한가
다른 언어에서는 에러 처리를 예외(Exception)나 특수 반환 객체로 처리하는 경우가 많습니다. 예를 들어 Java는 `throw new Exception()`, JavaScript는 `throw new Error()`를 씁니다. Go는 에러를 **값**으로 반환합니다.

이것이 중요한 이유는:
1. **에러가 흐름을 갑자기 끊지 않습니다.** 호출자가 명시적으로 에러를 체크해야 하므로, 에러 처리를 깜빡하기 어렵습니다.
2. **여러 값을 한 번에 반환할 수 있습니다.** `(data, error)`처럼 반환하면, 데이터가 없어도 에러 메시지를 함께 전달할 수 있습니다.
3. **이름 붙은 반환값**은 함수 내부에서 변수를 선언한 것처럼 보이게 하여 코드를 읽기 쉽게 만듭니다. 하지만 `defer`와 함께 사용하면 **반환값이.defer 내부에서 덮어쓰이는 함정**에 빠지기 쉽습니다.

Go는 왜 이렇게 했을까요?
- **명확성**: 에러를 값으로 반환함으로써 호출자가 "이 함수가 실패할 수 있다"는 것을 코드로 명시합니다.
- **제어**: 호출자가 에러를 무시하거나 처리할지 결정할 수 있습니다.
- **간결성**: 이름 붙은 반환값과 `naked return`을 통해 불필요한 변수 선언을 줄입니다.

## 기본 사용

### 1. 다중 반환값 (값, error)
```go
func splitName(full string) (string, string, error) {
    parts := strings.Fields(full)
    if len(parts) < 2 {
        return "", "", fmt.Errorf("이름이 부족합니다: %q", full)
    }
    return parts[0], parts[1], nil
}
```
- 반환 타입 괄호 `()` 안에 반환값 타입을 나열합니다.
- 에러가 없으면 `nil`을 반환합니다.
- 호출 시 `first, last, err := splitName("Kim Min-su")`처럼 받습니다.

### 2. 이름 붙은 반환값
```go
func countWords(text string) (count int, maxLen int) {
    for _, word := range strings.Fields(text) {
        count++
        if len(word) > maxLen {
            maxLen = len(word)
        }
    }
    return count, maxLen // 또는 그냥 `return` (naked return)
}
```
- 반환값에 이름을 붙이면 함수 내부에서 변수처럼 사용할 수 있습니다.
- `return`만 쓰면 이름 붙은 반환값을 자동으로 반환합니다(`naked return`).

### 3. defer와 이름 붙은 반환값
```go
func deferInFunc(success bool) (result int, err error) {
    defer func() {
        if !success {
            err = fmt.Errorf("처리 실패: %v", result) // err를 덮어씀
        }
    }()
    result = 42
    if !success {
        return 0, nil // 이 시점의 err는 nil이지만, defer가 err를 바꿈
    }
    return result, nil
}
```
- **주의**: `defer` 함수는 `return` **후에** 실행되지만, 이름 붙은 반환값을 **덮어쓸 수 있습니다**.
- 이로 인해 예상치 못한 에러가 반환될 수 있습니다.

## 돌려보기

아래 명령을 실행하여 코드의 동작을 확인하세요.

```bash
go vet ./...      # 정적 검사 (컴파일 오류 체크)
go run .          # 시연 실행 (출력으로 실수/고친 버전 차이 확인)
go test -v ./...  # 테스트 실행 (테이블 주도 테스트 확인)
```

- `go run .` 실행 시:
  - `deferInFunc` (실수 버전)은 `success=false`일 때 **err이 nil이 아닌 값**을 반환합니다.
  - `deferInFuncFixed` (고친 버전)은 `success=false`일 때 **명시적 err**를 반환합니다.
- `go test -v ./...` 실행 시:
  - `TestSplitName`, `TestCountWords`, `TestDeferInFuncFixed`가 통과해야 합니다.

## 실전에서는 이렇게 쓴다

### 1. 표준 라이브러리 패턴
```go
// os.ReadFile는 (content []byte, err error)를 반환합니다.
content, err := os.ReadFile("file.txt")
if err != nil {
    log.Fatal(err)
}
```

### 2. 에러 처리 관례
```go
func divide(a, b int) (int, error) {
    if b == 0 {
        return 0, fmt.Errorf("0으로 나눌 수 없습니다")
    }
    return a / b, nil
}
```

## 흔한 실수

### 1. defer에서 이름 붙은 반환값을 덮어쓰기
**틀린 코드**:
```go
func deferInFunc(success bool) (result int, err error) {
    defer func() {
        if !success {
            err = fmt.Errorf("처리 실패: %v", result) // err를 덮어씀
        }
    }()
    result = 42
    if !success {
        return 0, nil // err가 nil인 줄 알지만, defer가 err를 바꿈
    }
    return result, nil
}
```
**어떤 일이 벌어지는지**:
- `success=false`일 때, `return 0, nil`을 하지만 `defer`가 `err`를 `fmt.Errorf(...)`로 덮어씁니다.
- 결과: `err`가 `nil`이 아닌 에러 메시지를 반환합니다.

**왜 그런지**:
- `defer` 함수는 `return` **후에** 실행됩니다.
- 이름 붙은 반환값 `err`를 `defer` 내부에서 수정하면, **반환 전에** 수정이 반영됩니다.

**고친 코드**:
```go
func deferInFuncFixed(success bool) (int, error) {
    var result int
    defer func() {
        if !success {
            fmt.Println("[defer] cleanup: result was", result)
        }
    }()
    result = 42
    if !success {
        return 0, fmt.Errorf("처리 실패: %v", result)
    }
    return result, nil
}
```

### 2. naked return을 남용하기
**틀린 코드**:
```go
func countWords(text string) (count int, maxLen int) {
    for _, word := range strings.Fields(text) {
        count++
        if len(word) > maxLen {
            maxLen = len(word)
        }
    }
    return // naked return
}
```
**어떤 일이 벌어지는지**:
- 코드는 컴파일되고 실행됩니다.
- 하지만 `return` 위치가 명확하지 않아, 나중에 코드를 추가할 때 실수하기 쉽습니다.

**왜 그런지**:
- `naked return`은 함수가 짧고 반환값이 명확할 때만 사용해야 합니다.
- 복잡한 로직에서는 명시적 `return count, maxLen`이 더 읽기 좋습니다.

**고친 코드**:
```go
func countWords(text string) (count int, maxLen int) {
    for _, word := range strings.Fields(text) {
        count++
        if len(word) > maxLen {
            maxLen = len(word)
        }
    }
    return count, maxLen // 명시적 반환
}
```

### 3. 에러를 무시하기
**틀린 코드**:
```go
_, err := splitName("Kim Min-su")
if err != nil {
    log.Fatal(err)
}
// firstName, lastName을 사용하지 않음
```
**어떤 일이 벌어지는지**:
- `_`로 에러를 무시하면, 에러가 있어도 프로그램이 계속 실행됩니다.
- 결과: 빈 문자열이나 잘못된 데이터가 사용될 수 있습니다.

**왜 그런지**:
- Go는 에러를 값으로 반환하므로, 호출자가 **명시적으로** 처리해야 합니다.
- 에러를 무시하면 디버깅이 매우 어려워집니다.

**고친 코드**:
```go
first, last, err := splitName("Kim Min-su")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("이름: %s %s\n", first, last)
```

## 주의할 점

1. **defer와 이름 붙은 반환값**:
   - `defer`가 이름 붙은 반환값을 덮어쓸 수 있습니다.
   - 가능한 한 일반 반환값을 사용하고, `defer`에서는 cleanup만 수행하세요.

2. **naked return 남용**:
   - 함수가 길거나 반환값이 많으면 명시적 `return`을 사용하세요.
   - `naked return`은 코드를 읽기 어렵게 만들 수 있습니다.

3. **에러 무시**:
   - `_`로 에러를 무시하지 마세요.
   - 에러는 항상 처리하거나, 명시적으로 무시한다고 주석을 달세요.

## 코드로 확인하기

### `go run .` 출력 해석
- **deferInFunc (실수 버전)**:
  - `success=false`일 때: `err`가 `nil`이 아닌 에러 메시지를 반환합니다.
  - 이유: `defer`가 `err`를 덮어썼기 때문입니다.
- **deferInFuncFixed (고친 버전)**:
  - `success=false`일 때: 명시적 `err`를 반환합니다.
  - 이유: `defer`는 cleanup만 하고, 반환값은 `return`으로 결정됩니다.

### `go test -v ./...` 출력 해석
- `TestSplitName`: `splitName`의 성공/실패 케이스를 검증합니다.
- `TestCountWords`: `countWords`의 단어 수와 최대 길이를 검증합니다.
- `TestDeferInFuncFixed`: `deferInFuncFixed`가 올바른 반환값을 반환하는지 검증합니다.

## 연습 문제

1. **쉬운 문제**: `splitName` 함수를 수정하여, 이름이 3개 이상이면 세 번째 이름을 `middleName`으로 반환하는 버전을 만드세요. (힌트: 반환값에 `middleName` 추가)
2. **중간 문제**: `countWords` 함수를 수정하여, 단어가 10개 이상이면 에러를 반환하는 버전을 만드세요. (힌트: `(count, maxLen, error)` 반환)
3. **어려운 문제**: `deferInFunc` 함수를 수정하여, `defer`가 반환값을 덮어쓰지 않도록 하세요. (힌트: 이름 붙은 반환값 대신 일반 반환값 사용)

## 더 읽기
- [Go Tour: Functions](https://go.dev/tour/basics/6)
- [Effective Go: Errors](https://go.dev/doc/effective_go#errors)
- [Go Blog: Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover)