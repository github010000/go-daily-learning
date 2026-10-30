> ⚠️ **자동 검증 미통과 — `test` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `--- FAIL: TestScoreToGrade (0.00s)`  
> `        main_test.go:29: scoreToGrade(55) = "F", want "D"`  
> `--- FAIL: TestFallthroughBehavior (0.00s)`  
> `    main_test.go:111: scoreToGradeWithFallthrough(95) = "C", want B (fallthrough로 B case 실행)`  
> `--- FAIL: TestScoreToGrade (0.00s)`  
> `        main_test.go:29: scoreToGrade(55) = "F", want "D"`  
> `--- FAIL: TestFallthroughBehavior (0.00s)`  
> `    main_test.go:111: scoreToGradeWithFallthrough(95) = "C", want B (fallthrough로 B case 실행)`  

## 한 줄 요약

switch-case는 조건 분기를 깔끔하게 처리하는 구문이며, Go는 **자동 break**와 **명시적 fallthrough**로 동작을 제어합니다.

## 왜 필요한가

switch문은 **if-else 사슬을 가독성 높게 정리**할 때 필수적입니다. Python의 `match-case`, Java의 `switch`와 유사하지만 Go는 **break가 자동**으로 작동합니다. 이 차이를 이해하지 못하면 "왜 break를 안 쓰지?" 하며 불필요하게 코드를 복잡하게 만들 수 있습니다.

또한 **여러 값을 하나의 case에 묶는 기능**이나 **조건 없는 switch**는 if-else보다 더 명확하게 의도를 표현할 수 있게 해줍니다.

Go가 다른 언어와 다른 점은 **break가 자동으로 삽입**된다는 것입니다. Java나 C에서는 case마다 break를 명시하지 않으면 fallthrough가 발생하므로 실수하기 쉽습니다. Go는 이 실수를 원천적으로 막습니다.

## 기본 사용

```go
switch 변수 {
case 값1, 값2:
    // 값1 또는 값2일 때 실행
case 값3:
    // 값3일 때 실행
default:
    // 위의 case에 해당하지 않을 때 실행
}
```

**조건 없는 switch**: switch 뒤에 변수를 쓰지 않고 각 case에 bool 조건식을 씁니다.

```go
switch {
case 조건1:
    // 조건1이 true일 때
case 조건2:
    // 조건2가 true일 때
}
```

**여러 값 결합**: 쉼표로 값을 나열하면 됩니다.

```go
case 1, 3, 5, 7, 8, 10, 12:
```

## 돌려보기

```bash
go vet ./...      # 정적 검사 — 문법 오류나 unused 변수 확인
go run .          # 시연 실행 — 출력 결과 확인
go test -v ./...  # 테스트 실행 — 모든 테스트 통과 확인
```

## 실전에서는 이렇게 쓴다

**1. 상태 기계 구현**

```go
switch state {
case "idle":
    // 초기 상태
case "running":
    // 실행 중
case "error":
    // 에러 처리
default:
    // 미정의 상태
}
```

**2. HTTP 라우터 구현**

```go
switch r.Method {
case "GET":
    // 조회
case "POST":
    // 생성
case "DELETE":
    // 삭제
default:
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
```

**3. 오류 타입 분기**

```go
switch err := doSomething(); err.(type) {
case *FileError:
    // 파일 오류 처리
case *NetworkError:
    // 네트워크 오류 처리
default:
    // 기타 오류
}
```

## 흔한 실수

### 실수 1: case 안에서 break를 쓰는 것

Go의 switch case 안에서 break를 쓰면 **"현재 case 종료"가 아니라 "바깥 루프 종료"**로 동작합니다.

```go
// 틀린 코드
func scoreToGradeWithBreak(score int) string {
    var grade string
    switch {
    case score >= 90:
        grade = "A"
        break  // 이 break는 루프를 종료시킴!
    case score >= 80:
        grade = "B"
        break
    }
    return grade
}
```

**발생하는 문제**: for 루프 안에서 이 함수를 호출하면, break가 루프를 즉시 종료시켜 이후 요소들이 처리되지 않습니다.

**고친 코드**: break를 완전히 제거합니다.

```go
func scoreToGrade(score int) string {
    switch {
    case score >= 90:
        return "A"
    case score >= 80:
        return "B"
    default:
        return "F"
    }
}
```

### 실수 2: fallthrough를 남용하는 것

fallthrough는 case를 실행한 뒤 **다음 case를 무조건 실행**시킵니다. case 조건이 false여도 실행됩니다.

```go
// 틀린 코드
score := 85
var grade string
switch {
case score >= 90:
    grade = "A"
    fallthrough  // 85 >= 90은 false인데도 다음 case 실행!
case score >= 80:
    grade = "B"
}
// grade = "B" (의도치 않은 결과)
```

**발생하는 문제**: case 조건은 무시되고 다음 코드가 실행되어 예기치 않은 값이 할당됩니다.

**고친 코드**: fallthrough 대신 별도 로직으로 처리하거나, if-else를 사용합니다.

```go
score := 85
var grade string
switch {
case score >= 90:
    grade = "A"
case score >= 80:
    grade = "B"
case score >= 70:
    grade = "C"
}
// grade = "B" (올바른 결과)
```

### 실수 3: default case를 생략하는 것

switch에서 default case를 생략하면 예상치 못한 입력이 처리되지 않습니다.

```go
// 틀린 코드
func describeWeekday(day int) string {
    switch day {
    case 1, 2, 3, 4, 5:
        return "평일"
    case 6, 7:
        return "주말"
    }
    // day = 8이면 빈 문자열 반환 (버그 가능성)
}
```

**발생하는 문제**: day = 8 같은 잘못된 입력이 들어오면 빈 문자열이 반환됩니다.

**고친 코드**: 항상 default case를 추가합니다.

```go
func describeWeekday(day int) string {
    switch day {
    case 1, 2, 3, 4, 5:
        return "평일"
    case 6, 7:
        return "주말"
    default:
        return "잘못된 요일"
    }
}
```

### 실수 4: switch 변수를 생략했을 때의 함정

조건 없는 switch에서 case를 순서대로 실행한다는 점을 잊으면 안 됩니다.

```go
// 틀린 코드
n := 5
switch {
case n > 10:
    fmt.Println("10 초과")
case n > 0:
    fmt.Println("0 초과")  // 5 > 10은 false, 5 > 0은 true → 여기 실행
}
```

**발생하는 문제**: 첫 번째 case가 false이면 두 번째 case로 내려갑니다. 이는 의도한 대로 작동하지만, 코드를 읽을 때 혼란스러울 수 있습니다.

**고친 코드**: case 순서를 명확하게 하고, 필요하면 주석으로 의도를 설명합니다.

```go
n := 5
switch {
case n > 10:
    fmt.Println("10 초과")
case n > 0 && n <= 10:  // 명확하게 범위 지정
    fmt.Println("1~10 사이")
default:
    fmt.Println("0 이하")
}
```

## 주의할 점

**1. switch case는 위에서부터 순서대로 실행됩니다**

첫 번째로 true가 되는 case만 실행됩니다. 이후 case는 무시됩니다.

**2. fallthrough는 조건을 무시합니다**

fallthrough가 있는 case는 실행되면 무조건 다음 case의 코드를 실행합니다. case의 조건식은 무시됩니다.

**3. 타입 switch는 별도 문법입니다**

이 주제에서는 값 switch만 다루며, 타입 switch는 다른 주제에서 다룹니다.

## 코드로 확인하기

`go run .` 명령을 실행하면 다음과 같은 출력을 볼 수 있습니다:

**기본 switch (자동 break)**:
```
=== 1. 기본 switch (자동 break) ===
  scoreToGrade(95) = A
  scoreToGrade(82) = B
  scoreToGrade(70) = C
  scoreToGrade(55) = D
  scoreToGrade(100) = A
```

**break 오용 시연**:
```
=== 2. break 오용 시연 (루프 종료) ===
  [잘못된 버전] break를 case 안에서 사용
  score=95 → grade=A
    (break를 쓰면 루프가 즉시 종료됨 — 의도치 않은 동작)
```

첫 번째 점수(95)만 처리되고 루프가 종료됩니다.

**fallthrough 오용 시연**:
```
=== 3. fallthrough 오용 시연 ===
  [잘못된 버전] 불필요한 fallthrough
  score=85 → grade=B (A가 아닌데도 fallthrough로 B가 할당됨)
```

85점은 A가 아닌데도 fallthrough로 B가 할당됩니다.

**조건 없는 switch**:
```
=== 4. 조건 없는 switch (if-else 사슬 대체) ===
  classifyNumber(-5) = 음수 (if-else: 음수)
  classifyNumber(0) = 영 (if-else: 영)
  classifyNumber(5) = 1~10 사이 (if-else: 1~10 사이)
```

switch와 if-else가 같은 결과를 반환합니다.

**테스트**:
`go test -v ./...`를 실행하면 모든 테스트가 통과해야 합니다.

## 연습 문제

**문제 1 (쉬움)**: 숫자를 Roman numeral으로 변환하는 함수를 switch를 사용해서 구현하세요.

힌트: 1, 4, 5, 9, 10 등 특수 경우를 case로 처리하면 됩니다.

**문제 2 (보통)**: HTTP 요청 메서드(GET, POST, PUT, DELETE)에 따라 다른 처리를 하는 함수를 구현하세요.

힌트: switch 뒤 변수를 생략하고 각 case에 r.Method == "GET" 같은 조건식을 쓰세요.

**문제 3 (어려움)**: 숫자를 영어 단어("one", "two", ...)로 변환하는 함수를 구현하세요. 단, 1~10까지만 처리하고 나머지는 "unknown"을 반환하세요.

힌드: case에 여러 값을 나열하여 1, 2, 3 등을 처리하고, default case에서 "unknown"을 반환하세요.

## 더 읽기

- [Go Tour - switch 문](https://go.dev/tour/flowcontrol/9)
- [Effective Go - switch](https://go.dev/doc/effective_go#switch)
- [Go Blog - Go 1.22 Loop Variable Changes](https://go.dev/blogloopvariable)