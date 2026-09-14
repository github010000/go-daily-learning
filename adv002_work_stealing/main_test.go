package main

import "testing"

// TestEnqueueRunnextMovesOldSlotToRunq는 runnext의 기본 불변식을 검증한다.
// 새 작업이 runnext에 들어가면 기존 runnext는 runq tail로 밀려나고,
// popLocal은 runnext를 가장 먼저 꺼내야 한다.
func TestEnqueueRunnextMovesOldSlotToRunq(t *testing.T) {
	p := newProc(0)

	p.enqueue(QueueRunnext, Task{ID: 1, Producer: 0})
	p.enqueue(QueueRunnext, Task{ID: 2, Producer: 0})

	// runnext에는 가장 최근 작업인 2가 있어야 한다.
	if p.Runnext == nil || p.Runnext.ID != 2 {
		t.Fatalf("runnext should be task 2, got %+v", p.Runnext)
	}
	// 기존 runnext였던 1은 runq tail로 밀려났어야 한다.
	if len(p.Runq) != 1 || p.Runq[0].ID != 1 {
		t.Fatalf("runq should contain task 1, got %+v", p.Runq)
	}

	p.enqueue(QueueRunnext, Task{ID: 3, Producer: 0})
	if p.Runnext == nil || p.Runnext.ID != 3 {
		t.Fatalf("runnext should be task 3, got %+v", p.Runnext)
	}
	if len(p.Runq) != 2 || p.Runq[0].ID != 1 || p.Runq[1].ID != 2 {
		t.Fatalf("runq should be [1 2], got %+v", p.Runq)
	}

	// popLocal은 runnext를 먼저 꺼내므로 3, 1, 2 순서가 되어야 한다.
	got, ok := p.popLocal()
	if !ok || got.ID != 3 {
		t.Fatalf("first pop should be task 3, got %+v ok=%v", got, ok)
	}
	got, ok = p.popLocal()
	if !ok || got.ID != 1 {
		t.Fatalf("second pop should be task 1, got %+v ok=%v", got, ok)
	}
	got, ok = p.popLocal()
	if !ok || got.ID != 2 {
		t.Fatalf("third pop should be task 2, got %+v ok=%v", got, ok)
	}
}

// TestRunnextReducesHotTaskLatency는 같은 시나리오에서 runnext 모드가
// hot task를 더 일찍 실행하는지 검증한다.
// FIFO 모드에서는 hot task가 runq tail에서 대기하기 때문에 더 늦게 실행된다.
func TestRunnextReducesHotTaskLatency(t *testing.T) {
	fifo := runHotLatency(QueueFIFO)
	runnext := runHotLatency(QueueRunnext)

	if fifo.HotTick < 0 || runnext.HotTick < 0 {
		t.Fatal("hot task was not executed")
	}
	if runnext.HotTick >= fifo.HotTick {
		t.Fatalf("runnext should reduce latency, runnext tick=%d fifo tick=%d", runnext.HotTick, fifo.HotTick)
	}
	if fifo.Stolen < 1 {
		t.Fatalf("FIFO scenario should involve stealing, got %d", fifo.Stolen)
	}
}

// TestStealHalfTakesHeadBatch는 도둑 P가 희생 P의 runq 앞쪽 절반(올림)을
// 가져오는지 확인한다. Go 런타임은 runqgrab에서 n - n/2 공식을 사용한다.
func TestStealHalfTakesHeadBatch(t *testing.T) {
	victim := newProc(0)
	for i := 1; i <= 5; i++ {
		victim.Runq = append(victim.Runq, Task{ID: i, Producer: 0})
	}

	thief := newProc(1)
	stolen := thief.stealHalf(victim)

	// 5개 중 올림 절반은 3개다.
	if len(stolen) != 3 {
		t.Fatalf("steal half of 5 should be 3, got %d", len(stolen))
	}
	// runq head에서 훔쳐야 하므로 1,2,3이 stolen이어야 한다.
	if stolen[0].ID != 1 || stolen[1].ID != 2 || stolen[2].ID != 3 {
		t.Fatalf("stolen batch should be [1 2 3], got %+v", stolen)
	}
	if len(victim.Runq) != 2 || victim.Runq[0].ID != 4 || victim.Runq[1].ID != 5 {
		t.Fatalf("victim should have [4 5], got %+v", victim.Runq)
	}
}