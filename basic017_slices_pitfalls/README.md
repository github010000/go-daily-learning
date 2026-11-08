## 한 줄 요약
슬라이스는 "배열을 가리키는 헤더"일 뿐이다 — append, 공유, copy를 잘못 쓰면 원본이 죽거나 메모리가 새거나 버그가 난다.

## 왜 필요한가
Python의 list, JavaScript의 Array, Java의 ArrayList는 모두 "객체 자체"처럼 다룬다. append하면 객체가 커지고, slice하면 새 객체가 생긴다. Go는 다르다. Go의 슬라이스는 **배열에 대한 레이어**(포인터·길이·용량 3종 세트)일 뿐이다. 이 차이를 모르면 코드가 이상하게 동작한다.

Go가 이렇게 설계한 이유는 **명확한 메모리 통제** 때문이다. Python은 가비지 컬렉터가 알아서 처리하지만, Go는 시스템 프로그래밍에도 쓰인다. 네트워크 패킷, 이미지 처리, 대규모 데이터 처리에서 "이 슬라이스가 이 메모리를 가리킨다"는 것을 개발자가 정확히 알아야 한다. slice 연산자 `s[i:j]`가 새 배열을 할당하지 않는 이유도 바로 이 통제력 때문이다.

이걸 모르면 생기는 문제가 세 가지다. 첫째, **append 후 원본이 바뀐다** — 하위 슬라이스를 만들면 원래 배열의 여분 공간을 append가 사용한다. 둘째, **함수에 슬라이스를 넘기면 side effect가 생긴다** — 배열을 공유하기 때문이다. 셋째, **하위 슬라이스가 큰 배열을 붙잡아 메모리 누수를 일으킨다** — cap이 큰 배열을 가리키기 때문이다.

## 기본 사용
슬라이스 리터럴 `[]int{1, 2, 3}`은 내부에 3원소 배열을 만들고 그 배열을 가리키는 헤더를 생성한다.

```go
s := []int{1, 2, 3}
sub := s[1:3] // len=2, cap=2. underlying array는 s와 동일
```

append는 **슬라이스 헤더를 반환**한다. 새 배열이 할당되면 포인터가 바뀐다. 하지만 **원본 슬라이스 헤더는 변하지 않는다**(len/cap이 바뀌지 않는다). 단, underlying array의 내용이 바뀌면 원본 슬라이스가 그 변경을 본다 — 같은 배열을 공유하기 때문이다.

copy(dst, src)는 **min(len(dst), len(src))** 만큼 복사하고 그 수를 반환한다. dst가 더 길면 남은 곳은 0으로 채워진다.

## 돌려보기

```bash
go vet ./...      # 정적 검사 — unused variable 없이 깨끗한지 확인
go run .          # 시연 실행 — append, 공유, 메모리, copy 출력 결과 확인
go test -v ./...  # 테스트 — 모든 단정 통과해야 함
```

`go run .` 출력에서 다음을 봐야 한다:
1. "실수 버전" 섹션에서 base가 `10 20 99` → `10 20 88`으로 바뀌는 것 확인
2. "고친 버전" 섹션에서 base2가 변하지 않고 original2만 바뀌는 것 확인
3. 메모리 누수 시연에서 safeTiny가 10바이트만 가리키는 것 확인
4. copy 규칙에서 반환값이 min(len(dst), len(src))인 것 확인

## 실전에서는 이렇게 쓴다

**패턴 1: 함수 인자로 받은 슬라이스를 안전하게 처리하기**
서버 프레임워크나 라이브러리에서 요청 데이터를 받은 뒤 처리할 때, 원본을 변조하지 않으려면 copy로 분리한다.

```go
func ProcessRequest(data []byte) ([]byte, error) {
    // copy로 독립적인 버퍼 생성
    buf := make([]byte, len(data))
    copy(buf, data)
    // buf를 자유롭게 수정 가능
    return buf, nil
}
```

**패턴 2: 하위 슬라이스로 처리한 뒤 메모리 회수하기**
대용량 파일에서 특정 부분만 처리할 때, 처리가 끝나면 하위 슬라이스를 버퍼로 복사해 큰 배열을 해제한다.

```go
func ReadChunk(f *os.File, offset, size int) ([]byte, error) {
    // 임시로 큰 슬라이스 할당
    buf := make([]byte, size)
    _, err := f.ReadAt(buf, int64(offset))
    if err != nil {
        return nil, err
    }
    // copy로 독립적인 슬라이스 생성 후 큰 슬라이스 해제
    result := make([]byte, size)
    copy(result, buf)
    return result, nil
}
```

## 흔한 실수

### 1. append 후 하위 슬라이스가 원본 배열을 변조한다

**틀린 코드**
```go
base := []int{10, 20, 30}
sub := base[1:2]          // sub는 [20], cap=2
sub = append(sub, 99, 88) // base가 [10, 20, 99, 88]로 변함!
```

**어떤 일이 벌어지는지**
`base`가 출력된다: `[10 20 99 88]`. 30이 99로 overwritten 된다. `sub`의 cap=2이지만 `base`의 cap=3이므로, `append`가 `base`의 여분 공간(30 위치)을 사용한다.

**왜 그런지**
슬라이스는 underlying array를 공유한다. `sub`는 `base`의 2번째 원소부터 시작하지만, underlying array는 동일하다. `append`가 용량이 부족할 때 새 배열을 할당하는 게 아니라, 같은 배열의 여분 공간을 사용한다.

**고친 코드**
```go
base := []int{10, 20, 30}
sub := base[1:2]
original := make([]int, len(sub))
copy(original, sub)       // 깊은 복사
original = append(original, 99, 88)
// base는 그대로 [10 20 30]
```

### 2. 함수에 슬라이스를 넘기면 side effect가 생긴다

**틀린 코드**
```go
func CleanData(data []int) []int {
    data[0] = 0             // callers의 배열을 직접 수정!
    return data
}

func main() {
    d := []int{1, 2, 3}
    CleanData(d)
    fmt.Println(d)           // [0 2 3] — callers가 의도치 않게 변조됨
}
```

**어떤 일이 벌어지는지**
`d[0]`이 0으로 변한다. 함수는 슬라이스 헤더(포인터·len·cap)를 값으로 받지만, 포인터가 가리키는 배열은 공유된다.

**왜 그런지**
Go는 모든 인자를 값 전달(pass-by-value)한다. 하지만 슬라이스 헤더에 포함된 "배열 포인터"는 공유 메모리 주소를 의미한다. 함수가 그 포인터를 따라 배열을 수정하면 callers도 그 변경을 본다.

**고친 코드**
```go
func CleanData(data []int) []int {
    copied := make([]int, len(data))
    copy(copied, data)      // 깊은 복사
    copied[0] = 0
    return copied
}
```

### 3. 하위 슬라이스가 큰 배열을 메모리에 고정시킨다

**틀린 코드**
```go
func ExtractFirstTen(data []byte) []byte {
    result := data[:10]     // result.cap = len(data)
    return result
}

func main() {
    big := make([]byte, 1_000_000)
    small := ExtractFirstTen(big)
    big = nil               // GC가 1MB를 회수하지 못함!
    _ = small
}
```

**어떤 일이 벌어지는지**
`big`를 nil로 만들어도 1MB 메모리가 해제되지 않는다. `small`의 cap이 `len(big)`을 가리키기 때문이다.

**왜 그런지**
슬라이스 헤더의 cap 필드는 underlying array의 끝까지를 가리킨다. 하위 슬라이스를 만들어도 cap은 변경되지 않는다(단, `s[:0:5]`처럼 3개 슬라이스 연산을 쓰면 cap을 제한할 수 있다). GC는 슬라이스가 살아있는 한 그 슬라이스가 가리키는 배열을 회수하지 않는다.

**고친 코드**
```go
func ExtractFirstTen(data []byte) []byte {
    result := make([]byte, 10)
    copy(result, data[:10]) // 독립적인 배열 할당
    return result
}
```

### 4. copy의 반환값을 무시한다

**틀린 코드**
```go
src := []int{1, 2, 3}
dst := make([]int, 2)
copy(dst, src)            // 2개만 복사, 반환값 2를 무시
fmt.Println(dst)           // [1 2] — 의도한 대로 동작하지만, 왜 2개인지 모름
```

**어떤 일이 벌어지는지**
의도한 대로 동작하지만, `copy`가 정확히 몇 개를 복사했는지 알 수 없다. dst가 더 길거나 src가 더 짧을 때 버그가 숨는다.

**왜 그런지**
`copy`는 `min(len(dst), len(src))`를 반환한다. 이 반환값을 체크하지 않으면 dst가 충분히 크지 않아 데이터가 잘려도 모를 수 있다.

**고친 코드**
```go
src := []int{1, 2, 3}
dst := make([]int, 2)
n := copy(dst, src)
if n != len(src) {
    // dst가 src 전체를 담지 못함 — 에러 처리 또는 dst 확장
}
```

### 5. append를 할당 없이 호출한다

**틀린 코드**
```go
s := []int{1, 2}
append(s, 3)              // 반환값을 버림! s는 여전히 [1 2]
fmt.Println(s)             // [1 2] — append된 게 없음
```

**어떤 일이 벌어지는지**
`s`는 변하지 않는다. `append`는 새 슬라이스 헤더를 반환하는데, 그것을 버렸기 때문이다.

**왜 그런지**
Go의 append는 슬라이스를 수정하지 않고 **새 슬라이스를 반환**한다. Python의 `list.append()`가 인자를 수정하는 것과 다르다. 이 차이를 모르면 "append했는데 안 됨" 버그에 빠진다.

**고친 코드**
```go
s := []int{1, 2}
s = append(s, 3)          // 반환값을 반드시 할당
fmt.Println(s)             // [1 2 3]
```

## 주의할 점

**Go 1.22 이전의 for-range 변수 재사용**
Go 1.22 이전에는 `for i, v := range slice`에서 `v`가 매 iteration마다 같은 변수를 재사용한다. 고루틴으로 전달할 때 주의:

```go
// Go 1.22 이전: 모든 고루틴이 마지막 v를 본다
for _, v := range slice {
    go func() { fmt.Println(v) }()  // 버그 가능
}
// 고친 방법: 로컬 복사
for _, v := range slice {
    v := v
    go func() { fmt.Println(v) }()
}
```

**슬라이스 3개 연산으로 cap 제한**
`s[i:j:k]`에서 k를 쓰면 cap을 제한할 수 있다. 이건 메모리 누수 방지와 안전성에 도움된다:

```go
base := []int{1, 2, 3, 4, 5}
sub := base[1:3:3]  // len=2, cap=2. append하면 새 배열 할당
```

**nil 슬라이스와 빈 슬라이스의 차이**
`var s []int`는 nil 슬라이스, `s := []int{}`는 빈 슬라이스다. `len(nilSlice) == 0`이지만 `nilSlice == nil`은 true, 빈 슬라이스는 false다. JSON 직렬화, API 응답에서 이 차이가 중요하다. nil은 `"null"`, 빈 슬라이스는 `[]`로 직렬화된다.

## 코드로 확인하기

`go run .`을 실행하면 다음과 같은 출력이 나온다:

**1번 append 시연**: 실수 버전에서 `base`가 `10 20 30`에서 `10 20 99`로 변했다가 `10 20 88`이 된다. `sub`가 `base`의 여분 공간을 침범했기 때문이다. 고친 버전에서는 `base2`가 `10 20 30` 그대로이고 `original2`만 `10 20 99 88`이다. copy로 독립적인 슬라이스를 만들었기 때문이다.

**2번 공유 슬라이스 시연**: 실수 버전에서 `data`가 호출 후 `0 200 300 400`으로 변한다. 함수가 원본 배열을 수정했기 때문이다. 고친 버전에서는 `data2`가 `100 200 300 400` 그대로이고 반환값만 `0 200 300 400`이다.

**3번 메모리 누수 시연**: 1MB 배열을 만들고 하위 슬라이스를 만든 뒤 큰 배열을 nil로 만든다. 실수 버전에서는 GC가 회수하지 못한다. 고친 버전에서는 copy로 독립적인 10바이트만 남기고 1MB를 해제할 수 있다.

**4번 copy 규칙 시연**: copy는 min(len(dst), len(src))만큼 복사하고 그 수를 반환한다. dst가 더 길면 남은 곳은 0으로 채워진다. 오버랩 복사도 안전하다.

테스트는 `TestAppendDoesNotMutateOriginal`(append가 원본 슬라이스 헤더를 변조하지 않음), `TestMutateSliceSafePreservesOriginal`(깊은 복사가 원본을 보존함), `TestCopyReturnsCorrectLength`(copy 반환값 검증), `TestDeepCopyCreatesIndependentSlice`(깊은 복사가 독립적임)를 검증한다.

## 연습 문제

1. **쉬운**: `s := []int{1, 2, 3, 4, 5}`가 있을 때, `s`를 변경하지 않고 `s[1:4]`의 깊은 복사를 만드는 코드를 작성하라. 힌트: `make` + `copy`를 사용한다.

2. **중간**: `append`를 사용할 때 underlying array가 재할당되는지 여부를 확인하는 함수 `needsReallocation(s []int, n int) bool`을 작성하라. 힌트: `cap(s)`와 `len(s)+n`을 비교한다.

3. **어려운**: 큰 슬라이스에서 특정 범위의 하위 슬라이스를 반환하는 함수를 작성하라. 이때 큰 슬라이스가 메모리에 남지 않도록 copy로 분리하고, 반환값이 append되어도 원본이 변하지 않도록 하라. 힌트: 3개 슬라이스 연산과 copy를 조합한다.

## 더 읽기
- [Go Slices: usage and internals](https://go.dev/blog/slices) — Go 블로그의 슬라이스 공식 문서. underlying array, append 알고리즘, cap 확장 규칙을 다룬다.
- [Effective Go - Slices](https://go.dev/doc/effective_go#slices) — Go 공식 문서의 관용적 슬라이스 사용법.
- [Go Wiki: SlicesTricks](https://github.com/golang/go/wiki/SliceTricks) — 슬라이스 연산의 다양한 패턴과 함정을 다룬다.