> ⚠️ **자동 검증 미통과 — `readme` 단계.** 본문 설명은 그대로 유효하지만, README 의 코드 조각 일부가 그대로는 컴파일되지 않습니다(main.go·테스트는 통과).
>
> `README 코드 조각 #4 (절: 실전에서는 이렇게 쓴다) — 컴파일 오류:`  
> `./main.go:10:6: addTag redeclared in this block`  
> `./main.go:5:6: other declaration of addTag`  

## 한 줄 요약

Go는 모든 인자를 값으로 복사하므로, 원본을 바꾸려면 **주소(&)와 포인터(\*)** 를 사용해야 한다.

## 왜 필요한가

다른 언어(Java, JavaScript, Python)에서는 객체나 큰 값을 함수에 넘길 때 "참조"가 전달됩니다. Go에서는 **모든 것이 값으로 복사**됩니다. 이는 일관성(convention) 때문입니다 — "변수는 항상 값을 가진다"는 원칙을 유지해서, 메모리 모델이 복잡해지지 않도록 했습니다.

Go가 다르게 한 점은, 포인터 연산이 제한적이라는 것입니다. C/C++처럼 포인터 산술(`p + 1`), 포인터 비교, 포인터 아리틱은 없습니다. 대신 **역참조(`*p`)와 주소 연산자(`&`)만** 허용합니다. 이는 메모리 안전성을 높이기 위한 설계 선택입니다.

초심자가 가장 혼동하는 점은, 슬라이스와 구조체를 함수에 넘겼을 때 "변경이 반영되기도 하고 안 되기도 한다"는 것입니다. 슬라이스는 "헤더(포인터+len+cap)"가 복사되지만, **underlying 배열**은 공유됩니다. 구조체는 필드가 값이면 복사되지만, 필드 자체가 포인터면 그 포인터만 복사됩니다.

## 기본 사용

```go
// 주소(&)로 포인터 생성, *(역참조)로 값 읽기/쓰기
var x int = 10
p := &x      // p는 x의 주소를 가리키는 *int 타입
fmt.Println(*p) // 10 — *p는 x와 같은 값

*p = 20      // x도 20이 됨
```

포인터를 함수 인자로 받을 때는 `*T` 타입을 사용합니다.

```go
func modify(p *int) { *p = 99 }
```

nil 포인터는 아직 값을 가리키지 않는 상태로, 역참조하면 panic이 발생합니다.

```go
var p *int // nil
fmt.Println(*p) // panic: runtime error: invalid memory address
```

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행 — 출력으로 값 복사 vs 포인터 전달 차이 확인
go test -v ./...  # 테스트
```

## 실전에서는 이렇게 쓴다

**1. 슬라이스를 반환하는 함수에서 포인터 대신 반환값 사용**

```go
// bad: 원본을 바꾸지 않음
func addTag(tags []string, tag string) {
    tags = append(tags, tag) // 원본에 영향 없음
}

// good: 반환값을 받아야 함
func addTag(tags []string, tag string) []string {
    return append(tags, tag)
}
```

**2. 구조체 포인터로 메모리 절약**

```go
type LargeStruct struct {
    Data [1000]int
}

func processByValue(s LargeStruct)  { /* 복사됨 — 비용 큼 */ }
func processByPointer(s *LargeStruct) { /* 포인터만 복사 — 비용 작음 */ }
```

실제로 Go 표준 라이브러리(`encoding/json`, `sync.Mutex`)도 포인터를 많이 사용합니다.

## 흔한 실수

### 1. 슬라이스 append 후 원본이 안 바뀌는 것

**무엇을 했는지 (틀린 코드)**

```go
// 틀린 코드
func badAppend(src []int) {
    src = append(src, 99) // src는 복사본, 원본은 untouched
}

s := []int{1, 2, 3}
badAppend(s)
fmt.Println(s) // [1 2 3] — 99가 추가되지 않음!
```

**어떤 일이 벌어지는지**

출력: `[1 2 3]` — 99가 추가되지 않습니다.

**왜 그런지**

슬라이스는 "헤더(포인터, 길이, 용량)"가 값으로 복사되어 전달됩니다. `append`는 새로운 헤더를 반환하지만, `badAppend`는 그 반환값을 무시합니다. 원본 슬라이스 헤더는 변하지 않습니다.

**고친 코드**

```go
func goodAppend(src []int) []int {
    return append(src, 99)
}

s := []int{1, 2, 3}
s = goodAppend(s)
fmt.Println(s) // [1 2 3 99]
```

### 2. nil 포인터 역참조 panic

**무엇을 했는지 (틀린 코드)**

```go
// 틀린 코드
var p *int
fmt.Println(*p) // panic!
```

**어떤 일이 벌어지는지**

panic 메시지: `runtime error: invalid memory address or nil pointer dereference`

프로그램이 강제 종료됩니다.

**왜 그런지**

`var p *int`는 nil 포인터입니다. nil은 유효한 메모리 주소를 가리키지 않으므로, 역참조하면 OS가 허용하지 않는 메모리에 접근하려는 시도가 됩니다. Go는 이를 보호하기 위해 panic을 일으킵니다.

**고친 코드**

```go
var p *int
if p != nil {
    fmt.Println(*p)
} else {
    fmt.Println("p is nil")
}
```

또는 값을 할당해서 포인터를 유효하게 만듭니다.

```go
x := 10
p := &x
fmt.Println(*p) // 10
```

### 3. 슬라이스 요소 수정은 반영되는데 append는 안 되는 것

**무엇을 했는지 (틀린 코드)**

```go
// 틀린 코드
func setFirst(src []int, v int) {
    src[0] = v // 반영됨
}

func add(src []int) int {
    return src[0] // 읽기는 가능
}

s := []int{1, 2, 3}
setFirst(s, 99)
fmt.Println(s) // [99 2 3] — 반영됨!
```

**어떤 일이 벌어지는지**

출력: `[99 2 3]` — 요소 수정은 원본에 반영됩니다.

**왜 그런지**

슬라이스 헤더(포인터, len, cap)는 복사되어 전달되지만, **포인터가 가리키는 underlying 배열**은 공유됩니다. `src[0] = v`는 underlying 배열의 첫 요소를 수정하므로, 원본 슬라이스가 보는 배열도 바뀝니다.

하지만 `append`는 슬라이스 헤더 자체(포인터, len, cap)를 바꿀 수 있으므로, 헤더 복사본만 바뀌고 원본 헤더는 untouched합니다.

**고친 코드**

```go
// 헤더를 바꾸려면 반환값을 받아야 함
func add(src []int) []int {
    return append(src, 99)
}

// 요소만 바꾸면 반환값 필요 없음
func setFirst(src []int, v int) {
    src[0] = v
}
```

### 4. 구조체 값 복사로 인한 예상치 못한 동작

**무엇을 했는지 (틀린 코드)**

```go
// 틀린 코드
type Config struct {
    Host string
    Port int
}

func modifyConfig(cfg Config) {
    cfg.Port = 8080 // 복사본만 바뀜
}

cfg := Config{Host: "localhost", Port: 3000}
modifyConfig(cfg)
fmt.Println(cfg.Port) // 3000 — 바뀐 게 없음!
```

**어떤 일이 벌어지는지**

출력: `3000` — 구조체 필드가 변경되지 않습니다.

**왜 그런지**

구조체 인자는 값으로 복사됩니다. `modifyConfig` 안에서 `cfg.Port = 8080`은 복사본의 필드만 바꿉니다. 원본 `cfg`는 변하지 않습니다.

**고친 코드**

```go
func modifyConfig(cfg *Config) {
    cfg.Port = 8080 // 원본을 직접 수정
}

cfg := Config{Host: "localhost", Port: 3000}
modifyConfig(&cfg)
fmt.Println(cfg.Port) // 8080
```

## 주의할 점

**1. 포인터 산술이 없다**

C/C++에서는 `p + 1`로 다음 요소로 이동할 수 있지만, Go에서는 불가능합니다. 슬라이스 인덱싱(`s[i]`)이나 포인터 배열(`*[]T`)을 사용해야 합니다.

**2. `new` vs `make`**

`new(T)`는 `T` 타입의 zero value를 가진 포인터를 반환합니다. `make(T, args)`는 슬라이스, 채널, map을 생성합니다. 포인터가 필요하면 `new`를, 컬렉션이 필요하면 `make`를 사용합니다.

```go
p := new(int) // *int, 값은 0
s := make([]int, 3) // []int, 길이 3
```

**3. Go 1.22+ 루프 변수**

Go 1.22부터 `for` 루프 변수는 매 iteration마다 새로 생성됩니다. 이전 버전에서는 포인터를 저장할 때 같은 주소를 가리킬 수 있었지만, 이제 각 iteration마다 고유한 변수가 생성됩니다.

## 코드로 확인하기

main.go를 실행하면:

```
=== 1. 값 복사 vs 포인터 전달 (swap) ===
swapVal 전: a=10, b=20
swapVal 후: a=10, b=20 (바뀌지 않음)  ← 값 복사로 원본 untouched
swapPtr 전: c=10, d=20
swapPtr 후: c=20, d=10 (바뀜!)        ← 포인터로 원본 직접 수정

=== 2. nil 포인터 역참조 panic ===
nilDeref panicked: runtime error: invalid memory address or nil pointer dereference

=== 3. 슬라이스: append 시 헤더 교체 vs 요소 수정 ===
badAppend 전: s1=[1 2 3] (len=3, cap=3)
badAppend 후: s1=[1 2 3] (len=3, cap=3) — unchanged!  ← 반환값 무시
goodAppend 후: s2=[1 2 3 99] (len=4, cap=4) — appended!  ← 반환값 사용

=== 4. 구조체: 값 복사 vs 포인터 전달 ===
copyPerson 후: p1.Age=30 (복사본만 바뀜)
ptrPerson 후: p2.Age=99 (원본이 바뀜!)
```

테스트는 `TestSwapValueVsPointer`, `TestNilDerefPanics`, `TestSliceAppend`, `TestSliceElementMutation`, `TestStructValueVsPointer`가 각각 값 복사 vs 포인터 전달의 차이를 검증합니다.

## 연습 문제

1. 슬라이스의 `copy` 함수를 사용해서 슬라이스 복사본을 만들고, 복사본을 수정해도 원본이 변하지 않는 것을 보여보세요. (힌트: `dst := make([]int, len(src)); copy(dst, src)`)

2. nil 포인터를 안전하게 처리하는 함수 `safeDeref(p *int, default int) int`를 작성하세요. nil이면 default를 반환하고, 아니면 *p를 반환합니다. (힌트: `if p != nil` 체크)

3. 구조체 슬라이스를 포인터로 처리했을 때와 값으로 처리했을 때 메모리 효율 차이를 비교해보세요. (힌트: `sync.Pool`이나 `runtime.ReadMemStats`로 메모리 사용량 측정)

## 더 읽기

- [Go Tour: Pointers](https://go.dev/tour/moretypes/1) — 공식 튜토리얼
- [Effective Go: Pointers vs Values](https://go.dev/doc/effective_go#pointers_vs_values) — 실제 코드 스타일 가이드
- [Go Blog: Go Slices: usage and internals](https://go.dev/blog/slices-intro) — 슬라이스 내부 구조 이해