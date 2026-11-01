// 주제 27: GC 3색 마킹과 write barrier.
//
// 이 파일은 두 부분이다.
//  1) 3색 마킹 시뮬레이터: 배리어 정책만 바꿔 가며 "살아있는 객체를 잃는가"를
//     결정적으로 재현한다. 실제 런타임에서 배리어를 끄고 불변식을 깨면 프로세스가
//     죽으므로, 관찰은 시뮬레이터로 하는 편이 낫다.
//  2) 실제 런타임 관찰: 배리어가 기계어에 어떻게 나타나는지, 마킹 중 포인터 쓰기
//     비용이 얼마인지, 힙이 커져도 STW 정지가 작은지.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// =====================================================================
// 1. 3색 마킹 시뮬레이터
// =====================================================================

type color uint8

const (
	white color = iota
	grey
	black
)

func (c color) String() string {
	switch c {
	case grey:
		return "회색"
	case black:
		return "검정"
	default:
		return "흰색"
	}
}

// node 는 힙 객체 하나다. 실제 런타임은 색을 따로 저장하지 않는다.
// gcmarkBits 의 비트가 흰색/비흰색을, workbuf 큐에 들어 있는지 여부가 회색을,
// 마킹 비트가 서 있고 큐에 없으면 검정을 뜻한다. 여기서는 눈에 보이게 둔다.
type node struct {
	tag   string
	color color
	slots []*node
}

type barrierMode int

const (
	barrierNone         barrierMode = iota // 배리어 없음 (1.5 이전 스타일의 사고 재현용)
	barrierInsert                          // Dijkstra 삽입 배리어: 새 값을 shade
	barrierDelete                          // Yuasa 삭제 배리어: 덮어쓰기 전 값을 shade
	barrierHybrid                          // Go 1.8+ : 삭제 + (스택이 회색일 때만) 삽입
	barrierInsertRescan                    // 삽입 배리어 + 마킹 종료 시 스택 재스캔
)

func (b barrierMode) String() string {
	switch b {
	case barrierInsert:
		return "insert"
	case barrierDelete:
		return "delete"
	case barrierHybrid:
		return "hybrid"
	case barrierInsertRescan:
		return "insert+rescan"
	default:
		return "none"
	}
}

func modes() []barrierMode {
	return []barrierMode{barrierNone, barrierInsert, barrierDelete, barrierHybrid, barrierInsertRescan}
}

// machine 은 힙 + 뮤테이터 스택 + 마커 작업 목록을 함께 들고 있다.
// 스택을 별도 필드로 두는 것이 이 주제의 핵심이다. Go 컴파일러는 스택 슬롯
// 쓰기에는 배리어를 넣지 않는다(넣으면 모든 함수 호출이 느려지고, 애초에
// 스택은 통째로 스캔되니까). 그래서 스택은 "배리어가 닿지 않는 메모리"이고,
// 그 사실이 배리어 두 가지를 모두 필요하게 만든다.
type machine struct {
	mode         barrierMode
	objs         []*node
	globals      []*node // 힙 루트(전역 변수)
	stack        []*node // 뮤테이터 고루틴의 지역 변수 슬롯
	stackScanned bool    // 이 스택이 이미 검게 스캔되었는가
	collecting   bool
	work         []*node // 회색 큐 (런타임의 workbuf 역할)
	shadeCalls   int
	scanned      int
	stackScans   int
}

func newMachine(mode barrierMode) *machine {
	return &machine{mode: mode, globals: nil, stack: nil}
}

// alloc 은 힙 객체를 만든다. 마킹 중 할당이면 즉시 검은색이다.
// 이는 runtime.gcmarknewobject 와 같은 동작이고, 할당에는 배리어가 필요 없는
// 이유이자 "마킹 중에 만든 쓰레기는 이번 사이클에 절대 안 걷힌다"의 이유다.
func (m *machine) alloc(tag string) *node {
	n := &node{tag: tag, slots: make([]*node, 1)}
	if m.collecting {
		n.color = black
	}
	m.objs = append(m.objs, n)
	return n
}

func (m *machine) shade(n *node) {
	if n == nil {
		return
	}
	m.shadeCalls++
	if n.color != white {
		return
	}
	n.color = grey
	m.work = append(m.work, n)
}

// storeHeap 은 힙 슬롯 쓰기다. 컴파일러가 여기에 배리어를 끼워 넣는다.
func (m *machine) storeHeap(dst *node, i int, src *node) {
	old := dst.slots[i]
	switch m.mode {
	case barrierNone:
		// 아무것도 하지 않는다. 마커는 이 쓰기를 영원히 모른다.
	case barrierInsert, barrierInsertRescan:
		m.shade(src)
	case barrierDelete:
		m.shade(old)
	case barrierHybrid:
		m.shade(old)
		if !m.stackScanned {
			m.shade(src)
		}
	}
	dst.slots[i] = src
}

// storeStack 은 지역 변수 쓰기다. 배리어가 없다. 이 함수 본문에는 분기도,
// shade 호출도 없는 것이 요점이다.
func (m *machine) storeStack(i int, src *node) {
	m.stack[i] = src
}

func (m *machine) loadHeap(src *node, i int) *node {
	return src.slots[i] // 읽기에는 배리어가 없다
}

func (m *machine) startMark() {
	m.collecting = true
	for _, o := range m.objs {
		o.color = white
	}
	for _, g := range m.globals {
		m.shade(g)
	}
}

func (m *machine) scanStack() {
	m.stackScans++
	m.stackScanned = true
	for _, s := range m.stack {
		m.shade(s)
	}
}

// step 은 회색 객체 하나를 스캔해서 검게 만든다.
func (m *machine) step() bool {
	if len(m.work) == 0 {
		return false
	}
	n := m.work[0]
	m.work = m.work[1:]
	for _, c := range n.slots {
		m.shade(c)
	}
	n.color = black
	m.scanned++
	return true
}

func (m *machine) drainN(limit int) {
	for i := 0; i < limit; i++ {
		if !m.step() {
			return
		}
	}
}

func (m *machine) drainAll() {
	for m.step() {
	}
}

// finishMark 는 마킹을 끝낸다. insert+rescan 모드는 Go 1.8 이전처럼 마킹 종료
// 시점에 모든 스택을 다시 훑는다. 이 단계는 세계를 멈추고 해야 하므로 고루틴
// 수에 비례하는 STW 비용이 된다.
func (m *machine) finishMark() {
	if m.mode == barrierInsertRescan {
		m.stackScanned = false
		m.scanStack()
	}
	m.drainAll()
}

type verdict struct {
	lost       int // 살아있는데 흰색 -> 회수되면 dangling
	floating   int // 죽었는데 흰색이 아님 -> 이번 사이클에 못 걷음
	shadeCalls int
	scanned    int
	stackScans int
}

func (m *machine) reachable() map[*node]bool {
	seen := make(map[*node]bool, len(m.objs))
	var walk func(*node)
	walk = func(n *node) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		for _, c := range n.slots {
			walk(c)
		}
	}
	for _, g := range m.globals {
		walk(g)
	}
	for _, s := range m.stack {
		walk(s)
	}
	return seen
}

func (m *machine) verify() verdict {
	live := m.reachable()
	v := verdict{shadeCalls: m.shadeCalls, scanned: m.scanned, stackScans: m.stackScans}
	for _, o := range m.objs {
		alive := live[o]
		if alive && o.color == white {
			v.lost++
		}
		if !alive && o.color != white {
			v.floating++
		}
	}
	return v
}

func dumpColors(m *machine) string {
	parts := make([]string, 0, len(m.objs))
	for _, o := range m.objs {
		parts = append(parts, fmt.Sprintf("%s=%s", o.tag, o.color))
	}
	return strings.Join(parts, " ")
}

// ---- 시나리오 ----

type scenario struct {
	name string
	desc string
	run  func(barrierMode) *machine
}

// scenarioQuiet: 위험한 간선 이동이 없는 대조군.
func scenarioQuiet(mode barrierMode) *machine {
	m := newMachine(mode)
	a := m.alloc("A")
	b := m.alloc("B")
	a.slots[0] = b
	m.globals = append(m.globals, a)
	m.stack = append(m.stack, nil)

	m.startMark()
	m.scanStack()
	m.drainAll()
	local := m.loadHeap(a, 0) // 이미 검정인 B
	m.storeStack(0, local)
	m.storeHeap(a, 0, local)
	m.finishMark()
	return m
}

// scenarioHeapHazard: 힙 안에서만 B 가 옮겨 다닌다.
// 삭제와 삽입이 둘 다 일어나므로 두 배리어 중 아무거나 하나만 있어도 살아남는다.
func scenarioHeapHazard(mode barrierMode) *machine {
	m := newMachine(mode)
	a := m.alloc("A")
	x := m.alloc("X")
	b := m.alloc("B")
	x.slots[0] = b
	m.globals = append(m.globals, a, x)
	m.stack = append(m.stack, nil)

	m.startMark()
	m.scanStack() // 스택을 먼저 검게 만든다
	m.drainN(1)   // A 만 스캔. X 는 회색, B 는 흰색으로 남는다
	local := m.loadHeap(x, 0)
	m.storeStack(0, local)   // 스택으로 복사: 배리어 없음
	m.storeHeap(x, 0, nil)   // X 에서 삭제
	m.storeHeap(a, 0, local) // A 에 삽입
	m.finishMark()
	return m
}

// scenarioStackGrey: 흰색 B 가 아직 스캔되지 않은 스택에 있다가 힙으로 옮겨지고,
// 스택 슬롯은 배리어 없이 비워진다. 삭제 배리어는 스택 쓰기를 볼 수 없으므로
// 이 경우를 놓친다. hybrid 의 "스택이 회색이면 삽입도 shade" 조항이 잡는다.
func scenarioStackGrey(mode barrierMode) *machine {
	m := newMachine(mode)
	a := m.alloc("A")
	b := m.alloc("B")
	m.globals = append(m.globals, a)
	m.stack = append(m.stack, b) // 지역 변수에 B 가 들어 있다

	m.startMark()
	m.drainAll()         // A 를 검게. 스택은 아직 회색(스캔 전)
	m.storeHeap(a, 0, b) // 흰색을 검은 객체에 넣는다
	m.storeStack(0, nil) // 지역 변수를 비운다: 배리어 없음
	m.scanStack()        // 이제서야 스택 스캔. 이미 nil 이다
	m.finishMark()
	return m
}

// scenarioStackBlack: 스택이 이미 검게 스캔된 뒤, 아직 스캔되지 않은 힙 객체 X 에서
// B 를 꺼내 검은 스택에 넣고 X 에서는 지운다. 삽입 배리어는 새 값만 보므로
// X 에서의 삭제를 놓친다. 삭제 배리어(또는 스택 재스캔)가 잡는다.
func scenarioStackBlack(mode barrierMode) *machine {
	m := newMachine(mode)
	keep := m.alloc("keep")
	x := m.alloc("X")
	b := m.alloc("B")
	x.slots[0] = b
	m.globals = append(m.globals, keep, x)
	m.stack = append(m.stack, nil)

	m.startMark()
	m.scanStack() // 스택을 먼저 검게 만든다
	m.drainN(1)   // keep 만 검게. X 는 회색, B 는 흰색
	local := m.loadHeap(x, 0)
	m.storeStack(0, local) // 검은 스택에 B 를 넣는다: 배리어 없음
	m.storeHeap(x, 0, nil) // X 에서 지운다
	m.finishMark()
	return m
}

// scenarioAllocDuringMark: 마킹 중 할당은 즉시 검정이다. 배리어 비용이 0 인 대신
// 그 객체는 이번 사이클에 회수되지 않는다.
func scenarioAllocDuringMark(mode barrierMode) *machine {
	m := newMachine(mode)
	a := m.alloc("A")
	m.globals = append(m.globals, a)
	m.stack = append(m.stack, nil)

	m.startMark()
	m.scanStack()
	m.drainAll()
	fresh := m.alloc("fresh")
	m.storeHeap(a, 0, fresh) // 살아있는 새 객체
	m.alloc("버려진 새 객체")    // 아무데도 저장하지 않은 새 객체
	m.finishMark()
	return m
}

func scenarios() []scenario {
	return []scenario{
		{"S1", "위험한 간선 이동 없음(대조군)", scenarioQuiet},
		{"S2", "힙에서 삭제 후 삽입", scenarioHeapHazard},
		{"S3", "회색 스택 + 힙 삽입", scenarioStackGrey},
		{"S4", "검은 스택 + 힙에서 삭제", scenarioStackBlack},
		{"S5", "마킹 중 할당", scenarioAllocDuringMark},
	}
}

func statusOf(v verdict) string {
	if v.lost > 0 {
		return "LOST"
	}
	return "ok"
}

func reportSimulation() {
	fmt.Println("  result=LOST 는 살아있는 객체가 회수된 것, shades=배리어가 부른 shade 횟수,")
	fmt.Println("  scans=스택 스캔 횟수, floating=죽었는데 이번 사이클에 못 걷은 객체 수")
	for _, sc := range scenarios() {
		fmt.Printf("-- %s: %s\n", sc.name, sc.desc)
		fmt.Printf("   %-14s %-6s %-7s %-6s %s\n", "mode", "result", "shades", "scans", "floating")
		for _, mode := range modes() {
			m := sc.run(mode)
			v := m.verify()
			line := fmt.Sprintf("   %-14s %-6s %-7d %-6d %d", mode, statusOf(v), v.shadeCalls, v.stackScans, v.floating)
			if v.lost > 0 {
				line += "   <- " + dumpColors(m)
			}
			fmt.Println(line)
		}
	}
	fmt.Println()
	for _, mode := range modes() {
		bad := 0
		for _, sc := range scenarios() {
			if sc.run(mode).verify().lost > 0 {
				bad++
			}
		}
		fmt.Printf("  %-14s : %d/%d 시나리오에서 살아있는 객체를 잃었다\n", mode, bad, len(scenarios()))
	}
}

func reportRescanCost() {
	const goroutines = 10000
	const stackBytes = 2048
	fmt.Printf("  insert+rescan 은 마킹 종료 시 고루틴 스택을 다시 훑는다: %d개 x %d B = %.1f MB 를 STW 로 훑는다.\n",
		goroutines, stackBytes, float64(goroutines*stackBytes)/(1<<20))
	fmt.Println("  hybrid 는 이 단계가 없다. 대신 포인터 쓰기마다 이전 값을 한 번 더 읽는다.")
}

// =====================================================================
// 2. 배리어가 기계어에 어떻게 나타나는가
// =====================================================================

const probePointer = `package main

type cell struct{ next *cell }

// dst 가 힙을 가리킬 수 있으므로 컴파일러는 여기에 배리어를 넣어야 한다.
func storePtr(dst **cell, src *cell) { *dst = src }

// 필드 쓰기도 대상이다.
func storeField(dst *cell, src *cell) { dst.next = src }
`

const probeWord = `package main

// 정수 쓰기에는 배리어가 필요 없다. GC 는 정수를 포인터로 보지 않는다.
func storeWord(dst *int, v int) { *dst = v }
`

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, " | ")
}

func countWriteBarriers(src string) (int, []string, error) {
	dir, err := os.MkdirTemp("", "wbprobe")
	if err != nil {
		return 0, nil, err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "probe.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		return 0, nil, err
	}
	obj := filepath.Join(dir, "probe.o")
	// -l 로 인라인을 끈다. 인라인되면 함수 본문이 사라져 관찰이 어려워진다.
	cmd := exec.Command("go", "tool", "compile", "-l", "-S", "-p", "main", "-o", obj, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil, fmt.Errorf("compile 실패: %v: %s", err, firstLines(string(out), 3))
	}
	text := string(out)
	count := strings.Count(text, "gcWriteBarrier")
	var samples []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "writeBarrier") && len(samples) < 4 {
			samples = append(samples, strings.TrimSpace(line))
		}
	}
	return count, samples, nil
}

func reportAssembly() {
	n, samples, err := countWriteBarriers(probePointer)
	if err != nil {
		fmt.Println("  (go tool compile 을 실행하지 못했다:", err, ")")
		return
	}
	fmt.Printf("  포인터를 쓰는 함수: gcWriteBarrier 등장 %d회\n", n)
	for _, s := range samples {
		fmt.Println("    ", s)
	}
	n2, _, err := countWriteBarriers(probeWord)
	if err != nil {
		fmt.Println("  (go tool compile 을 실행하지 못했다:", err, ")")
		return
	}
	fmt.Printf("  정수를 쓰는 함수  : gcWriteBarrier 등장 %d회\n", n2)
	fmt.Println("  -> 배리어는 '힙을 가리킬 수 있는 포인터 쓰기'마다 붙는다. 정수 쓰기에는 안 붙는다.")
}

// =====================================================================
// 3. 실제 런타임 관찰
// =====================================================================

type cell struct {
	next *cell
	pad  [56]byte
}

var keepAlive []*cell

func buildLiveHeap(n int) []*cell {
	live := make([]*cell, n)
	for i := range live {
		live[i] = &cell{}
		if i > 0 {
			live[i-1].next = live[i]
		}
	}
	return live
}

func measurePauses(rounds int) []time.Duration {
	keepAlive = buildLiveHeap(120000)
	out := make([]time.Duration, 0, rounds)
	var ms runtime.MemStats
	for i := 0; i < rounds; i++ {
		runtime.GC()
		runtime.ReadMemStats(&ms)
		if ms.NumGC == 0 {
			continue
		}
		idx := (ms.NumGC + 255) % 256
		out = append(out, time.Duration(ms.PauseNs[idx]))
	}
	return out
}

func reportPauses() {
	pauses := measurePauses(6)
	var maxPause time.Duration
	for _, p := range pauses {
		if p > maxPause {
			maxPause = p
		}
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("  살아있는 힙 %.1f MB, GC %d회\n", float64(ms.HeapAlloc)/(1<<20), len(pauses))
	fmt.Printf("  각 GC 의 STW 정지: %v\n", pauses)
	fmt.Printf("  최대 정지 %.1f us. 힙 크기의 함수가 아니다(마킹은 뮤테이터와 같이 돌기 때문).\n",
		float64(maxPause)/float64(time.Microsecond))
}

func storePointerLoop(dst []*cell, src *cell, iters int) {
	mask := len(dst) - 1
	for i := 0; i < iters; i++ {
		dst[i&mask] = src
	}
}

func storeWordLoop(dst []uintptr, src uintptr, iters int) {
	mask := len(dst) - 1
	for i := 0; i < iters; i++ {
		dst[i&mask] = src
	}
}

func timeIt(f func()) time.Duration {
	start := time.Now()
	f()
	return time.Since(start)
}

type writeCost struct {
	offPointer time.Duration
	offWord    time.Duration
	onPointer  time.Duration
	onWord     time.Duration
}

// measureWriteCost 는 같은 루프를 네 조건에서 잰다. 포인터 쓰기와 정수 쓰기를
// 같은 GC 압력 아래에서 비교하면 "마킹이 P 시간을 가져간 몫"이 양쪽에 똑같이
// 들어가므로, 차이만 남겨 배리어 자체 비용을 대략 분리할 수 있다.
func measureWriteCost(iters, rounds int) writeCost {
	ptrs := make([]*cell, 1024)
	words := make([]uintptr, 1024)
	src := &cell{}
	res := writeCost{
		offPointer: time.Hour, offWord: time.Hour,
		onPointer: time.Hour, onWord: time.Hour,
	}

	old := debug.SetGCPercent(-1) // GC 정지: writeBarrier.enabled 가 꺼진다
	for r := 0; r < rounds; r++ {
		storePointerLoop(ptrs, src, iters/10) // 예열
		if d := timeIt(func() { storePointerLoop(ptrs, src, iters) }); d < res.offPointer {
			res.offPointer = d
		}
		if d := timeIt(func() { storeWordLoop(words, 1, iters) }); d < res.offWord {
			res.offWord = d
		}
	}
	debug.SetGCPercent(old)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				runtime.GC()
			}
		}
	}()
	for r := 0; r < rounds; r++ {
		if d := timeIt(func() { storePointerLoop(ptrs, src, iters) }); d < res.onPointer {
			res.onPointer = d
		}
		if d := timeIt(func() { storeWordLoop(words, 1, iters) }); d < res.onWord {
			res.onWord = d
		}
	}
	close(stop)
	wg.Wait()
	return res
}

func reportWriteCost() {
	const iters = 4_000_000
	c := measureWriteCost(iters, 3)
	per := func(d time.Duration) float64 { return float64(d) / float64(iters) }
	fmt.Printf("  %d회 쓰기, 3회 중 최솟값\n", iters)
	fmt.Printf("  정수 쓰기    GC 정지 %.2f ns/op | 마킹 중 %.2f ns/op\n", per(c.offWord), per(c.onWord))
	fmt.Printf("  포인터 쓰기  GC 정지 %.2f ns/op | 마킹 중 %.2f ns/op\n", per(c.offPointer), per(c.onPointer))
	barrier := (per(c.onPointer) - per(c.onWord)) - (per(c.offPointer) - per(c.offWord))
	fmt.Printf("  배리어 몫 대략 %.2f ns/op\n", barrier)
	fmt.Println("  -> 마킹이 도는 동안에만 배리어가 per-P 버퍼에 (슬롯, 값) 쌍을 쌓는다.")
	fmt.Println("     GC 가 멈춰 있으면 배리어는 전역 플래그 검사 한 번으로 끝난다.")
}

func main() {
	fmt.Println("== 1. 3색 마킹 시뮬레이터: 배리어별 정확성과 비용 ==")
	reportSimulation()
	reportRescanCost()
	fmt.Println()

	fmt.Println("== 2. 실제 Go 코드에서 배리어는 어디에 나타나는가 ==")
	reportAssembly()
	fmt.Println()

	fmt.Println("== 3. 실제 런타임: 힙이 커도 STW 정지는 작다 ==")
	reportPauses()
	fmt.Println()

	fmt.Println("== 4. 실제 런타임: 마킹 중 포인터 쓰기 비용 ==")
	reportWriteCost()
	fmt.Println()

	fmt.Println("GODEBUG=gccheckmark=1 로 다시 실행하면 런타임이 마킹 불변식을 직접 검사한다.")
	fmt.Println("GODEBUG=gctrace=1 로 실행하면 마킹/스윕 단계와 정지 시간이 찍힌다.")
}