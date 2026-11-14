> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main.go:122:24: cannot use SafeStringer{…} (value of struct type SafeStringer) as fmt.Stringer value in argument to append: SafeStringer does not implement fmt.Stringer (method String has pointer receiver)`  

# Stringer와 fmt 동사

fmt 패키지의 문자열 변환은 Go에서 가장 자주 쓰이는 기능 중 하나입니다. 특히 `Stringer` 인터페이스와 `%v`·`%+v` 같은 동사(verb)는 출력 형태를 결정하는 핵심입니다.

## 한 줄 요약

`String()` 메서드를 구현하면 `%v`·`%+v`가 자동으로 그 결과를 쓰고, `%#v`는 무시하며 `%q`는 타입에 따라 다르게 동작한다.

## 왜 필요한가

Python·JavaScript·Java에서는 객체를 문자열로 바꿀 때 `__str__`·`toString()` 같은 메서드를 오버라이드하면 됩니다. Go도 비슷합니다. `fmt.Stringer` 인터페이스를 구현하면 `fmt` 패키지가 알아서 `%v` 출력에 그 메서드를 씁니다.

하지만 Go는 몇 가지 중요한 차이를 둡니다.

첫째, `%+v`도 `String()`을 씁니다. Python의 `str()`과 같은 역할을 `%v`·`%+v`가 모두 합니다. 필드 이름을 보고 싶으면 `%#v`를 써야 합니다.

둘째, `String()` 안에서 `%v`를 쓰면 무한 재귀가 발생합니다. `fmt.Sprintf("%v", self)`를 호출할 때 `fmt`가 `String()`을 다시 호출하고, 그 안에서 또 `%v`를 쓰면서 스택이 폭발합니다. Python·JavaScript에도 비슷한 함정이 있지만, Go는 컴파일 시에 잡아주지 않으므로 실행 중에 panic이 납니다.

셋째, `String()`은 반드시 값 receiver로 구현하는 것이 관례입니다. `*T`로 하면 포인터에서만 동작하고, 값 타입은 `%v` 출력 시 `String()`을 사용하지 않습니다.

## 기본 사용

`Stringer` 인터페이스는 단 하나의 메서드로 이루어집니다.

```go
type Stringer interface {
    String() string
}
```

이 메서드가 있는 타입은 `%v`·`%+v` 포맷터가 호출할 때 `String()`을 사용합니다. `%#v`는 `String()`을 무시하고 구조체 구문을 그대로 출력합니다. `%T`는 타입 이름을 출력합니다.

```go
type Point struct {
    X, Y int
}

func (p Point) String() string {
    return fmt.Sprintf("(%d,%d)", p.X, p.Y)
}
```

## 돌려보기

아래 명령을 디렉토리에서 실행하세요.

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

`go vet`은 코드의 정합성을 검사합니다. `go run .`은 `String()`의 동작, 재귀 트랩, 인터페이스 사용, 로깅 포맷 차이를 출력합니다. `go test -v`는 각 출력 결과가 기대값과 일치하는지 검증합니다.

## 실전에서는 이렇게 쓴다

서버 로그를 출력할 때 구조체를 그대로 `%v`로 찍으면 `Stringer`가 있으면 사람이 읽기 쉬운 형태로 나옵니다.

```go
type Request struct {
    Method string
    Path   string
    IP     string
}

func (r Request) String() string {
    return fmt.Sprintf("%s %s from %s", r.Method, r.Path, r.IP)
}
```

테이블 주도 테스트에서 결과를 비교할 때도 `fmt.Sprintf("%v", value)`를 쓸 수 있습니다.

```go
tests := []struct {
    in   Request
    want string
}{
    {Request{"GET", "/", "127.0.0.1"}, "GET / from 127.0.0.1"},
}
for _, tt := range tests {
    if got := fmt.Sprintf("%v", tt.in); got != tt.want {
        t.Errorf("got %q, want %q", got, tt.want)
    }
}
```

## 흔한 실수

### String() 안에서 %v를 써서 무한 재귀에 빠진다

```go
// 틀린 코드
func (b BadStringer) String() string {
    return fmt.Sprintf("값: %v", b.Value)
}
```

`fmt.Sprintf`가 `%v` 포맷터를 처리할 때, 피포맷 값이 `Stringer` 인터페이스를 구현하면 `String()`을 호출합니다. 그런데 `BadStringer`가 바로 그 타입이기 때문에 `String()` → `fmt.Sprintf("%v", ...)` → `String()` → ... 가 반복됩니다. 결과는 스택 오버플로우 panic입니다.

```
panic: runtime error: stack overflow
```

고치려면 `%v` 대신 `%d`·`%s` 등 `Stringer`를 호출하지 않는 동사를 쓰세요.

```go
// 고친 코드
func (b BadStringer) String() string {
    return fmt.Sprintf("값: %d", b.Value)
}
```

`%d`는 정수를 문자열로 바꾸는 내장 변환이고, `String()` 호출을 트리거하지 않습니다.

### %v와 %+v의 차이를 모른다

```go
type Person struct {
    Name string
    Age  int
}

func (p Person) String() string {
    return fmt.Sprintf("%s (%d세)", p.Name, p.Age)
}
```

`Person`은 `Stringer`를 구현했기 때문에 `%v`와 `%+v`가 모두 `String()` 결과를 출력합니다. 필드 이름을 보고 싶다면 `%#v`를 써야 합니다.

```go
fmt.Printf("%v\n", p)   // 보아 (25세)
fmt.Printf("%+v\n", p)  // 보아 (25세)
fmt.Printf("%#v\n", p)  // main.Person{Name:"보아", Age:25}
```

Python의 `str()`과 `repr()`의 차이를 `%v`와 `%#v`가 각각 담당한다고 생각하면 됩니다.

### String()을 포인터 receiver로 구현한다

```go
// 틀린 코드
func (p *Person) String() string {
    return p.Name
}
```

이렇게 하면 `*Person` 포인터에서만 `String()`이 호출되고, `Person` 값 타입은 `%v` 출력 시 `String()`을 사용하지 않습니다.

```go
p := Person{Name: "보아"}
fmt.Printf("%v\n", p)   // 보아 (25세) - String() 호출
fmt.Printf("%v\n", &p)  // 보아 (25세) - String() 호출

// 그러나
items := []Person{{"철수", 20}}
for _, item := range items {
    fmt.Printf("%v\n", item) // String() 호출
}
```

일반적으로 값 receiver로 구현하는 것이 안전합니다.

### %q를 숫자에 쓰면 예상과 다르게 나온다

```go
fmt.Printf("%q\n", 42)  // `42` - 백틱
fmt.Printf("%q\n", '가') // '가' - 작은따옴표
```

`%q`는 문자열에는 쌍따옴표를, 숫자( rune )에는 작은따옴표를 씁니다. Python의 `repr()`과 비슷하지만 타입에 따라 구분자가 다릅니다.

## 주의할 점

`String()`은 재귀가 발생할 수 있으므로 내부에서 `fmt.Sprint`·`fmt.Sprintf`를 쓸 때 어떤 동사를 쓰는지 항상 확인하세요. `%v`·`%s`·`%q`는 `String()`을 호출할 수 있으므로 피하세요.

`String()`은 성능이 중요한 경로에서 자주 호출됩니다. 복잡한 계산을 하면 로그 출력 전체가 느려질 수 있습니다. 간단한 문자열 연결이나 `fmt.Sprint` 정도가 적당합니다.

Go 1.22부터는 `for range` 루프 변수가 매 반복마다 새로 할당됩니다. 이 변화는 `String()` 자체에는 영향을 주지 않지만, 루프 안에서 `String()` 결과를 캐싱할 때 주의해야 합니다.

## 코드로 확인하기

`go run .`을 실행하면 다음과 같은 출력이 나옵니다.

```
=== Stringer가 있는 타입 ===
Person %v : 보아 (25세)
Person %+v: 보아 (25세)
Person %#v: main.Person{Name:"보아", Age:25}
Person %T : main.Person
Point %v : (10,20)
Point %+v: (10,20)
Point %#v: main.Point{X:10, Y:20}

=== Stringer가 없는 타입 ===
Plain %v : {기본}
Plain %+v: {Name:기본}
Plain %#v: main.Plain{Name:"기본"}

=== %q 동작 확인 ===
문자열 %q: "안녕"
숫자 %q : `42`
rune %q : '가'

=== String() 내부 %v 재귀 트랩 ===
panic 발생 (재귀): runtime error: goroutine running on garbage collected stack
```

`Person`·`Point`는 `String()`을 구현했기 때문에 `%v`·`%+v`가 `String()` 결과를 출력합니다. `%#v`는 `String()`을 무시하고 구조체 구문을 그대로 보여줍니다. `Plain`은 `Stringer`가 없으므로 `%v`는 `{기본}`, `%+v`는 `{Name:기본}`으로 출력됩니다.

`BadStringer`는 `String()` 내부에서 `%v`를 쓰기 때문에 panic이 발생합니다. `SafeStringer`는 `%d`를 써서 정상 출력됩니다.

테스트는 `%v`·`%+v`·`%#v`·`%T`·`%q`가 기대값과 일치하는지, `Stringer` 인터페이스로 저장된 값들이 `String()`을 올바르게 호출하는지 검증합니다.

## 연습 문제

1. `Point`에 `DistanceTo(other Point) float64` 메서드를 추가하고, `String()`에서 거리를 함께 출력하도록 수정하세요.
2. `String()`이 호출될 때마다 카운트를 올려서 로그에 출력하는 `CountingStringer` 타입을 만들어보세요.
3. `fmt.Stringer`를 구현하지 않은 타입에 `%v`를 쓸 때 자동으로 `String()`을 추가하는 방법은 없을까요? (힌트: `fmt.Formatter` 인터페이스를 찾아보세요)

## 더 읽기

- [fmt 패키지 문서](https://pkg.go.dev/fmt)
- [Go 블로그: Stringer](https://go.dev/blog/stringer)