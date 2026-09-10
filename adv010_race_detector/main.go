package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// vectorClockDemo는 ThreadSanitizer가 사용하는 벡터 클록 비교를
// 아주 작은 2개 goroutine 예시로 재구성한다.
// 실제 race detector의 내부 클록은 더 크고 goroutine 생성/채널/mutex마다
// 갱신되지만, 여기서는 "동시적 접근"과 "동기화로 순서가 생긴 접근"의
// 판정 기준이 어떻게 달라지는지 보여준다.
func vectorClockDemo() string {
	type access struct {
		name  string
		clock [2]int
		write bool
	}

	lessOrEqual := func(a, b [2]int) bool {
		return a[0] <= b[0] && a[1] <= b[1]
	}
	relation := func(a, b [2]int) string {
		switch {
		case lessOrEqual(a, b) && lessOrEqual(b, a):
			return "same"
		case lessOrEqual(a, b):
			return "a happened-before b"
		case lessOrEqual(b, a):
			return "b happened-before a"
		default:
			return "concurrent"
		}
	}
	formatClock := func(c [2]int) string {
		return fmt.Sprintf("[%d %d]", c[0], c[1])
	}

	var b strings.Builder

	unsyncWrites := []access{
		{name: "g1 write x", clock: [2]int{1, 0}, write: true},
		{name: "g2 write x", clock: [2]int{0, 1}, write: true},
	}
	b.WriteString("1) no sync between two writes to x\n")
	for _, a := range unsyncWrites {
		b.WriteString(fmt.Sprintf("   %-12s clock=%s\n", a.name, formatClock(a.clock)))
	}
	rel := relation(unsyncWrites[0].clock, unsyncWrites[1].clock)
	b.WriteString(fmt.Sprintf("   relation: %s => race? %v\n\n", rel, rel == "concurrent"))

	syncWrites := []access{
		{name: "g1 write x", clock: [2]int{1, 0}, write: true},
		{name: "g1 send ch", clock: [2]int{2, 0}, write: false},
		{name: "g2 recv ch", clock: [2]int{2, 2}, write: false},
		{name: "g2 write x", clock: [2]int{2, 3}, write: true},
	}
	b.WriteString("2) channel sync orders g1 write before g2 write\n")
	for _, a := range syncWrites {
		b.WriteString(fmt.Sprintf("   %-12s clock=%s\n", a.name, formatClock(a.clock)))
	}
	rel = relation(syncWrites[0].clock, syncWrites[3].clock)
	b.WriteString(fmt.Sprintf("   relation: %s => race? %v\n", rel, rel == "concurrent"))

	return b.String()
}

// sumConcurrentlyUnsafe는 의도적으로 data race를 만든다.
// counter++은 읽기-수정-쓰기 세 단계로 이루어지고,
// 여러 goroutine이 잠금 없이 수행하면 갱신이 유실된다.
// 이 함수는 -race 실행에서 보고를 만들기 위한 시연 전용이다.
func sumConcurrentlyUnsafe(goroutines int) int64 {
	var counter int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				counter++
			}
		}()
	}
	wg.Wait()
	return counter
}

// sumConcurrentlySafe는 같은 카운터 증가를 atomic.AddInt64로 수행한다.
// atomic 연산은 단일 메모리 트랜잭션이므로 유실이 없고,
// race detector도 동기화된 접근으로 판정한다.
func sumConcurrentlySafe(goroutines int) int64 {
	var counter int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				atomic.AddInt64(&counter, 1)
			}
		}()
	}
	wg.Wait()
	return counter
}

// conditionalRaceDemo는 "실행된 경로만 잡는다"는 race detector의 한계를 보여준다.
// trigger가 false면 data에 대한 쓰기 경로가 아예 실행되지 않으므로
// -race로 돌려도 이 부분에서는 race 보고가 나오지 않는다.
// trigger가 true면 unsynchronized write와 read가 함께 실행되어 보고가 나온다.
// time.Sleep은 데모에서 실행 기회를 주기 위한 것일 뿐 happens-before를 만들지 않는다.
func conditionalRaceDemo(trigger bool) {
	var data int
	go func() {
		if trigger {
			data = 42
		}
	}()
	time.Sleep(time.Millisecond)
	_ = data
}

func main() {
	fmt.Println("=== race detector: vector clock demo ===")
	fmt.Println(vectorClockDemo())

	fmt.Println("=== safe vs unsafe concurrent counter ===")
	const goroutines = 5
	// -race 없이 실행하면 unsafe 결과가 5000보다 작게 나오는 경우가 많다.
	// -race로 실행하면 아래 두 줄 사이에서 race 보고가 출력된다.
	fmt.Printf("unsafe result: %d (expected %d)\n", sumConcurrentlyUnsafe(goroutines), goroutines*1000)
	fmt.Printf("safe result  : %d\n", sumConcurrentlySafe(goroutines))

	fmt.Println("=== conditional race demo ===")
	conditionalRaceDemo(false)
	fmt.Println("trigger=false finished without race report")
}