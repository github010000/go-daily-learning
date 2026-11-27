package main

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// gen 은 파이프라인의 첫 단계입니다. 값을 내보내는 쪽(송신자)이 채널을 닫는 것이 Go 의 관례입니다.
// done 이 닫히면 받는 쪽이 떠났다는 뜻이므로 즉시 반환해야 고루틴이 새지 않습니다.
// active 는 살아 있는 고루틴 수를 세기 위한 것으로, 누수를 눈으로 확인하려는 용도입니다.
// done 에 nil 을 넘기면 그 case 는 영원히 준비되지 않으므로 "done 이 없는 버전"과 같아집니다.
func gen(done <-chan struct{}, active *int64, nums ...int) <-chan int {
	out := make(chan int)
	atomic.AddInt64(active, 1)
	go func() {
		defer atomic.AddInt64(active, -1)
		defer close(out)
		for _, n := range nums {
			select {
			case out <- n:
			case <-done:
				return
			}
		}
	}()
	return out
}

// sq 는 값을 제곱하는 중간 단계입니다. 입력 채널이 닫히면 for range 가 끝나고 out 도 닫힙니다.
func sq(done <-chan struct{}, active *int64, in <-chan int) <-chan int {
	out := make(chan int)
	atomic.AddInt64(active, 1)
	go func() {
		defer atomic.AddInt64(active, -1)
		defer close(out)
		for n := range in {
			select {
			case out <- n * n:
			case <-done:
				return
			}
		}
	}()
	return out
}

// merge 는 fan-in 입니다. 채널마다 고루틴을 하나씩 두고, 전부 끝난 뒤에 out 을 한 번만 닫습니다.
// 닫는 책임을 WaitGroup 하나에 모으면 "close of closed channel" panic 을 피할 수 있습니다.
func merge(done <-chan struct{}, active *int64, cs ...<-chan int) <-chan int {
	var wg sync.WaitGroup
	out := make(chan int)
	forward := func(c <-chan int) {
		defer wg.Done()
		defer atomic.AddInt64(active, -1)
		for v := range c {
			select {
			case out <- v:
			case <-done:
				return
			}
		}
	}
	wg.Add(len(cs))
	atomic.AddInt64(active, int64(len(cs))+1)
	for _, c := range cs {
		go forward(c)
	}
	// Wait 을 고루틴 안에서 부르는 이유: 여기서 직접 부르면 out 을 읽을 사람이 없어 교착됩니다.
	go func() {
		defer atomic.AddInt64(active, -1)
		wg.Wait()
		close(out)
	}()
	return out
}

// collect 는 채널이 닫힐 때까지 모두 읽어 슬라이스로 돌려줍니다.
func collect(c <-chan int) []int {
	var res []int
	for v := range c {
		res = append(res, v)
	}
	return res
}

// Pipeline 은 gen -> sq -> sq 를 한 줄로 연결합니다. 단계가 하나뿐이라 순서가 보존됩니다.
func Pipeline(nums []int) []int {
	var active int64
	done := make(chan struct{})
	defer close(done)
	return collect(sq(done, &active, sq(done, &active, gen(done, &active, nums...))))
}

// FanOut 은 같은 입력 채널을 workers 개의 sq 가 나눠 읽고(fan-out), merge 로 모읍니다(fan-in).
// 작업자마다 속도가 다르므로 결과 순서는 보장되지 않아 정렬해서 돌려줍니다.
func FanOut(nums []int, workers int) []int {
	var active int64
	done := make(chan struct{})
	defer close(done)
	in := gen(done, &active, nums...)
	outs := make([]<-chan int, workers)
	for i := range outs {
		outs[i] = sq(done, &active, in)
	}
	res := collect(merge(done, &active, outs...))
	sort.Ints(res)
	return res
}

// waitZero 는 살아 있는 고루틴 수가 0 이 될 때까지 최대 timeout 동안 기다린 뒤 현재 값을 돌려줍니다.
func waitZero(active *int64, timeout time.Duration) int64 {
	deadline := time.Now().Add(timeout)
	for atomic.LoadInt64(active) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	return atomic.LoadInt64(active)
}

// LeakyFirst 는 실수 버전입니다. done 없이 첫 값만 읽고 떠나면 gen 은 다음 send 에서 영원히 막힙니다.
func LeakyFirst(nums []int) (int, int64) {
	var active int64
	first := <-gen(nil, &active, nums...)
	return first, atomic.LoadInt64(&active)
}

// SafeFirst 는 고친 버전입니다. 필요한 만큼만 읽고 done 을 닫으면 상류 고루틴이 모두 종료됩니다.
func SafeFirst(nums []int) (int, int64) {
	var active int64
	done := make(chan struct{})
	first := <-sq(done, &active, gen(done, &active, nums...))
	close(done)
	return first, waitZero(&active, 2*time.Second)
}

func demoPipeline() {
	fmt.Println("파이프라인 (1..5 를 두 번 제곱):", Pipeline([]int{1, 2, 3, 4, 5}))
}

func demoFanOut() {
	nums := []int{1, 2, 3, 4, 5, 6, 7, 8}
	fmt.Println("fan-out 1 명:", FanOut(nums, 1))
	fmt.Println("fan-out 4 명:", FanOut(nums, 4))
}

func demoLeak() {
	v, n := LeakyFirst([]int{1, 2, 3})
	fmt.Printf("실수 버전: 첫 값 %d 을 받고 떠난 뒤 살아 있는 고루틴 = %d (영원히 막혀 있음)\n", v, n)
	v, n = SafeFirst([]int{1, 2, 3})
	fmt.Printf("고친 버전: 첫 값 %d 을 받고 done 을 닫은 뒤 살아 있는 고루틴 = %d\n", v, n)
}

func main() {
	demoPipeline()
	demoFanOut()
	demoLeak()
}