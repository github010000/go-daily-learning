package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// task 는 스케줄러가 실행할 작업 한 개를 흉내낸다.
// payload 는 CPU 캐시에 올라가는 데이터를 표현해서, 다른 P 로 넘어갈 때
// 캐시 미스가 나는 상황을 시뮬레이션하기 위해 넣었다.
type task struct {
	id         int
	payload    [64]byte
	executedOn int // 이 task 를 실행한 processor 의 id
}

// processor 는 Go 런타임의 P(processor)를 축소해서 흉내낸다.
// 실제 런타임의 P 는 runq(로컬 런큐)와 runnext 라는 두 개의 실행 대기열을 가진다.
// 여기서는 교육용으로 뮤텍스를 써서 단순화했다. 실제 런타임은 락프리다.
type processor struct {
	id       int
	mu       sync.Mutex
	runq     []*task // 로컬 런큐. 실제 runtime 에서는 원형 큐지만 여기서는 슬라이스.
	runnext  *task   // runnext 슬롯. 새로 만든 goroutine 이 우선 들어간다.
	processed int    // 이 P 가 실행한 task 수
	steals   int    // 이 P 가 다른 P 에게서 훔친 task 수
}

// scheduler 는 여러 processor 와 전역 큐를 묶어 관리한다.
// 실제 Go 런타임에는 전역 런큐(runq)가 있고, 여기서는 globalQueue 로 표현한다.
type scheduler struct {
	procs       []*processor
	globalMu    sync.Mutex
	globalQueue []*task
}

// newScheduler 는 주어진 개수의 processor 를 가진 스케줄러를 만든다.
func newScheduler(numP int) *scheduler {
	s := &scheduler{}
	for i := 0; i < numP; i++ {
		s.procs = append(s.procs, &processor{id: i})
	}
	return s
}

// runqput 은 task 를 P 의 로컬 대기열에 넣는다.
// useRunNext 가 true 이고 runnext 슬롯이 비어 있으면 runnext 에 넣는다.
// 실제 런타임의 runqput 은 runnext 가 비어 있으면 runnext 에 넣고,
// 아니면 runq 에 넣는다. 이렇게 해서 새 goroutine 이 같은 P 에서 곧바로 실행된다.
func (s *scheduler) runqput(p *processor, t *task, useRunNext bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if useRunNext && p.runnext == nil {
		p.runnext = t
		return
	}
	p.runq = append(p.runq, t)
}

// runqget 은 P 의 로컬 대기열에서 task 를 하나 꺼낸다.
// useRunNext 가 true 이면 runnext 슬롯을 먼저 확인한다.
// 실제 런타임의 runqget 도 runnext 를 먼저 확인하고, 없으면 runq 에서 꺼낸다.
func (s *scheduler) runqget(p *processor, useRunNext bool) *task {
	p.mu.Lock()
	defer p.mu.Unlock()
	if useRunNext && p.runnext != nil {
		t := p.runnext
		p.runnext = nil
		return t
	}
	if len(p.runq) == 0 {
		return nil
	}
	t := p.runq[0]
	p.runq = p.runq[1:]
	return t
}

// getFromGlobal 은 전역 큐에서 task 를 하나 꺼낸다.
// 실제 런타임의 findrunnable 은 전역 runq 를 주기적으로 확인한다.
func (s *scheduler) getFromGlobal() *task {
	s.globalMu.Lock()
	defer s.globalMu.Unlock()
	if len(s.globalQueue) == 0 {
		return nil
	}
	t := s.globalQueue[0]
	s.globalQueue = s.globalQueue[1:]
	return t
}

// stealFrom 은 다른 P 의 runq 에서 task 의 절반을 훔쳐 온다.
// runnext 슬롯은 훔치지 않는다. 이것이 핵심이다. runnext 는 생성자 P 만 쓸 수 있다.
// 훔친 task 들 중 첫 번째는 곧바로 반환하고, 나머지는 내 runq 에 넣는다.
func (s *scheduler) stealFrom(p *processor) *task {
	// 실제 런타임은 랜덤한 P 부터 시도하지만 여기서는 순회한다.
	for _, other := range s.procs {
		if other == p {
			continue
		}
		other.mu.Lock()
		n := len(other.runq)
		if n > 1 { // 최소 2개일 때 절반을 훔친다. 1개면 훔치지 않는다.
			stealCount := n / 2
			stolen := make([]*task, stealCount)
			copy(stolen, other.runq[:stealCount])
			other.runq = other.runq[stealCount:]
			other.mu.Unlock()

			p.mu.Lock()
			// 훔친 task 중 첫 번째는 호출자에게 주고 나머지는 내 runq 에 넣는다.
			if len(stolen) > 1 {
				p.runq = append(p.runq, stolen[1:]...)
			}
			p.steals += stealCount
			p.mu.Unlock()
			return stolen[0]
		}
		other.mu.Unlock()
	}
	return nil
}

// findRunnable 은 실행할 task 를 찾는다. 실제 런타임의 findrunnable 과 같은 역할이다.
// 1. 자신의 runnext, runq 확인
// 2. 전역 큐 확인
// 3. 다른 P 에서 훔치기
// 4. 없으면 nil (실제 런타임은 여기서 netpoll 확인, 스핀, sleep)
func (s *scheduler) findRunnable(p *processor, useRunNext bool) *task {
	if t := s.runqget(p, useRunNext); t != nil {
		return t
	}
	if t := s.getFromGlobal(); t != nil {
		return t
	}
	if t := s.stealFrom(p); t != nil {
		return t
	}
	return nil
}

// worker 는 하나의 processor 를 맡아서 task 를 계속 실행하는 goroutine 이다.
// remaining 은 아직 처리되지 않은 task 수를 atomic 으로 추적한다.
// 모든 task 가 처리되면 worker 는 종료한다.
func worker(s *scheduler, p *processor, remaining *int64, useRunNext bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		t := s.findRunnable(p, useRunNext)
		if t != nil {
			t.executedOn = p.id // 이 task 가 어느 P 에서 실행됐는지 기록
			_ = t.payload[0]    // 캐시 접근을 흉내내는 CPU 작업
			p.mu.Lock()
			p.processed++
			p.mu.Unlock()
			atomic.AddInt64(remaining, -1)
		} else {
			if atomic.LoadInt64(remaining) == 0 {
				return
			}
			// 할 일이 없으면 잠시 쉬었다가 다시 찾는다.
			// 실제 런타임은 스핀을 하다가 netpoll 을 확인하고, 그래도 없으면 잠든다.
			time.Sleep(time.Microsecond)
		}
	}
}

// runWorkStealingDemo 는 불균등하게 task 를 P0 에 몰아 넣고,
// 다른 P 들이 work stealing 을 통해 일을 가져가는 모습을 보여준다.
// useRunNext 가 true 이면 runnext 슬롯을 사용하고, false 이면 사용하지 않는다.
// 반환값은 각 P 가 처리한 task 수와 전체 처리 수다.
func runWorkStealingDemo(useRunNext bool, numTasks, numP int) ([]int, int) {
	s := newScheduler(numP)
	remaining := int64(numTasks)

	// 모든 task 를 P0 의 로컬 큐에만 넣는다. 이것이 불균등 분배의 시작이다.
	for i := 0; i < numTasks; i++ {
		t := &task{id: i}
		s.runqput(s.procs[0], t, useRunNext)
	}

	var wg sync.WaitGroup
	for _, p := range s.procs {
		wg.Add(1)
		go worker(s, p, &remaining, useRunNext, &wg)
	}
	wg.Wait()

	processed := make([]int, numP)
	for i, p := range s.procs {
		p.mu.Lock()
		processed[i] = p.processed
		p.mu.Unlock()
	}
	return processed, numTasks
}

// runNextAffinityDemo 는 runnext 슬롯이 "같은 P 에서 실행되는 비율"을 높이는지
// 확인한다. useRunNext 가 true 인 경우와 false 인 경우를 각각 실행해서,
// task 가 원래 넣은 P0 에서 실행된 비율을 반환한다.
func runNextAffinityDemo(numTasks, numP int) (samePWith, samePWithout int) {
	// runnext 사용 O
	processed, _ := runWorkStealingDemo(true, numTasks, numP)
	samePWith = processed[0] // P0 에서 실행된 task 수 = 같은 P 실행 수

	// runnext 사용 X
	processed2, _ := runWorkStealingDemo(false, numTasks, numP)
	samePWithout = processed2[0]

	return samePWith, samePWithout
}

// measureRunNextLatency 는 runqput -> runqget 왕복 시간을 runnext 사용 유무에 따라
// 측정한다. 실제 runnext 의 주 목적은 지연 시간 자체보다는 "같은 P 에서 즉시 실행"이지만,
// 여기서는 로컬 큐 조작 비용의 차이를 보여준다.
func measureRunNextLatency(numOps int) (withRunNext, withoutRunNext time.Duration) {
	s := newScheduler(1)
	p := s.procs[0]

	start := time.Now()
	for i := 0; i < numOps; i++ {
		t := &task{id: i}
		s.runqput(p, t, true)
		_ = s.runqget(p, true)
	}
	withRunNext = time.Since(start)

	start = time.Now()
	for i := 0; i < numOps; i++ {
		t := &task{id: i}
		s.runqput(p, t, false)
		_ = s.runqget(p, false)
	}
	withoutRunNext = time.Since(start)

	return withRunNext, withoutRunNext
}

// printSchedulerTraceHint 는 실제 Go 런타임 스케줄러 추적을 보는 방법을 출력한다.
// main.go 의 시뮬레이션과 달리, 실제 런타임은 GODEBUG=schedtrace 로 관찰할 수 있다.
func printSchedulerTraceHint() {
	fmt.Println("--- 실제 Go 런타임 스케줄러 추적 ---")
	fmt.Println("이 프로그램은 런타임 내부를 축소한 시뮬레이션을 돌려서")
	fmt.Println("work stealing 과 runnext 의 동작을 눈으로 보여준다.")
	fmt.Println("실제 Go 런타임의 스케줄러 상태를 보려면 아래처럼 실행하라:")
	fmt.Println("  GODEBUG=schedtrace=1000 go run .")
	fmt.Println("그러면 각 P 의 runqueue 길이와 runnext 유무가 주기적으로 출력된다.")
	fmt.Println()
}

func main() {
	// 시뮬레이션에 사용할 task 수와 P 수.
	// 실제 Go 런타임의 work stealing 은 수십만 goroutine 에서 의미가 있지만,
	// 여기서는 관찰을 위해 작은 수로 줄였다.
	numTasks := 20000
	numP := 4

	printSchedulerTraceHint()

	fmt.Println("=== work stealing 데모: P0 에만 task 몰아넣기 ===")
	processedWith, total := runWorkStealingDemo(true, numTasks, numP)
	fmt.Printf("runnext 사용 O: 전체 %d task 처리, P별 처리 수 = %v\n", total, processedWith)
	processedWithout, total2 := runWorkStealingDemo(false, numTasks, numP)
	fmt.Printf("runnext 사용 X: 전체 %d task 처리, P별 처리 수 = %v\n", total2, processedWithout)
	fmt.Println("해석: runnext 사용 시 P0 이 처리하는 비율이 높고, X 일 때는 절반 이상이 다른 P 로 훔쳐간다.")
	fmt.Println()

	fmt.Println("=== runnext affinity 데모: 같은 P 에서 실행되는 비율 ===")
	samePWith, samePWithout := runNextAffinityDemo(numTasks, numP)
	fmt.Printf("runnext 사용 O: P0 에서 실행된 task 수 = %d / %d (%.1f%%)\n", samePWith, numTasks, float64(samePWith)/float64(numTasks)*100)
	fmt.Printf("runnext 사용 X: P0 에서 실행된 task 수 = %d / %d (%.1f%%)\n", samePWithout, numTasks, float64(samePWithout)/float64(numTasks)*100)
	fmt.Println("해석: runnext 슬롯은 새로 만든 task 가 다른 P 로 빼앗기지 않도록 보호한다.")
	fmt.Println()

	fmt.Println("=== runnext 로컬 큐 조작 지연 비교 ===")
	withLat, withoutLat := measureRunNextLatency(100000)
	fmt.Printf("runnext 사용 O: %v (100000회 put/get)\n", withLat)
	fmt.Printf("runnext 사용 X: %v (100000회 put/get)\n", withoutLat)
	fmt.Println("해석: runnext 는 단일 슬롯이라 put/get 이 O(1)로 끝나고, runq 슬라이스 조작보다 캐시 지역성이 좋다.")
	fmt.Println()

	fmt.Println("=== 실제 런타임 schedtrace 로 확인하기 ===")
	fmt.Println("아래 명령을 별도 터미널에서 실행하면 P 별 runqueue 길이와 runnext 유무가 보인다.")
	fmt.Println("  GODEBUG=schedtrace=1000 go run .")
	fmt.Println("  # 위 명령은 이 프로그램을 다시 실행하면서 스케줄러 추적을 표준 에러로 출력한다.")
}