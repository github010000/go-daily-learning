> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main_test.go:63:3: undefined: assertEqual`  

## 한 줄 요약
테스트 케이스를 구조체 슬라이스로 관리하고, `t.Run()`으로 반복 실행하는 테이블 주도 테스트 패턴을 익힙니다.

## 왜 필요한가
테스트를 쓸 때마다 "이 케이스도 넣어야지"하면서 코드를 복사·붙여넣기 한 적 있나요? 조건문이 늘어나면 테스트 코드도 덩달아 늘어납니다. Go의 테이블 주도 테스트는 **데이터(테이블)와 로직(반복문)을 분리**합니다. 새로운 케이스는 테이블에 행을 추가하는 것만으로 끝납니다.

Python의 `pytest.mark.parametrize`나 JavaScript의 Jest `.each`와 비슷해 보이지만, Go는 표준 라이브러리만으로 이 일을 합니다. `testing` 패키지의 `t.Run` 메서드가 핵심입니다. 다른 언어에서는 테스트 프레임워크가 제공해주는 기능을 Go는 `for` 루프 하나로 구현합니다. 이게 Go의 철학입니다: **무거운 도구가 아닌, 작은 빌딩 블록을 조합해서 해결한다**.

## 기본 사용
테이블 주도 테스트의 핵심은 **구조체 슬라이스**와 **`t.Run`**입니다.

```go
table := []struct {
    name     string
    input    string
    expected int
    wantErr  bool
}{
    {"양수", "123", 123, false},
    {"음수", "-42", 0, true},
}

for _, tc := range table {
    t.Run(tc.name, func(t *testing.T) {
        // tc를 사용해서 검증
    })
}
```

중요한 점: `for` 루프 변수 `tc`는 **매 반복마다 재사용**됩니다. Go 1.22 이전에는 클로저가 마지막 값을 공유할 수 있었지만, 지금은 각 반복마다 새 변수가 할당됩니다. 그래도 명시적으로 `tc := tc`로 복사하는 습관이 안전합니다.

`t.Run(name, func(t *testing.T))`은 **서브테스트**를 만듭니다. 서브테스트는:
- `-run` 플래그로 선택 실행 가능 (`go test -run '테이블/양수'`)
- 실패 시 어느 케이스가 문제인지 명확히 알림
- 병렬 실행 가능 (`t.Run` 내부에서 `t.Parallel()` 호출)

## 돌려보기
이 디렉토리에서 그대로 실행할 수 있는 명령입니다.

```bash
go vet ./...      # 정적 검사 — 컴파일 오류나 vet 경고가 없는지 확인
go run .          # 시연 실행 — 실수 버전과 고친 버전의 출력 차이 확인
go test -v ./...  # 테스트 — 테이블 주도 테스트가 어떻게 실행되는지 상세 출력 확인
go test -run '테이블/양수'  # 서브테스트 선택 실행 — 특정 케이스만 검증
```

## 실전에서는 이렇게 쓴다
표준 라이브러리와 유명 오픈소스에서도 테이블 주도 테스트를 흔히 씁니다.

**1. `strings` 패키지 테스트**
Go 표준 라이브러리의 `strings` 패키지 테스트를 보면, `TestContains`, `TestIndex` 같은 함수들이 모두 테이블 주도 패턴을 사용합니다. 케이스가 수십 개씩 되어도 코드가 깔끔합니다.

**2. HTTP 핸들러 테스트**
`net/http/httptest`와 결합하면 HTTP API 테스트도 테이블 주도 형태로 작성합니다:
```go
tests := []struct {
    name       string
    path       string
    wantStatus int
}{
    {"healthy", "/health", 200},
    {"not_found", "/missing", 404},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        req := httptest.NewRequest("GET", tt.path, nil)
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, req)
        if rec.Code != tt.wantStatus {
            t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
        }
    })
}
```

**3. JSON 언마셜링 테스트**
구조체의 JSON 직렬화/역직렬화 검증에도 유용합니다. 입력 JSON과 기대 출력 구조체를 테이블로 관리하면, 새로운 필드가 추가될 때 테스트도 함께 확장하기 쉽습니다.

## 흔한 실수

### 1. `t.Run` 내부에서 `t.Parallel()`을 잘못 쓰기

**무엇을 했는지**
서브테스트를 병렬로 실행하려고 `t.Parallel()`을 호출하는데, **부모 테스트에서도** `t.Parallel()`을 호출하는 실수입니다.

```go
// 틀린 코드
func TestBadParallel(t *testing.T) {
    t.Parallel() // 부모가 병렬이면 자식도 병렬이어야 합니다
    table := []struct{...}{}
    for _, tc := range table {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            // ...
        })
    }
}
```

**어떤 일이 벌어지는지**
Go 테스트 실행기는 "부모가 병렬인데 자식이 병렬이 아니다" 또는 그 반대의 경우 **패널티**를 줍니다. 정확히는 실행 순서가 보장되지 않거나, 테스트가 일시적으로 멈출 수 있습니다. 특히 `t.Helper()`와 함께 쓰면 에러 위치가 엉뚱한 곳으로 표시됩니다.

**왜 그런지**
`testing` 패키지의 병렬 실행 모델은 계층적입니다. 부모가 `t.Parallel()`을 호출하면 모든 자식도 병렬로 실행되어야 합니다. 그렇지 않으면 실행기가 혼란스러워합니다.

**고친 코드**
```go
func TestGoodParallel(t *testing.T) {
    t.Parallel()
    table := []struct{...}{}
    for _, tc := range table {
        tc := tc // Go 1.22 이전 안전 복사
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            // 검증 로직
        })
    }
}
```
반대로, 부모가 병렬이 아니면 자식도 `t.Parallel()`을 호출하지 않아야 합니다.

### 2. 에러 메시지를 구체적으로 쓰지 않기

**무엇을 했는지**
`t.Errorf("test failed")`처럼 모호한 메시지만 남깁니다.

```go
// 틀린 코드
t.Errorf("test failed")
```

**어떤 일이 벌어지는지**
테스트가 실패했을 때 로그를 보면 `"test failed"`만 보입니다. 어떤 값이 들어왔고, 어떤 값이 기대되었는지 알 수 없습니다. 디버깅하려면 소스 코드를 뒤져야 합니다.

**왜 그런지**
테스트 실패 메시지는 **자동으로 기록**됩니다. 나중에 다시 봐도 문제가 뭔지 알 수 있어야 합니다. "왜 실패했는지"를 메시지에 포함시키는 것이 테스트 작성자의 책임입니다.

**고친 코드**
```go
t.Errorf("input=%q: expected %d, got %d", tc.input, tc.expected, got)
```
이렇게 쓰면 실패 로그를 보자마자:
- 어떤 입력이었는지 (`input="abc"`)
- 무엇을 기대했는지 (`expected 123`)
- 무엇을 받았는지 (`got 0`)

를 한눈에 알 수 있습니다.

### 3. `t.Helper()`를 쓰지 않아 에러 위치가 엉뚱하게 표시되기

**무엇을 했는지**
검증 로직을 헬퍼 함수로 뽑았는데 `t.Helper()`를 호출하지 않습니다.

```go
// 틀린 코드
func assertEqual(t *testing.T, got, want int) {
    if got != want {
        t.Errorf("expected %d, got %d", want, got)
    }
}
```

**어떤 일이 벌어지는지**
테스트가 실패하면 에러 위치가 `assertEqual` 함수 내부라고 표시됩니다. 개발자는 에러가 발생한 테스트 파일의 `t.Run` 콜백으로 가서 확인해야 합니다. 헬퍼 함수를 여러 개 쓰면 어디가 문제인지 추적하기 힘듭니다.

**왜 그런지**
`t.Helper()`는 Go 컴파일러/테스트 실행기에게 "이 함수는 헬퍼이므로, 에러 위치를 **이 함수를 부른 곳**으로 표시하라"는 신호입니다. 이 주석이 없으면 표준 에러 위치 보고 기능을 사용하지 않습니다.

**고친 코드**
```go
func assertEqual(t *testing.T, got, want int) {
    t.Helper() // 이 줄이 핵심입니다
    if got != want {
        t.Errorf("expected %d, got %d", want, got)
    }
}
```

### 4. 테이블에서 `for` 루프 변수를 클로저에 직접 쓰기 (Go 1.22 이전)

**무엇을 했는지**
`for _, tc := range table` 루프에서 `tc`를 직접 클로저에 사용합니다.

```go
// 틀린 코드 (Go 1.22 이전 위험)
for _, tc := range table {
    t.Run(tc.name, func(t *testing.T) {
        // Go 1.22 이전: 모든 클로저가 같은 tc 변수를 공유
        // 마지막 반복의 tc 값만 사용될 수 있음
        use(tc.input)
    })
}
```

**어떤 일이 벌어지는지**
Go 1.22 이전에는 `for` 루프 변수가 반복마다 재사용되었습니다. 모든 클로저가 같은 메모리 위치를 참조하므로, 루프가 끝난 후 모든 서브테스트가 **마지막 케이스의 데이터**를 사용합니다. 테스트는 모두 "통과"하지만, 실제로는 잘못된 데이터로 검증합니다.

**왜 그런지**
Go의 `for` 루프 변수는 루프 블록 외부에서도 수명이 유지됩니다. 클로저는 변수의 **값**이 아닌 **변수 자체**를 참조합니다.

**고친 코드**
```go
for _, tc := range table {
    tc := tc // 새 변수 할당 (Go 1.22 이전 안전)
    t.Run(tc.name, func(t *testing.T) {
        use(tc.input)
    })
}
```
Go 1.22부터는 이 문제가 자동으로 해결됩니다. 그래도 명시적 복사는 여전히 좋은 습관입니다.

### 5. `wantErr` 플래그와 실제 에러 메시지를 혼동하기

**무엇을 했는지**
에러가 기대되는 케이스에서 `t.Errorf`로만 검증하고, 에러 메시지 내용을 체크하지 않습니다.

```go
// 틀린 코드
if tc.wantErr {
    if err == nil {
        t.Errorf("expected error for %q, but got nil", tc.input)
    }
    // err != nil이면 통과 — 하지만 에러 메시지가 무엇인지 체크하지 않음
    return
}
```

**어떤 일이 벌어지는지**
모든 에러 케이스가 "에러가 발생했다"는 사실만으로 통과합니다. 그런데 `parsePositiveIntFixed("-42")`가 `"negative not allowed"`가 아닌 `"wrong sign"`이라는 에러를 반환해도 테스트는 통과합니다. 에러 메시지가 바뀌어도 모릅니다.

**왜 그런지**
테이블 주도 테스트는 **검증의 엄격함**을 데이터로 관리해야 합니다. 에러 메시지까지 테이블에 포함시키면, 메시지 변경도 테스트로 잡을 수 있습니다.

**고친 코드**
```go
table := []struct {
    name        string
    input       string
    expected    int
    wantErr     bool
    wantErrText string // 에러 메시지 부분 일치 체크
}{
    {"음수", "-42", 0, true, "negative"},
    {"빈 문자열", "", 0, true, "empty"},
}

for _, tc := range table {
    t.Run(tc.name, func(t *testing.T) {
        result, err := parsePositiveIntFixed(tc.input)
        if tc.wantErr {
            if err == nil {
                t.Errorf("expected error for %q, but got nil", tc.input)
            } else if !strings.Contains(err.Error(), tc.wantErrText) {
                t.Errorf("expected error containing %q, got %v", tc.wantErrText, err)
            }
            return
        }
        // ...
    })
}
```

## 주의할 점
- **테이블은 `var`이 아닌 `const`나 함수 내부에**: 테스트 테이블을 패키지의 `var`으로 선언하면, 한 테스트가 테이블을 수정할 때 다른 테스트에 영향을 줄 수 있습니다. 항상 함수 내부에 선언하거나 `t.Parallel()` 사용 시 복사본을 만들겠습니다.
- **테스트 실행 순서 불확실**: `t.Run`으로 만든 서브테스트는 실행 순서가 보장되지 않습니다. 테스트 간 의존성을 만들지 마세요.
- **`t.Fatalf` vs `t.Errorf`**: `t.Fatalf`는 그 서브테스트를 즉시 종료합니다. `t.Errorf`는 계속 실행됩니다. 에러가 나면 더 이상 검증할 의미가 없을 때 `Fatalf`를 쓰세요.
- **Go 1.22 이전 루프 변수 문제**: Go 1.22 이전 버전을 사용하는 환경에서는 `tc := tc` 복사가 필수입니다. 현재 표준은 Go 1.22+이지만, 레거시 코드를 수정할 때 마주칠 수 있습니다.

## 코드로 확인하기
`go run .`을 실행하면 두 버전의 출력이 나옵니다.

**실수 버전**에서는 에러 메시지가 각기 다릅니다:
- `""` → `empty input`
- `"-42"` → `negative not allowed`
- `"abc"` → `invalid integer: ...`
- `"12a"` → `invalid integer: ...`

메시지 포맷이 `"empty input"`, `"negative not allowed"`처럼 제멋대로입니다. 새로운 에러 케이스를 추가하면 또 다른 포맷이 생길 것입니다.

**고친 버전**에서는 에러 메시지가 더 일관되어 보입니다. `parsePositiveIntFixed`는 빈 문자열과 음수만 특별히 처리하고, 나머지는 `strconv.Atoi`에 맡깁니다. 테이블 주도 테스트는 이 동작을 **데이터로 명시**합니다.

`go test -v ./...`를 실행하면 각 서브테스트가 독립적으로 실행되는 것을 볼 수 있습니다:
```
=== RUN   TestParsePositiveInt_TableDriven
=== RUN   TestParsePositiveInt_TableDriven/양수
=== RUN   TestParsePositiveInt_TableDriven/영
=== RUN   TestParsePositiveInt_TableDriven/음수
...
```
`-run '테이블/양수'`로 특정 케이스만 선택 실행도 가능합니다.

`main_test.go`의 `TestParsePositiveInt_TableDriven`은 테이블의 모든 케이스를 검증합니다. `TestParsePositiveInt_CommonMistake`는 고친 버전의 기본 동작을, `TestAssertEqual`은 헬퍼 함수의 동작을 각각 검증합니다.

## 연습 문제
1. **쉬운 것**: `parsePositiveIntFixed`에 "소수점 포함" 케이스("3.14")를 추가하고, 에러가 반환되도록 테스트 테이블을 확장하세요. 힌트: `strconv.Atoi`는 소수점을 허용하지 않으므로 에러가 반환됩니다.
2. **중간 것**: 테이블 주도 테스트에서 `wantErrText` 필드를 추가해, 에러 메시지에 특정 키워드가 포함되는지 검증하는 테스트를 작성하세요. 힌트: `strings.Contains(err.Error(), keyword)`를 사용하세요.
3. **어려운 것**: HTTP 핸들러 테스트를 테이블 주도 형태로 작성하세요. `httptest` 패키지를 사용하고, 경로별 기대 상태 코드를 테이블로 관리하세요. 힌트: `httptest.NewRecorder()`와 `httptest.NewRequest()`를 조합하세요.

## 더 읽기
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) — 공식 위키의 테이블 주도 테스트 가이드
- [Go Testing Package Documentation](https://pkg.go.dev/testing) — `t.Run`, `t.Helper`, 병렬 테스트 문서
- [Effective Go: Tests](https://go.dev/doc/effective_go#testing) — Go 테스트 작성 관례