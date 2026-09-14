package main

import (
	"fmt"
	"runtime"
)

// Go 런타임의 work stealing과 runnext를 관찰하기 위한 작은 모델이다.
// 실제 runtime/proc.go를 그대로 가져온 것은 아니지만, runq와 runnext가
// 어떻게 스케줄링 지연을 바꾸는지 결정적으로 보여준다.
//
// 이 코드는 P가 2개뿐인 상황에서 한 P에 오래된 작업 10개가 쌓여 있고,
// 그중 첫 작업이 "hot task"를 새로 만들어 내는 시나리오를 재현한다.
// hot task는 channel close, mutex unlock처럼 방금 실행 가능해진 goroutine에
// 해당한다. runnext가 있으면 hot task가 같은 P에서 즉시 실행되고,
// 없으면 runq tail에 붙어 앞선 작업들이 처리될 때까지 기다린다.

const (
	hotTaskID     = 999
	triggerTaskID = 1
)

// QueueMode는 지역 큐에 작업을 넣는 두 가지 방식을 나타낸다.
// QueueFIFO는 runnext가 없는 단순 FIFO 모델이고,
// QueueRunnext는 Go 런타임이 실제로 사용하는 runq + runnext 모델이다.
type QueueMode int

const (
	QueueFIFO QueueMode = iota
	QueueRunnext
)

func (m QueueMode) String() string {
	if m == QueueRunnext {
		return "runnext"
	}
	return "FIFO"
}

type Task struct {
	ID       int
	Producer int // 이 작업을 만든 P ID
}

// Proc는 Go 런타임의 p 구조체에서 runq와 runnext만 간추린 것이다.
// 실제 runtime/runtime2.go의 p는 runq [256]guintptr, runqhead, runqtail,
// runnext guintptr를 가진다.
type Proc struct {
	ID      int
	Runq    []Task
	Runnext *Task
}

func newProc(id int) *Proc {
	return &Proc{ID: id}
}

func (p *Proc) hasWork() bool {
	return p.Runnext != nil || len(p.Runq) > 0
}

// enqueue는 runtime/proc.go의 runqput에 해당한다.
// Go 런타임은 next가 true이면 먼저 runnext 슬롯을 채우고,
// 기존 runnext가 있으면 runq tail로 밀어낸다.
// FIFO 모드에서는 그냥 runq tail에만 넣는다.
func (p *Proc) enqueue(mode QueueMode, t Task) {
	if mode == QueueRunnext {
		if p.Runnext == nil {
			p.Runnext = &t
		} else {
			// 이미 runnext가 있으면 기존 값을 runq 뒤로 보내고
			// 새 작업을 runnext에 둔다. 이것이 최근 작업 우선 실행의 핵심이다.
			p.Runq = append(p.Runq, *p.Runnext)
			p.Runnext = &t
		}
		return
	}
	p.Runq = append(p.Runq, t)
}

// popLocal은 runtime/proc.go의 runqget에 해당한다.
// runnext를 먼저 확인하고, 없으면 runq head에서 가져온다.
func (p *Proc) popLocal() (Task, bool) {
	if p.Runnext != nil {
		t := *p.Runnext
		p.Runnext = nil
		return t, true
	}
	if len(p.Runq) == 0 {
		return Task{}, false
	}
	t := p.Runq[0]
	p.Runq = p.Runq[1:]
	return t, true
}

// stealHalf는 runtime/proc.go의 runqgrab이 하는 일을 단순화한 것이다.
// 실제 runqgrab은 희생 P의 runq에서 올림 절반(n - n/2)을 가져온다.
// 여기서는 slice 복사로 같은 효과를 낸다.
func (p *Proc) stealHalf(victim *Proc) []Task {
	n := len(victim.Runq)
	if n == 0 {
		return nil
	}
	steal := (n + 1) / 2 // ceil(n/2). 실제 runqgrab은 n = t-h; n = n - n/2
	stolen := make([]Task, steal)
	copy(stolen, victim.Runq[:steal])
	victim.Runq = victim.Runq[steal:]
	return stolen
}

type TraceEvent struct {
	Tick   int
	P      int
	TaskID int
	Note   string
}

func (e TraceEvent) String() string {
	return fmt.Sprintf("tick=%2d P=%d task=%4d %s", e.Tick, e.P, e.TaskID, e.Note)
}

type LatencyResult struct {
	Mode    QueueMode
	HotTick int
	Stolen  int
	Events  []TraceEvent
}

// runHotLatency는 P 2개짜리 결정적 시나리오를 실행해
// hot task가 몇 번째 tick에 실행되는지 기록한다.
//
// 초기 상태: P0 runq에 task 1..10이 들어 있다. P1은 비어 있다.
// tick 0: P0이 task 1을 실행하면서 hot task(#999)를 같은 P에 enqueue 한다.
//         P1은 idle이므로 P0의 runq에서 절반을 훔쳐 first task를 즉시 실행한다.
// 그 뒤부터는 각 P가 local work를 하나씩 실행한다.
//
// runnext 모드에서는 hot task가 P0의 runnext에 들어가므로
// 다음 tick에서 P0이 즉시 실행한다.
// FIFO 모드에서는 hot task가 P0의 runq tail에 붙으므로
// 앞에 남은 7,8,9,10이 먼저 실행된 뒤에야 실행된다.
func runHotLatency(mode QueueMode) LatencyResult {
	p0 := newProc(0)
	p1 := newProc(1)

	// P0 runq에 task 1..10을 순서대로 채운다. task 1이 head다.
	for i := 1; i <= 10; i++ {
		p0.Runq = append(p0.Runq, Task{ID: i, Producer: 0})
	}

	res := LatencyResult{
		Mode:    mode,
		HotTick: -1,
	}

	tick := 0
	for tick < 100 {
		// P0이 local work를 하나 실행한다.
		if t, ok := p0.popLocal(); ok {
			res.Events = append(res.Events, TraceEvent{tick, 0, t.ID, "run"})
			if t.ID == hotTaskID {
				res.HotTick = tick
				break
			}
			if t.ID == triggerTaskID {
				// 첫 tick에서 hot task를 만들어 낸다.
				p0.enqueue(mode, Task{ID: hotTaskID, Producer: 0})
				res.Events = append(res.Events, TraceEvent{tick, 0, hotTaskID, fmt.Sprintf("enqueue via %s", mode)})
			}
		}

		// P1이 local work를 하나 실행하거나, idle이면 P0에서 훔친다.
		if t, ok := p1.popLocal(); ok {
			res.Events = append(res.Events, TraceEvent{tick, 1, t.ID, "run"})
			if t.ID == hotTaskID {
				res.HotTick = tick
				break
			}
			if t.ID == triggerTaskID {
				p1.enqueue(mode, Task{ID: hotTaskID, Producer: 1})
				res.Events = append(res.Events, TraceEvent{tick, 1, hotTaskID, fmt.Sprintf("enqueue via %s", mode)})
			}
		} else {
			// P1이 idle이다. 실제 런타임은 fastrand로 희생 P를 고르지만,
			// 여기서는 P가 두 개뿐이므로 P0을 고른다.
			stolen := p1.stealHalf(p0)
			if len(stolen) > 0 {
				res.Stolen += len(stolen)
				first := stolen[0]
				// 실제 runqsteal은 훔친 batch 중 하나를 즉시 실행하고
				// 나머지를 자신의 runq tail에 붙인다.
				res.Events = append(res.Events, TraceEvent{tick, 1, first.ID, fmt.Sprintf("steal+run first, stole %d", len(stolen))})
				if first.ID == hotTaskID {
					res.HotTick = tick
					break
				}
				if first.ID == triggerTaskID {
					p1.enqueue(mode, Task{ID: hotTaskID, Producer: 1})
				}
				p1.Runq = append(p1.Runq, stolen[1:]...)
			} else if p0.Runnext != nil {
				// 실제 runqgrab은 runq가 비었을 때만 runnext를 훔친다.
				// 이 시나리오에서는 P0 runq가 항상 남아 있어 실행되지 않는다.
				// 하지만 runnext가 있는데 runq만 비어 있는 경우를 대비해
				// 코드 위치를 남겨 둔다.
			}
		}

		tick++
	}

	return res
}

func printRuntimeInfo() {
	fmt.Println("Go scheduler work stealing + runnext 관찰 모델")
	fmt.Printf("GOMAXPROCS=%d NumCPU=%d\n", runtime.GOMAXPROCS(0), runtime.NumCPU())
	fmt.Println("이 모델은 P 2개짜리 결정적 시나리오로 runnext가 hot task 지연을 줄이는 모습을 보여줍니다.")
	fmt.Println("실제 런타임의 schedtrace를 보려면: GODEBUG=schedtrace=1000 go run .")
}

func printResult(r LatencyResult) {
	fmt.Printf("\n[%s mode]\n", r.Mode)
	for _, e := range r.Events {
		fmt.Println("  ", e.String())
	}
	fmt.Printf("hot task(#%d) executed at tick=%d, stolen tasks=%d\n", hotTaskID, r.HotTick, r.Stolen)
}

func main() {
	printRuntimeInfo()

	fifo := runHotLatency(QueueFIFO)
	runnext := runHotLatency(QueueRunnext)

	printResult(fifo)
	printResult(runnext)

	fmt.Println()
	fmt.Println("해석:")
	fmt.Println("  runnext가 있으면 hot task가 한 tick 만에 같은 P에서 실행된다.")
	fmt.Println("  FIFO만 있으면 hot task가 runq tail에 붙어 앞선 작업들이 처리될 때까지 기다린다.")
}