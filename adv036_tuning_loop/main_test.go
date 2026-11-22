package main

import (
	"testing"
)

// 모든 변형이 baseline 과 같은 결과를 내는지 검증한다.
// 최적화가 결과를 바꾸면 빠른 것이 아니라 틀린 것이므로, 속도 비교 이전의 전제 조건이다.
func TestVariantsAgree(t *testing.T) {
	in := genInput(50, 6)
	in = append(in, "", "a;;b=c", "x=y=z;k=v", "=v;k=")
	want := aggBaseline(in)
	for name, f := range map[string]func([]string) map[string]int{
		"presize": aggPresize, "fixed": aggFixed, "scratch": aggScratch,
	} {
		if got := f(in); !sameResult(want, got) {
			t.Errorf("%s 의 결과가 baseline 과 다름: %v vs %v", name, got, want)
		}
	}
}

// 추측으로 한 수정(map 미리 키우기)이 할당 횟수를 거의 줄이지 못함을 검증한다.
// 시간이 아니라 allocs/op 를 쓰는 이유는 이 값이 결정적이라 CI 에서 흔들리지 않기 때문이다.
func TestGuessDoesNotHelpAllocs(t *testing.T) {
	in := genInput(500, 6)
	base := testing.AllocsPerRun(5, func() { sinkMap = aggBaseline(in) })
	guess := testing.AllocsPerRun(5, func() { sinkMap = aggPresize(in) })
	if guess < base*0.9 {
		t.Errorf("가설이 맞았다면 allocs 가 크게 줄었어야 하는데 기대와 다름: base=%.0f guess=%.0f", base, guess)
	}
}

// 프로파일이 가리킨 곳(Split, Sprintf)을 고치면 할당이 크게 줄어듦을 검증한다.
// 반증 가능한 가설이 실제로 확증된 경우의 기준선이다.
func TestProfileDrivenFixCutsAllocs(t *testing.T) {
	in := genInput(500, 6)
	base := testing.AllocsPerRun(5, func() { sinkMap = aggBaseline(in) })
	fixed := testing.AllocsPerRun(5, func() { sinkMap = aggFixed(in) })
	if fixed*5 > base {
		t.Errorf("allocs 가 5분의 1 이하로 줄지 않음: base=%.0f fixed=%.0f", base, fixed)
	}
}

// 과최적화(고정 크기 선할당)가 작은 입력에서 메모리를 크게 낭비함을 검증한다.
// 큰 입력만 보고 채택하면 놓치는 회귀가 작은 워크로드에 숨어 있다는 점을 확인한다.
func TestOverOptimizationHurtsSmallInput(t *testing.T) {
	small := genInput(4, 6)
	base := bytesPerCall(aggBaseline, small, 20)
	pre := bytesPerCall(aggPresize, small, 20)
	fixed := bytesPerCall(aggFixed, small, 20)
	scratch := bytesPerCall(aggScratch, small, 20)
	if pre < 2*base {
		t.Errorf("presize 가 작은 입력에서 baseline 의 2배 이상 할당해야 함: base=%d pre=%d", base, pre)
	}
	if scratch < 10*fixed {
		t.Errorf("scratch 가 fixed 의 10배 이상 할당해야 함: fixed=%d scratch=%d", fixed, scratch)
	}
}

// Decide 의 판정 규칙을 합성 숫자로 검증한다.
// 실제 시간을 재지 않으므로 부하와 무관하게 항상 같은 결과가 나온다.
func TestDecideRules(t *testing.T) {
	base := []Measure{
		{"small", 1000, 1000, 10},
		{"large", 100000, 50000, 1000},
	}
	scale := func(ns, by, al float64) []Measure {
		out := make([]Measure, len(base))
		for i, b := range base {
			out[i] = Measure{b.Workload, b.NsPerOp * ns, int64(float64(b.BytesPerOp) * by), int64(float64(b.AllocsPerOp) * al)}
		}
		return out
	}
	cases := []struct {
		name string
		cand []Measure
		keep bool
	}{
		{"전반적 개선", scale(0.5, 0.5, 0.1), true},
		{"변화 없음", scale(1.0, 1.0, 1.0), false},
		{"노이즈 안의 ns 증가와 확실한 할당 감소", scale(1.05, 0.5, 0.5), true},
		{"한 지표만 크게 악화", scale(0.5, 10, 0.5), false},
	}
	for _, c := range cases {
		v := Decide(base, c.cand, 0.10)
		if v.Keep != c.keep {
			t.Errorf("%s: Keep=%v 기대=%v 이유=%v", c.name, v.Keep, c.keep, v.Reasons)
		}
		if !v.Keep && len(v.Reasons) == 0 {
			t.Errorf("%s: 되돌리는데 이유가 비어 있음", c.name)
		}
	}
}

// 입력 생성기가 결정적인지 검증한다. 입력이 매번 달라지면 어떤 벤치마크 비교도 성립하지 않는다.
func TestGenInputDeterministic(t *testing.T) {
	a, b := genInput(30, 6), genInput(30, 6)
	if len(a) != 30 {
		t.Fatalf("줄 수가 다름: %d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("%d 번째 줄이 실행마다 다름", i)
		}
	}
}

// 대표 벤치마크: 입력 생성은 타이머 밖에 두기 위해 ResetTimer 를 쓴다.
func benchVariant(b *testing.B, f func([]string) map[string]int, lines int) {
	in := genInput(lines, 6)
	b.ReportAllocs()
	b.ResetTimer() // 입력 준비 시간이 결과에 섞이지 않게 한다
	for i := 0; i < b.N; i++ {
		sinkMap = f(in)
	}
}

func BenchmarkBaselineLarge(b *testing.B) { benchVariant(b, aggBaseline, 2000) }
func BenchmarkPresizeLarge(b *testing.B)  { benchVariant(b, aggPresize, 2000) }
func BenchmarkFixedLarge(b *testing.B)    { benchVariant(b, aggFixed, 2000) }
func BenchmarkBaselineSmall(b *testing.B) { benchVariant(b, aggBaseline, 4) }
func BenchmarkScratchSmall(b *testing.B)  { benchVariant(b, aggScratch, 4) }
func BenchmarkFixedSmall(b *testing.B)    { benchVariant(b, aggFixed, 4) }