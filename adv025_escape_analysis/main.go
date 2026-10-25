package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// sink* 전역 변수는 측정 대상 함수의 결과를 "일부러" 붙잡아 두기 위한 것이다.
// 이게 없으면 컴파일러가 결과를 아무도 안 쓰는 호출을 통째로 없앨 수 있어서
// 측정값이 실제보다 작게 나온다. 즉 이 변수들은 측정 장치의 일부다.
var (
	sinkInt   int
	sinkPtr   *int
	sinkAny   any
	sinkFn    func() int
	sinkSlice []int
)

// ---------- 1. 지역 변수의 주소 ----------

//go:noinline
func localPointerUse(n int) int {
	// n 의 주소를 잡는다. 하지만 이 포인터는 함수 밖으로 나가지 않는다.
	// 그래서 n 은 스택 프레임에 그대로 남고 힙 할당은 0 이다.
	// "포인터를 만들었으니 힙" 이라는 통념이 틀렸다는 것을 보여주는 최소 예제.
	// noinline 을 붙인 이유: 인라인 되면 이 함수가 사라지고 판정이 호출자 쪽으로
	// 옮겨 가서 -m 출력과 측정값이 서로 다른 함수를 가리키게 된다.
	p := &n
	*p++
	return *p
}

//go:noinline
func returnedPointer(n int) *int {
	// 같은 포인터지만 이번엔 반환된다. 호출자가 언제 해제할지 컴파일러는 알 수
	// 없으므로 스택에 둘 수 없다. 회수 책임을 GC 에 넘기는 순간이 여기다.
	p := new(int)
	*p = n
	return p
}

// ---------- 2. 인터페이스 변환 ----------

//go:noinline
func boxSmall(n int) any {
	// any(n) 은 runtime.convT64 호출로 내려간다. convT64 는 값이 256 미만이면
	// runtime.staticuint64s 라는 전역 배열을 가리키는 포인터를 그냥 돌려준다.
	// 할당이 없다. 즉 "인터페이스 = 힙" 도 값에 따라 갈린다.
	return n
}

//go:noinline
func boxLarge(n int) any {
	// 소스는 boxSmall 과 완전히 같다. 다른 것은 런타임에 들어오는 값뿐이고,
	// 256 이상이면 convT64 가 mallocgc(8) 로 새 상자를 뜬다.
	return n
}

// ---------- 3. 슬라이스 확장 ----------

//go:noinline
func growNaive(n int) []int {
	var s []int
	for i := 0; i < n; i++ {
		// cap 이 모자랄 때마다 runtime.growslice 이 새 배열을 잡고 원소를 복사한다.
		// 원소 수가 64 면 cap 이 1,2,4,...,64 로 자라므로 할당이 7번이다.
		s = append(s, i)
	}
	return s
}

//go:noinline
func growPrealloc(n int) []int {
	// 필요한 크기를 이미 알고 있다면 make 로 한 번에 잡는다. 할당 1번, 복사 0번.
	// append 최적화의 8할은 이 한 줄이다.
	s := make([]int, 0, n)
	for i := 0; i < n; i++ {
		s = append(s, i)
	}
	return s
}

//go:noinline
func stackArraySum(n int) int {
	// 배열을 슬라이스로 감쌌지만 이 슬라이스도 함수 밖으로 나가지 않는다.
	// 그래서 buf 는 스택에 남는다. "슬라이스 = 힙" 도 아니다.
	var buf [8]int
	s := buf[:n]
	total := 0
	for i := range s {
		s[i] = i * i
		total += s[i]
	}
	return total
}

// ---------- 4. 클로저 캡처 ----------

//go:noinline
func closureLocal(n int) int {
	// 클로저 리터럴 f 가 함수 밖으로 나가지 않으므로 클로저 객체(funcval)도
	// 스택에 놓인다. 캡처한 n 도 프레임을 떠나지 않는다.
	f := func() int { return n + 1 }
	return f()
}

//go:noinline
func closureEscapes(n int) func() int {
	// 클로저가 반환되는 순간, 캡처한 n 은 클로저보다 오래 살아야 한다.
	// 그래서 n 이 힙으로 옮겨진다. -m 이 "moved to heap: n" 이라고 찍는 지점.
	return func() int { return n + 1 }
}

// ---------- 측정 장치 ----------

type row struct {
	name  string
	alloc float64
	why   string
}

// measureAllocs 는 f 를 n 번 호출했을 때 "호출 1회당 새로 생긴 힙 객체 수"를 돌려준다.
// HeapAlloc 바이트가 아니라 Mallocs 를 쓰는 이유: 객체 크기는 함수마다 제각각이라
// 바이트로는 비교가 안 되고, 객체 개수는 "탈출했는가"라는 이분법과 1 대 1 로 대응한다.
func measureAllocs(f func(), n int) float64 {
	// 워밍업. 첫 호출에만 필요한 지연 초기화(itab 생성 등)를 측정에서 뺀다.
	f()

	// GC 를 잠시 끈다. 마킹 도중의 런타임 내부 할당이 카운터에 섞이면
	// 0 이어야 할 값이 0.03 처럼 나와서 "할당 없음" 판정이 흔들린다.
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < n; i++ {
		f()
	}
	runtime.ReadMemStats(&after)
	return float64(after.Mallocs-before.Mallocs) / float64(n)
}

// collectRows 는 모든 데모를 실제로 돌려 측정값을 모은다. main 과 테스트가 같이 쓴다.
func collectRows() []row {
	const iters = 300
	return []row{
		{"localPointerUse", measureAllocs(func() { sinkInt = localPointerUse(41) }, iters),
			"주소를 잡아도 함수를 안 벗어나면 스택"},
		{"returnedPointer", measureAllocs(func() { sinkPtr = returnedPointer(41) }, iters),
			"포인터가 반환되면 호출자가 언제 쓸지 몰라 힙"},
		{"boxSmall(42)", measureAllocs(func() { sinkAny = boxSmall(42) }, iters),
			"256 미만 정수는 staticuint64s 재사용, 할당 0"},
		{"boxLarge(1<<20)", measureAllocs(func() { sinkAny = boxLarge(1 << 20) }, iters),
			"인터페이스에 담기는 순간 값이 힙 상자로 복사"},
		{"stackArraySum", measureAllocs(func() { sinkInt = stackArraySum(8) }, iters),
			"슬라이스로 감싸도 밖으로 안 나가면 스택"},
		{"growNaive(64)", measureAllocs(func() { sinkSlice = growNaive(64) }, iters),
			"cap 이 모자랄 때마다 growslice, 약 7회"},
		{"growPrealloc(64)", measureAllocs(func() { sinkSlice = growPrealloc(64) }, iters),
			"make 로 cap 을 미리 잡으면 1회"},
		{"closureLocal", measureAllocs(func() { sinkInt = closureLocal(41) }, iters),
			"클로저가 안 나가면 funcval 도 스택"},
		{"closureEscapes", measureAllocs(func() { sinkFn = closureEscapes(41) }, iters),
			"클로저가 반환되면 캡처한 변수가 힙으로"},
	}
}

func printRows(rows []row) {
	fmt.Printf("%-20s %10s  %s\n", "함수", "할당/호출", "왜 그런가")
	fmt.Println(strings.Repeat("-", 78))
	for _, r := range rows {
		fmt.Printf("%-20s %10.2f  %s\n", r.name, r.alloc, r.why)
	}
}

// printCompilerDiagnostics 는 실제 컴파일러를 다시 불러 -m 출력을 보여 준다.
// 분석은 컴파일 타임에만 일어나므로 실행 중에는 흉내조차 낼 수 없다.
// 그래서 직접 go 명령을 부른다. 실패하면(go 가 없는 환경) 안내만 하고 넘어간다.
func printCompilerDiagnostics() {
	fmt.Println("=== 실제 컴파일러 진단 ===")
	fmt.Println("$ go build -gcflags=-m -o <devnull> .")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-gcflags=-m", "-o", os.DevNull, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("  go 명령을 실행하지 못했습니다. 터미널에서 직접 돌려 보세요:")
		fmt.Println("  go build -gcflags=-m -o /dev/null .")
		fmt.Printf("  원인: %v\n", err)
		return
	}
	printed := 0
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "escapes to heap") &&
			!strings.Contains(line, "moved to heap") &&
			!strings.Contains(line, "does not escape") {
			continue
		}
		fmt.Println(" ", line)
		printed++
		if printed >= 30 {
			fmt.Println("  ... (이하 생략. -gcflags='-m -m' 으로 이유까지 볼 수 있다)")
			break
		}
	}
	if printed == 0 {
		fmt.Println("  (관련 키워드가 출력되지 않았습니다)")
	}
}

func main() {
	fmt.Println("=== 이스케이프 분석: 무엇이 힙으로 가는가 ===")
	fmt.Println("측정: runtime.MemStats.Mallocs 증가분 / 호출 횟수 (GC 를 끄고 측정)")
	fmt.Println("0.00 = 호출당 힙 할당 없음(스택), 7.00 = 호출당 힙 객체 7개")
	fmt.Println()
	printRows(collectRows())
	fmt.Println()
	printCompilerDiagnostics()
}