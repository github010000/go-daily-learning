// 주제 26: 할당기 내부 — size class 와 tiny allocator.
//
// 이 프로그램이 보여 주려는 것:
//   1) 런타임이 공개하는 size class 표의 실제 모습과, 그 창구가 잘려 있다는 사실
//   2) 요청 크기가 클래스로 올림되며 생기는 내부 단편화
//   3) 16바이트 미만 + 포인터 없음 객체를 한 블록에 뭉치는 tiny allocator
//   4) 필드 순서 하나가 클래스를 바꿔 객체당 비용을 바꾸는 모습
//   5) goroutine 마다 붙은 mcache 가 락 없이 스팬을 나눠 쓰는 구조
//
// 측정은 runtime.MemStats.TotalAlloc 의 증가분으로 한다. HeapAlloc 이 아니라
// 누적치를 쓰는 이유는 chargePerObject 주석에 적었다. 요약하면 HeapAlloc 은
// 살아 있는 바이트라 측정 도중 줄어들 수 있고, uint64 뺄셈이 감싸 돌면
// 4.6e14 같은 쓰레기 값이 나온다.
package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sort"
	"sync"
	"unsafe"
)

const (
	// maxTinySize 는 runtime 이 tiny allocator 로 묶는 상한이다
	// (runtime/malloc.go). 이보다 작고 포인터가 없어야 tiny 경로를 탄다.
	maxTinySize = 16

	// numSizeClasses 는 runtime/sizeclasses.go 의 _NumSizeClasses.
	// 이 값 자체는 공개 API 가 아니라 소스에 적힌 사실이다.
	numSizeClasses = 68

	// maxSmallSize 는 small object 상한. 이 위로는 클래스를 고르지 않고
	// 페이지를 통째로 붙여 준다.
	maxSmallSize = 32768
)

func init() {
	// 메모리 프로파일러는 mallocgc 안에서 샘플 버킷을 할당한다. 그 할당이
	// TotalAlloc 에 섞이면 객체당 비용이 부풀려지므로 측정 전에 끈다.
	// 프로파일러를 쓰지 않는 학습용 프로그램이라 잃는 것이 없다.
	runtime.MemProfileRate = 0
}

// ---------- 1. size class 표 ----------

// sizeClasses 는 런타임이 공개하는 클래스 크기를 오름차순으로 돌려준다.
//
// runtime/sizeclasses.go 의 class_to_size 를 코드에 그대로 옮겨 적지 않는
// 이유: 그 표는 mksizeclasses.go 생성기의 산출물이라 툴체인 메이저 버전마다
// 다시 만들어진다. 표를 하드코딩하면 Go 를 올리는 순간 거짓말이 된다.
// MemStats.BySize 는 ReadMemStats 안에서 class_to_size 로 채워지는 유일한
// 공개 창구다.
//
// 다만 이 창구는 61칸짜리 고정 배열이라 클래스 61번 이후(19072~32768)는
// 보이지 않는다. 내부 구조를 관찰하는 창이 이렇게 좁다는 것 자체가
// 이 주제에서 배울 거리다.
func sizeClasses() []uint32 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out := make([]uint32, 0, len(ms.BySize))
	for _, b := range ms.BySize {
		if b.Size == 0 {
			continue // 0번 칸은 실제 할당에 쓰이지 않는 자리표시자
		}
		out = append(out, b.Size)
	}
	return out
}

// classFor 는 요청 크기 n 을 담을 수 있는 가장 작은 클래스를 찾는다.
// 올림이 일어나는 지점이 곧 내부 단편화가 생기는 지점이다.
func classFor(sizes []uint32, n uint32) (uint32, bool) {
	i := sort.Search(len(sizes), func(i int) bool { return sizes[i] >= n })
	if i == len(sizes) {
		return 0, false
	}
	return sizes[i], true
}

type fragRow struct {
	requested uint32
	class     uint32
}

// waste 는 클래스 안에서 아무도 못 쓰고 버려지는 비율. 33 -> 48 이면 0.3125.
func (r fragRow) waste() float64 { return 1 - float64(r.requested)/float64(r.class) }

// worstFragmentation 은 각 클래스의 "경계 바로 위" 요청이 얼마나 낭비되는지
// 계산해 나쁜 순으로 k 개를 돌려준다. 단편화는 큰 클래스가 아니라 표 앞자리
// (9 -> 16 같은 곳)에서 가장 심하다는 사실이 여기서 드러난다.
func worstFragmentation(sizes []uint32, k int) []fragRow {
	rows := make([]fragRow, 0, len(sizes))
	for i := 1; i < len(sizes); i++ {
		rows = append(rows, fragRow{requested: sizes[i-1] + 1, class: sizes[i]})
	}
	sort.Slice(rows, func(a, b int) bool { return rows[a].waste() > rows[b].waste() })
	if len(rows) > k {
		rows = rows[:k]
	}
	return rows
}

// ---------- 2. 실측 ----------

// chargePerObject 는 fill 이 n 개 객체를 만드는 동안 TotalAlloc 이 얼마나
// 늘었는지 객체당으로 나눈다. 이 값이 이 주제의 핵심 수치다: 요청한 크기가
// 아니라 "스팬에서 잘라 준 클래스 크기"가 청구된다.
//
// HeapAlloc 대신 TotalAlloc 을 쓰는 이유 두 가지.
//   - TotalAlloc 은 누적치라 절대 줄지 않는다. HeapAlloc 은 살아 있는 바이트라
//     백그라운드 sweep 이 밀린 스팬을 회수하면 줄어들 수 있고, uint64 뺄셈이
//     감싸 돌아 4.6e14 같은 값이 튀어나온다.
//   - GC 가 돌면 TotalAlloc 이외의 값은 의미가 흔들린다. GC 를 꺼 두면
//     "할당한 만큼 정확히" 증가한다.
//
// dst 슬라이스는 스냅샷 전에 잡아서 측정 대상에서 뺀다. 슬라이스 헤더
// 24바이트 때문에 8바이트 객체 측정이 왜곡되면 안 된다.
func chargePerObject[T any](n int, fill func([]T)) float64 {
	dst := make([]T, n)
	old := debug.SetGCPercent(-1)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	fill(dst)
	runtime.ReadMemStats(&after)
	debug.SetGCPercent(old)
	runtime.KeepAlive(dst)
	return float64(after.TotalAlloc-before.TotalAlloc) / float64(n)
}

// blob33 은 33바이트 무포인터. 16 이상이라 tiny 대상이 아니고 클래스 48 로
// 올림된다. 이 주제의 상징적인 숫자다.
type blob33 struct{ a [33]byte }

// blob5000 은 클래스 5376 으로 올림된다. 큰 객체일수록 상대 낭비가 줄어든다는
// 것을 33바이트와 나란히 보여 주기 위한 대조군이다.
type blob5000 struct{ a [5000]byte }

// ptrWord 는 8바이트지만 포인터가 하나 있어 tiny 후보가 아니다.
// 크기만 보면 1바이트 객체보다 크지만 대우는 정반대다.
type ptrWord struct{ p *byte }

// badLayout 은 정렬 패딩 때문에 24바이트가 된다. bool 뒤에 int64 를 두면
// 7바이트가 통째로 버려지고, 그 24 라는 크기가 다시 클래스 24 로 청구된다.
// 낭비가 두 번 곱해지는 셈이다.
type badLayout struct {
	a bool
	b int64
	c bool
}

// goodLayout 은 같은 필드를 큰 것부터 늘어놓아 16바이트다. 클래스 16.
type goodLayout struct {
	b int64
	a bool
	c bool
}

func chargeBlob33(n int) float64 {
	return chargePerObject[*blob33](n, func(dst []*blob33) {
		for i := range dst {
			dst[i] = &blob33{}
		}
	})
}

func chargeBlob5000(n int) float64 {
	return chargePerObject[*blob5000](n, func(dst []*blob5000) {
		for i := range dst {
			dst[i] = &blob5000{}
		}
	})
}

func chargeBad(n int) float64 {
	return chargePerObject[*badLayout](n, func(dst []*badLayout) {
		for i := range dst {
			dst[i] = &badLayout{}
		}
	})
}

func chargeGood(n int) float64 {
	return chargePerObject[*goodLayout](n, func(dst []*goodLayout) {
		for i := range dst {
			dst[i] = &goodLayout{}
		}
	})
}

// ---------- 3. tiny allocator 를 주소로 확인 ----------

// gaps 는 newObj 로 n 개 객체를 잡고 이웃한 두 주소의 간격을 돌려준다.
//
// 바이트 회계가 아니라 주소를 보는 이유: 회계 방식은 런타임 내부 사정이라
// 버전에 따라 달라질 수 있지만, "한 16바이트 블록을 16개가 나눠 쓴다"는
// 사실은 주소 간격 1로 직접 드러난다. 관찰하려는 현상에 가장 가까운
// 측정을 고르는 것이 이런 데모의 요령이다.
//
// 측정 중 GC 를 끄는 이유: GC 가 돌면 mcache 의 tiny 블록이 정리될 수 있어
// 간격이 들쭉날쭉해진다.
func gaps[T any](n int, newObj func() *T) []int {
	dst := make([]*T, n)
	old := debug.SetGCPercent(-1)
	for i := range dst {
		dst[i] = newObj()
	}
	debug.SetGCPercent(old)
	runtime.KeepAlive(dst)
	out := make([]int, 0, n-1)
	for i := 1; i < len(dst); i++ {
		out = append(out, int(uintptr(unsafe.Pointer(dst[i]))-uintptr(unsafe.Pointer(dst[i-1]))))
	}
	return out
}

// modeGap 은 가장 흔한 간격과 그 비율을 돌려준다. 평균을 쓰지 않는 이유는
// 스팬을 새로 받는 지점에서 수천 바이트짜리 튐이 섞이기 때문이다.
// 최빈값이 "이 객체들이 어떤 단위로 잘렸는지"를 가장 또렷하게 말해 준다.
func modeGap(g []int) (int, float64) {
	if len(g) == 0 {
		return 0, 0
	}
	counts := make(map[int]int, 8)
	best, bestN := 0, -1
	for _, d := range g {
		counts[d]++
		if counts[d] > bestN {
			best, bestN = d, counts[d]
		}
	}
	return best, float64(bestN) / float64(len(g))
}

// ---------- 4. 동시 할당 ----------

// parallelFill 은 goroutine 마다 자기 슬라이스만 채운다. 공유 쓰기가 없으므로
// -race 로 돌려도 깨끗하다. mcache 가 P 마다 따로 있어서 락 없이 스팬을
// 잘라 쓰는 구조를 흉내내는 부분이다.
func parallelFill(parts [][]*blob33) {
	var wg sync.WaitGroup
	wg.Add(len(parts))
	for g := range parts {
		go func(g int) {
			defer wg.Done()
			xs := parts[g]
			for i := range xs {
				xs[i] = &blob33{}
			}
		}(g)
	}
	wg.Wait()
}

// countUnique 는 서로 다른 주소 개수를 센다. 할당기가 살아 있는 두 객체에
// 같은 주소를 주면 이 값이 전체 개수보다 작아진다.
func countUnique(parts [][]*blob33) int {
	seen := make(map[*blob33]struct{}, 1024)
	for _, xs := range parts {
		for _, p := range xs {
			seen[p] = struct{}{}
		}
	}
	return len(seen)
}

// ---------- 5. 시연 ----------

func demoTable() {
	sizes := sizeClasses()
	if len(sizes) == 0 {
		fmt.Println("[1] 클래스 표를 얻지 못함")
		return
	}
	fmt.Printf("[1] 런타임이 공개한 size class %d 개: %d .. %d\n",
		len(sizes), sizes[0], sizes[len(sizes)-1])
	fmt.Printf("    runtime/sizeclasses.go: _NumSizeClasses=%d, 최대 %d\n",
		numSizeClasses, maxSmallSize)
	fmt.Printf("    => 공개 창구는 %d 에서 끊긴다. 그 위 클래스는 소스로만 볼 수 있다\n",
		sizes[len(sizes)-1])
	head := sizes
	if len(head) > 16 {
		head = head[:16]
	}
	fmt.Printf("    앞부분:")
	for _, s := range head {
		fmt.Printf(" %d", s)
	}
	fmt.Println()
}

func demoRounding() {
	const (
		n33 = 20000
		n5k = 2000
	)
	sizes := sizeClasses()
	fmt.Println("[2] 요청 크기 -> 실제 청구 크기 (내부 단편화)")
	for _, req := range []uint32{1, 9, 17, 33, 100, 1000, 5000} {
		c, ok := classFor(sizes, req)
		if !ok {
			continue
		}
		fmt.Printf("    %5d B 요청 -> %5d B 클래스  (+%.1f%% 더 씀)\n",
			req, c, 100*(float64(c)/float64(req)-1))
	}
	b33 := chargeBlob33(n33)
	b5k := chargeBlob5000(n5k)
	fmt.Printf("    실측: 33B 객체 %.2f B/obj, 5000B 객체 %.2f B/obj\n", b33, b5k)
}

func demoWorst() {
	rows := worstFragmentation(sizeClasses(), 5)
	fmt.Println("[3] 올림 손실 상위 5 (경계 바로 위 요청)")
	for _, r := range rows {
		fmt.Printf("    %5d B 요청 -> %5d B  %.1f%% 낭비\n",
			r.requested, r.class, 100*r.waste())
	}
}

func demoTiny() {
	const n = 4096
	gTiny := gaps(n, func() *byte { return new(byte) })
	gPtr := gaps(n, func() *ptrWord { return &ptrWord{} })
	dTiny, fTiny := modeGap(gTiny)
	dPtr, fPtr := modeGap(gPtr)
	fmt.Println("[4] tiny allocator (16B 미만 + 포인터 없음)")
	fmt.Printf("    new(byte) %d 개 : 가장 흔한 이웃 간격 %d B (%.1f%%) -> 16B 블록 하나를 나눠 쓴다\n",
		n, dTiny, 100*fTiny)
	fmt.Printf("    &ptrWord{} %d 개 : 가장 흔한 이웃 간격 %d B (%.1f%%) -> 포인터가 있으면 tiny 경로를 못 탄다\n",
		n, dPtr, 100*fPtr)
}

func demoFieldOrder() {
	const n = 20000
	bad := chargeBad(n)
	good := chargeGood(n)
	fmt.Println("[5] 필드 순서가 클래스를 바꾼다")
	fmt.Printf("    badLayout  %2d B -> %.2f B/obj\n", unsafe.Sizeof(badLayout{}), bad)
	fmt.Printf("    goodLayout %2d B -> %.2f B/obj\n", unsafe.Sizeof(goodLayout{}), good)
}

func demoParallel() {
	const (
		nG  = 8
		per = 2000
	)
	parts := make([][]*blob33, nG)
	for g := range parts {
		parts[g] = make([]*blob33, per)
	}
	parallelFill(parts)
	fmt.Printf("[6] goroutine %d 개 x %d 개 할당: 서로 다른 주소 %d / %d\n",
		nG, per, countUnique(parts), nG*per)
}

func main() {
	fmt.Println("=== 주제 26: size class 와 tiny allocator ===")
	demoTable()
	demoRounding()
	demoWorst()
	demoTiny()
	demoFieldOrder()
	demoParallel()
}