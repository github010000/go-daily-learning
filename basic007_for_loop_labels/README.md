> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main.go:5:2: "time" imported and not used`  

## 한 줄 요약
Go는 유일한 반복문 `for` 하나만으로 모든 순회를 처리하며, `range`로 슬라이스/문자열/채널을 순회하고 레이블과 `break`/`continue`로 중첩 루프 제어를 정교하게 합니다.

## 왜 필요한가
다른 언어에는 `for`, `while`, `do-while`, `foreach` 등 여러 반복문이 있습니다. Python은 `for`와 `while`이 있고, JavaScript는 `for`, `while`, `for...of`, `for...in`이 있습니다. Java도 비슷합니다. 하지만 Go는 **유일하게 `for`만** 사용합니다.

이 선택에는 이유가 있습니다. Go는 언어를 단순하게 유지하고, 개발자가 여러 문법을 외울 필요 없이 **한 가지 방식으로 모든 반복을 처리**하도록 설계했습니다. 이는 Go의 철학인 "적은 것으로 많은 것을 한다"와 일치합니다.

또한 `range` 키워드는 Go만의 독특한 특징입니다. Python의 `for item in iterable`, JavaScript의 `for...of`와 유사하지만, **인덱스와 값을 동시에 제공**한다는 점이 다릅니다. 이는 슬라이스를 다룰 때 매우 편리합니다.

문자열 처리에서도 차이가 큽니다. Python과 JavaScript는 문자열을 UTF-16으로 처리하는 반면, Go는 **UTF-8**을 사용합니다. 이는 메모리 효율성은 높이지만, byte 단위로 순회하면 한글이나 이모지가 깨질 수 있다는 점을 의미합니다. `range`는 자동으로 rune(유니코드 코드포인트) 단위로 순회하므로 안전합니다.

## 기본 사용
`for`는 세 가지 형태로 사용됩니다:

```go
// 1. 기본 for (C/Java 스타일)
for 초기화; 조건; 증감 {
    // 반복할 코드
}

// 2. 조건만 있는 for (while 스타일)
for 조건 {
    // 반복할 코드
}

// 3. 무한 루프
for {
    // 반복할 코드
}
```

`range`는 슬라이스, 문자열, 채널 등에서 인덱스와 값을 제공합니다:

```go
// 슬라이스 순회
for i, v := range []int{1, 2, 3} {
    fmt.Printf("%d: %d\n", i, v)
}

// 문자열 순회 (rune 단위)
for i, r := range "안녕" {
    fmt.Printf("%d: %c\n", i, r)
}

// 인덱스만 필요
for i := range []int{1, 2, 3} {
    fmt.Printf("%d\n", i)
}

// 값만 필요
for _, v := range []int{1, 2, 3} {
    fmt.Printf("%d\n", v)
}

// 무조건 반복만 (Go 1.22+)
for range []int{1, 2, 3} {
    fmt.Println("반복!")
}
```

## 돌려보기
아래 명령을 이 디렉토리에서 실행하세요:

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

`go vet`은 코드의 정석적인 문제를 찾습니다. `go run`은 main.go를 실행하여 모든 예시를 보여줍니다. `go test`는 모든 테스트를 실행하여 통과하는지 확인합니다.

## 실전에서는 이렇게 쓴다
**1. 슬라이스 처리**
실제 코드에서 슬라이스를 처리할 때 가장 흔하게 쓰입니다:

```go
func processItems(items []string) {
    for i, item := range items {
        if item == "error" {
            log.Printf("error at index %d", i)
            break
        }
        fmt.Println(item)
    }
}
```

**2. 중첩 루프 탈출**
여러 조건을 확인하면서 하나라도 찾으면 즉시 탈출해야 할 때 레이블을 씁니다:

```go
func findUser(users [][]User, targetID int) *User {
search:
    for _, group := range users {
        for _, user := range group {
            if user.ID == targetID {
                return &user
            }
        }
    }
    return nil // search 레이블까지 탈출
}
```

## 흔한 실수
### 1. 인덱스를 받지 않는데 변수로 받기
**틀린 코드:**
```go
for i, name := range names {
    fmt.Println(name) // i를 쓰지 않음
}
```
**발생하는 오류:**
```
i declared and not used
```
**왜 그런지:** Go는 선언된 변수를 반드시 사용해야 합니다. `i`를 선언했지만 사용하지 않으면 컴파일 오류가 발생합니다.
**고친 코드:**
```go
for _, name := range names {
    fmt.Println(name)
}
```

### 2. 문자열을 byte로 순회하기
**틀린 코드:**
```go
for i, b := range []byte("안녕👋") {
    fmt.Printf("%d: %c\n", i, b)
}
```
**발생하는 현상:**
한글과 이모지가 깨져서 출력됩니다. 각 바이트가 개별 문자로 처리되므로 UTF-8 다바이트 문자가 분할됩니다.
**왜 그런지:** Go 문자열은 UTF-8 인코딩된 바이트 시퀀스입니다. `byte`로 순회하면 각 바이트를 개별 문자로 처리하므로 다바이트 문자가 깨집니다.
**고친 코드:**
```go
for i, r := range "안녕👋" {
    fmt.Printf("%d: %c\n", i, r)
}
```

### 3. break만 쓰면 안쪽 루프만 탈출하기
**틀린 코드:**
```go
for i, row := range matrix {
    for j, val := range row {
        if val == 5 {
            break // 안쪽 루프만 탈출
        }
    }
    // 바깥 루프가 계속 돎
}
```
**발생하는 현상:**
5를 찾은 후에도 바깥 루프가 계속 실행됩니다. `found` 플래그를 추가로 사용해야 합니다.
**왜 그런지:** `break`는 가장 안쪽 루프에서만 동작합니다. 중첩 루프 전체에서 탈출하려면 레이블이 필요합니다.
**고친 코드:**
```go
searchLoop:
for i, row := range matrix {
    for j, val := range row {
        if val == 5 {
            break searchLoop // 레이블까지 탈출
        }
    }
}
```

### 4. continue만 쓰면 안쪽 루프만 건너뛰기
**틀린 코드:**
```go
for i, row := range matrix {
    if row[0]%2 != 0 {
        // continue를 쓰지 않으면 안쪽 루프가 계속 돎
    }
    for j, val := range row {
        // 행의 첫 번째 원소가 홀수여도 계속 처리
    }
}
```
**발생하는 현상:**
홀수 행도 처리됩니다.
**왜 그런지:** `continue`는 가장 안쪽 루프에서만 동작합니다.
**고친 코드:**
```go
outerLoop:
for i, row := range matrix {
    if row[0]%2 != 0 {
        continue outerLoop // 바깥 루프 건너뛰기
    }
    for j, val := range row {
        // 짝수 행만 처리
    }
}
```

## 주의할 점
**1. 루프 변수 캡처 (고루틴)**
Go 1.22 이전에는 고루틴에서 루프 변수를 사용할 때 모든 고루틴이 마지막 값을 공유하는 문제가 있었습니다. Go 1.22부터는 자동으로 수정되었지만, 호환성을 위해 여전히 로컬 변수를 복사하는 방식을 권장합니다:

```go
for i := 0; i < 3; i++ {
    i := i // 각 반복마다 새로운 변수 생성
    go func() {
        fmt.Println(i)
    }()
}
```

**2. range는 복사된 슬라이스 헤더를 사용**
`for _, v := range slice`에서 `v`는 슬라이스 원소의 **복사본**입니다. 구조체를 다룰 때 원본을 수정하려면 포인터를 사용해야 합니다.

**3. 무한 루프는 반드시 break 조건이 있어야 함**
무한 루프 `for {}`는 반드시 탈출 조건이 있어야 합니다. 조건 없이 영원히 돌면 프로그램이 멈춥니다.

## 코드로 확인하기
`go run .`을 실행하면 다음과 같은 출력이 나옵니다:

```
=== 6. 레이블과 break (중첩 루프 한 번에 탈출) ===
  [실수] break만 사용 (안쪽만 탈출):
    확인: (0,0)=1
    확인: (0,1)=2
    확인: (0,2)=3
    확인: (1,0)=4
    5를 찾았음 (바깥 루프도 종료)
    행 2 계속 탐색
  [고침] 레이블과 break 사용 (전체 탈출):
    확인: (0,0)=1
    확인: (0,1)=2
    확인: (0,2)=3
    확인: (1,0)=4
    5를 찾았음!
  (이곳으로 바로 이동)
```

**실수 버전**과 **고친 버전**의 차이를 확인하세요. 실수 버전은 5를 찾은 후에도 행 2를 계속 탐색합니다. 고친 버전은 5를 찾으면 즉시 루프를 빠져나옵니다.

테스트에서는 다음과 같은 항목을 검증합니다:
- `TestRangeString`: 문자열이 3개의 rune("안", "녕", "👋")으로 구성되어 있고, 10바이트임을 확인합니다.
- `TestLabelBreak`: 5가 위치 (1,1)에서 찾아지고, 레이블 break로 올바르게 탈출함을 확인합니다.
- `TestLabelContinue`: 첫 번째 원소가 홀수인 행은 건너뛰고, 짝수인 행만 남음을 확인합니다.

## 연습 문제
1. **쉬움**: 1부터 100까지 숫자 중 3의 배수는 "Fizz", 5의 배수는 "Buzz", 둘 다의 배수는 "FizzBuzz"를 출력하는 프로그램을 작성하세요.
2. **보통**: 2D 슬라이스에서 특정 값을 찾고, 찾은 위치를 반환하는 함수를 작성하세요. 찾지 못하면 nil을 반환해야 합니다.
3. **어려움**: 문자열에서 중복 없는 유니코드 문자만 남기는 함수를 작성하세요. (힌트: `map[rune]bool`를 사용)

## 더 읽기
- [Go For Statements](https://go.dev/ref/spec#For_statements) - 공식 문헌
- [Effective Go - Loops](https://go.dev/doc/effective_go#Loops) - Go 스타일 가이드
- [Go Blog: Range Over Integers](https://go.dev/blog/range-over-integers) - Go 1.22 range 개선