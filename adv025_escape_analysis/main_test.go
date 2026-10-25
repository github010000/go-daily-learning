package main

import "testing"

// TestPointerEscapeIsAboutLifetime 은 "포인터를 만들었는가"가 아니라
// "그 포인터가 함수를 벗어났는가"가 힙행을 가른다는 것을 검증한다.
// 이 둘을 헷갈리면 &x 를 볼 때마다 힙이라고 오진하게 된다.
func TestPointerEscapeIsAboutLifetime(t *testing.T) {
	if got := localPointerUse(41); got != 42 {
		t.Fatalf("localPointerUse(41) = %d, 원하는 값 42", got)
	}
	p := returnedPointer(41)
	if p == nil || *p != 41 {
		t.Fatalf("returnedPointer(41) = %v, 원하는 값 41 을 가리키는 포인터", p)
	}

	const iters = 300
	onStack := measureAllocs(func() { sinkInt = localPointerUse(41) }, iters)
	onHeap := measureAllocs(func() { sinkPtr = returnedPointer(41) }, iters)
	t.Logf("localPointerUse=%.3f allocs/call, returnedPointer=%.3f allocs/call", onStack, onHeap)

	if onStack >= 0.5 {
		t.Errorf("주소를 잡기만 하고 안 나갔는데 할당이 %.3f 회/호출이다. 스택에 있어야 한다", onStack)
	}
	if onHeap < 0.9 {
		t.Errorf("포인터가 반환됐는데 할당이 %.3f 회/호출이다. 힙에 있어야 한다", onHeap)
	}
	if onStack >= onHeap {
		t.Errorf("스택 버전(%.3f)이 힙 버전(%.3f)보다 적게 할당해야 한다", onStack, onHeap)
	}
}

// TestInterfaceBoxingDependsOnValue 는 소스가 완전히 같은 두 함수라도
// 인터페이스에 담기는 값에 따라 할당 여부가 갈린다는 것을 검증한다.
// 256 미만 정수는 runtime.staticuint64s 를 재사용하므로 할당이 없다.
func TestInterfaceBoxingDependsOnValue(t *testing.T) {
	if v := boxSmall(42).(int); v != 42 {
		t.Fatalf("boxSmall(42) = %d, 원하는 값 42", v)
	}
	if v := boxLarge(1 << 20).(int); v != 1<<20 {
		t.Fatalf("boxLarge(1<<20) = %d", v)
	}

	const iters = 300
	small := measureAllocs(func() { sinkAny = boxSmall(42) }, iters)
	large := measureAllocs(func() { sinkAny = boxLarge(1 << 20) }, iters)
	t.Logf("boxSmall=%.3f allocs/call, boxLarge=%.3f allocs/call", small, large)

	if small >= 0.5 {
		t.Errorf("256 미만 정수는 staticuint64s 재사용이라 할당이 없어야 하는데 %.3f 회/호출", small)
	}
	if large < 0.9 {
		t.Errorf("큰 정수를 any 로 바꾸면 상자가 필요하므로 1 회/호출이어야 하는데 %.3f", large)
	}
}

// TestSliceGrowthAllocationCount 는 append 로만 자란 슬라이스와
// cap 을 미리 잡은 슬라이스의 할당 횟수를 비교한다.
// 원소 64 개면 cap 이 1,2,4,...,64 로 7번 자라므로 7 대 1 이 나와야 한다.
func TestSliceGrowthAllocationCount(t *testing.T) {
	s := growPrealloc(64)
	if len(s) != 64 || s[0] != 0 || s[63] != 63 {
		t.Fatalf("growPrealloc(64) 결과가 이상하다: len=%d", len(s))
	}
	if n := growNaive(64); len(n) != 64 || n[63] != 63 {
		t.Fatalf("growNaive(64) 결과가 이상하다: len=%d", len(n))
	}

	const iters = 200
	naive := measureAllocs(func() { sinkSlice = growNaive(64) }, iters)
	pre := measureAllocs(func() { sinkSlice = growPrealloc(64) }, iters)
	t.Logf("growNaive=%.3f allocs/call, growPrealloc=%.3f allocs/call", naive, pre)

	if pre >= 1.5 {
		t.Errorf("cap 을 미리 잡았으면 할당이 1 회여야 하는데 %.3f 회/호출", pre)
	}
	if naive <= pre {
		t.Errorf("cap 없이 append 하면 더 많이 할당해야 하는데 naive=%.3f pre=%.3f", naive, pre)
	}
}

// TestClosureCaptureEscape 는 클로저 자체가 아니라
// "캡처한 변수가 클로저보다 오래 사는가"가 힙행을 결정한다는 것을 검증한다.
func TestClosureCaptureEscape(t *testing.T) {
	if got := closureLocal(41); got != 42 {
		t.Fatalf("closureLocal(41) = %d, 원하는 값 42", got)
	}
	f := closureEscapes(41)
	if f == nil || f() != 42 {
		t.Fatalf("closureEscapes(41)() 가 42 가 아니다")
	}

	const iters = 300
	local := measureAllocs(func() { sinkInt = closureLocal(41) }, iters)
	esc := measureAllocs(func() { sinkFn = closureEscapes(41) }, iters)
	t.Logf("closureLocal=%.3f allocs/call, closureEscapes=%.3f allocs/call", local, esc)

	if local >= 0.5 {
		t.Errorf("클로저가 안 나갔는데 할당이 %.3f 회/호출이다. funcval 은 스택에 있어야 한다", local)
	}
	if esc <= local {
		t.Errorf("클로저가 반환되면 캡처 변수가 힙으로 가야 하는데 esc=%.3f local=%.3f", esc, local)
	}
}

// TestCollectRowsIsConsistent 는 main 이 출력하는 표의 재료가
// 비어 있지 않고, 이름이 중복되지 않으며, 값이 음수가 아님을 확인한다.
// 표에 행이 조용히 빠지면 데모가 절반만 보이는데도 아무도 모르기 때문이다.
func TestCollectRowsIsConsistent(t *testing.T) {
	rows := collectRows()
	if len(rows) < 9 {
		t.Fatalf("행이 %d 개뿐이다. 데모 항목이 빠졌다", len(rows))
	}
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.name == "" {
			t.Errorf("이름이 빈 행이 있다")
		}
		if seen[r.name] {
			t.Errorf("중복된 행 이름: %s", r.name)
		}
		seen[r.name] = true
		if r.alloc < 0 {
			t.Errorf("%s 의 할당 수가 음수다: %.3f", r.name, r.alloc)
		}
	}
	for _, want := range []string{"localPointerUse", "returnedPointer", "boxSmall(42)",
		"boxLarge(1<<20)", "growNaive(64)", "growPrealloc(64)", "closureEscapes"} {
		if !seen[want] {
			t.Errorf("필수 행이 빠졌다: %s", want)
		}
	}
}

func BenchmarkReturnedPointer(b *testing.B) {
	b.ReportAllocs()
	var p *int
	for i := 0; i < b.N; i++ {
		p = returnedPointer(i)
	}
	sinkPtr = p
}

func BenchmarkGrowNaive62(b *testing.B) {
	// 준비 단계는 측정 대상이 아니므로 타이머를 리셋하고 시작한다.
	// 여기서 sinkSlice 를 채우는 비용이 ns/op 에 섞이면 비교가 망가진다.
	sinkSlice = growPrealloc(64)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkSlice = growNaive(64)
	}
}

func BenchmarkGrowPrealloc64(b *testing.B) {
	sinkSlice = growNaive(64)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkSlice = growPrealloc(64)
	}
}