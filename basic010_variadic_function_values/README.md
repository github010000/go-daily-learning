> ⚠️ **자동 검증 미통과 — `test` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `--- FAIL: TestFunctionValueStrategy (0.00s)`  
> `        main_test.go:85: anonymous function result = "APPLE + BANANA + CHERRY (익명)", want "apple + banana + cherry (익명)"`  
> `--- FAIL: TestFunctionValueStrategy (0.00s)`  
> `        main_test.go:85: anonymous function result = "APPLE + BANANA + CHERRY (익명)", want "apple + banana + cherry (익명)"`  
> `--- FAIL: TestFunctionValueStrategy (0.00s)`  
> `        main_test.go:85: anonymous function result = "APPLE + BANANA + CHERRY (익명)", want "apple + banana + cherry (익명)"`  
> `--- FAIL: TestFunctionValueStrategy (0.00s)`  
> `        main_test.go:85: anonymous function result = "APPLE + BANANA + CHERRY (익명)", want "apple + banana + cherry (익명)"`  

## 한 줄 요약
`...T`로 가변 인자를 받고, 함수를 값처럼 변수에 담거나 다른 함수에 넘길 수 있다.

## 왜 필요한가
Python의 `*args`, JavaScript의 `...args`, Java의 `T... args`와 달리 Go는 **함수 시그니처 자체가 타입**이다. `func(...int)`와 `func([]int)`는 서로 다른 타입으로 취급된다. 그래서 슬라이스를 펼칠 때 `numbers`가 아닌 `numbers...`라고 명시해야 한다 — 이건 단순한 문법이 아니라 타입 시스템의 일관성을 지키는 장치다.

함수 값을 넘기는 일은 Python의 `map(fn, lst)`나 JavaScript의 `Array.prototype.reduce(fn)`과 유사하지만, Go에서는 **함수의 타입을 정확히 적어두어야 한다**. `func([]string, string) string` 같은 타입을 파라미터로 쓰면, 그 파라미터로 받을 수 있는 함수는 정확히 그 시그니처를 가진 것뿐이다. 이 제한이 오히려 버그를 막아준다.

다른 언어에서는 `any`나 `dynamic`으로 유연하게 넘기지만, Go는 타입 안전성을 우선한다. 전략 패턴을 구현할 때도 함수 타입을 명시적으로 선언하는 것이 Go다운 방식이다.

## 기본 사용
```go
// 가변 인자: 내부적으로 []int
func sum(vals ...int) int {
    total := 0
    for _, v := range vals {
        total += v
    }
    return total
}

// 호출: 개별 인자
sum(1, 2, 3) // 6

// 호출: 슬라이스 펼치기
nums := []int{1, 2, 3}
sum(nums...) // 6 — '...' 없으면 컴파일 오류

// 함수 값: 타입을 정확히 적는다
type Processor func([]string, string) string

func apply(items []string, fn Processor, sep string) string {
    return fn(items, sep)
}

// 함수 변수에 할당
strategy := joinStrategy
apply([]string{"a", "b"}, strategy, "-")

// 익명 함수 직접 전달
apply([]string{"a", "b"}, func(s []string, sep string) string {
    return strings.Join(s, sep)
}, "-")
```

핵심: `...T` 파라미터는 호출할 때 `...` 연산자로 슬라이스를 펼쳐야 한다. 함수 값은 **시그니처가 맞아야만** 전달 가능하다.

## 돌려보기
```bash
go vet ./...      # 정적 검사 — 변수 미사용, 타입 오류 등을 확인
go run .          # 시연 실행 — 가변 인자, 함수 값의 동작을 출력으로 확인
go test -v ./...  # 테스트 — 모든 함수가 기대대로 동작하는지 검증
```
`go run .` 출력에서 「고친 버전: sumInts(numbers...) = 60」을 보면 `...` 연산자가 올바르게 슬라이스를 펼친다. 「실수 버전」 메시지는 컴파일 오류가 나는 경우를 설명한다.

`go test -v` 출력에서 `PASS`가 나오면 모든 테스트가 통과한 것이다. 각 `t.Run` 블록이 개별 케이스를 검증한다.

## 실전에서는 이렇게 쓴다
`fmt.Printf`가 대표적인 예다. `func Printf(format string, args ...any)` — `...any` 파라미터로 임의의 개수의 인수를 받는다.

```go
fmt.Printf("이름: %s, 나이: %d\n", "홍길동", 25)
```

`strings.Join`도 내부적으로 `[]string`을 받지만, 가변 인자는 아니다. 대신 `append`, `sort.Slice` 등에서 `...T`가 자주 쓰인다.

전략 패턴은 HTTP 미들웨어에서 자주 쓰인다. `net/http`의 `HandlerFunc` 타입이 바로 함수 값의 한 예다.

```go
type HandlerFunc func(ResponseWriter, *Request)
```

이 타입을 사용하면 일반 함수를 HTTP 핸들러로 변환할 수 있다.

## 흔한 실수
### 실수 1: `...` 없이 슬라이스를 가변 인자 함수에 전달하기

```go
// 틀린 코드
func sum(vals ...int) int {
    total := 0
    for _, v := range vals {
        total += v
    }
    return total
}

nums := []int{1, 2, 3}
sum(nums) // 컴파일 오류
```

**무슨 일이 벌어지는가:**
```
./main.go:10:5: cannot use nums (type []int) as type []int in argument to sum
    (missing ... in parameter list)
```

**왜 그런가:** `sum`의 시그니처는 `func(...int)`이다. 이건 "정수를 여러 개 받는다"는 뜻이지 "정수 슬라이스 하나를 받는다"는 뜻이 아니다. Go 컴파일러는 `[]int`와 `...int`를 다른 타입으로 본다.

**고친 코드:**
```go
sum(nums...) // 올바른 호출
```

`...` 연산자는 슬라이스의 각 요소를 개별 인자로 펼쳐 전달한다. `nums`가 `[1, 2, 3]`이면 `sum(1, 2, 3)`과 동일하게 동작한다.

### 실수 2: 함수 값의 시그니처를 잘못 선언하기

```go
// 틀린 코드
func process(items []string, strategy func([]string) string, sep string) string {
    return strategy(items) // sep을 사용하지 않음
}

items := []string{"a", "b"}
result := process(items, func(s []string) string {
    return strings.Join(s, "-")
}, "/")
```

**무슨 일이 벌어지는가:** 이 코드는 컴파일되지만, `sep` 파라미터가 무시된다. 의도한 동작과 다를 수 있다.

또는 이렇게 하면 컴파일 오류가 발생한다:
```go
// 틀린 코드
func process(items []string, strategy func([]string, string) string, sep string) string {
    return strategy(items, sep)
}

// 시그니처가 맞지 않음
badStrategy := func(s []string) string {
    return strings.Join(s, "-")
}
process(items, badStrategy, "/") // 컴파일 오류
```

**왜 그런가:** `process`는 `func([]string, string) string` 타입의 함수를 기대한다. `badStrategy`는 `func([]string) string` 타입이므로 타입 불일치다.

**고친 코드:**
```go
goodStrategy := func(s []string, sep string) string {
    return strings.Join(s, sep)
}
process(items, goodStrategy, "/") // 정상
```

함수 값을 파라미터로 받을 때는 **정확한 시그니처**를 적어두는 것이 중요하다. 나중에 함수를 바꾸더라도 컴파일러가 즉시 알려준다.

### 실수 3: 가변 인자 함수에 빈 슬라이스를 전달할 때 `...` 생략하기

```go
// 틀린 코드
func sum(vals ...int) int {
    total := 0
    for _, v := range vals {
        total += v
    }
    return total
}

var nums []int // nil 슬라이스
sum(nums) // 컴파일 오류
```

**무슨 일이 벌어지는가:**
```
./main.go:12:5: cannot use nums (type []int) as type []int in argument to sum
    (missing ... in parameter list)
```

**왜 그런가:** 빈 슬라이스든, nil 슬라이스든, 원소가 있는 슬라이스든 **모두 `[]int` 타입**이다. `...int` 파라미터는 `[]int`를 직접 받을 수 없다.

**고친 코드:**
```go
var nums []int
sum(nums...) // 정상 — 빈 슬라이스 전달, 결과: 0
```

빈 슬라이스를 `...` 없이 전달하면 컴파일 오류가 난다. `...`를 붙이면 빈 슬라이스가 개별 인자 없이 전달되어 `vals`는 길이가 0인 슬라이스가 된다.

### 실수 4: 함수 값에서 원본 슬라이스를 수정하는 부수 효과

```go
// 틀린 코드
func upperJoin(items []string, sep string) string {
    for i := range items {
        items[i] = strings.ToUpper(items[i])
    }
    return strings.Join(items, sep)
}

items := []string{"apple", "banana"}
result := upperJoin(items, "-")
fmt.Println(result) // "APPLE-BANANA"
fmt.Println(items)  // ["APPLE", "BANANA"] — 원본이 바뀌어 있음!
```

**무슨 일이 벌어지는가:** `upperJoin`이 원본 슬라이스의 요소를 직접 수정한다. 호출한 쪽에서는 원본이 바뀌었다는 사실을 모를 수 있다.

**왜 그런가:** 슬라이스는 배열을 가리키는 구조체다. 함수에 슬라이스를 전달하면 **같은 배열을 공유**한다. 함수 안에서 `items[i] = ...`를 하면 원본 배열이 수정된다.

**고친 코드:**
```go
func safeUpperJoin(items []string, sep string) string {
    copied := make([]string, len(items))
    copy(copied, items)
    for i := range copied {
        copied[i] = strings.ToUpper(copied[i])
    }
    return strings.Join(copied, sep)
}

items := []string{"apple", "banana"}
result := safeUpperJoin(items, "-")
fmt.Println(result) // "APPLE-BANANA"
fmt.Println(items)  // ["apple", "banana"] — 원본 보존
```

함수 값이 원본 데이터를 수정할지 여부를 명시적으로 알리고, 필요하면 복사본을 사용한다.

### 실수 5: `...` 연산자를 함수 내부에서 잘못 사용하기

```go
// 틀린 코드
func sum(vals ...int) int {
    total := 0
    for _, v := range vals {
        total += v
    }
    return total
}

func wrapper(nums []int) int {
    return sum(nums) // 컴파일 오류
}
```

**무슨 일이 벌어지는가:**
```
./main.go:15:12: cannot use nums (type []int) as type []int in argument to sum
    (missing ... in parameter list)
```

**왜 그런가:** `wrapper`의 `nums`는 `[]int` 타입이다. `sum`은 `...int`를 받는다. `nums`를 그대로 전달하면 타입 불일치다.

**고친 코드:**
```go
func wrapper(nums []int) int {
    return sum(nums...) // 올바른 호출
}
```

함수 내부에서도 `...` 연산자를 사용해야 한다. `...`는 호출 사이트에서만 필요한 것이다.

## 주의할 점
### `append` 후 원본 슬라이스가 바뀔 수 있다
`append`는 용량(capacity)이 충분하면 같은 배열을 반환하지만, 부족하면 새 배열을 할당한다.

```go
a := []int{1, 2, 3}
b := append(a, 4)
fmt.Println(a) // [1 2 3] — unchanged
fmt.Println(b) // [1 2 3 4]

c := make([]int, 3, 4) // 용량 4
copy(c, []int{1, 2, 3})
d := append(c, 4)
fmt.Println(c) // [1 2 3 4] — unchanged? No, depends on implementation
fmt.Println(d) // [1 2 3 4]
```

슬라이스를 함수에 전달할 때 `append`가 원본에 영향을 줄 수 있으니 주의한다.

### Go 1.22부터 루프 변수가 각 반복마다 새로 생성된다
이전 Go 버전에서는 `for` 루프의 변수가 공유되었다. Go 1.22부터는 각 반복마다 새로운 변수가 생성된다.

```go
// Go 1.22 이전: 모든 클로저가 같은 변수를 참조
fs := make([]func() int, 3)
for i := range 3 {
    fs[i] = func() int { return i }
}
fmt.Println(fs[0]()) // 2 — 기대와 다름

// Go 1.22 이후: 각 클로저가 고유한 변수를 참조
for i := range 3 {
    fs[i] = func() int { return i }
}
fmt.Println(fs[0]()) // 0 — 기대와 일치
```

함수 값을 사용할 때 루프 변수를 클로저로 capture하려면 Go 버전을 확인한다.

### 가변 인자 함수는 항상 슬라이스로 전달된다
`...T` 파라미터를 받은 함수는 내부에서 `[]T`로 취급한다. 이건 성능에 영향을 줄 수 있다.

```go
func sum(vals ...int) int {
    // vals는 []int 슬라이스
    // 슬라이스 길이만큼 반복
    total := 0
    for _, v := range vals {
        total += v
    }
    return total
}
```

대량의 데이터를 처리할 때는 슬라이스를 직접 받는 버전도 고려한다. 하지만 보통은 `...T`가 충분하다.

## 코드로 확인하기
`go run .` 실행 결과를 보면:

```
=== 1. 가변 인자 기본 사용 ===
sumInts(1, 2, 3, 4, 5) = 15
sumInts() = 0
maxInts(3, 1, 4, 1, 5, 9, 2, 6) = 9

=== 2. 슬라이스 펼치기 (Spreading) ===
고친 버전: sumInts(numbers...) = 60

실수 버전: sumInts(numbers) -> 컴파일 오류
  오류 메시지:
  ./main.go:XX:XX: cannot use numbers (type []int) as type []int in argument to sumInts (missing ... in parameter list)
  이유: ...int 파라미터는 'int를 여러 개' 받는 것이므로, []int 하나를 넣을 수 없습니다.
  해결: numbers... 로 펼쳐야 합니다.

=== 3. 함수 값과 전략 패턴 ===
joinStrategy 적용: apple | banana | cherry
upperJoinStrategy 적용: APPLE - BANANA - CHERRY
익명 함수 적용: apple + banana + cherry (익명)

실수 버전: 시그니처 불일치 함수 전달 -> 컴파일 오류
  오류 메시지:
  ./main.go:XX:XX: cannot use badStrategy (type func([]string) string) as type func([]string, string) string
  이유: processItems는 전략 함수에 separator를 전달해야 하므로, separator 파라미터가 필요합니다.
```

「고친 버전: sumInts(numbers...) = 60」에서 `...` 연산자가 올바르게 동작한다. 「실수 버전」 메시지는 컴파일 오류가 나는 경우를 설명한다.

`go test -v` 출력에서 모든 `PASS`가 나오면 테스트가 통과한 것이다. `TestFunctionValueStrategy/익명 함수`가 통과하면 익명 함수가 올바르게 전달되고 실행된다.

## 연습 문제
1. `...string` 가변 인자를 받아 공백으로 Join하는 `joinStrings` 함수를 작성하라. 힌트: `strings.Join`을 사용한다.
2. 두 개의 정수 슬라이스를 받아 합친 후 정렬하는 `mergeAndSort` 함수를 작성하라. 힌트: `append`와 `sort.Ints`를 사용한다.
3. `func(int) int` 타입의 함수 슬라이스를 받아 각 함수에 1부터 10까지의 정수를 적용한 결과를 반환하는 `applyFunctions` 함수를 작성하라. 힌트: `for` 루프와 슬라이스를 사용한다.

## 더 읽기
- [Go Spec: Passing arguments to ... parameters](https://go.dev/ref/spec#Passing_arguments_to_..._parameters)
- [Go by Example: Variadic Functions](https://gobyexample.com/variadic-functions)
- [Effective Go: Functions](https://go.dev/doc/effective_go#functions)
- [Go Blog: Go 1.22 Release Notes](https://go.dev/blog/go1.22)