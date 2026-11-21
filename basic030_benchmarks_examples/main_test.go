package main

import (
	"fmt"
	"testing"
)

// 테이블 주도 테스트: Fib 의 기본 동작을 여러 입력으로 한 번에 검증합니다.
func TestFib(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{"0", 0, 0},
		{"1", 1, 1},
		{"10", 10, 55},
		{"15", 15, 610},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Fib(tt.n); got != tt.want {
				t.Errorf("Fib(%d) = %d, want %d", tt.n, got, tt.want)
			}
		})
	}
}

// 두 이어 붙이기 구현이 같은 결과를 내는지 검증합니다.
// 벤치마크로 빠른 쪽을 고르기 전에, 먼저 둘이 같은 일을 하는지 확인해야 하기 때문입니다.
func TestJoinSame(t *testing.T) {
	for _, n := range []int{0, 1, 5, 100} {
		t.Run(fmt.Sprint("n=", n), func(t *testing.T) {
			parts := MakeParts(n)
			a, b := JoinNaive(parts), JoinBuilder(parts)
			if a != b {
				t.Errorf("결과가 다름: %q vs %q", a, b)
			}
			if len(a) != 10*n {
				t.Errorf("길이 = %d, want %d", len(a), 10*n)
			}
		})
	}
}

// Square 와 Abs 의 기본 동작 검증입니다.
func TestSquareAbs(t *testing.T) {
	if Square(7) != 49 {
		t.Error("Square(7) != 49")
	}
	if Abs(-4) != 4 || Abs(4) != 4 || Abs(0) != 0 {
		t.Error("Abs 가 잘못됨")
	}
}

// 고친 벤치마크가 실제로 결과를 전역 변수에 남기는지 검증합니다.
// testing.Benchmark 로 짧게 돌려 보고, sinkInt 가 Square(7) 값이 되었는지만 봅니다(시간은 보지 않음).
func TestBenchSinkStoresResult(t *testing.T) {
	sinkInt = 0
	r := testing.Benchmark(benchSink)
	if r.N < 1 {
		t.Fatalf("N = %d, want >= 1", r.N)
	}
	if sinkInt != 49 {
		t.Errorf("sinkInt = %d, want 49", sinkInt)
	}
}

// 벤치마크: go test -bench=. 를 줄 때만 실행됩니다. b.ResetTimer 앞의 준비는 측정에서 빠집니다.
func BenchmarkJoinNaive(b *testing.B) {
	parts := MakeParts(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = JoinNaive(parts)
	}
}

func BenchmarkJoinBuilder(b *testing.B) {
	parts := MakeParts(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = JoinBuilder(parts)
	}
}

// 하위 벤치마크: b.Run 으로 입력 크기별 결과를 한 번에 비교합니다.
func BenchmarkFib(b *testing.B) {
	for _, n := range []int{5, 10, 15} {
		b.Run(fmt.Sprint("n=", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				sinkInt = Fib(n)
			}
		})
	}
}

// Example 함수: 이름이 Example + 대상 이름이면 문서에 붙고,
// 아래 Output 주석과 실제 출력이 다르면 go test 가 실패시킵니다.
func ExampleFib() {
	fmt.Println(Fib(10))
	// Output: 55
}

func ExampleAbs() {
	fmt.Println(Abs(-3), Abs(3))
	// Output: 3 3
}

func ExampleJoinBuilder() {
	fmt.Println(JoinBuilder([]string{"go", "-", "lang"}))
	// Output: go-lang
}