package main

import (
	"fmt"
	"sync"
	"time"
)

// fanOutFanIn: 여러 고루틴으로 작업을 분산 처리(Fan-out) 후 결과를 하나의 채널로 수집(Fan-in)
func fanOutFanIn(done <-chan struct{}, nums []int, out chan<- int) {
	var wg sync.WaitGroup
	// 각 숫자에 대해 고루틴 생성하여 병렬 처리
	for _, n := range nums {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			// done 채널로 취소 신호를 감지하거나 정상 결과 전송
			select {
			case <-done:
				return
			case out <- val * val:
			}
		}(n)
	}
	// 모든 고루틴이 완료되면 out 채널을 닫아 수신자가 반복을 안전하게 종료
	go func() {
		wg.Wait()
		close(out)
	}()
}

// pipeline: 채널을 연결해 데이터 처리 단계를 거치는 패턴
func pipeline(done <-chan struct{}, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			select {
			case <-done:
				return
			case out <- v + 1:
			}
		}
	}()
	return out
}

// semaphorePattern: 제한된 동시성으로 작업을 처리하는 패턴
func semaphorePattern(done <-chan struct{}, sem chan struct{}, tasks []int) []int {
	results := make([]int, 0, len(tasks))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, t := range tasks {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			sem <- struct{}{}        // 세마포어 획득 (동시성 제한)
			defer func() { <-sem }() // 고루틴 종료 시 세마포어 반납

			select {
			case <-done:
				return
			default:
			}
			// 실제 작업 시뮬레이션
			time.Sleep(100 * time.Millisecond)
			mu.Lock()
			results = append(results, val*2)
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	return results
}

func main() {
	// 1. Fan-out / Fan-in 패턴
	fmt.Println("=== 1. Fan-out / Fan-in 패턴 ===")
	done1 := make(chan struct{})
	nums := []int{1, 2, 3, 4, 5}
	out1 := make(chan int)
	fanOutFanIn(done1, nums, out1)
	for v := range out1 {
		fmt.Printf("제곱 결과: %d ", v)
	}
	fmt.Println()

	// 2. Pipeline 패턴
	fmt.Println("\n=== 2. Pipeline 패턴 ===")
	inChan := make(chan int)
	go func() {
		for _, n := range []int{10, 20, 30} {
			inChan <- n
		}
		close(inChan)
	}()
	step1 := pipeline(done1, inChan)
	for v := range step1 {
		fmt.Printf("파이프라인 변환 결과: %d ", v)
	}
	fmt.Println()

	// 3. Done 채널을 이용한 취소 패턴
	fmt.Println("\n=== 3. Done 채널을 통한 취소 패턴 ===")
	doneCancel := make(chan struct{})
	cancelIn := make(chan int, 3)
	cancelOut := make(chan int)
	// 데이터 생성과 동시에 취소 신호를 보내는 시뮬레이션
	go func() {
		cancelIn <- 100
		cancelIn <- 200
		close(cancelIn)
		time.Sleep(200 * time.Millisecond)
		close(doneCancel) // 처리 중 취소 신호 전달
	}()
	cancelStep := pipeline(doneCancel, cancelIn)
	for v := range cancelStep {
		fmt.Printf("취소 테스트 결과: %d ", v)
	}
	fmt.Println()

	// 4. 세마포어 패턴
	fmt.Println("\n=== 4. 세마포어 패턴 ===")
	doneSem := make(chan struct{})
	tasks := []int{1, 2, 3, 4, 5}
	sem := make(chan struct{}, 2) // 최대 동시 2개 작업 허용
	results := semaphorePattern(doneSem, sem, tasks)
	fmt.Printf("세마포어 처리 결과: %v\n", results)
}