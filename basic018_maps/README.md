# 맵(map): 키-값 저장소

## 한 줄 요약
맵은 키로 값을 빠르게 찾거나 추가·삭제하는 참조 타입으로, nil 상태에서는 쓰기만 panic 합니다.

## 왜 필요한가

맵은 해시 테이블 기반의 데이터 구조로, O(1) 평균 시간 복잡도로 값을 찾거나 추가할 수 있습니다. Python의 `dict`, JavaScript의 `Object`/`Map`, Java의 `HashMap`에 해당하는 역할을 합니다.

다만 Go 맵은 다른 언어와 몇 가지 중요한 차이가 있습니다. 첫째, **맵은 항상 참조 타입**으로 선언만 해서는 쓸 수 없습니다 (nil 맵은 panic). 둘째, **순회 순서가 보장되지 않아** 매번 실행할 때마다 순서가 달라집니다. 이는 내부 해시 알고리즘이 키의 순서를 임의로 배치하기 때문입니다.

Go 1.22부터는 맵이 값 비교 연산자(`==`, `!=`)를 지원하게 되어, 맵을 구조체 필드로 사용할 수 있게 되었습니다. 또한 `range` 문이 맵의 key만 반환할 때 값 변수를 선언하지 않아도 컴파일됩니다 (기존에는 unused variable 에러).

## 기본 사용

```go
// 선언 및 초기화 (make 필수)
m := make(map[string]int)

// 쓰기
m["apple"] = 1

// 읽기 (없는 키는 0 반환, panic 없음)
v := m["apple"]

// 존재 확인 (comma-ok)
v, ok := m["grape"] // ok = false 이면 키 없음

// 삭제
delete(m, "apple")

// 크기
n := len(m)
```

## 돌려보기

```bash
go vet ./...      # 정적 검사 — 에러 없이 통과
go run .          # 시연 실행 — nil 맵 panic, comma-ok, 순회 무작위성 등 출력 확인
go test -v ./...  # 테스트 — 모든 함수 동작 검증
```

## 실전에서는 이렇게 쓴다

**1. 카운터/집계 데이터**

```go
wordCount := make(map[string]int)
for _, word := range words {
    wordCount[word]++
}
```

**2. ID로 사용자 조회**

```go
users := make(map[int]*User)
users[1] = &User{Name: "Alice"}
if u := users[id]; u != nil {
    // ...
}
```

**3. 의존성 주입 맵 (DI Container)**

```go
deps := map[string]interface{}{
    "db":   dbConn,
    "cache": cache,
}
```

## 흔한 실수

### 실수 1: nil 맵에 쓰기

```go
// 틀린 코드
var m map[string]int
m["key"] = 1 // panic: assignment to entry in nil map
```

맵을 선언만 하고 쓰면 `panic: assignment to entry in nil map`이 발생합니다. 맵은 참조 타입이라 선언만 하면 nil 상태이며, 이 상태에서 쓰기는 런타임 오류입니다.

```go
// 고친 코드
m := make(map[string]int)
m["key"] = 1 // 정상 동작
```

`make`로 초기화하면 비어 있지만 사용 가능한 맵이 됩니다. 읽기(`m["key"]`)는 nil 맵에서도 안전하지만, 쓰기는 반드시 `make` 이후에 해야 합니다.

### 실수 2: comma-ok 없이 값 존재 확인

```go
// 틀린 코드
score := m["key"]
if score == 0 {
    // '없는 키'인지 '값이 0인 키'인지 구분 불가
}
```

맵에서 없는 키를 읽으면 해당 타입의 zero value(정수면 0, 문자열이면 빈 문자열)가 반환됩니다. 따라서 0 반환이 "없음"인지 "실제로 0"인지 구분할 수 없습니다.

```go
// 고친 코드
score, ok := m["key"]
if !ok {
    // 키가 없음
} else if score == 0 {
    // 값이 0임
}
```

comma-ok 패턴은 존재 여부를 명확히 구분합니다. `ok`가 `false`이면 키가 없다는 뜻이고, `true`이면 키가 있고 `score`는 실제 값입니다.

### 실수 3: 맵 값의 필드를 직접 수정하려 함

```go
// 틀린 코드
type Person struct {
    Name string
    Age  int
}
m := map[string]Person{"kim": {Name: "김철수", Age: 30}}
m["kim"].Age = 31 // 컴파일 오류: assignment not allowed
```

Go 맵의 값은 **값 타입**으로 반환됩니다. 즉 `m["kim"]`은 실제 값을 복사해서 돌려주므로, 그 값을 수정해도 원래 맵에는 영향이 없습니다. Go 컴파일러는 이런 실수를 미리 막습니다.

```go
// 고친 코드 1: 전체 재할당
m["kim"] = Person{Name: "김철수", Age: 31}

// 고친 코드 2: 포인터 사용 (권장)
m2 := map[string]*Person{"kim": {Name: "김철수", Age: 30}}
m2["kim"].Age = 31 // 포인터를 통해 원본 수정
```

포인터를 쓰면 맵에 저장된 포인터가 가리키는 값을 수정하므로, 맵 내부 데이터도 바뀝니다. 단, nil 포인터가 있을 수 있으므로 접근 전에 nil 체크가 필요합니다.

### 실수 4: 순회 순서 기대

```go
// 틀린 코드
for k, v := range m {
    // 순서를 기대하고 순차 처리 — 매번 다른 순서로 실행됨
}
```

맵 순회는 내부 해시 배치 순서에 따라 결정되며, 실행할 때마다 순서가 다릅니다. 순서가 필요한 경우 키를 슬라이스로 꺼내서 정렬해야 합니다.

```go
// 고친 코드
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}
sort.Strings(keys)
for _, k := range keys {
    v := m[k]
    // 순서 보장
}
```

## 주의할 점

맵은 동시성을 지원하지 않습니다. 여러 고루틴이 동시에 읽기만 한다면 안전하지만, 쓰기가 포함되면 **데이터 레이스**가 발생합니다. 동시성을 요구하면 `sync.Map`을 사용하거나, 일반 맵 앞에 `sync.Mutex`를 감싸야 합니다.

맵의 키로 사용하는 타입은 `==` 연산자를 지원해야 합니다. 슬라이스, 맵, 함수는 키로 사용할 수 없습니다 (컴파일 오류). 포인터, 문자열, 정수, 부동소수점, 구조체(모든 필드가 비교 가능한 경우)는 키로 사용할 수 있습니다.

맵의 용량(cap)은 `cap(m)`으로 알 수 있지만, `append`처럼 자동 확장되지 않습니다. `len(m)`이 커지면 내부 재할당이 자동으로 일어나지만, 이는 예측할 수 없습니다.

## 코드로 확인하기

`go run .` 실행 시 나오는 출력은 다음과 같은 순서로 이해하면 됩니다.

1. **nil 맵 panic**: "실수 버전"에서 panic이 발생하고 recover로 잡힌 메시지가 나옵니다. "고친 버전"에서는 `make`로 초기화했으므로panic 없이 정상 출력됩니다.
2. **comma-ok**: 존재하는 키는 점수를, 없는 키는 "없음" 메시지를 출력합니다. comma-ok 없이 읽은 버전에서는 "0"이 출력되어 "없는 값"인지 "0점"인지 구분되지 않는 문제가 드러납니다.
3. **순회 무작위성**: "순회 결과"에서 3번 순회할 때마다 순서가 바뀝니다. "정렬된 순서"에서는 알파벳 순으로 일관되게 나옵니다.
4. **값 필드 수정**: "실수 버전"은 컴파일 오류가 나는 코드를 주석으로 보여주고, "고친 버전 1"은 재할당으로 수정, "고친 버전 2"는 포인터로 직접 수정합니다. nil 체크도 함께 보여줍니다.

테스트는 `nil 맵`, `comma-ok`, `포인터 수정`, `순회 무작위성`, `nil 포인터`, `delete/len` 동작을 각각 검증합니다.

## 연습 문제

1. 맵에 키가 없으면 기본값을 설정하는 함수 `GetOrSet(m map[string]int, key string, default int) int`를 작성하세요.
2. 문자열 슬라이스에서 중복을 제거하고 정렬된 결과를 반환하는 함수를 맵을 사용해서 작성하세요.
3. 여러 고루틴이 동시에 맵에 쓰는 상황에서 데이터 레이스 없이 처리하는 코드를 `sync.Mutex`로 작성하세요.

## 더 읽기

- [Go Maps in Action — Go Blog](https://go.dev/blog/maps)
- [Go Spec: Maps](https://go.dev/ref/spec#Map_types)
- [Go by Example: Maps](https://gobyexample.com/maps)