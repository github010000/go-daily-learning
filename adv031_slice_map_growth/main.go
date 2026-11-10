package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
	"unsafe"
)

// sink 는 "이 결과를 아무도 안 본다"는 컴파일러 판단을 막는 용도다.
// 이게 없으면 append 루프와 map 삽입 루프가 통째로 사라져서 벤치마크가
// 0 ns/op 으로 나온다. 최적화 코드를 재는 사람이 가장 먼저 밟는 함정이다.
var (
	sinkSlice []int
	sinkMap   map[int]int
)

// Go 1.18 부터 runtime.nextslicecap 이 쓰는 임계값. 이 아래에서는 2배.
// 1.17 까지는 이 값이 1024 였고, 그래서 1024 근처에서 계단이 생겼다.
const growThreshold = 256

// growModel 은 runtime/slice.go 의 nextslicecap 안에서 newcap 을 계산하는 부분만
// 그대로 옮긴 것이다. mallocgc 의 size class 올림(roundupsize)은 일부러 뺐다.
// 왜냐하면 "성장 알고리즘"과 "할당 크기 반올림"은 서로 다른 층이고, 둘을 섞으면
// 왜 그 cap 이 나왔는지 설명할 수 없게 되기 때문이다. 2절에서 둘을 나란히 본다.
func growModel(oldCap, newLen int) int {
	newcap := oldCap
	doublecap := newcap + newcap
	if newLen > doublecap {
		// 한 번에 2배보다 많이 필요하면 계산이고 뭐고 없다. 요청한 길이가 곧 cap.
		return newLen
	}
	if oldCap < growThreshold {
		return doublecap
	}
	// cap 이 커질수록 증가분이 (newcap + 768) / 4 로 수렴한다.
	// 상수항 192(=3*256/4) 덕분에 작은 cap 에서는 거의 2배, 큰 cap 에서는 1.25배가 된다.
	for newcap < newLen {
		newcap += (newcap + 3*growThreshold) / 4
	}
	return newcap
}

// growModelPre118 은 Go 1.17 의 식이다. 임계값 1024, 상수항 없음.
// 이 둘을 나란히 두는 순간 "1.18 이 무엇을 고쳤는지"가 숫자로 보인다.
func growModelPre118(oldCap, newLen int) int {
	newcap := oldCap
	doublecap := newcap + newcap
	if newLen > doublecap {
		return newLen
	}
	if oldCap < 1024 {
		return doublecap
	}
	for newcap < newLen {
		newcap += newcap / 4
	}
	return newcap
}

// doubleModel 은 "무조건 2배"라는 가장 단순한 전략이다. 비교 기준선으로만 쓴다.
func doubleModel(oldCap, newLen int) int {
	if newLen > 2*oldCap {
		return newLen
	}
	return 2 * oldCap
}

// modelSeries 는 성장식을 n 개까지 돌려 가며 재할당 직후의 cap 을 모은다.
//
//	c 가 0 인 상태에서 시작하는 이유: 실제 append 도 빈 슬라이스에서 시작하고,
//	"첫 성장"의 입력이 0 이라는 사실이 size class 해석에 중요하기 때문이다.
func modelSeries(n int, grow func(int, int) int) []int {
	caps := make([]int, 0, 16)
	c := 0
	for c < n {
		next := grow(c, c+1)
		if next <= c { // 성장식이 멈추면 무한 루프가 되므로 방어한다
			break
		}
		c = next
		caps = append(caps, c)
		if len(caps) > 4096 {
			break
		}
	}
	return caps
}

// growthSeries 는 실제 런타임에서 cap 이 바뀐 순간의 cap 만 모은다.
// 모델과 결정적으로 다른 점: 여기서 나온 cap 은 mallocgc 의 size class 로
// 올림된 값이고, 그 올림된 값이 다음 성장의 입력이 된다.
func growthSeries(n int) []int {
	var caps []int
	s := make([]int, 0)
	prev := -1
	for i := 0; i < n; i++ {
		s = append(s, i)
		if c := cap(s); c != prev {
			caps = append(caps, c)
			prev = c
		}
	}
	runtime.KeepAlive(s) // 반환 전에 s 가 살아 있음을 컴파일러에 알린다
	return caps
}

// totalCopied 는 각 재할당에서 새 배열로 memmove 되는 원소 수의 합이다.
// growslice 는 새 배열을 잡고 oldLen 개를 통째로 복사하므로, 재할당 직전의
// cap 이 곧 그때 복사되는 원소 수가 된다.
func totalCopied(caps []int) int {
	total, prev := 0, 0
	for _, c := range caps {
		total += prev
		prev = c
	}
	return total
}

// wastePct 는 n 개를 담고도 남는 빈 칸의 비율이다.
func wastePct(caps []int, n int) float64 {
	if len(caps) == 0 || n == 0 {
		return 0
	}
	return float64(caps[len(caps)-1]-n) * 100 / float64(n)
}

// avgOverhead 는 [from, to] 구간의 모든 n 에 대해 (최종 cap / n) 의 평균을 낸다.
// n 하나만 보면 결론이 뒤집힌다. 예를 들어 n 이 정확히 2의 거듭제곱이면
// "2배 전략"의 낭비율은 0% 인데, 그 옆의 n 에서는 99% 까지 치솟는다.
// 구간 평균으로 봐야 전략의 성질이 보인다.
func avgOverhead(grow func(int, int) int, from, to int) float64 {
	sum, cnt := 0.0, 0
	for n := from; n <= to; n++ {
		caps := modelSeries(n, grow)
		if len(caps) == 0 {
			continue
		}
		sum += float64(caps[len(caps)-1]) / float64(n)
		cnt++
	}
	if cnt == 0 {
		return 0
	}
	return sum / float64(cnt)
}

// factorCurve 는 cap 이 c 일 때 "다음 성장이 몇 배인지"를 찍는다.
// 1.17 식에서 512 -> 1024 구간의 배율이 2.0 에서 1.25 로 뚝 떨어지는 것이
// 그 유명한 1024 절벽이고, 1.18 식은 그 지점이 매끄럽다.
func factorCurve() string {
	out := ""
	for _, c := range []int{1, 16, 64, 128, 256, 512, 1024, 4096, 65536, 1 << 20} {
		out += fmt.Sprintf("  cap=%-8d 다음 배율  1.18=%.3f   1.17=%.3f\n",
			c,
			float64(growModel(c, c+1))/float64(c),
			float64(growModelPre118(c, c+1))/float64(c))
	}
	return out
}

func printGrowthTable(n int) {
	fmt.Printf("  목표 원소 수 n = %d (모델 계산, size class 반올림 제외)\n", n)
	fmt.Printf("  %-14s %8s %14s %12s %9s\n", "성장식", "재할당", "누적 복사", "최종 cap", "낭비%")
	rows := []struct {
		name string
		g    func(int, int) int
	}{
		{"2배 고정", doubleModel},
		{"1.17 식", growModelPre118},
		{"1.18 식", growModel},
	}
	for _, r := range rows {
		caps := modelSeries(n, r.g)
		last := 0
		if len(caps) > 0 {
			last = caps[len(caps)-1]
		}
		fmt.Printf("  %-14s %8d %14d %12d %8.1f%%\n",
			r.name, len(caps), totalCopied(caps), last, wastePct(caps, n))
	}
	fmt.Printf("  n=1..4096 구간 평균 (최종 cap / n):  2배=%.3f  1.17=%.3f  1.18=%.3f\n",
		avgOverhead(doubleModel, 1, 4096),
		avgOverhead(growModelPre118, 1, 4096),
		avgOverhead(growModel, 1, 4096))
}

func printRuntimeSeries(n int) {
	caps := growthSeries(n)
	fmt.Printf("  n=%d 까지 append 했을 때 cap 이 바뀐 순간들: %d회 (원소 크기 %d 바이트)\n",
		n, len(caps), int(unsafe.Sizeof(int(0))))
	fmt.Printf("  실제: %v\n", caps)
	fmt.Printf("  모델: %v\n", modelSeries(n, growModel))
	fmt.Println("  실제 값이 모델과 다른 이유: growslice 끝에서 size class 로 올림하기 때문")
	fmt.Println("  (예: 8바이트 원소 5개 = 40바이트 -> 48바이트 클래스 -> cap 6)")
}

// ---- 사전 할당 비용 측정 ----

func appendGrow(n int) {
	s := make([]int, 0)
	for i := 0; i < n; i++ {
		s = append(s, i)
	}
	sinkSlice = s
}

func appendPrealloc(n int) {
	s := make([]int, 0, n)
	for i := 0; i < n; i++ {
		s = append(s, i)
	}
	sinkSlice = s
}

func mapGrow(n int) {
	m := make(map[int]int)
	for i := 0; i < n; i++ {
		m[i] = i
	}
	sinkMap = m
}

func mapPrealloc(n int) {
	m := make(map[int]int, n)
	for i := 0; i < n; i++ {
		m[i] = i
	}
	sinkMap = m
}

// nsPerOp 은 원소 하나 추가당 나노초를 잰다. 3회 중 최솟값을 쓰는 이유:
// 다른 프로세스나 GC 는 시간을 더하기만 하지 줄이지는 않는다. 최솟값이
// "간섭 없는 실행"에 가장 가깝다.
func nsPerOp(n int, f func()) float64 {
	best := 0.0
	for r := 0; r < 3; r++ {
		start := time.Now()
		f()
		d := float64(time.Since(start).Nanoseconds()) / float64(n)
		if r == 0 || d < best {
			best = d
		}
	}
	return best
}

// measureAllocs 는 f() 한 번이 만든 할당 횟수와 누적 바이트를 잰다.
// GC 를 끄는 이유: 켜 두면 GC 자신의 내부 할당이 카운터에 섞인다.
// 여기서 알고 싶은 것은 "우리 코드가 몇 번 mallocgc 를 불렀는가" 뿐이다.
func measureAllocs(f func()) (allocs, bytes uint64) {
	old := debug.SetGCPercent(-1)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	debug.SetGCPercent(old)
	return after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc
}

func ratio(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func allocReport(n int) string {
	gA, gB := measureAllocs(func() { appendGrow(n) })
	pA, pB := measureAllocs(func() { appendPrealloc(n) })
	mA, mB := measureAllocs(func() { mapGrow(n) })
	qA, qB := measureAllocs(func() { mapPrealloc(n) })

	gT := nsPerOp(n, func() { appendGrow(n) })
	pT := nsPerOp(n, func() { appendPrealloc(n) })
	mT := nsPerOp(n, func() { mapGrow(n) })
	qT := nsPerOp(n, func() { mapPrealloc(n) })

	out := fmt.Sprintf("  원소 수 n = %d\n", n)
	out += fmt.Sprintf("  %-24s %10s %14s %12s\n", "작업", "할당 횟수", "누적 바이트", "ns/원소")
	row := func(name string, a, b uint64, ns float64) string {
		return fmt.Sprintf("  %-24s %10d %14d %12.1f\n", name, a, b, ns)
	}
	out += row("append 재할당 허용", gA, gB, gT)
	out += row("append cap=n 사전 할당", pA, pB, pT)
	out += row("map 재해시 허용", mA, mB, mT)
	out += row("map hint=n 사전 할당", qA, qB, qT)
	out += fmt.Sprintf("  -> 할당 횟수 감소: append %.1f배, map %.1f배\n",
		ratio(gA, pA), ratio(mA, qA))
	out += "  ns/원소는 머신 부하에 흔들린다. 믿을 것은 할당 횟수와 바이트다.\n"
	return out
}

// ---- map 내부 관찰 ----

// hmapMirror 는 runtime.hmap 의 앞부분만 옮긴 사본이다. 필드 순서와 정렬이
// runtime 쪽과 같아야 한다. 여기서는 포인터를 역참조하지 않고 스칼라 필드만
// 읽으므로, 레이아웃이 달라도 최악의 경우 "이상한 숫자"이지 크래시는 아니다.
// 그래서 count 와 B 로 sanity check 를 하고 실패하면 관찰을 포기한다.
type hmapMirror struct {
	count      int
	flags      uint8
	B          uint8
	noverflow  uint16
	hash0      uint32
	buckets    unsafe.Pointer
	oldbuckets unsafe.Pointer
	nevacuate  uintptr
	extra      unsafe.Pointer
}

func peekMap(m map[int]int) (hmapMirror, bool) {
	hp := *(*unsafe.Pointer)(unsafe.Pointer(&m)) // map 값 자체가 *hmap 이다
	if hp == nil {
		return hmapMirror{}, false
	}
	h := *(*hmapMirror)(hp)
	// 런타임 map 구현이 바뀌면(예: swiss table) count 가 절대 맞을 수 없다.
	if h.count != len(m) || h.B > 40 {
		return h, false
	}
	return h, true
}

// observeMapGrowth 는 삽입을 하면서 B, oldbuckets, nevacuate 가 바뀌는 순간만
// 기록한다. oldbuckets 가 살아 있는 구간이 곧 "증분 재해시가 진행 중"인 구간이다.
func observeMapGrowth(n int) []string {
	out := make([]string, 0, 32)
	m := make(map[int]int)
	prevB := uint8(200)
	prevGrowing := false
	for i := 1; i <= n; i++ {
		m[i] = i
		h, ok := peekMap(m)
		if !ok {
			return append(out, "  이 Go 버전의 map 내부 레이아웃이 달라 관찰을 건너뜁니다 (count 검증 실패)")
		}
		growing := h.oldbuckets != nil
		if h.B != prevB || growing != prevGrowing {
			out = append(out, fmt.Sprintf(
				"  삽입 %6d  count=%6d  B=%2d  버킷수=%6d  이주중=%-5v  nevacuate=%d",
				i, h.count, h.B, 1<<h.B, growing, h.nevacuate))
			prevB = h.B
			prevGrowing = growing
		}
	}
	runtime.KeepAlive(m)
	if len(out) == 0 {
		out = append(out, "  B 변화가 관찰되지 않았다")
	}
	return out
}

func main() {
	fmt.Println("=== 슬라이스/맵 성장 전략과 재할당 비용 ===")
	fmt.Println()
	fmt.Println("[1] 성장식 모델: 2배 / 1.17 / 1.18")
	printGrowthTable(1 << 20)
	fmt.Print(factorCurve())
	fmt.Println()
	fmt.Println("[2] 실제 런타임이 만드는 cap 시리즈")
	printRuntimeSeries(4096)
	fmt.Println()
	fmt.Println("[3] 사전 할당 효과")
	fmt.Print(allocReport(1 << 13))
	fmt.Println()
	fmt.Println("[4] map 성장 관찰: B 는 6.5 * 2^B 를 넘는 순간 증가한다")
	for _, line := range observeMapGrowth(1 << 14) {
		fmt.Println(line)
	}
}