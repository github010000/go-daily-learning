package main

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// 1. 런타임이 실제로 내주는 것: "텍스트 덤프"
// ---------------------------------------------------------------------------

// goroutineStacks 는 살아 있는 모든 goroutine 의 스택을 텍스트로 돌려준다.
//
// Go 는 "지금 살아 있는 goroutine 목록"을 구조체로 주는 API 를 노출하지 않는다.
// 유일한 창구가 runtime.Stack(buf, true) 이고 이건 사람이 읽는 텍스트다.
// goleak, pprof, 각종 누수 탐지기가 전부 이 텍스트를 파싱하는 이유가 여기 있다.
// "마법의 목록 API 가 있다"가 아니라 "텍스트를 읽는다"가 이 주제의 출발점이다.
//
// 버퍼가 모자라면 runtime.Stack 은 잘라서 채우고 len(buf) 을 반환한다.
// 이 재시도 루프를 빼면 goroutine 이 늘어나는 순간 덤프가 조용히 잘리고,
// 잘린 목록으로 "누수 없음"이라고 판정하는 거짓 음성이 생긴다.
// 누수 탐지기에서 거짓 음성은 치명적이다. 있는 누수를 없다고 말하기 때문이다.
func goroutineStacks() []byte {
	buf := make([]byte, 64<<10)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

// GoroutineInfo 는 덤프 한 블록에서 뽑아낸 요약이다.
//
// Raw 를 함께 들고 다니는 이유가 중요하다. TopFunc 만 보면 "지금 어디서 멈췄나"는
// 알지만 "누가 이걸 만들었나"는 모른다. 누수를 고치려면 후자가 필요하다.
// 덤프의 마지막 줄 "created by main.foo in goroutine 1" 이 범인 지목의 근거다.
type GoroutineInfo struct {
	ID      uint64
	State   string // chan send / chan receive / select / sleep / IO wait / running ...
	TopFunc string // 스택 맨 위 함수 = 지금 어디서 멈춰 있는가
	Raw     string // 이 goroutine 블록 전체 텍스트 (created by 줄 포함)
}

// parseStacks 는 덤프 텍스트를 GoroutineInfo 목록으로 바꾼다.
//
// 블록 경계를 빈 줄과 "goroutine " 접두사 둘 다로 판정하는 이유:
// 런타임 버전에 따라 블록 사이 빈 줄 유무가 달라질 수 있어서다.
// 한쪽만 믿으면 블록이 통째로 붙어 버리고, 두 goroutine 이 하나로 세어진다.
func parseStacks(text string) []GoroutineInfo {
	lines := strings.Split(text, "\n")
	var out []GoroutineInfo
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "goroutine ") {
			continue
		}
		// 블록 끝까지 모은다. 다음 헤더나 빈 줄이 나오면 거기서 끊는다.
		end := i + 1
		for end < len(lines) && lines[end] != "" && !strings.HasPrefix(lines[end], "goroutine ") {
			end++
		}
		info := GoroutineInfo{Raw: strings.Join(lines[i:end], "\n")}

		rest := strings.TrimPrefix(lines[i], "goroutine ")
		if sp := strings.IndexByte(rest, ' '); sp > 0 {
			info.ID, _ = strconv.ParseUint(rest[:sp], 10, 64)
			rest = rest[sp:]
		}
		if lb := strings.IndexByte(rest, '['); lb >= 0 {
			if rb := strings.IndexByte(rest[lb:], ']'); rb > lb {
				state := rest[lb+1 : lb+rb]
				// 5분 넘게 멈춰 있으면 "chan send, 5 minutes" 처럼 꼬리가 붙는다.
				// 상태별로 묶어 세고 싶으니 꼬리는 잘라낸다.
				if c := strings.IndexByte(state, ','); c > 0 {
					state = state[:c]
				}
				info.State = state
			}
		}
		// 헤더 바로 다음 줄이 최상단 프레임이다. 인자 목록은 잘라낸다.
		if i+1 < end {
			fn := lines[i+1]
			if p := strings.IndexByte(fn, '('); p > 0 {
				fn = fn[:p]
			}
			info.TopFunc = strings.TrimSpace(fn)
		}
		out = append(out, info)
		i = end - 1
	}
	return out
}

// isOurs 는 "우리 패키지 코드가 만든 goroutine"인지 판정한다.
//
// TopFunc 접두사만 보면 안 된다. 워커가 런타임 함수(chan send 대기 중인
// selectruntime 같은 것)나 표준 라이브러리 내부 프레임에서 멈춰 있으면
// 최상단 이름이 main. 으로 시작하지 않는다. 그래서 블록 전체를 훑어
// 스택 어딘가에 main. 프레임이 있는지, 혹은 created by main. 인지를 본다.
// 테스트 프레임워크나 런타임이 만든 goroutine 까지 세면 거짓 양성이 쏟아져서
// 아무도 이 도구를 안 쓰게 된다. 그게 탐지기를 죽이는 가장 흔한 방식이다.
func isOurs(g GoroutineInfo) bool {
	if strings.Contains(g.Raw, "\nmain.") {
		return true
	}
	return strings.Contains(g.Raw, "created by main.")
}

// GSnapshot 은 특정 시점의 goroutine id 집합이다.
//
// 개수 비교가 아니라 id 비교를 쓰는 이유: 개수는 "하나 죽고 하나 생김"에 속는다.
// goid 는 sched.goidgen 에서 증가만 하므로 재사용되지 않는다. 그래서 차집합이 성립한다.
type GSnapshot map[uint64]struct{}

func takeSnapshot() GSnapshot {
	snap := make(GSnapshot)
	for _, g := range parseStacks(string(goroutineStacks())) {
		snap[g.ID] = struct{}{}
	}
	return snap
}

// newSince 는 base 이후 새로 생겨 지금도 살아 있는 "우리" goroutine 만 골라낸다.
func newSince(base GSnapshot) []GoroutineInfo {
	var out []GoroutineInfo
	for _, g := range parseStacks(string(goroutineStacks())) {
		if _, seen := base[g.ID]; seen {
			continue
		}
		if !isOurs(g) {
			continue
		}
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LeakChecker 는 goleak 이 하는 일의 뼈대다. 생성 시점 스냅숏을 잡아 두고,
// 나중에 덤프를 떠서 차집합을 보고한다.
type LeakChecker struct {
	base GSnapshot
}

func NewLeakChecker() *LeakChecker { return &LeakChecker{base: takeSnapshot()} }

func (lc *LeakChecker) Report() []GoroutineInfo { return newSince(lc.base) }

// waitFor 는 cond 가 참이 될 때까지 양보하며 기다린다.
//
// "100ms 자고 확인" 같은 고정 sleep 을 쓰지 않는 이유: 부하 걸린 CI 에서
// goroutine 시작이 느려지면 그 테스트는 이유 없이 깨진다. 조건을 기다려야 한다.
// goroutine 실행 순서와 시점은 스케줄러가 정한다. 우리가 정하지 않는다.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Microsecond)
	}
}

func printGoroutines(list []GoroutineInfo) {
	if len(list) == 0 {
		fmt.Println("  (없음)")
		return
	}
	for _, g := range list {
		fmt.Printf("  goroutine %-4d [%-11s] %s\n", g.ID, g.State, g.TopFunc)
	}
}

// ---------------------------------------------------------------------------
// 2. 일부러 새는 코드 — 3대 원인
// ---------------------------------------------------------------------------
//
// 세 함수 모두 started 채널을 함께 돌려준다. 데모/테스트에서 "고 띄웠으니
// 언젠가 실행되겠지" 하고 바로 덤프를 뜨면 아직 시작도 안 한 goroutine 을 보고
// 0개라고 착각한다. go 문은 goroutine 을 런큐에 넣고 즉시 반환할 뿐이다.
// 그래서 "내가 지금 달리고 있다"를 워커가 직접 알리게 만든다.
//
// release 는 데모를 정리하기 위한 탈출구다. 실제 버그에는 이런 게 없다.
// sync.Once 로 감싼 이유: t.Cleanup 과 직접 호출이 겹쳐도 두 번 닫거나
// 두 번 받으려다 deadlock/panic 이 나지 않게 하기 위해서다.

// 원인 1: 채널 송신 대기. 받는 쪽이 없으면 송신자는 영원히 park 한다.
func leakChannelSend() (started <-chan struct{}, release func()) {
	ch := make(chan int) // 버퍼 없음 = 송신자와 수신자가 만나야 진행
	s := make(chan struct{})
	var once sync.Once
	go func() {
		close(s)
		ch <- 1 // 받을 사람이 없으므로 여기서 영원히 멈춘다
	}()
	return s, func() { once.Do(func() { <-ch }) }
}

// 원인 1b: close 누락. range 는 채널이 닫혀야 끝난다.
func leakRangeNoClose() (started <-chan struct{}, release func()) {
	jobs := make(chan int)
	s := make(chan struct{})
	var once sync.Once
	go func() {
		close(s)
		for range jobs { // close 되지 않으면 range 는 끝나지 않는다
		}
	}()
	return s, func() { once.Do(func() { close(jobs) }) }
}

// 원인 2: context 취소 누락.
//
// WithCancel 은 goroutine 을 만들지 않는다. 새는 건 context 가 아니라
// "ctx.Done() 을 기다리는 goroutine"이다. 그리고 그 goroutine 이 사는 동안
// context 체인과 거기 매달린 값들이 전부 도달 가능 상태로 남는다.
// 즉 goroutine 누수는 거의 항상 메모리 누수를 동반한다.
func leakContextNeverCancelled() (started <-chan struct{}, release func()) {
	ctx, cancel := context.WithCancel(context.Background())
	s := make(chan struct{})
	var once sync.Once
	go func() {
		close(s)
		<-ctx.Done() // parent 가 Background 라 신호가 영원히 오지 않는다
	}()
	return s, func() { once.Do(cancel) }
}

// 원인 3: 종료 신호 누락. 루프는 stop 을 보지만, 아무도 닫지 않으면 같다.
func leakLoopStopNotSignaled() (started <-chan struct{}, release func()) {
	stop := make(chan struct{})
	s := make(chan struct{})
	var once sync.Once
	go func() {
		close(s)
		for {
			select {
			case <-stop:
				return
			default:
			}
			time.Sleep(time.Millisecond) // 일하는 척
		}
	}()
	return s, func() { once.Do(func() { close(stop) }) }
}

// ---------------------------------------------------------------------------
// 3. 고친 코드 — "워커가 끝난 것까지 확인하고" 반환한다
// ---------------------------------------------------------------------------
//
// 고치는 방법은 항상 같다: 종료를 보장하는 신호를 만들고, 신호를 보낸 뒤,
// 워커가 실제로 끝났다는 확인(done 채널 또는 WaitGroup)까지 기다린다.
// 마지막 확인을 빼먹으면 함수는 반환했는데 goroutine 은 아직 살아 있는
// "반쯤 고친" 상태가 된다. 테스트에서 이걸 잡으려고 조건 대기를 쓴다.

func fixedChannelSend() {
	ch := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ch <- 1
	}()
	<-ch // 받는 쪽을 같은 함수 안에서 보장한다
	wg.Wait()
}

func fixedRangeClose() {
	jobs := make(chan int, 4)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range jobs {
		}
	}()
	close(jobs) // 보내는 쪽이 끝났음을 알리는 유일한 방법
	wg.Wait()
}

func fixedContextCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 어떤 경로로 반환해도 자식 goroutine 을 깨운다
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
	}()
	cancel() // 실제 코드에서는 상위 요청 취소 지점에서 불린다
	<-done   // 워커가 정말 끝난 것까지 확인
}

func fixedLoopStop() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			time.Sleep(time.Millisecond)
		}
	}()
	close(stop)
	<-done
}

// ---------------------------------------------------------------------------
// 4. 데모 (main 은 호출만 한다)
// ---------------------------------------------------------------------------

func demoDumpFormat() {
	all := parseStacks(string(goroutineStacks()))
	fmt.Printf("현재 goroutine 총 %d개. 앞의 2개 블록을 원본 그대로 보여준다.\n", len(all))
	for i, g := range all {
		if i == 2 {
			break
		}
		for _, ln := range strings.Split(g.Raw, "\n") {
			fmt.Println("  " + ln)
		}
		fmt.Println()
	}
	fmt.Println("  -> 둘째 줄이 지금 멈춰 있는 함수, 마지막 'created by' 줄이 만든 쪽이다.")
}

// awaitReport 는 min 개 이상 보일 때까지 조건 대기하고 마지막 보고를 돌려준다.
func awaitReport(lc *LeakChecker, timeout time.Duration, min int) []GoroutineInfo {
	var last []GoroutineInfo
	waitFor(timeout, func() bool {
		last = lc.Report()
		return len(last) >= min
	})
	return last
}

func demoLeaks() {
	fmt.Println("[원인 1] 받는 쪽이 없는 채널 송신")
	lc := NewLeakChecker()
	started, release := leakChannelSend()
	<-started
	printGoroutines(awaitReport(lc, 2*time.Second, 1))
	fmt.Println("  -> CPU 는 안 먹는다. gopark 로 재워졌기 때문이다. 그래서 프로파일러에 안 보인다.")
	release()
	fmt.Printf("  release 후 남은 수: %d\n", len(awaitReport(lc, 2*time.Second, 0)))

	fmt.Println("[원인 1b] close 누락된 range")
	lc = NewLeakChecker()
	started, release = leakRangeNoClose()
	<-started
	printGoroutines(awaitReport(lc, 2*time.Second, 1))
	release()
	fmt.Printf("  release 후 남은 수: %d\n", len(awaitReport(lc, 2*time.Second, 0)))

	fmt.Println("[원인 2] cancel 이 불리지 않는 context")
	lc = NewLeakChecker()
	started, release = leakContextNeverCancelled()
	<-started
	printGoroutines(awaitReport(lc, 2*time.Second, 1))
	release()
	fmt.Printf("  release 후 남은 수: %d\n", len(awaitReport(lc, 2*time.Second, 0)))

	fmt.Println("[원인 3] stop 신호를 아무도 안 닫는 루프")
	lc = NewLeakChecker()
	started, release = leakLoopStopNotSignaled()
	<-started
	printGoroutines(awaitReport(lc, 2*time.Second, 1))
	release()
	fmt.Printf("  release 후 남은 수: %d\n", len(awaitReport(lc, 2*time.Second, 0)))
}

func demoFixed() {
	lc := NewLeakChecker()
	fixedChannelSend()
	fixedRangeClose()
	fixedContextCancel()
	fixedLoopStop()
	if waitFor(2*time.Second, func() bool { return len(lc.Report()) == 0 }) {
		fmt.Println("고친 버전 4종 실행 후 남은 goroutine: 0개")
		return
	}
	fmt.Println("고친 버전인데 남은 goroutine 이 있다:")
	printGoroutines(lc.Report())
}

func demoGoleakStyle() {
	fmt.Println("goleak 흉내: 시작 스냅숏 -> 끝 덤프 -> 차집합 -> created by 로 범인 지목")
	lc := NewLeakChecker()
	started, release := leakContextNeverCancelled()
	<-started
	report := awaitReport(lc, 2*time.Second, 1)
	for _, g := range report {
		fmt.Printf("  LEAK goroutine %d [%s] %s\n", g.ID, g.State, g.TopFunc)
		for _, ln := range strings.Split(g.Raw, "\n")[1:] {
			fmt.Println("      " + ln)
		}
	}
	release()
	fmt.Printf("  정리 후 남은 수: %d\n", len(awaitReport(lc, 2*time.Second, 0)))
}

func main() {
	fmt.Println("=== 1) 런타임이 주는 goroutine 덤프의 생김새 ===")
	demoDumpFormat()

	fmt.Println("\n=== 2) 새는 코드 3대 원인 ===")
	demoLeaks()

	fmt.Println("\n=== 3) 고친 코드 4종 ===")
	demoFixed()

	fmt.Println("\n=== 4) goleak 흉내 ===")
	demoGoleakStyle()
}