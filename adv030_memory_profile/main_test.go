package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withRate 는 테스트마다 추출률을 정해 두고 반드시 원래 값으로 되돌린다.
// 추출률은 전역 상태라서, 되돌리지 않으면 뒤따르는 테스트의 수치가 오염된다.
func withRate(t *testing.T, rate int) {
	t.Helper()
	save := runtime.MemProfileRate
	runtime.MemProfileRate = rate
	t.Cleanup(func() { runtime.MemProfileRate = save })
}

// TestParseHeapHeader 는 프로파일 머리말 파싱 규칙을 고정한다.
// 이 파싱이 틀리면 inuse 와 alloc 을 뒤집어 읽게 되고, 그 순간
// "누수 없음"과 "누수 있음"이 정반대로 뒤집힌다.
func TestParseHeapHeader(t *testing.T) {
	cases := []struct {
		name string
		line string
		want HeapSnapshot
		ok   bool
	}{
		{
			name: "일반적인 줄",
			line: "heap profile: 3: 1234 [15: 5678] @ heap/524288",
			want: HeapSnapshot{InuseObjects: 3, InuseBytes: 1234, AllocObjects: 15, AllocBytes: 5678, Rate: 524288},
			ok:   true,
		},
		{
			name: "프로파일을 끈 상태",
			line: "heap profile: 0: 0 [0: 0] @ heap/0",
			want: HeapSnapshot{},
			ok:   true,
		},
		{
			name: "공백이 여러 개여도 읽는다",
			line: "heap profile:  7:  2156144  [9:  3145728]  @ heap/1048576",
			want: HeapSnapshot{InuseObjects: 7, InuseBytes: 2156144, AllocObjects: 9, AllocBytes: 3145728, Rate: 1048576},
			ok:   true,
		},
		{
			name: "스택 줄은 머리말이 아니다",
			line: "1: 4096 [1: 4096] @ 0x4a1b2c 0x4a1d3e",
			ok:   false,
		},
		{name: "빈 줄", line: "", ok: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseHeapHeader(c.line)
			if ok != c.ok {
				t.Fatalf("파싱 성공 여부가 다르다: got=%v want=%v (line=%q)", ok, c.ok, c.line)
			}
			if ok && got != c.want {
				t.Errorf("값이 다르다: got=%+v want=%+v", got, c.want)
			}
		})
	}
}

// TestScaleRawSample 은 pprof 가 원자료에 곱하는 역산 계수를 검증한다.
// 추출률이 1 이면 표본이 전부이므로 보정이 없어야 하고, 추출률이 객체
// 크기보다 크면 계수가 그만큼 커져야 한다. 이 관계가 깨지면 프로파일
// 숫자를 실제 할당량으로 환산할 수 없다.
func TestScaleRawSample(t *testing.T) {
	// 추출률 1: 표본이 곧 전체다. 보정하면 안 된다.
	o, b := scaleRawSample(100, 409600, 1)
	if o != 100 || b != 409600 {
		t.Errorf("추출률 1 에서 보정이 일어났다: %d, %d", o, b)
	}
	// 표본이 없으면 보정할 대상도 없다.
	if o, b := scaleRawSample(0, 0, 512<<10); o != 0 || b != 0 {
		t.Errorf("표본 0 개인데 값이 나왔다: %d, %d", o, b)
	}
	// 평균 크기와 추출률이 같으면 포함 확률이 1-e^-1 이므로 계수가 1 과 2 사이.
	o, b = scaleRawSample(100, 100*4096, 4096)
	if b <= 100*4096 || b >= 2*100*4096 {
		t.Errorf("계수가 예상 범위를 벗어났다: raw=%d scaled=%d", 100*4096, b)
	}
	if o <= 100 {
		t.Errorf("객체 수도 같이 늘어야 한다: %d", o)
	}
	// 추출률이 아주 크면 원자료 하나가 아주 많은 바이트를 대표한다.
	if _, b := scaleRawSample(1, 4096, 4096*100); b < 100*4096 {
		t.Errorf("큰 추출률에서 계수가 너무 작다: %d", b)
	}
}

// TestAllocVsInuse 는 "만들고 전부 버린" 상황에서 할당은 크게 늘지만
// 살아 있는 메모리는 늘지 않는다는 불변식을 확인한다. 이게 성립하지
// 않으면 alloc_space 를 보고 누수를 판정하는 혼동이 생긴다.
func TestAllocVsInuse(t *testing.T) {
	withRate(t, 512<<10)

	const n, size = 2000, 4096 // 8 MiB
	res := measureChurn(n, size)
	want := uint64(n * size)

	if res.ExactAllocBytes < want*8/10 {
		t.Fatalf("할당이 실제로 일어나지 않았다: exact=%d want>=%d", res.ExactAllocBytes, want*8/10)
	}
	// GC 뒤에는 버린 8 MiB 가 살아 있으면 안 된다.
	if res.HeapAllocAfter > res.HeapAllocBefore+(2<<20) {
		t.Errorf("버린 메모리가 힙에 남았다: before=%d after=%d",
			res.HeapAllocBefore, res.HeapAllocAfter)
	}
	// 프로파일의 inuse 증가분도 마찬가지로 작아야 한다.
	if res.ProfileInuseBytes > 2<<20 {
		t.Errorf("프로파일 inuse 가 줄지 않았다: %+d", res.ProfileInuseBytes)
	}
}

// TestLeakVsChurn 은 alloc 이 거의 같은 두 시나리오를 inuse 가 갈라내는지
// 확인한다. 누수를 찾을 때 alloc 이 아니라 inuse 를 봐야 하는 이유가
// 바로 이 불변식이다. 추출률을 촘촘히 잡아 표본 부족으로 판정이 흔들리지
// 않게 한다.
func TestLeakVsChurn(t *testing.T) {
	withRate(t, 16<<10)
	t.Cleanup(releaseLeak)

	const n, size = 2000, 4096 // 8 MiB
	res := leakChurnExperiment(n, size)
	want := int64(n * size)

	if got := int64(res.LeakHeapAlloc) - int64(res.ChurnHeapAlloc); got < want*8/10 {
		t.Fatalf("붙잡은 8 MiB 가 HeapAlloc 에 안 보인다: churn=%d leak=%d",
			res.ChurnHeapAlloc, res.LeakHeapAlloc)
	}
	// 참조를 놓으면 GC 가 회수하므로 다시 내려가야 한다.
	if int64(res.AfterHeapAlloc)-int64(res.ChurnHeapAlloc) > 2<<20 {
		t.Errorf("참조를 놓았는데 힙이 안 줄었다: churn=%d after=%d",
			res.ChurnHeapAlloc, res.AfterHeapAlloc)
	}
	// 프로파일 쪽에서도 inuse 증가분이 붙잡은 양을 따라가야 한다.
	if res.LeakProfileInuse-res.ChurnProfileInuse < 1<<20 {
		t.Errorf("프로파일 inuse 가 붙잡은 양을 못 따라간다: churn=%+d leak=%+d",
			res.ChurnProfileInuse, res.LeakProfileInuse)
	}
}

// TestSamplingAccuracyAtRateOne 은 추출률 1(모든 할당 기록)에서는
// 프로파일 원자료가 정확값과 사실상 같아진다는 것을 확인한다.
// 표본 오차가 정말 "표본 때문"인지 확인하는 대조군이다.
func TestSamplingAccuracyAtRateOne(t *testing.T) {
	res := measureAlloc(1, func() { allocChurn(2000, 4096) }) // 8 MiB
	want := uint64(2000 * 4096)

	if res.ExactAllocB < want*8/10 {
		t.Fatalf("할당이 실제로 일어나지 않았다: %+v", res)
	}
	if res.ProfileAllocB == 0 {
		t.Fatalf("전수 기록인데 표본이 0 이다: %+v", res)
	}
	ratio := ratioOf(res.ProfileAllocB, res.ExactAllocB)
	if ratio < 0.5 || ratio > 2.0 {
		t.Errorf("전수 기록인데 원자료가 어긋난다: ratio=%.3f (%+v)", ratio, res)
	}
}

// TestSamplingGranularity 는 추출 간격이 총 할당량보다 훨씬 크면
// 프로파일이 "조금 틀린" 값이 아니라 "0 이거나 크게 튄" 값이 된다는 것을
// 확인한다. 작은 표본을 보고 내린 결론이 왜 위험한지 보여주는 핵심이다.
// 보정 계수를 곱하는지 여부에 상관없이 정확값 근처에 못 온다는 점만 본다.
func TestSamplingGranularity(t *testing.T) {
	res := measureAlloc(1<<20, func() { allocChurn(64, 4096) }) // 256 KiB
	if res.ExactAllocB < 200<<10 {
		t.Fatalf("할당이 실제로 일어나지 않았다: %+v", res)
	}
	if res.ProfileAllocB == 0 {
		t.Logf("표본이 하나도 안 잡혔다(가장 흔한 결과): %+v", res)
		return
	}
	ratio := ratioOf(res.ProfileAllocB, res.ExactAllocB)
	if ratio >= 0.5 && ratio <= 2.0 {
		t.Errorf("1 MiB 간격으로 256 KiB 를 쟀는데 정확할 수 없다: ratio=%.3f (%+v)", ratio, res)
	}
}

// TestProfileDisabled 는 추출률 0 이 "오차가 큰 프로파일"이 아니라
// "아무것도 안 잡히는 프로파일"이라는 걸 확인한다. 이걸 모르면
// 프로파일을 끈 상태의 0 을 "누수 없음"으로 오독한다.
func TestProfileDisabled(t *testing.T) {
	res := measureAlloc(0, func() { allocChurn(500, 4096) }) // 2 MiB

	if res.ProfileAllocB != 0 || res.ProfileSamples != 0 {
		t.Errorf("프로파일을 껐는데 표본이 잡혔다: %+v", res)
	}
	if res.ExactAllocB < 1<<20 {
		t.Errorf("할당 자체는 일어났어야 한다: %+v", res)
	}
}

// TestWriteHeapProfile 은 파일 저장 경로가 실제로 읽을 수 있는 산출물을
// 남기는지, 그리고 같은 시점의 inuse 가 붙잡은 양을 반영하는지 확인한다.
func TestWriteHeapProfile(t *testing.T) {
	withRate(t, 1<<10)
	t.Cleanup(releaseLeak)

	releaseLeak()
	runtime.GC()
	before := profileTotals()

	leakGrow(256, 4096) // 1 MiB 붙잡기
	runtime.GC()
	after := profileTotals()

	if after.InuseBytes-before.InuseBytes < 512<<10 {
		t.Fatalf("붙잡은 1 MiB 가 inuse 에 충분히 안 보인다: before=%d after=%d",
			before.InuseBytes, after.InuseBytes)
	}

	path := filepath.Join(t.TempDir(), "mem.prof")
	if err := writeHeapProfile(path); err != nil {
		t.Fatalf("프로파일 저장 실패: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("저장한 파일을 찾을 수 없다: %v", err)
	}
	if st.Size() == 0 {
		t.Fatal("프로파일 파일이 비어 있다")
	}
}

// BenchmarkHeapProfileRead 는 힙 프로파일 한 장을 뜨는 비용을 잰다.
// 이 값이 "공짜 진단"이 아니라는 근거다. 한 번 뜰 때마다 GC 가 한 번
// 돌기 때문에, 상시 모니터링에 넣으면 지연 분포가 그대로 망가진다.
func BenchmarkHeapProfileRead(b *testing.B) {
	leakGrow(64, 4096)
	b.Cleanup(releaseLeak)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := readHeapText(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkProfileSnapshot 은 GC 를 강제하지 않는 쪽(runtime.MemProfile)
// 의 비용이다. 프로파일 파일을 만드는 것보다 싸지만, 역시 STW 를 유발한다.
func BenchmarkProfileSnapshot(b *testing.B) {
	leakGrow(64, 4096)
	b.Cleanup(releaseLeak)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if p := profileTotals(); p.InuseBytes < 0 {
			b.Fatal("inuse 가 음수일 수 없다")
		}
	}
}