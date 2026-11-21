## 한 줄 요약

벤치마크는 `go test -bench` 로 "얼마나 빠른가" 를 재고, Example 함수는 "이렇게 쓴다" 를 문서에 싣는 동시에 `go test` 가 출력까지 검사해 주는 코드입니다. 둘 다 별도 프레임워크 없이 `testing` 패키지만으로 됩니다.

## 왜 필요한가

성능을 이야기할 때 "느낌" 은 거의 틀립니다. Python 에는 `timeit`, Java 에는 JMH, JavaScript 에는 `console.time` 이나 외부 라이브러리가 있는데, 대부분 별도 도구를 설치하고 설정해야 합니다. Go 는 이것을 언어 도구 체인에 넣었습니다. `BenchmarkXxx(b *testing.B)` 함수를 `_test.go` 파일에 쓰고 `go test -bench=.` 를 실행하면 끝입니다. 반복 횟수 `b.N` 은 프레임워크가 알아서 늘려 가며 측정 시간이 충분히 안정될 때까지 같은 함수를 다시 부릅니다. 그래서 "몇 번 돌릴까" 를 고민하지 않아도 됩니다.

문서 쪽도 비슷합니다. 다른 언어에서는 README 나 주석에 사용 예를 적는데, 코드가 바뀌면 예제는 조용히 낡아 갑니다. Java 의 Javadoc 예제나 Python 의 docstring 예제(doctest 는 선택 사항)가 그런 문제를 겪습니다. Go 는 `ExampleXxx` 함수를 만들고 마지막에 `// Output:` 주석을 달면, `go doc` 과 pkg.go.dev 에 예제로 표시되고 `go test` 때는 실제로 실행되어 출력이 주석과 다르면 실패합니다. 예제가 곧 테스트이기 때문에 문서가 거짓말을 할 수 없습니다.

벤치마크에는 Go 특유의 함정이 하나 있습니다. Go 컴파일러는 작은 함수를 인라인하고, 결과가 쓰이지 않는 순수한 계산은 지워 버릴 수 있습니다. 그러면 측정하려던 일이 사라져서 터무니없이 빠른 숫자가 나옵니다. Go 1.24 부터는 `for b.Loop() { ... }` 형태가 생겨서 이런 문제와 준비 코드 시간 제외를 자동으로 처리해 주지만, 이 디렉토리의 `go.mod` 는 1.23 이므로 여기서는 전통적인 `b.N` 루프와 전역 변수 대입(sink)으로 같은 문제를 해결합니다. 자신의 Go 버전이 1.24 이상이면 `b.Loop` 도 알아 두세요.

## 기본 사용

벤치마크 함수는 `Benchmark` 로 시작하고 `*testing.B` 를 받습니다.

```go
var sinkStr string

func BenchmarkJoinBuilder(b *testing.B) {
	parts := MakeParts(1000) // 준비 코드
	b.ReportAllocs()         // 할당 횟수와 바이트도 함께 출력
	b.ResetTimer()           // 준비에 쓴 시간은 버림
	for i := 0; i < b.N; i++ {
		sinkStr = JoinBuilder(parts) // 결과를 전역 변수에 대입
	}
}
```

하위 벤치마크는 `b.Run` 으로 만듭니다. 입력 크기별로 한 번에 비교할 때 편합니다.

```go
var sinkInt int

func BenchmarkFib(b *testing.B) {
	for _, n := range []int{5, 10, 15} {
		b.Run(fmt.Sprint("n=", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sinkInt = Fib(n)
			}
		})
	}
}
```

Example 함수는 `Example` 뒤에 문서를 붙일 대상 이름을 씁니다. 마지막 `// Output:` 주석이 있어야 실행되고 검사됩니다.

```go
func ExampleAbs() {
	fmt.Println(Abs(-3), Abs(3))
	// Output: 3 3
}
```

`Example` 만 쓰면 패키지 전체 예제, `ExampleFib` 는 `Fib` 함수의 예제, `ExampleFib_second` 처럼 소문자로 시작하는 접미사는 같은 대상의 추가 예제입니다.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

- `go vet ./...`: 아무 출력이 없으면 통과입니다. `testing.Loop requires go1.24` 같은 메시지가 나오면 `go.mod` 의 Go 버전보다 새 API 를 쓴 것입니다.
- `go run .`: 「결과를 버리는 벤치마크」 와 「전역 변수에 대입한 벤치마크」 의 ns/op 를 비교해 보세요. 숫자는 환경마다 다르니, 두 값의 차이와 "Builder 가 더 빠른가" 줄을 보면 됩니다. 몇 초 걸립니다.
- `go test -v ./...`: `TestFib`, `TestJoinSame` 같은 테스트와 `ExampleFib` 등이 `PASS` 로 나오는지 보세요. Example 도 테스트로 실행됩니다.

벤치마크 자체를 돌리려면 다음을 실행합니다.

```bash
go test -bench=. -benchmem -run='^$'
```

`-run='^$'` 는 일반 테스트를 건너뛰게 하고, `-benchmem` 은 할당 정보를 보여 줍니다. `JoinNaive` 와 `JoinBuilder` 의 `allocs/op` 차이를 보세요.

## 실전에서는 이렇게 쓴다

**1. 두 구현 중 빠른 쪽 고르기.** 표준 라이브러리 `strings`, `bytes`, `sort` 패키지의 `_test.go` 에는 구현을 바꿀 때마다 돌려 보는 벤치마크가 가득합니다. 최적화 PR 에는 "before/after" 벤치마크 결과를 붙이는 것이 관례입니다. 같은 벤치마크를 여러 번(`-count=10`) 돌려 `benchstat` 도구로 비교하면 노이즈를 걸러 낼 수 있습니다.

**2. 입력 크기별 하위 벤치마크.** `b.Run` 으로 크기를 바꿔 가며 돌리면 알고리즘이 어떻게 커지는지 한눈에 보입니다. 이 자료의 `BenchmarkFib` 가 그 모양입니다.

**3. 패키지 문서로서의 Example.** 표준 라이브러리의 `strings`, `sort`, `time` 문서에 있는 예제 코드는 대부분 `example_test.go` 의 Example 함수입니다. 보통 `package strings_test` 처럼 외부 테스트 패키지에 두어서, 사용자가 쓰는 것과 똑같이 패키지를 import 해서 호출하는 모습을 보여 줍니다.

**4. 정렬되지 않은 출력.** 맵을 순회하는 예제는 `// Unordered output:` 으로 쓰면 줄 순서와 상관없이 비교합니다.

## 흔한 실수

### 1. 결과를 버려서 계산이 지워짐

```go
// 틀린 코드
func BenchmarkSquare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Square(i) // 결과를 아무도 쓰지 않음
	}
}
```

`Square` 는 작고 부작용이 없어서 인라인됩니다. 결과가 쓰이지 않으면 컴파일러가 호출을 통째로 없앨 수 있고, 그러면 1 ns 도 안 되는 수치처럼 "빨라 보이는" 값이 나옵니다. 아무 일도 측정하지 않은 것입니다. 에러 메시지도 없이 숫자만 틀리기 때문에 더 위험합니다. `go run .` 으로 두 버전의 ns/op 를 비교해 확인하세요. 항상 지워지는 것은 아니며 컴파일러 버전과 환경에 따라 다릅니다. 그래서 더 알아채기 어렵습니다. 고치려면 결과를 패키지 수준 변수에 대입합니다.

```go
var sinkInt int
var inputInt = 7

func BenchmarkSquareFixed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkInt = Square(inputInt)
	}
}
```

입력도 상수 `7` 대신 변수로 두면 컴파일러가 결과를 미리 계산해 버리는 것을 막을 수 있습니다.

### 2. 준비 코드를 측정에 포함시킴

```go
// 틀린 코드
func BenchmarkJoinSetupIncluded(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parts := MakeParts(1000) // 매 반복마다 준비까지 측정됨
		sinkStr = JoinBuilder(parts)
	}
}
```

측정 대상이 `JoinBuilder` 인데 `MakeParts` 시간이 섞여서, 두 구현의 차이가 희석됩니다. 준비는 루프 밖에서 한 번만 하고 `b.ResetTimer()` 로 그때까지의 시간을 지우세요.

```go
func BenchmarkJoinSetupExcluded(b *testing.B) {
	parts := MakeParts(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = JoinBuilder(parts)
	}
}
```

### 3. b.N 을 무시하고 고정 횟수만 돎

```go
// 틀린 코드
func BenchmarkFibFixed(b *testing.B) {
	for i := 0; i < 1000; i++ {
		sinkInt = Fib(15)
	}
}
```

프레임워크는 `b.N` 을 키워 가며 다시 부르는데, 함수가 `b.N` 을 쓰지 않으면 매번 같은 일만 하므로 "1회당 시간" 이 `b.N` 으로 나뉘어 터무니없이 작게 보고됩니다. 반드시 `for i := 0; i < b.N; i++` 로 도세요(Go 1.24 이상이면 `b.Loop`).

### 4. Output 주석을 빼먹거나 형식을 틀림

```go
// 틀린 코드
func ExampleAbsNoOutput() {
	fmt.Println(Abs(-3))
}
```

`// Output:` 주석이 없는 Example 은 컴파일만 되고 **실행되지 않습니다.** 문서에는 보이는데 검증은 안 되는 가장 위험한 상태입니다. 또 Output 주석은 함수 본문의 마지막 주석이어야 합니다. 출력의 앞뒤 공백은 무시되지만 줄 안의 내용은 정확히 같아야 합니다. 고친 코드는 다음과 같습니다.

```go
func ExampleAbsWithOutput() {
	fmt.Println(Abs(-3))
	// Output: 3
}
```

### 5. 존재하지 않는 대상 이름의 Example

```go
// 틀린 코드
func ExampleSquareRoot() {
	fmt.Println(Square(3))
	// Output: 9
}
```

`go vet` 가 "ExampleSquareRoot refers to unknown identifier: SquareRoot" 라는 취지의 경고를 냅니다. Example 이름은 실제 존재하는 함수·타입 이름이어야 합니다. 대상이 `Square` 라면 `ExampleSquare` 로 쓰세요.

### 6. 벤치마크 숫자를 한 번 보고 결론 냄

한 번 돌린 결과는 CPU 온도, 다른 프로그램, 전원 설정에 따라 크게 흔들립니다. 두 구현의 차이가 몇 퍼센트 수준이면 `-count=10` 으로 반복하고 `benchstat` 으로 비교하세요. 노트북이 배터리 모드라면 더 흔들립니다.

## 주의할 점

- **Go 버전에 따라 API 가 다릅니다.** `b.Loop` 은 Go 1.24 에서 생겼습니다. `go.mod` 의 `go` 줄이 그보다 낮으면 `go vet` 가 "requires go1.24 or later" 로 알려 줍니다. 오래된 코드에는 `b.N` 루프가 많으니 두 방식을 모두 읽을 줄 알아야 합니다.
- **`testing.Benchmark` 는 `main` 에서도 쓸 수 있습니다.** 이 자료의 `go run .` 이 그 예입니다. 다만 일반적인 방식은 아니므로, 실무에서는 `_test.go` 에 두고 `go test -bench` 로 돌리세요.
- **마이크로 벤치마크는 현실과 다릅니다.** 캐시가 따뜻하고 분기 예측이 잘 맞는 좁은 상황만 측정하므로, 실제 서비스에서는 다른 결과가 나올 수 있습니다. 병목은 `pprof` 로 먼저 찾고, 벤치마크로는 "바꾼 뒤 나아졌는가" 를 확인하는 용도로 쓰세요.
- **벤치마크는 기본적으로 실행되지 않습니다.** `go test` 만 쳐서는 `Benchmark` 함수가 돌지 않습니다. 반드시 `-bench` 플래그가 필요합니다. 반면 Example 은 Output 주석이 있으면 항상 실행됩니다.
- **정확성 먼저.** 느린 쪽과 빠른 쪽이 같은 결과를 내는지 테스트로 확인한 뒤 벤치마크하세요. 틀린 결과를 빨리 내는 코드는 의미가 없습니다. 이 자료의 `TestJoinSame` 이 그 역할입니다.
- **다른 언어 습관.** JMH 같은 도구는 워밍업 반복을 따로 설정하지만, Go 는 `b.N` 을 늘려 가며 자동으로 시간을 맞춥니다. 대신 위에서 본 것처럼 코드 작성자가 결과를 살려 두는 책임을 집니다.

## 코드로 확인하기

`go run .` 은 먼저 `Fib(10)` 과 `Abs(-3)` 값, 그리고 `JoinNaive` 와 `JoinBuilder` 가 같은 결과를 내는지 출력합니다. 이어서 「실수 버전 (결과 버림)」 과 「고친 버전 (전역 변수에 대입)」 의 ns/op 를 찍습니다. 실수 버전은 호출이 지워졌다면 눈에 띄게 작은 값이 나오고, 고친 버전은 곱셈과 전역 변수 쓰기 비용이 보이는 값이 나옵니다. 환경에 따라 차이가 작을 수도 있으니, 숫자 자체보다 "버려진 결과가 측정을 어떻게 왜곡할 수 있는가" 를 읽으세요. 그다음 조각 1000개를 이어 붙이는 `JoinNaive` 와 `JoinBuilder` 를 비교하고, 마지막에 Builder 가 더 빠른지 `true`/`false` 로 알려 줍니다.

`main_test.go` 는 `Fib` 를 표로 검증하는 `TestFib`, 두 Join 구현이 같은 결과·같은 길이를 내는지 확인하는 `TestJoinSame`, `Square`·`Abs` 기본 동작, 고친 벤치마크가 결과를 전역 변수에 남기는지 보는 `TestBenchSinkStoresResult` 를 담고 있습니다. 시간 값은 단정하지 않습니다. 또한 `ExampleFib`, `ExampleAbs`, `ExampleJoinBuilder` 가 Output 주석과 실제 출력이 같은지 확인합니다. Example 의 Output 값을 일부러 바꿔 보면 `go test` 가 실패하는 것을 볼 수 있습니다.

## 연습 문제

1. `ExampleAbs` 의 `// Output:` 값을 일부러 틀리게 바꿔서 `go test` 가 어떤 메시지로 실패하는지 읽어 보세요. (힌트: `got:` 과 `want:` 두 줄이 나옵니다.)
2. `JoinBuilder` 에서 `sb.Grow(total)` 줄을 지우고 `go test -bench=Join -benchmem -run='^$'` 를 돌려 `allocs/op` 가 어떻게 달라지는지 비교하세요. (힌트: 숫자가 늘어납니다. 얼마나인지는 직접 확인하세요.)
3. 맵을 순회해 출력하는 함수를 만들고 `// Unordered output:` 을 쓰는 Example 을 작성한 뒤, `Output:` 으로 바꾸면 어떻게 되는지 `-count=20` 으로 확인하세요. (힌트: 맵 순회 순서는 실행마다 달라집니다.)

## 더 읽기

- testing 패키지 문서: https://pkg.go.dev/testing
- Go 블로그, "Testable Examples in Go": https://go.dev/blog/examples
- Go 블로그, "More predictable benchmarking with testing.B.Loop": https://go.dev/blog/testing-b-loop
- benchstat 도구: https://pkg.go.dev/golang.org/x/perf/cmd/benchstat