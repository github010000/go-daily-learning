package main

import (
	"testing"
	"unsafe"
)

// TestSizeClassTable 은 런타임이 공개하는 클래스 표의 불변식만 검사한다.
//
// 표 자체를 하드코딩하지 않는 이유: class_to_size 는 mksizeclasses.go 가
// 만드는 산출물이라 툴체인 메이저 버전마다 다시 생성된다. 개수와 값을 못
// 박으면 Go 를 올릴 때마다 테스트가 깨진다. 대신 "단조 증가, 8의 배수,
// 가장 작은 클래스는 8, 상한은 maxSmallSize 이하, 공개된 칸 수는
// _NumSizeClasses 이하" 라는 구조적 사실만 검사한다.
func TestSizeClassTable(t *testing.T) {
	sizes := sizeClasses()
	if len(sizes) < 50 {
		t.Fatalf("공개된 클래스가 %d 개뿐: 창구가 비정상", len(sizes))
	}
	if len(sizes) > numSizeClasses {
		t.Fatalf("공개된 클래스가 %d 개: _NumSizeClasses(%d)보다 많다", len(sizes), numSizeClasses)
	}
	if sizes[0] != 8 {
		t.Errorf("가장 작은 클래스: got %d, want 8", sizes[0])
	}
	for i := 1; i < len(sizes); i++ {
		if sizes[i] <= sizes[i-1] {
			t.Fatalf("단조 증가 위반: [%d]=%d, [%d]=%d", i-1, sizes[i-1], i, sizes[i])
		}
		if sizes[i]%8 != 0 {
			t.Fatalf("클래스 %d 가 8의 배수가 아님", sizes[i])
		}
	}
	if last := sizes[len(sizes)-1]; last > maxSmallSize {
		t.Errorf("마지막 클래스 %d 가 small object 상한 %d 를 넘음", last, maxSmallSize)
	}
}

// TestClassFor 는 요청 크기가 어느 클래스로 올림되는지 검증한다.
// 1 -> 8, 9 -> 16, 33 -> 48 은 Go 1.5 이후 표가 바뀌어도 유지된 값이라
// 고정해도 안전하다. 나머지는 "표 범위를 넘으면 클래스가 없다" 는
// 불변식으로 확인한다.
func TestClassFor(t *testing.T) {
	sizes := sizeClasses()
	cases := []struct{ in, want uint32 }{
		{1, 8}, {8, 8}, {9, 16}, {16, 16}, {17, 24}, {33, 48}, {100, 112},
	}
	for _, c := range cases {
		got, ok := classFor(sizes, c.in)
		if !ok {
			t.Fatalf("%d: 클래스를 찾지 못함", c.in)
		}
		if got != c.want {
			t.Errorf("classFor(%d) = %d, want %d", c.in, got, c.want)
		}
	}
	last := sizes[len(sizes)-1]
	if _, ok := classFor(sizes, last+1); ok {
		t.Errorf("%d 는 공개 표 범위 밖인데 클래스가 나왔다", last+1)
	}
}

// TestFragmentationBound 는 올림 손실의 상한을 확인한다. 어떤 요청이든
// 클래스가 요청의 2배를 넘지 않아야 한다. 이 상한이 깨지면 size class 를
// 쓰는 의미 자체가 없어지므로 회귀 감시 가치가 있다. worstFragmentation 이
// 낭비 내림차순으로 정렬되는지도 함께 본다.
func TestFragmentationBound(t *testing.T) {
	sizes := sizeClasses()
	for r := sizes[0] + 1; r <= sizes[len(sizes)-1]; r++ {
		c, ok := classFor(sizes, r)
		if !ok {
			t.Fatalf("%d 에 해당하는 클래스가 없음", r)
		}
		if float64(c) >= 2*float64(r) {
			t.Fatalf("요청 %d -> 클래스 %d: 2배 이상 올림", r, c)
		}
	}
	rows := worstFragmentation(sizes, 4)
	if len(rows) != 4 {
		t.Fatalf("worst 4 를 기대: %d", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].waste() > rows[i-1].waste() {
			t.Errorf("낭비 순 정렬이 아님: [%d]=%.4f > [%d]=%.4f",
				i, rows[i].waste(), i-1, rows[i-1].waste())
		}
	}
	if rows[0].waste() <= 0 || rows[0].waste() >= 1 {
		t.Errorf("최악 낭비율 %.4f: 0 과 1 사이여야 함", rows[0].waste())
	}
}

// TestTinyAllocatorPacks 는 16바이트 미만 무포인터 객체가 정말 한 블록에
// 뭉치는지, 포인터가 붙으면 그 경로를 못 타는지 주소 간격으로 확인한다.
//
// 바이트 회계(TotalAlloc)가 아니라 주소를 보는 이유: 회계 방식은 런타임
// 내부 사정이라 버전에 따라 달라질 수 있지만, 이웃 주소가 1바이트 차이면
// "같은 16바이트 블록을 나눠 썼다" 는 사실이 직접 증명된다. 시간에 의존하지
// 않으므로 부하 상황에서도 결과가 같다.
func TestTinyAllocatorPacks(t *testing.T) {
	const n = 4096
	tinyGap, tinyFrac := modeGap(gaps(n, func() *byte { return new(byte) }))
	if tinyGap != 1 {
		t.Errorf("1B 객체의 최빈 간격 %d B: tiny 블록 공유가 일어나지 않음", tinyGap)
	}
	if tinyFrac < 0.5 {
		t.Errorf("간격 1바이트 비율 %.2f: 기대만큼 촘촘하지 않음", tinyFrac)
	}
	ptrGap, ptrFrac := modeGap(gaps(n, func() *ptrWord { return &ptrWord{} }))
	if ptrGap != 8 {
		t.Errorf("포인터 있는 8B 객체의 최빈 간격 %d B: 클래스 8 배정이 아님", ptrGap)
	}
	if ptrFrac < 0.9 {
		t.Errorf("간격 8 비율 %.4f: 8바이트 배치가 일정하지 않음", ptrFrac)
	}
}

// TestSizeClassRounding 은 33바이트 구조체 하나가 실제로 클래스 48 로
// 청구되는지 확인한다. 기대값을 상수 48 로 박지 않고 classFor 로 얻는 이유는
// 표가 바뀌면 이 테스트가 따라가게 하기 위해서다. 함께 보는 5000B -> 5376
// 대조로 "큰 객체는 상대 낭비가 작다" 는 성질도 검증한다.
func TestSizeClassRounding(t *testing.T) {
	const (
		n33 = 20000
		n5k = 2000
	)
	sizes := sizeClasses()

	want33, ok := classFor(sizes, uint32(unsafe.Sizeof(blob33{})))
	if !ok {
		t.Fatalf("blob33 크기가 공개 표 범위 밖")
	}
	got33 := chargeBlob33(n33)
	if got33 < float64(want33)*0.9 || got33 > float64(want33)*1.4 {
		t.Fatalf("33B 객체당 %.2f B: 클래스 %d 근처가 아님", got33, want33)
	}

	want5k, ok := classFor(sizes, uint32(unsafe.Sizeof(blob5000{})))
	if !ok {
		t.Fatalf("blob5000 크기가 공개 표 범위 밖")
	}
	got5k := chargeBlob5000(n5k)
	if got5k < float64(want5k)*0.9 || got5k > float64(want5k)*1.4 {
		t.Errorf("5000B 객체당 %.2f B: 클래스 %d 근처가 아님", got5k, want5k)
	}

	if got5k/5000 >= got33/33 {
		t.Errorf("큰 객체의 상대 낭비가 더 크다: %.3f vs %.3f", got5k/5000, got33/33)
	}
}

// TestFieldOrderChangesHeap 은 같은 필드를 순서만 바꿨을 때 객체당 청구
// 바이트가 줄어드는지 확인한다. 기대값을 상수로 박지 않고 unsafe.Sizeof 로
// 얻은 크기의 클래스와 비교하므로 32비트/64비트 어느 쪽에서도 성립한다.
func TestFieldOrderChangesHeap(t *testing.T) {
	const n = 20000
	sizes := sizeClasses()

	bad := chargeBad(n)
	good := chargeGood(n)

	szBad := uintptr(unsafe.Sizeof(badLayout{}))
	szGood := uintptr(unsafe.Sizeof(goodLayout{}))
	if szBad <= szGood {
		t.Fatalf("필드 순서를 바꿨는데 Go 수준 크기가 줄지 않음: %d, %d", szBad, szGood)
	}

	cBad, ok := classFor(sizes, uint32(szBad))
	if !ok {
		t.Fatalf("badLayout 크기가 공개 표 범위 밖")
	}
	if bad < float64(cBad)*0.9 || bad > float64(cBad)*1.4 {
		t.Errorf("badLayout 객체당 %.2f B: 클래스 %d 근처가 아님", bad, cBad)
	}
	if bad <= good {
		t.Errorf("힙 청구가 줄지 않음: bad=%.2f good=%.2f", bad, good)
	}
}

// TestParallelAllocationsUnique 는 여러 goroutine 이 동시에 할당해도 주소가
// 겹치지 않는지 확인한다. 공유 상태는 goroutine 별 슬라이스뿐이라
// -race 로 돌려도 통과해야 한다.
func TestParallelAllocationsUnique(t *testing.T) {
	const (
		nG  = 4
		per = 1000
	)
	parts := make([][]*blob33, nG)
	for g := range parts {
		parts[g] = make([]*blob33, per)
	}
	parallelFill(parts)
	if got := countUnique(parts); got != nG*per {
		t.Errorf("서로 다른 주소 %d 개: 기대 %d (중복 할당 의심)", got, nG*per)
	}
}

var (
	benchByteSink *byte
	benchBlobSink *blob33
)

// BenchmarkTinyAlloc 은 1바이트 무포인터 객체를 만드는 비용을 잰다.
// 살아 있는 객체를 하나만 두어 힙이 무한정 자라지 않게 했다.
func BenchmarkTinyAlloc(b *testing.B) {
	b.ReportAllocs()
	var sink *byte
	for i := 0; i < b.N; i++ {
		sink = new(byte)
	}
	benchByteSink = sink
}

// BenchmarkBlob33Alloc 은 33바이트 객체(클래스 48)를 만드는 비용을 잰다.
// -benchmem 으로 두 벤치마크의 B/op 를 나란히 보면 tiny 경로와 클래스 경로의
// 차이가 그대로 드러난다.
func BenchmarkBlob33Alloc(b *testing.B) {
	b.ReportAllocs()
	var sink *blob33
	for i := 0; i < b.N; i++ {
		sink = &blob33{}
	}
	benchBlobSink = sink
}