> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main_test.go:99:2: declared and not used: originalNamePtr`  

## 한 줄 요약
Go의 임베딩(embedding)은 '상속'이 아니라 '위임(delegation)'입니다. 필드와 메서드가 승격되고, 이름 충돌 규칙과 인터페이스 임베딩으로 유연한 타입 조합이 가능합니다.

## 왜 필요한가
다른 언어(Python, JavaScript, Java)에서는 클래스 간 상속(inheritance)을 통해 코드를 재사용합니다. 상속은 "자식은 부모의 모든 것을 물려받는다"는 개념이지만, Go는 이를 "자식은 부모의 필드와 메서드를 위임받는다"로 해석합니다. 즉, 임베딩은 타입을 확장하는 방식이지, 타입 계층을 만드는 방식이 아닙니다.

Go가 임베딩을 다르게 설계한 이유는 복잡도를 줄이기 위함입니다. 상속은 부모-자식 관계가 깊어질수록 변경 영향이 폭발적으로 커집니다. 반면 임베딩은 컴포저블(composable)합니다. 필요한 기능을 가진 타입을 조합해서 새로운 타입을 만들 뿐, 부모의 내부 구현을 직접 건드리는 일이 없습니다. 또한 인터페이스를 임베딩하면 해당 메서드 집합을 자동으로 충족시켜, 다형성을 자연스럽게 구현할 수 있습니다.

## 기본 사용
임베딩은 구조체 내부에 다른 타입을 명시적으로 선언하지 않고 필드 없이 넣는 방식으로 합니다.

```go
type Shape struct {
    Name string
    X, Y int
}

func (s Shape) Info() string {
    return fmt.Sprintf("Shape[%s] at (%d,%d)", s.Name, s.X, s.Y)
}

type Circle struct {
    Shape   // 임베딩
    Radius int
}
```

이때 `Circle` 인스턴스에서 `c.Name`, `c.Info()`처럼 `Shape`의 멤버를 직접 쓸 수 있습니다. 이를 '승격(promotion)'이라고 합니다.

```go
c := Circle{Shape: Shape{Name: "원", X: 10, Y: 20}, Radius: 5}
fmt.Println(c.Name) // "원" - Shape의 Name 필드가 승격됨
fmt.Println(c.Info()) // "Shape[원] at (10,20)" - Shape의 Info() 메서드가 승격됨
```

인터페이스 임베딩은 여러 인터페이스를 합칠 때 유용합니다.

```go
type Describable interface {
    Description() string
}

type PrintableCircle struct {
    Shape
}

func (pc PrintableCircle) Description() string {
    return fmt.Sprintf("%s의 면적은 %.2f", pc.Name, pc.Area())
}
```

`PrintableCircle`은 `Describable` 인터페이스를 자동으로 충족합니다. `Shape`에는 `Area()`가 없지만, `PrintableCircle`에 추가했기 때문에 문제가 없습니다.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

`go vet`은 컴파일 오류가 없는지, `go run`은 출력 결과를, `go test`는 단위 테스트 통과 여부를 확인합니다.

## 실전에서는 이렇게 쓴다
1. **HTTP 서버 라우터 조합**: `net/http` 패키지에서 `ServeMux`는 경로의 핸들러를 등록할 때, 각 핸들러가 `http.Handler` 인터페이스를 충족하는지 임베딩으로 간접 확인합니다.
2. **구조체 확장**: 표준 라이브러리인 `http.Server`는 내부에 `net.Listener`와 `net.Conn`를 임베딩하여, 기본 동작은 유지하면서 특정 메서드만 재정의할 수 있게 합니다.

## 흔한 실수
### 1. 상속처럼 생각해서 필드 변경이 자식에 반영된다고 착각하기
```go
// 틀린 코드
base := Shape{Name: "부모", X: 1}
child := Circle{Shape: base, Radius: 2}

base.Name = "수정된 부모"
fmt.Println(child.Name) // "부모" - 변경 안 됨!
```
**발생 현상**: `child.Name`이 변하지 않아 당황합니다.
**원인**: Go의 임베딩은 참조가 아닌 '값 복사'입니다. `base`를 수정해도 `child`의 내부 `Shape`에는 영향이 없습니다.
**고친 코드**:
```go
basePtr := &Shape{Name: "부모", X: 1}
childPtr := Circle{Shape: *basePtr, Radius: 2}

basePtr.Name = "수정된 부모"
fmt.Println(childPtr.Name) // "수정된 부모" - 포인터라서 반영됨
```
포인터 임베딩을 사용하면 참조 동작이 이루어집니다.

### 2. 이름 충돌 시 명시적 접근을 잊기
```go
// 틀린 코드
type NamedCircle struct {
    Name string // 충돌!
    Circle
}

nc := NamedCircle{Name: "특별한 원", Circle: Circle{Shape: Shape{Name: "기본 원"}, Radius: 3}}
fmt.Println(nc.Name) // "특별한 원" - 최상위 필드가 우선
fmt.Println(nc.Shape.Name) // "기본 원" - 명시적 접근 필요
```
**발생 현상**: `nc.Name`이 예상과 다르게 나옵니다.
**원인**: 이름이 충돌하면 최상위 필드/메서드가 우선합니다.
**고친 코드**:
```go
fmt.Println(nc.Name) // "특별한 원" - NamedCircle의 Name
fmt.Println(nc.Circle.Shape.Name) // "기본 원" - 명시적 경로로 접근
```

### 3. 인터페이스 충족을 위해 메서드 구현을 빼먹기
```go
// 틀린 코드
type PrintableCircle struct {
    Shape
}
// Area() 메서드가 없어서 Describable 인터페이스를 충족하지 못함
```
**발생 현상**: 컴파일 오류가 납니다.
**원인**: `Describable` 인터페이스는 `Description()`을 요구합니다. `Description()` 내부에서 `pc.Area()`를 호출하므로, `PrintableCircle`에 `Area()` 메서드가 필요합니다.
**고친 코드**:
```go
type PrintableCircle struct {
    Shape
}

func (pc PrintableCircle) Area() float64 {
    return 3.14 * float64(1) * float64(1)
}

func (pc PrintableCircle) Description() string {
    return fmt.Sprintf("%s의 면적은 %.2f", pc.Name, pc.Area())
}
```

### 4. 메서드 재정의 시 기본 구현을 무시하기
```go
// 틀린 코드
type DrawCircle struct {
    Circle
}

func (dc DrawCircle) Draw() string {
    // Circle의 Draw()를 호출하지 않고 완전히 새로운 구현만 작성
    return "원 그리기"
}
```
**발생 현상**: `DrawCircle`의 `Draw()`가 `Circle`의 기본 동작과 달라집니다.
**원인**: 메서드 재정의는 완전한 대체입니다.
**고친 코드**:
```go
func (dc DrawCircle) Draw() string {
    return fmt.Sprintf("원 그리기: %s", dc.Circle.Draw()) // 기본 구현 활용
}
```

## 주의할 점
- **값 타입 임베딩은 복사입니다.** 부모 객체를 수정해도 자식에 반영되지 않습니다. 포인터 임베딩을 사용하면 참조 동작이 이루어집니다.
- **이름 충돌은 최상위 필드/메서드가 우선합니다.** 명시적 경로를 사용해야 내부 멤버에 접근할 수 있습니다.
- **인터페이스 충족은 메서드 집합으로 판단됩니다.** 필드나 생성자가 아닌, 메서드 시그니처만 중요합니다.

## 코드로 확인하기
`main.go` 실행 결과를 보면:
- **승격**: `c.Name`, `c.Info()`처럼 `Shape`의 멤버를 `Circle`에서 직접 쓸 수 있습니다.
- **이름 충돌**: `nc.Name`은 `NamedCircle`의 `Name`, `nc.Shape.Name`은 `Shape`의 `Name`입니다.
- **메서드 재정의**: `dc.Draw()`는 `DrawCircle`의 재정의된 동작, `dc.Info()`는 `Circle`의 기본 동작입니다.
- **위임**: `pc.Description()`은 `Describable` 인터페이스를 충족하며, `pc.Area()`를 호출합니다.

`main_test.go`는 승격, 인터페이스 충족, 포인터 임베딩 동작, 메서드 재정의, 면적 계산 등을 검증합니다.

## 연습 문제
1. `Square` 구조체를 만들고 `Shape`를 임베딩하여 `Area()` 메서드를 구현하세요. 힌트: `Width` 필드를 추가하세요.
2. `Rotatable` 인터페이스를 정의하고, `Shape`에 `Rotate()` 메서드를 추가하여 `Rotatable`을 충족하게 하세요. 힌트: `Angle` 필드를 사용하세요.
3. `Polygon` 구조체를 만들고 `Shape`와 `Describable` 인터페이스를 임베딩하여 완전한 다각형 타입을 만드세요. 힌트: `Vertices` 필드를 추가하세요.

## 더 읽기
- [Effective Go: Embedding](https://go.dev/doc/effective_go#embedding) - 공식 문서
- [Go Blog: Go Slices: usage and internals](https://go.dev/blog/slices) - 슬라이스와 임베딩의 관계 이해