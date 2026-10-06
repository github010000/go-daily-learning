package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// observeCancel은 부모-자식-손자 트리에서 취소 신호가 어디까지 전파되는지 관찰한다.
//
// cancelChild가 true면 child를, false면 parent를 취소한다. context의 Done 채널은
// goroutine 여러 개가 동시에 기다려도 안전하게 닫히는 단방향 채널이므로,
// 여기서는 각 레벨의 Done이 닫힐 때 doneCh로 이름을 보내 취소 도달 범위를 확인한다.
//
// 채널 버퍼를 3으로 주는 이유는 각 감시 goroutine이 관찰자가 준비될 때까지
// 송신에서 블록하지 않게 하기 위해서다. 버퍼가 없으면 goroutine이 채널 송신에서
// 멈추어 마치 누수처럼 보일 수 있다.
func observeCancel(cancelChild bool) (parentDone, childDone, grandchildDone bool) {
	parent, parentCancel := context.WithCancel(context.Background())
	child, childCancel := context.WithCancel(parent)
	grandchild, grandchildCancel := context.WithCancel(child)

	doneCh := make(chan string, 3)

	spawn := func(name string, ctx context.Context) {
		go func() {
			<-ctx.Done()
			doneCh <- name
		}()
	}

	spawn("parent", parent)
	spawn("child", child)
	spawn("grandchild", grandchild)

	if cancelChild {
		// child만 취소한다. context 취소는 부모 방향으로는 전파되지 않으므로
		// grandchild는 취소되고 parent는 살아 있어야 한다.
		childCancel()

		// child와 grandchild 두 개의 완료 신호를 기다린다.
		for i := 0; i < 2; i++ {
			name := <-doneCh
			switch name {
			case "child":
				childDone = true
			case "grandchild":
				grandchildDone = true
			}
		}

		// parent는 아직 취소되지 않았어야 하므로 non-blocking select로 확인한다.
		// 이 시점에 parent가 잘못 취소되었다면 parent 완료 신호가 도착했을 것이다.
		select {
		case name := <-doneCh:
			if name == "parent" {
				parentDone = true
			}
		default:
			// parent가 살아 있다는 뜻이다.
		}

		// 남은 parent 완료 신호를 받아 goroutine을 정리한다.
		parentCancel()
		<-doneCh
	} else {
		// parent를 취소하면 cancelCtx의 children 맵을 따라 child와 grandchild가
		// 순서와 관계없이 모두 취소된다.
		parentCancel()

		for i := 0; i < 3; i++ {
			name := <-doneCh
			switch name {
			case "parent":
				parentDone = true
			case "child":
				childDone = true
			case "grandchild":
				grandchildDone = true
			}
		}
	}

	// context의 cancel 함수는 여러 번 불러도 안전하다. 이미 err가 설정되어 있으면
	// 아무 동작도 하지 않는다. 이 마지막 호출들은 모든 경로에서 canceler를 부르게 해
	// go vet의 lostcancel 검사를 통과시키면서 혹시 남을 수 있는 정리를 보장한다.
	parentCancel()
	childCancel()
	grandchildCancel()
	return
}

// measureLeakedChildren은 cancel을 부르지 않고 자식 context를 만들었을 때
// 부모가 자식을 계속 참조해 객체가 GC되지 않는 현상을 HeapObjects로 보여준다.
//
// 여기서는 go vet의 lostcancel 경고를 피하기 위해 각 childCancel을 defer로
// 예약해 둔다. defer는 함수가 반환된 뒤에 실행되므로, GC 측정 시점에는 아직
// cancel이 호출되지 않은 상태를 관찰할 수 있다. 실제 누수는 부모의 children 맵이
// 자식을 붙잡고 있는 것이며, defer 스택은 vet를 통과시키기 위한 장치일 뿐이다.
func measureLeakedChildren(n int) uint64 {
	if n <= 0 {
		n = 5000
	}

	parent, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	for i := 0; i < n; i++ {
		_, childCancel := context.WithCancel(parent)
		defer childCancel()
	}

	// 이 시점에는 childCancel들이 아직 실행되지 않았다. 부모의 children 맵과
	// defer 스택이 자식 canceler를 강하게 참조하고 있으므로 GC를 돌려도
	// 객체 수가 줄어들지 않는다.
	runtime.GC()
	runtime.ReadMemStats(&after)

	if after.HeapObjects > before.HeapObjects {
		return after.HeapObjects - before.HeapObjects
	}
	return 0
}

// measureWithTimeoutRetention은 WithTimeout으로 만든 context를
// cancel하지 않고 방치했을 때 타이머가 자원을 계속 붙잡는 모습을 보여준다.
//
// cancelEach가 true면 각 cancel을 즉시 호출해 타이머를 멈춘다.
// false면 cancel을 defer로 미뤄 GC 측정 순간에는 아직 타이머가 살아 있게 한다.
// time.Hour를 쓰지만 실제로 시간을 기다리지는 않는다. 관찰 시점이 핵심이기 때문이다.
func measureWithTimeoutRetention(n int, cancelEach bool) uint64 {
	if n <= 0 {
		n = 5000
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	if cancelEach {
		for i := 0; i < n; i++ {
			_, cancel := context.WithTimeout(context.Background(), time.Hour)
			cancel()
		}
	} else {
		for i := 0; i < n; i++ {
			_, cancel := context.WithTimeout(context.Background(), time.Hour)
			defer cancel()
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	if after.HeapObjects > before.HeapObjects {
		return after.HeapObjects - before.HeapObjects
	}
	return 0
}

func main() {
	fmt.Println("=== context 취소 전파 ===")
	p1, c1, g1 := observeCancel(true)
	fmt.Printf("child 취소 -> parent: %v, child: %v, grandchild: %v\n", p1, c1, g1)

	p2, c2, g2 := observeCancel(false)
	fmt.Printf("parent 취소 -> parent: %v, child: %v, grandchild: %v\n", p2, c2, g2)

	fmt.Println("\n=== cancel을 부르기 전까지 남는 참조 ===")
	retained := measureLeakedChildren(5000)
	fmt.Printf("자식 context %d개를 cancel 없이 만든 직후 HeapObjects 증가: %d\n", 5000, retained)

	fmt.Println("\n=== WithTimeout 타이머 정리 ===")
	withCancel := measureWithTimeoutRetention(5000, true)
	fmt.Printf("WithTimeout %d개 즉시 cancel: HeapObjects 증가 %d\n", 5000, withCancel)

	noCancel := measureWithTimeoutRetention(5000, false)
	fmt.Printf("WithTimeout %d개 cancel 지연(함수 종료까지 보류): HeapObjects 증가 %d\n", 5000, noCancel)
}