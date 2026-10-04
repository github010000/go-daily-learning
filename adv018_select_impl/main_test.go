package main

import "testing"

// NonBlockingReceive가 default 경로를 타는지, 값이 있으면 실제 값을
// 반환하는지 검증합니다. 채널이 비어 있으면 블록하지 않고 즉시
// ok=false여야 하고, 값이 들어 있으면 그 값을 돌려줘야 합니다.
func TestNonBlockingReceive(t *testing.T) {
	ch := make(chan int, 1)

	if _, ok := NonBlockingReceive(ch); ok {
		t.Fatalf("empty channel should return ok=false")
	}

	ch <- 42
	v, ok := NonBlockingReceive(ch)
	if !ok {
		t.Fatalf("ready channel should return ok=true")
	}
	if v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
}

// SelectFairnessDemo의 반환값이 시행 횟수와 정확히 일치하는지 검증합니다.
// 선택 분포는 무작위이므로 분포 자체를 통계적으로 단정하지 않고,
// 합계와 길이 같은 결정적인 불변식만 확인합니다.
func TestSelectFairnessDemoCountsSum(t *testing.T) {
	trials := 10000
	counts := SelectFairnessDemo(trials)

	sum := 0
	for i, c := range counts {
		if c < 0 {
			t.Fatalf("case %d count %d is negative", i, c)
		}
		sum += c
	}
	if sum != trials {
		t.Fatalf("sum of counts = %d, want %d", sum, trials)
	}
	if len(counts) != demoChannels {
		t.Fatalf("len(counts) = %d, want %d", len(counts), demoChannels)
	}
}

// FixedOrderDemo는 잘못된 고정 순서가 나머지 채널을 굶기는 현상을
// 결정적으로 보여줍니다. 모든 채널이 항상 준비되어 있으므로 0번
// 케이스만 100번 선택되고 나머지는 0이어야 합니다.
func TestFixedOrderDemoStarvesOthers(t *testing.T) {
	counts := FixedOrderDemo(100)

	if counts[0] != 100 {
		t.Fatalf("fixed order should always pick case 0, got %d", counts[0])
	}
	for i := 1; i < len(counts); i++ {
		if counts[i] != 0 {
			t.Fatalf("case %d should starve, got %d", i, counts[i])
		}
	}
}

// BenchmarkNonBlockingReceive는 default가 포함된 단일 수신이 블로킹 없이
// 얼마나 빠른지 관찰하기 위한 벤치마크입니다. 빈 unbuffered 채널이므로
// 매번 default 경로가 실행됩니다.
func BenchmarkNonBlockingReceive(b *testing.B) {
	ch := make(chan int)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NonBlockingReceive(ch)
	}
}