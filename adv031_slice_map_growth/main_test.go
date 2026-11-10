package main

import "testing"

// TestGrowModelMatchesRuntimeFormula 는 런타임 성장식을 옮긴 모델을 알려진 값에
// 고정한다. 이 함수가 틀리면 이 파일과 main.go 가 뽑는 모든 수치(복사량, 낭비율,
// 배율)가 무의미해지므로 가장 먼저 검증한다. 경계값 256 과 1024 를 반드시 포함한다.
func TestGrowModelMatchesRuntimeFormula(t *testing.T) {
	cases := []struct {
		oldCap, newLen, want int
	}{
		{0, 1, 1},        // 빈 슬라이스의 첫 성장
		{1, 2, 2},        // 임계값 미만은 정확히 2배
		{4, 5, 8},        //
		{255, 256, 510},  // 255 < 256 이므로 아직 2배
		{256, 257, 512},  // 경계: 256 은 2배 구간에 속한다
		{512, 513, 832},  // 512 + (512+768)/4 = 832
		{832, 833, 1232}, //
		{1232, 1233, 1732},
		{1000, 1001, 1442}, // 1000 + (1000+768)/4
		{10, 100, 100},     // 2배로도 부족하면 요청 길이 그대로
	}
	for _, c := range cases {
		if got := growModel(c.oldCap, c.newLen); got != c.want {
			t.Errorf("growModel(%d, %d) = %d, want %d", c.oldCap, c.newLen, got, c.want)
		}
	}

	// 1.17 식은 임계값이 1024 라는 점이 핵심이다. 같은 입력에서 결과가 다르다.
	old := []struct {
		oldCap, newLen, want int
	}{
		{512, 513, 1024},   // 1024 미만은 2배
		{1000, 1001, 2000}, // 1000 < 1024 이므로 여전히 2배
		{1024, 1025, 1280}, // 여기서부터 1.25배: 절벽
		{4096, 4097, 5120},
	}
	for _, c := range old {
		if got := growModelPre118(c.oldCap, c.newLen); got != c.want {
			t.Errorf("growModelPre118(%d, %d) = %d, want %d", c.oldCap, c.newLen, got, c.want)
		}
	}

	// 두 식이 1024 근처에서 갈라진다는 것 자체를 못 박아 둔다.
	if growModel(1024, 1025) == growModelPre118(1024, 1025) {
		t.Errorf("1024 에서 두 식이 같다: 1.18 변경의 핵심이 사라진다")
	}
}

// TestGrowthSeriesInvariants 는 실제 append 가 만드는 cap 시리즈의 불변식을 본다.
// cap 의 정확한 값은 size class 테이블과 컴파일러의 스택 할당 판단에 의존하므로
// 절대 고정값으로 단정하지 않는다. 대신 "단조 증가", "요청 길이 담보",
// "재할당 횟수가 로그 스케일" 세 가지만 검증한다. 첫 cap 을 1 로 가정하면
// 컴파일러가 더 큰 초기 버퍼를 잡는 환경에서 즉시 깨진다.
func TestGrowthSeriesInvariants(t *testing.T) {
	const n = 4096
	caps := growthSeries(n)
	if len(caps) < 4 {
		t.Fatalf("cap 변화가 %d회뿐이다: %v", len(caps), caps)
	}
	if caps[0] < 1 {
		t.Errorf("첫 cap = %d, want >= 1", caps[0])
	}
	for i := 1; i < len(caps); i++ {
		if caps[i] <= caps[i-1] {
			t.Errorf("cap 이 단조 증가하지 않는다 (%d번째): %v", i, caps)
			break
		}
	}
	if last := caps[len(caps)-1]; last < n {
		t.Errorf("마지막 cap %d 가 n %d 보다 작다", last, n)
	}
	// 1.25배 성장이면 log_1.25(4096) ~= 37 회. 여유를 두고 80 으로 잡는다.
	// 이 상한이 깨지면 성장이 사실상 선형이라는 뜻이고, 그건 심각한 회귀다.
	if len(caps) > 80 {
		t.Errorf("재할당이 %d회면 로그 스케일이 아니다", len(caps))
	}
}

// TestActualSeriesNotSmallerThanModel 은 "size class 올림은 크기를 줄이지 않는다"는
// 성질을 실제 런타임 시리즈로 확인한다. 올림된 cap 이 다음 성장의 입력이 되므로
// 실제 시리즈는 모델 시리즈보다 항상 앞서 있어야 하고, 따라서 재할당 횟수는
// 모델보다 많을 수 없고 최종 cap 은 모델보다 작을 수 없다.
func TestActualSeriesNotSmallerThanModel(t *testing.T) {
	const n = 4096
	actual := growthSeries(n)
	model := modelSeries(n, growModel)
	if len(actual) == 0 || len(model) == 0 {
		t.Fatalf("빈 시리즈: actual=%v model=%v", actual, model)
	}
	if len(actual) > len(model) {
		t.Errorf("실제 재할당 %d회 > 모델 %d회: 올림이 성장을 늦췄다는 뜻이라 있을 수 없다",
			len(actual), len(model))
	}
	if a, m := actual[len(actual)-1], model[len(model)-1]; a < m {
		t.Errorf("최종 cap: 실제 %d < 모델 %d", a, m)
	}

	// 사전 할당한 슬라이스는 재할당이 아예 없어야 한다. 이게 3절의 주장이다.
	s := make([]int, 0, n)
	before := cap(s)
	for i := 0; i < n; i++ {
		s = append(s, i)
	}
	if cap(s) != before {
		t.Errorf("사전 할당했는데 cap 이 %d 에서 %d 로 바뀌었다", before, cap(s))
	}
	if len(s) != n {
		t.Errorf("len = %d, want %d", len(s), n)
	}
}

// TestTotalCopiedAndWaste 는 비용 계산 헬퍼를 정의대로 고정한다.
// totalCopied 는 "재할당마다 직전 cap 만큼 memmove 된다"는 모델이다.
func TestTotalCopiedAndWaste(t *testing.T) {
	if got := totalCopied([]int{1, 2, 4, 8}); got != 0+1+2+4 {
		t.Errorf("totalCopied = %d, want 7", got)
	}
	if got := totalCopied(nil); got != 0 {
		t.Errorf("totalCopied(nil) = %d, want 0", got)
	}
	if got := wastePct([]int{1, 2, 4}, 3); got < 33 || got > 34 {
		t.Errorf("wastePct = %v, want ~33.3", got)
	}
	if got := wastePct(nil, 0); got != 0 {
		t.Errorf("wastePct(빈, 0) = %v, want 0", got)
	}

	// 2배 성장은 1.25배 성장보다 복사를 적게 한다(같은 최종 cap 기준으로 약 1/4).
	// 반대로 메모리 낭비는 더 크다. 이 트레이드오프가 성장 계수를 고르는 기준이다.
	const n = 1 << 16
	dbl := modelSeries(n, doubleModel)
	newer := modelSeries(n, growModel)
	if totalCopied(dbl) >= totalCopied(newer) {
		t.Errorf("2배 성장의 누적 복사(%d)가 1.25배(%d)보다 적지 않다: 모델이 틀렸다",
			totalCopied(dbl), totalCopied(newer))
	}
	if avgOverhead(doubleModel, 1, 512) <= avgOverhead(growModel, 1, 512) {
		t.Errorf("2배 성장의 평균 메모리 오버헤드가 더 크지 않다: 모델이 틀렸다")
	}
}

// TestPreallocCutsAllocations 는 사전 할당이 실제 mallocgc 호출을 줄이는지 본다.
// 시간이 아니라 MemStats.Mallocs 카운터만 보므로 머신 부하에 흔들리지 않는다.
func TestPreallocCutsAllocations(t *testing.T) {
	const n = 1 << 13
	grow, growBytes := measureAllocs(func() { appendGrow(n) })
	pre, preBytes := measureAllocs(func() { appendPrealloc(n) })
	if grow == 0 || pre == 0 {
		t.Fatalf("측정 실패: grow=%d pre=%d", grow, pre)
	}
	if pre >= grow {
		t.Errorf("사전 할당이 할당 횟수를 줄이지 않았다: grow=%d pre=%d", grow, pre)
	}
	if pre > 8 {
		t.Errorf("사전 할당인데 할당이 %d회나 발생했다", pre)
	}
	if preBytes >= growBytes {
		t.Errorf("사전 할당이 누적 바이트를 줄이지 않았다: grow=%d pre=%d", growBytes, preBytes)
	}
}

// TestMapPreallocCutsAllocations 는 map 도 같은 원리임을 확인한다.
// 버킷 배열은 B 가 커질 때마다 새로 잡히고, hint 를 주면 그 배열을 한 번에 잡는다.
// 주의: map 은 원소를 모두 지워도 버킷 배열을 반납하지 않는다는 것도 같은 이유다.
func TestMapPreallocCutsAllocations(t *testing.T) {
	const n = 1 << 13
	grow, growBytes := measureAllocs(func() { mapGrow(n) })
	pre, preBytes := measureAllocs(func() { mapPrealloc(n) })
	if grow == 0 || pre == 0 {
		t.Fatalf("측정 실패: grow=%d pre=%d", grow, pre)
	}
	if pre >= grow {
		t.Errorf("map 사전 할당이 할당 횟수를 줄이지 않았다: grow=%d pre=%d", grow, pre)
	}
	if preBytes >= growBytes {
		t.Errorf("map 사전 할당이 누적 바이트를 줄이지 않았다: grow=%d pre=%d", growBytes, preBytes)
	}

	// 삽입한 개수가 전부 살아 있어야 한다. 사전 할당이 값을 덮어쓰면 여기서 걸린다.
	m := make(map[int]int, n)
	for i := 0; i < n; i++ {
		m[i] = i * 2
	}
	if len(m) != n {
		t.Fatalf("len = %d, want %d", len(m), n)
	}
	if v, ok := m[n-1]; !ok || v != (n-1)*2 {
		t.Errorf("m[%d] = %d, %v", n-1, v, ok)
	}
}

// BenchmarkAppendGrow 는 재할당을 허용한 append 의 비용을 잰다.
// b.ReportAllocs 는 -benchmem 없이도 allocs/op 를 보여 준다.
func BenchmarkAppendGrow(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		appendGrow(1 << 12)
	}
}

// BenchmarkAppendPrealloc 은 같은 작업을 사전 할당으로 처리한 비용이다.
// 두 벤치마크의 allocs/op 차이가 이 주제의 전부다.
func BenchmarkAppendPrealloc(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		appendPrealloc(1 << 12)
	}
}

// BenchmarkMapGrow 는 재해시를 허용한 map 삽입 비용을 잰다.
func BenchmarkMapGrow(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapGrow(1 << 12)
	}
}

// BenchmarkMapPrealloc 는 같은 작업을 hint 로 처리한 비용이다.
func BenchmarkMapPrealloc(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapPrealloc(1 << 12)
	}
}