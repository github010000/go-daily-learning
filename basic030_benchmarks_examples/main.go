package main

import (
	"fmt"
	"strings"
	"testing"
)

// Fib 는 일부러 재귀로 짠 피보나치입니다. 벤치마크 대상으로 쓰기 좋은 "작지만 공짜는 아닌" 일입니다.
func Fib(n int) int {
	if n < 2 {
		return n
	}
	return Fib(n-1) + Fib(n-2)
}

// Square 는 아주 작고 부작용이 없어서 컴파일러가 인라인하기 쉽습니다.
// 그래서 "결과를 버리는" 벤치마크의 함정을 보여 주기에 알맞습니다.
func Square(x int) int {
	return x * x
}

// Abs 는 Example 테스트의 재료입니다.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// MakeParts 는 n 개의 짧은 문자열 조각을 만듭니다.
func MakeParts(n int) []string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "abcdefghij"
	}
	return parts
}

// JoinNaive 는 += 로 이어 붙입니다. 매번 새 문자열을 만들어 복사하므로 조각이 많을수록 느려집니다.
func JoinNaive(parts []string) string {
	s := ""
	for _, p := range parts {
		s += p
	}
	return s
}

// JoinBuilder 는 strings.Builder 로 이어 붙입니다. 필요한 크기를 미리 잡아 복사를 줄입니다.
func JoinBuilder(parts []string) string {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	var sb strings.Builder
	sb.Grow(total)
	for _, p := range parts {
		sb.WriteString(p)
	}
	return sb.String()
}

// 패키지 수준 변수는 "결과를 받아 줄 곳" 입니다. 컴파일러는 전역 변수에 대한 쓰기를
// 함부로 지울 수 없으므로, 벤치마크 결과를 여기에 대입하면 계산이 살아남습니다.
// 입력도 전역 변수로 두면 컴파일 시점에 상수로 접어 버리지 못합니다.
var (
	sinkInt   int
	sinkStr   string
	inputInt  = 7
	inputSize = 1000
)

// benchDiscardedN 은 「실수 버전」입니다. 결과를 버리는 데다 호출이 인라인되면
// 컴파일러가 호출 자체를 지워 버려서 비정상적으로 빠른 값이 나올 수 있습니다.
func benchDiscardedN(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Square(i)
	}
}

// benchSink 는 「고친 버전」입니다. 결과를 전역 변수에 대입해서 계산이 지워지지 않게 합니다.
// (Go 1.24 이상이라면 b.Loop 가 이 일을 대신해 주지만, 이 모듈은 go 1.23 이라 b.N 을 씁니다.)
func benchSink(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkInt = Square(inputInt)
	}
}

// benchJoin 은 준비 코드를 루프 밖에 두고 b.ResetTimer 로 측정에서 뺍니다.
func benchJoin(join func([]string) string) func(*testing.B) {
	return func(b *testing.B) {
		parts := MakeParts(inputSize) // 준비 코드
		b.ResetTimer()                // 지금까지 걸린 시간은 측정하지 않음
		for i := 0; i < b.N; i++ {
			sinkStr = join(parts)
		}
	}
}

// measure 는 testing.Benchmark 로 벤치마크 함수를 테스트 바깥(main)에서 돌리고 ns/op 를 찍습니다.
func measure(name string, f func(*testing.B)) float64 {
	r := testing.Benchmark(f)
	ns := float64(r.T.Nanoseconds()) / float64(r.N)
	fmt.Printf("%-30s %10.2f ns/op\n", name, ns)
	return ns
}

func demoDiscard() {
	fmt.Println("== 결과를 버리는 벤치마크 (숫자는 환경마다 다름) ==")
	measure("실수 버전 (결과 버림)", benchDiscardedN)
	measure("고친 버전 (전역 변수에 대입)", benchSink)
	fmt.Println("실수 버전이 1 ns 아래로 나왔다면 호출이 통째로 지워졌을 가능성이 큽니다.")
}

func demoJoin() {
	fmt.Println("== += 대 strings.Builder (조각 1000개) ==")
	naive := measure("JoinNaive", benchJoin(JoinNaive))
	builder := measure("JoinBuilder", benchJoin(JoinBuilder))
	fmt.Println("Builder 가 더 빠른가:", builder < naive)
}

func main() {
	fmt.Println("Fib(10) =", Fib(10), ", Abs(-3) =", Abs(-3))
	fmt.Println("두 방식의 결과가 같은가:",
		JoinNaive(MakeParts(5)) == JoinBuilder(MakeParts(5)))
	demoDiscard()
	demoJoin()
}