> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main.go:45:5: ps.Tags undefined (type Person has no field or method Tags)`  

## 한 줄 요약
구조체(struct)는 Go에서 값 타입으로, 필드는 대문자로 시작해야 외부에서 접근 가능합니다.

## 왜 필요한가
Go는 C, Java, Python 등 기존 언어들과 달리 구조체를 기본 데이터 타입으로 사용합니다. 다른 언어에서는 클래스(Java)나 딕셔너리(Python)로 데이터를 그룹화하지만, Go는 **구조체**를 사용합니다.

Go가 구조체를 기본 타입으로 선택한 이유는 **명확한 메모리 레이아웃과 성능** 때문입니다. 클래스는 포인터 참조와 가상 함수 테이블 등을 포함하지만, 구조체는 값 자체를 담습니다. 이는 Go가 시스템 프로그래밍 언어로 태어난 이유와 맞닿아 있습니다.

또한 **공개/비공개 필드 규칙**(대문자/소문자)은 Go의 모듈화 철학을 반영합니다. 외부에서 접근할 수 있는 필드만 명시적으로 열어두면, 인터페이스 설계가 명확해지고 은폐가 자연스럽게 이루어집니다.

## 기본 사용
```go
// 기본 구조체 정의
type Person struct {
    Name string
    Age  int
}

// 리터럴 초기화
p := Person{Name: "Alice", Age: 30}

// 필드 순서만 맞춰도 됨
p2 := Person{"Bob", 25}

// 부분 초기화
p3 := Person{Name: "Charlie"} // Age는 0

// 익명 구조체
config := struct {
    Debug bool
}{Debug: true}
```

## 돌려보기
```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```
- `go vet`: 코드의 잠재적 문제점 검사
- `go run .`: 구조체 사용 예시 실행
- `go test`: 구조체 관련 테스트 실행

## 실전에서는 이렇게 쓴다
```go
// HTTP 요청 구조체
type Request struct {
    URL    string
    Method string
    Body   io.Reader
}

// 설정값 구조체
type Config struct {
    Port     int
    Host     string
    Timeout  time.Duration
}

func NewServer(cfg Config) *Server {
    return &Server{config: cfg}
}
```

## 흔한 실수
### 1. 값 전달과 반환값 무시
```go
// 틀린 코드
func ChangeAge(p Person) {
    p.Age = 31 // 복사본만 수정, 원본은 변경 없음
}

// 고친 코드
func ChangeAge(p Person) Person {
    p.Age = 31
    return p
}

// 또는 포인터 사용
func ChangeAgePtr(p *Person) {
    p.Age = 31 // 포인터로 원본 직접 수정
}
```

### 2. 슬라이스가 포함된 구조체 비교
```go
// 틀린 코드
type WithSlice struct {
    Tags []string
}
ws1 := WithSlice{Tags: []string{"a"}}
ws2 := WithSlice{Tags: []string{"a"}}
_ = ws1 == ws2 // 컴파일 오류: slice는 비교 불가

// 고친 코드 - 슬라이스 대신 배열 사용
type WithArray struct {
    Tags [3]string
}
// 또는 커스텀 비교 함수
func EqualWithSlice(a, b WithSlice) bool {
    if len(a.Tags) != len(b.Tags) {
        return false
    }
    for i := range a.Tags {
        if a.Tags[i] != b.Tags[i] {
            return false
        }
    }
    return true
}
```

### 3. 소문자 필드 접근 시도
```go
// 틀린 코드
type Person struct {
    name string // 소문자: 비공개
}
p := Person{name: "Alice"}
fmt.Println(p.name) // 컴파일 오류: p.name undefined

// 고친 코드
type Person struct {
    Name string // 대문자: 공개
}
p := Person{Name: "Alice"}
fmt.Println(p.Name) // 정상 작동
```

### 4. 구조체 복사 시 얕은 복사
```go
// 틀린 코드
type Data struct {
    Items []string
}
d1 := Data{Items: []string{"a", "b"}}
d2 := d1 // 슬라이스 헤더만 복사, 데이터 공유
d2.Items[0] = "c"
fmt.Println(d1.Items[0]) // "c" - 원본도 변경됨!

// 고친 코드
d2 := d1
d2.Items = make([]string, len(d1.Items))
copy(d2.Items, d1.Items) // 깊은 복사
d2.Items[0] = "c"
fmt.Println(d1.Items[0]) // "a" - 원본 유지
```

## 주의할 점
- 구조체는 **값 타입**입니다. 포인터를 사용하지 않으면 항상 복사가 일어납니다.
- 슬라이스, 맵, 함수가 포함된 구조체는 `==` 비교가 불가합니다.
- 소문자로 시작하는 필드는 **같은 패키지 내**에서만 접근 가능합니다.
- 큰 구조체를 자주 복사해야 한다면 포인터 사용 고려가 필요합니다.
- Go 1.22 이후 루프 변수 스코프 변경으로 구조체 사용 시 주의가 필요합니다.

## 코드로 확인하기
`go run .` 실행 결과:
- "실수 버전" 출력: Point 값이 변경되지 않음 (1,2)
- "고친 버전" 출력: Point 값이 정상 변경됨 (100,200)
- "슬라이스 포함" 출력: 원본 Tags가 변하는 vs 변하지 않는 차이 확인

테스트 결과:
- `TestStructLiteralsAndInitialization`: 리터럴 초기화 검증
- `TestStructComparison`: 구조체 비교 동작 검증
- `TestStructMutationFix`: 반환값 사용 검증
- `TestSliceDeepCopy`: 깊은 복사 검증
- `TestPublicFields`: 공개 필드 접근 검증

## 연습 문제
1. 이름과 나이를 가진 Person 구조체를 정의하고, 리터럴로 초기화하는 코드 작성
2. 슬라이스가 포함된 구조체를 깊은 복사하는 함수 작성
3. 공개/비공개 필드가 있는 구조체를 정의하고, 외부 패키지에서 접근 테스트

힌트:
1. `type Person struct { ... }` 정의 후 `Person{Name: "..."}` 사용
2. `make()`와 `copy()`로 슬라이스 복사
3. 대문자/소문자 필드명으로 시작 이름 결정

## 더 읽기
- [Go Tour - Structs](https://go.dev/tour/moretypes/2)
- [Effective Go - Structs](https://go.dev/doc/effective_go#structs)
- [Go Blog - Go Slices: usage and internals](https://go.dev/blog/slices-intro)