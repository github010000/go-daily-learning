package main

import (
	"math"
	"runtime"
	"runtime/debug"
	"testing"
)

// 목표 힙이 "살아 있는 힙에 비례"한다는 것이 GOGC 의 정의다. 이 비례식이 깨지면
// 컨테이너 한도 계산이 전부 틀어지므로 식 자체를 고정해 둔다.
func TestGcPercentHeapGoalIsProportional(t *testing.T) {
	const live = 100 * mib
	cases := []struct {
		gcPercent int
		want      uint64
	}{
		{100, 2 * live},
		{200, 3 * live},
		{400, 5 * live},
		{50, live + live/2},
	}
	for _, c := range cases {
		if got := gcPercentHeapGoal(live, c.gcPercent); got != c.want {
			t.Errorf("gcPercentHeapGoal(%d, %d) = %d, want %d", uint64(live), c.gcPercent, got, c.want)
		}
	}
	if got := gcPercentHeapGoal(live, -1); got != math.MaxUint64 {
		t.Errorf("GOGC=off 는 상한이 없어야 한다: got %d", got)
	}
	if got := gcPercentHeapGoal(mib, 100); got != 4*mib {
		t.Errorf("아주 작은 힙에서는 최소 목표 4MiB 가 적용되어야 한다: got %d", got)
	}
}

// GOGC 목표와 GOMEMLIMIT 목표 중 작은 쪽이 실제 목표가 된다. 한도가 살아 있는
// 힙보다 작으면 목표는 한도가 아니라 "지금 살아 있는 힙"으로 올라간다.
// 이것이 soft limit 의 정의이므로 회귀하면 안 되는 성질이다.
func TestEffectiveHeapGoalPicksSmallerSide(t *testing.T) {
	const live = 64 * mib
	if got, want := effectiveHeapGoal(live, 100, noLimit), uint64(2*live); got != want {
		t.Errorf("한도가 없으면 GOGC 목표 그대로여야 한다: got %d want %d", got, want)
	}
	got := effectiveHeapGoal(live, 100, 96*mib)
	if got > uint64(96*mib) {
		t.Errorf("목표가 한도를 넘었다: %d", got)
	}
	if got >= 2*live {
		t.Errorf("한도가 GOGC 목표보다 작은데 목표가 줄지 않았다: %d", got)
	}
	if got := effectiveHeapGoal(2048*mib, 100, 512*mib); got < uint64(2048*mib) {
		t.Errorf("살아 있는 힙이 한도보다 크면 목표는 live 이상이어야 한다: got %d", got)
	}
}

// 컨테이너 한도에서 Go 몫을 계산할 때 여유분을 반드시 떼야 한다. GOMEMLIMIT 은
// Go 가 추적하는 메모리만 세기 때문이다.
func TestRecommendMemoryLimit(t *testing.T) {
	const container = int64(2 << 30) // 2GiB
	if got, want := recommendMemoryLimit(container, 256*mib), container-256*mib; got != want {
		t.Errorf("got %d want %d", got, want)
	}
	if got := recommendMemoryLimit(container, 0); got != container {
		t.Errorf("여유분이 0 이면 한도 그대로여야 한다: got %d", got)
	}
	if got := recommendMemoryLimit(container, 4<<30); got <= 0 || got > container {
		t.Errorf("여유분이 한도보다 크면 최소값을 돌려야 한다: got %d", got)
	}
	if got := recommendMemoryLimit(0, 0); got != noLimit {
		t.Errorf("한도가 없으면 무제한이어야 한다: got %d", got)
	}
}

// GOGC 를 끄면 GC 를 부르는 것은 GOMEMLIMIT 뿐이다. 같은 양의 쓰레기를 만들었을
// 때 한도가 있는 쪽의 최대 힙이 확실히 작아야 한다. 이것이 "비례 제어에는 상한이
// 없다"는 빈틈을 GOMEMLIMIT 이 메우는 지점이다.
func TestMemoryLimitCapsHeapWhenGcPercentOff(t *testing.T) {
	unlimited := runPhase("off-no-limit", -1, noLimit, 8, 128)
	limited := runPhase("off-limit-32MiB", -1, 32*mib, 8, 128)

	if unlimited.NumGC != 0 {
		t.Fatalf("GOGC=off + 무제한이면 GC 가 돌면 안 된다: %d회", unlimited.NumGC)
	}
	if limited.NumGC == 0 {
		t.Fatalf("한도가 있는데 GC 가 한 번도 안 돌았다")
	}
	if limited.PeakHeapAlloc >= unlimited.PeakHeapAlloc {
		t.Fatalf("한도가 있는 쪽 peak 가 더 크다: limited=%d unlimited=%d",
			limited.PeakHeapAlloc, unlimited.PeakHeapAlloc)
	}
	if limited.PeakHeapAlloc > unlimited.PeakHeapAlloc/2 {
		t.Errorf("한도 효과가太小: limited=%d unlimited=%d",
			limited.PeakHeapAlloc, unlimited.PeakHeapAlloc)
	}
}

// 메모리 한도가 살아 있는 힙보다 작으면 한도는 지켜지지 않는다(soft). 대신 runtime
// 은 GC 가 CPU 를 독식하지 않도록 제한한다. 여기서는 "한도를 넘는다"는 사실만
// 검증한다. CPU 비율은 장비 의존이라 단정하지 않는다.
func TestMemoryLimitIsSoftWhenBelowLiveHeap(t *testing.T) {
	res := runPhase("limit-below-live", 100, 8*mib, 32, 64)
	if res.NumGC == 0 {
		t.Fatalf("한도를 넘겼는데 GC 가 한 번도 안 돌았다")
	}
	if res.PeakHeapAlloc <= uint64(8*mib) {
		t.Fatalf("soft limit 인데도 한도 아래로 눌렸다: peak=%d", res.PeakHeapAlloc)
	}
	if res.PeakHeapAlloc < res.LiveBytes {
		t.Errorf("peak 가 살아 있는 힙보다 작을 수 없다: peak=%d live=%d", res.PeakHeapAlloc, res.LiveBytes)
	}
}

// GOGC=100 은 목표를 live 힙의 2배로 잡는다. 실제 관측 peak 가 그 근처에 머무는지
// (몇 배씩 폭주하지 않는지) 확인한다.
func TestGcPercent100KeepsHeapNearTwiceLive(t *testing.T) {
	res := runPhase("gogc100", 100, noLimit, 16, 128)
	if res.NumGC == 0 {
		t.Fatalf("GOGC=100 인데 GC 가 안 돌았다")
	}
	if res.PeakHeapAlloc > 5*uint64(16*mib) {
		t.Errorf("peak 가 live 의 5배를 넘었다: peak=%d", res.PeakHeapAlloc)
	}
	if res.PeakGoal == 0 {
		t.Errorf("목표 힙 지표를 읽지 못했다")
	}
}

// 같은 쓰레기 양을 만들 때 GOGC 를 올리면 GC 를 덜 해서 처리량이 오르고, 대신 힙이
// 커진다. GOMEMLIMIT 은 그 힙의 상한을 되찾아 주는 대신 assist 로 처리량을 조금
// 토해 낸다. 그 맞교환을 수치로 본다.
func BenchmarkAllocThroughput(b *testing.B) {
	cases := []struct {
		name      string
		gcPercent int
		memLimit  int64
	}{
		{"GOGC=100", 100, noLimit},
		{"GOGC=400", 400, noLimit},
		{"GOGC=off+LIMIT=32MiB", -1, 32 * mib},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			prevPct := debug.SetGCPercent(c.gcPercent)
			prevLimit := debug.SetMemoryLimit(c.memLimit)
			live := allocLiveBlocks(8)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf := make([]byte, mib)
				buf[i%mib] = byte(i)
				sink = append(sink[:0], buf)
			}
			b.StopTimer()
			runtime.KeepAlive(live)
			debug.SetGCPercent(prevPct)
			debug.SetMemoryLimit(prevLimit)
		})
	}
}