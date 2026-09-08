package main

import (
	"fmt"
	"runtime"
	"sync"
)

// publishWithChannel은 채널 close를 release로, receive를 acquire로 사용한다.
// write는 close보다 프로그램 순서상 앞에 있으므로 receive 이후에는 반드시 보인다.
// 이 함수는 data race 없이 결정적으로 n*2를 반환한다.
func publishWithChannel(n int) int {
	var v int
	done := make(chan struct{})

	go func() {
		v = n * 2
		// close는 happens-before의 release 역할을 한다.
		close(done)
	}()

	// receive가 위 close와 synchronizes-before 관계를 만든다.
	<-done
	return v
}

// publishWithMutex는 mutex의 Unlock/Lock이 주는 happens-before 보장을 보여준다.
// writer가 ready=true로 바꾼 뒤 Unlock하고, reader는 Lock 후 ready를 관찰한다.
// busy wait 형태지만 실제로는 runtime.Gosched로 CPU를 양보하므로 과도하게 돌지 않는다.
func publishWithMutex(n int) int {
	var mu sync.Mutex
	var v int
	var ready bool

	go func() {
		mu.Lock()
		v = n * 2
		ready = true
		mu.Unlock()
	}()

	for {
		mu.Lock()
		val := v
		ok := ready
		mu.Unlock()

		if ok {
			return val
		}
		runtime.Gosched()
	}
}

// concurrentOnce는 여러 goroutine이 같은 sync.Once를 호출할 때
// 최초 한 번의 Do 안에서 기록한 값이 나머지 모든 호출에게 보이는 것을 보여준다.
// 채널 send/recv는 모든 goroutine의 종료를 기다리기 위한 추가 동기화다.
func concurrentOnce(n int) int {
	var once sync.Once
	var v int
	const workers = 8

	done := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		go func() {
			// completion of once.Do(f)는 모든 once.Do(f) 호출 반환보다 앞선다.
			once.Do(func() {
				v = n * 2
			})
			done <- struct{}{}
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
	}
	return v
}

// unsafeIncrement는 일부러 동기화 없이 공유 카운터를 증가시킨다.
// 이 함수는 data race를 일으키며, go run -race로 실행하면 race detector가 잡는다.
// 여기서는 lost update가 실제로 발생할 수 있다는 점만 관찰한다.
func unsafeIncrement(workers, loops int) (actual, expected int) {
	var counter int
	done := make(chan struct{}, workers)

	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < loops; j++ {
				// 동기화 없는 read-modify-write는 원자적이지 않다.
				counter++
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < workers; i++ {
		<-done
	}
	return counter, workers * loops
}

// runSafeDemos는 올바른 동기화 수단이 결정적 결과를 주는 것을 출력으로 보여준다.
func runSafeDemos() {
	fmt.Println("safe: channel, mutex, once는 항상 기대값을 준다")
	for i := 1; i <= 5; i++ {
		fmt.Printf("channel id=%d -> %d\n", i, publishWithChannel(i))
		fmt.Printf("mutex   id=%d -> %d\n", i, publishWithMutex(i))
		fmt.Printf("once    id=%d -> %d\n", i, concurrentOnce(i))
	}
}

// runUnsafeDemo는 동기화가 없을 때 lost update가 발생하는 모습을 보여준다.
// 매번 재현되지는 않지만, race detector가 있다면 즉시 지적할 수 있는 코드다.
func runUnsafeDemo() {
	fmt.Println("\nunsafe: 동기화 없이 공유 카운터 증가")
	rounds := 5
	totalLost := 0

	for r := 0; r < rounds; r++ {
		actual, expected := unsafeIncrement(8, 1000)
		lost := expected - actual
		totalLost += lost
		fmt.Printf("round %d: actual=%d expected=%d lost=%d\n", r+1, actual, expected, lost)
	}

	if totalLost > 0 {
		fmt.Println("lost update 발생: happens-before가 없으면 원자적 갱신도 보장되지 않는다")
	} else {
		fmt.Println("이번 실행은 lost update가 없었다. 그래도 race detector는 data race를 보고한다")
	}
}

func main() {
	// unsafe demo의 경합 가능성을 높이기 위해 멀티코어를 사용한다.
	runtime.GOMAXPROCS(runtime.NumCPU())

	runSafeDemos()
	runUnsafeDemo()
}