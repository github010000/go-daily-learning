package main

import (
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// TestSummarizePauses 는 STW 집계가 순수 함수라는 점을 이용해 결정적으로 검증한다.
// 실제 GC 를 돌려 "정확히 몇 마이크로초"를 단정할 수는 없지만, 이 집계가 틀리면
// 이 자료의 모든 수치 해석이 같이 틀어지므로 여기서 잡아 둔다.
func TestSummarizePauses(t *testing.T) {
	in := []time.Duration{
		50 * time.Microsecond, 10 * time.Microsecond, 30 * time.Microsecond,
		20 * time.Microsecond, 40 * time.Microsecond,
	}
	got := summarizePauses(in)
	if got.Count != 5 {
		t.Fatalf("Count = %d, 원하는 값 5", got.Count)
	}
	if want := 150 * time.Microsecond; got.Total != want {
		t.Fatalf("Total = %v, 원하는 값 %v", got.Total, want)
	}
	if got.Max != 50*time.Microsecond {
		t.Fatalf("Max = %v, 원하는 값 50us", got.Max)
	}
	if got.P50 != 30*time.Microsecond {
		t.Fatalf("P50 = %v, 원하는 값 30us", got.P50)
	}
	if got.P99 != 50*time.Microsecond {
		t.Fatalf("P99 = %v, 원하는 값 50us", got.P99)
	}
	// 입력 슬라이스는 "최신이 앞" 이라는 의미를 유지해야 한다. 여기서 제자리 정렬을 하면
	// 다음 ReadGCStats 가 같은 버퍼를 재사용할 때 순서가 뒤집혀 조용히 데이터가 오염된다.
	if in[0] != 50*time.Microsecond || in[1] != 10*time.Microsecond {
		t.Fatalf("summarizePauses 가 입력 슬라이스를 정렬해 버렸다: %v", in)
	}
	if empty := summarizePauses(nil); empty.Count != 0 || empty.Total != 0 || empty.Max != 0 {
		t.Fatalf("빈 입력 처리 오류: %+v", empty)
	}
}

// TestForcedGCAddsShortSTWPauses 는 강제 GC 한 번이 STW 기록을 남기는지,
// 그리고 그 기록이 누적 통계와 모순되지 않는지 본다.
// "사이클당 정확히 2회" 로 단정하지 않는 이유: 런타임은 정지 지점을 계속 합치고 줄여 왔고
// (예: sweep termination 을 다른 정지에 흡수), 개수를 고정하면 업그레이드마다 테스트가 깨진다.
// 그래서 순서에 의존하지 않는 불변식(개수 하한, 합계 상한)만 검사한다.
func TestForcedGCAddsShortSTWPauses(t *testing.T) {
	prev := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(prev)

	base := readGCStats()
	const cycles = 3
	for i := 0; i < cycles; i++ {
		runtime.GC()
	}
	after := readGCStats()

	if d := after.NumGC - base.NumGC; d != cycles {
		t.Fatalf("NumGC 증가분 = %d, 원하는 값 %d", d, cycles)
	}
	if after.PauseTotal <= base.PauseTotal {
		t.Fatalf("강제 GC %d회 뒤에도 PauseTotal 이 늘지 않았다: %v -> %v",
			cycles, base.PauseTotal, after.PauseTotal)
	}

	var nonzero int
	var sum, worst time.Duration
	for _, d := range after.Pause {
		if d > 0 {
			nonzero++
			sum += d
			if d > worst {
				worst = d
			}
		}
	}
	if nonzero < cycles {
		t.Fatalf("0보다 큰 일시정지 기록 %d개: 사이클 %d개면 최소 %d개는 있어야 한다",
			nonzero, cycles, cycles)
	}
	if sum > after.PauseTotal {
		t.Fatalf("링 버퍼 합계 %v 가 누적 PauseTotal %v 보다 크다", sum, after.PauseTotal)
	}
	// 절대값 단정은 CI 부하에서 깨지므로 상한만 둔다. 1초짜리 STW 는 2015년 이전 이야기다.
	if worst > time.Second {
		t.Fatalf("일시정지 최대값 %v: 동시 마킹 시대의 STW 로는 비정상", worst)
	}
}

// TestProbeShape 는 지연 측정 도구 자체를 검증한다.
// 지연 절대값은 CI 부하에 따라 수십 배 흔들리므로 단정하지 않고,
// "호출 횟수만큼 표본이 모였고 분위수 순서가 맞다"는 형태만 본다.
func TestProbeShape(t *testing.T) {
	const iters = 200
	st := probe(iters, 512)
	if st.Count != iters {
		t.Fatalf("표본 수 = %d, 원하는 값 %d", st.Count, iters)
	}
	if st.Max <= 0 {
		t.Fatalf("최대 지연 = %v", st.Max)
	}
	if !(st.Min <= st.P50 && st.P50 <= st.P99 && st.P99 <= st.Max) {
		t.Fatalf("분위수 순서가 깨졌다: min=%v p50=%v p99=%v max=%v",
			st.Min, st.P50, st.P99, st.Max)
	}
}

// TestChurnAllocatesRequestedBytes 는 부하 생성기가 실제로 요청한 만큼 힙을
// 건드리는지 본다. 이게 0 이면 위의 모든 측정이 빈 실행이 되고 만다.
func TestChurnAllocatesRequestedBytes(t *testing.T) {
	const rounds, size = 100, 256
	if got, want := churn(rounds, size), int64(rounds*size); got != want {
		t.Fatalf("churn 반환값 = %d, 원하는 값 %d", got, want)
	}
	if len(allocSink) == 0 {
		t.Fatal("allocSink 가 비었다: 할당이 살아남지 않으면 GC 가 볼 쓰레기가 없다")
	}
	// 크기가 아주 작아도 keep 계산이 0 이 되어 빈 조각을 잡으면 안 된다.
	churn(4, 1)
	for i, s := range allocSink {
		if len(s) == 0 {
			t.Fatalf("allocSink[%d] 길이 0: keep 하한 계산이 깨졌다", i)
		}
	}
}

// TestGCCPUMetricInvariants 는 /cpu/classes/gc/* 지표를 읽어 상위 항목과
// 하위 항목의 관계가 깨지지 않는지 확인한다. 이 관계가 깨지면 시연 2 의
// 백분율 해석이 통째로 무의미해진다.
func TestGCCPUMetricInvariants(t *testing.T) {
	before := readGCCPU()
	if !before.ok {
		t.Skip("이 런타임이 /cpu/classes/gc/* 지표를 제공하지 않는다")
	}
	if before.assist < 0 || before.dedicated < 0 || before.pause < 0 || before.total < 0 {
		t.Fatalf("음수 CPU 시간: %+v", before)
	}

	prev := debug.SetGCPercent(-1)
	holdLive(2 << 20)
	churn(500, 16*1024)
	runtime.GC()
	runtime.GC()
	debug.SetGCPercent(prev)

	after := readGCCPU()
	if !after.ok {
		t.Fatalf("두 번째 읽기에서 지표가 사라졌다")
	}
	// 강제 GC 는 반드시 CPU 를 쓴다. total 은 pause + assist + dedicated + idle + sweep 의 합이다.
	if after.total <= before.total {
		t.Fatalf("강제 GC 뒤에도 GC 총 CPU 가 늘지 않았다: %v -> %v", before.total, after.total)
	}
	if after.pause > after.total+1e-6 {
		t.Fatalf("pause(%v) 가 total(%v) 보다 크다: 상위/하위 지표 관계가 깨졌다",
			after.pause, after.total)
	}
}

// BenchmarkSummarizePauses 는 STW 통계 집계 비용을 잰다.
// 256개를 매번 정렬하므로, 이 집계를 요청 경로에 두면 안 된다는 근거가 된다.
func BenchmarkSummarizePauses(b *testing.B) {
	ps := make([]time.Duration, 256)
	for i := range ps {
		ps[i] = time.Duration(256-i) * time.Microsecond
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = summarizePauses(ps)
	}
}