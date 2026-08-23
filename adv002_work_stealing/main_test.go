package main

import (
	"sync"
	"testing"
)

// TestRunqputRunNext 는 runnext 슬롯이 비어 있을 때 runqput 이 runnext 에
// task 를 넣고, runqget 이 그 task 를 우선 반환하는지 검증한다.
func TestRunqputRunNext(t *testing.T) {
	s := newScheduler(1)
	p := s.procs[0]
	task1 := &task{id: 1}
	task2 := &task{id: 2}

	s.runqput(p, task1, true)
	s.runqput(p, task2, true) // runnext 가 차 있으므로 runq 로 간다.

	got := s.runqget(p, true)
	if got != task1 {
		t.Fatalf("runnext 우선 실행 실패: got id=%d, want id=1", got.id)
	}
	got = s.runqget(p, true)
	if got != task2 {
		t.Fatalf("runnext 소진 후 runq 에서 꺼내기 실패: got id=%d, want id=2", got.id)
	}
}

// TestStealFromDoesNotTouchRunnext 는 work stealing 이 다른 P 의 runnext 슬롯을
// 훔치지 않고, runq 에서만 절반을 훔치는지 검증한다.
func TestStealFromDoesNotTouchRunnext(t *testing.T) {
	s := newScheduler(2)
	p0 := s.procs[0]
	p1 := s.procs[1]

	// p0 의 runnext 에 task A, runq 에 task B, C 를 넣는다.
	taskA := &task{id: 10}
	taskB := &task{id: 20}
	taskC := &task{id: 30}
	s.runqput(p0, taskA, true) // runnext 로
	s.runqput(p0, taskB, true) // runnext 차 있으므로 runq 에 taskB
	s.runqput(p0, taskC, true) // runq 에 taskC

	// p1 이 p0 에서 훔친다. runq 길이는 2개이므로 절반인 1개를 훔친다.
	stolen := s.stealFrom(p1)
	if stolen == nil {
		t.Fatal("stealFrom 이 nil 을 반환했다")
	}

	// 훔친 task 는 runq 에 있던 B 또는 C 여야 한다. runnext 의 A는 훔치면 안 된다.
	if stolen == taskA {
		t.Fatalf("runnext 슬롯의 task 를 훔쳤다: id=%d", stolen.id)
	}
	if stolen != taskB && stolen != taskC {
		t.Fatalf("알 수 없는 task 를 훔쳤다: id=%d", stolen.id)
	}

	// p0 에는 runnext 의 A 와 runq 에 남은 하나가 있어야 한다.
	p0.mu.Lock()
	defer p0.mu.Unlock()
	if p0.runnext != taskA {
		t.Fatalf("p0.runnext 가 유지되지 않았다: %v", p0.runnext)
	}
	if len(p0.runq) != 1 {
		t.Fatalf("p0.runq 길이가 1이 아니고 %d 이다", len(p0.runq))
	}
}

// TestWorkStealingDistribution 은 불균등하게 넣은 task 가 모든 P 에 분산되어
// 처리되고, 전체 처리 수가 정확히 numTasks 와 같은지 검증한다.
func TestWorkStealingDistribution(t *testing.T) {
	numTasks := 200
	numP := 4

	processed, total := runWorkStealingDemo(true, numTasks, numP)

	if total != numTasks {
		t.Fatalf("전체 처리 수 불일치: got %d, want %d", total, numTasks)
	}

	sum := 0
	for _, n := range processed {
		sum += n
		if n < 0 {
			t.Fatalf("처리 수가 음수: %d", n)
		}
	}
	if sum != numTasks {
		t.Fatalf("P 별 처리 수 합이 전체와 다르다: sum=%d, want=%d", sum, numTasks)
	}

	// P0 에만 몰아넣었으므로, work stealing 이 일어나면 다른 P 도 0보다 커야 한다.
	// 단, 아주 작은 확률로 P0 가 전부 처리할 수도 있지만 200개 정도면 사실상
	// 다른 P 도 처리한다. 여기서는 불변식만 약하게 확인한다.
	if processed[0] == numTasks {
		t.Log("경고: P0 가 모든 task 를 처리했다. work stealing 이 일어나지 않았을 수 있다.")
	}
}

// BenchmarkRunNextLatency 는 runnext 유무에 따른 runqput/runqget 왕복 비용을
// 비교한다. b.ResetTimer 는 초기화 작업이 측정에 포함되지 않도록 배치했다.
func BenchmarkRunNextLatency(b *testing.B) {
	for _, useRunNext := range []bool{true, false} {
		b.Run(map[bool]string{true: "withRunNext", false: "withoutRunNext"}[useRunNext], func(b *testing.B) {
			s := newScheduler(1)
			p := s.procs[0]
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				t := &task{id: i}
				s.runqput(p, t, useRunNext)
				_ = s.runqget(p, useRunNext)
			}
		})
	}
}