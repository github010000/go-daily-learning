## 한 줄 요약
Go에서 메서드의 리시버는 **값**이어야 할 때와 **포인터**여야 할 때가 명확히 다르다. 이를 혼동하면 "왜 내 객체가 변하지 않을까?"라는 버그를 마주하거나, 인터페이스 구현을 놓칠 수 있다.

## 왜 필요한가
다른 언어(Java, Python, JavaScript)에서는 메서드가 항상 객체 참조(포인터)를 통해 동작한다. 따라서 메서드 내부에서 필드를 수정하면 원본이 항상 바뀐다. Go는 이 개념을 두 가지로 나눈다.

첫째, **값 리시버**는 메서드 호출 시 객체 전체를 복사한다. 복사 비용이 작은 기본 타입(struct)일 때 이득이다. 특히 "읽기 전용" 연산(예: 면적 계산)에는 값 리시버가 적합하다.
둘째, **포인터 리시버**는 객체의 주소를 받아온다. 필드를 수정할 때(예: 크기 키우기) 필수다. 복사를 피하고 원본을 직접 건드린다.

Go가 이를 구분한 이유는 **명확성** 때문이다. 메서드 시그니처만 봐도 "이 메서드는 객체를 변경하는가?"를 알 수 있어야 한다. Java의 `void grow()`와 `float64 area()`를 구분하듯, Go는 리시버 타입으로 그 의도를 코드에 담는다.

## 기본 사용
메서드는 구조체(또는 임의 타입)에 붙여쓴다.

```go
type Rectangle struct {
    Width, Height float64
}

// 값 리시버: 복사본으로 작업, 원본 불변. 읽기 전용 연산에 적합.
func (r Rectangle) Area() float64 {
    return r.Width * r.Height
}

// 포인터 리시버: 원본 직접 접근. 쓰기 연산에 필수.
func (r *Rectangle) Grow(factor float64) {
    r.Width *= factor
    r.Height *= factor
}
```

호출 방식:
- `r.Area()` — 값 리시버, 값 타입 `r`으로 호출.
- `r.Grow(2.0)` — 포인터 리시버, `r`이 값이라도 `&r`을 자동으로 감싸서 호출된다(편의 기능).
- `rect.Grow(2.0)` — `rect`가 `*Rectangle` 포인터이면 포인터 리시버를 직접 호출.

**핵심 규칙:** 메서드가 리시버를 **수정**하면 `*T`, **수정하지** 않으면 `T`를 쓰라.

## 돌려보기
다음 명령으로 코드를 직접 실행·검증하라.

```bash
go vet ./...      # 정적 검사: 변수 미사용, import 문제 등 확인
go run .          # 시연 실행: 값/포인터 리시버 동작 차이 출력
go test -v ./...  # 테스트: 모든 단정 통과 확인
```

## 실전에서는 이렇게 쓴다
실제 Go 프로젝트에서는 이 원칙이 일관되게 적용된다.

**1. 상태 변경 없는 조회 연산**
`time.Time`, `url.URL` 같은 표준 라이브러리 타입은 값 리시버 메서드가 대부분이다.
```go
t := time.Now()
fmt.Println(t.Format("2006-01-02")) // 값 리시버, 복사 비용 적음
```

**2. 상태 변경 연산**
웹 서버 라우터, 데이터베이스 연결 풀 등은 포인터 리시버를 쓴다.
```go
type Router struct {
    routes map[string]Handler
}
func (r *Router) Add(method, path string, h Handler) {
    r.routes[method+path] = h // 원본 수정
}
```

## 흔한 실수

### 1. 값 리시버로 수정 메서드를 정의한 경우
**틀린 코드**
```go
func (r Rectangle) Grow(factor float64) {
    r.Width *= factor // r은 복사본! 원본 rect는 변하지 않음
}
```
**어떤 일이 벌어지는지**
호출 후 `rect.Width`가 기대한 대로 두 배가 되지 않는다. "왜 안 바뀌지?"라고 당황하게 된다.

**왜 그런지**
리시버가 `Rectangle`(값)이므로, 메서드 내부의 `r`은 `rect`의 **복사본**이다. 이 복사본을 수정해도 원본 `rect`에는 영향을 주지 못한다.

**고친 코드**
```go
func (r *Rectangle) Grow(factor float64) {
    r.Width *= factor // 포인터 리시버: 원본 직접 수정
}
```

### 2. 포인터 리시버 메서드를 값 타입으로 호출하려는 함수 파라미터 설계
**틀린 코드**
```go
func process(r Rectangle) {
    r.Grow(2.0) // 에러: cannot use r (variable of type Rectangle) as type *Rectangle in argument to Grow
}
```
**어떤 일이 벌어지는지**
컴파일 에러: `cannot use r (variable of type Rectangle) as type *Rectangle in argument to Grow`. 값 타입 `Rectangle`은 포인터 리시버 메서드 `Grow()`를 호출할 수 없다.

**왜 그런지**
Go의 메서드 집합(method set) 규칙 때문이다. 값 타입 `T`의 메서드 집합에는 값 리시버 메서드만 포함된다. 포인터 리시버 메서드는 `*T`의 메서드 집합에만 포함된다. 따라서 `Rectangle` 변수는 `Grow()`를 호출할 수 없다.

**고친 코드**
```go
func process(r *Rectangle) {
    r.Grow(2.0) // 포인터 파라미터이므로 호출 가능
}
// 또는 호출 측에서 주소를 전달:
// process(&rect)
```

### 3. nil 포인터 리시버로 panic
**틀린 코드**
```go
var c *Circle // nil
area := c.Area() // panic: runtime error: invalid memory address or nil pointer dereference
```
**어떤 일이 벌어지는지**
`panic: runtime error: invalid memory address or nil pointer dereference`가 발생해 프로그램이 강제 종료된다.

**왜 그런지**
`c`가 nil이므로 `c.Radius`에 접근하려 할 때 메모리 접근 오류가 발생한다. 포인터 리시버 메서드는 내부에서 리시버 필드에 접근할 수 있다.

**고친 코드**
```go
func (c *Circle) SafeArea() float64 {
    if c == nil {
        return 0 // nil일 때 기본값 반환
    }
    return c.Area()
}
```

## 주의할 점
- **복사 비용:** 값 리시버는 struct 전체를 복사한다. struct가 크다면(예: 1MB 데이터 포함) 성능 문제가 될 수 있다. 이런 경우 포인터 리시버를 고려하라.
- **메서드 집합과 인터페이스:** 인터페이스를 구현하려면 메서드 집합이 일치해야 한다. `Area() float64`(값 리시버)만 구현해도 `Rectangle`과 `*Rectangle` 모두 그 인터페이스를 만족한다. 그러나 `Grow(*Rectangle)` 메서드가 있으면 `Rectangle`(값)은 그 인터페이스를 만족하지 않는다.
- **일관성:** 같은 구조체의 메서드 중 일부는 값 리시버, 일부는 포인터 리시버로 mixed 쓰기도 가능하다. 하지만 **상태를 수정하는 메서드가 하나라도 있다면**, 전체 구조체에 대해 포인터 리시버를 사용하는 것이 일관성 있는 코딩 스타일로 인정받는다.

## 코드로 확인하기
`go run .` 실행 결과:
- `=== 1. 값 리시버 vs 포인터 리시버 기본 동작 ===` 섹션에서 `Grow 후 사각형: 20.00 x 10.00`이 출력된다. 이는 `rect.Grow(2.0)`이 포인터 리시버이므로 원본 `rect`가 수정됐음을 보여준다.
- `--- 실수 버전: 값 리시버 GrowByValue() ---` 섹션에서 `GrowByValue 후 사각형: 10.00 x 5.00 (원본은 변하지 않음!)`이 출력된다. 값 리시버 메서드는 복사본만 수정했기 때문이다.
- `=== 4. nil 포인터 리시버 안전 처리 ===` 섹션에서 `nil 포인터로 SafeArea() 호출: 0`이 출력된다. nil 체크로 panic을 피한 결과다.

`main_test.go`는 다음을 검증한다:
- `TestValueReceiverDoesNotModifyOriginal`: `GrowByValue()` 호출 후 `Width`, `Height`가 원래 값(10, 5) 유지.
- `TestPointerReceiverModifiesOriginal`: `Grow()` 호출 후 `Width`, `Height`가 2배(20, 10)로 변함.
- `TestNilReceiverSafety`: nil `*Circle`이 `SafeArea()`를 호출해도 0 반환, panic 없음.
- `TestMethodSetOnInterface`: 값 타입 `Rectangle`, `Circle`과 포인터 타입 `*Rectangle`, `*Circle` 모두 interface를 만족함.

## 연습 문제
1. `Square` 구조체에 `Side float64` 필드를 추가하고, 면적을 반환하는 값 리시버 메서드 `Area()`와, 한 변의 길이를 두 배로 키우는 포인터 리시버 메서드 `DoubleSide()`를 작성하라.
2. `Counter` 구조체에 카운트를 증가시키는 포인터 리시버 메서드 `Inc()`와, 현재 값을 반환하는 값 리시버 메서드 `Value()`를 작성하라. `main()`에서 둘 다 호출하며 동작을 확인하라.
3. `Student` 구조체에 `Name string`, `Score int` 필드가 있다. 성적 평균을 계산하는 함수 `AvgScore(students []Student) float64`를 작성하고, 성적 평균이 90점 이상인 학생만 필터링하는 메서드 `HonorList() []Student`를 값 리시버로 작성하라. (단, `HonorList()`는 새 슬라이스를 반환해야 함)

## 더 읽기
- [Go Tour: Methods](https://go.dev/tour/methods/4) — 공식 튜토리얼의 메서드 챕터
- [Effective Go: Methods](https://go.dev/doc/effective_go#methods) — Go 커뮤니티가 권장하는 메서드 작성 관례
- [Go Blog: Go Slices: usage and internals](https://go.dev/blog/slices-intro) — 슬라이스와 연관된 값/포인터 복사 개념 이해에 도움
- [Go Spec: Method sets](https://go.dev/ref/spec#Method_sets) — 메서드 집합에 대한 공식 명세