package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// sharedState는 읽기 전용으로 다룰 상태다.
// 잠금 구현에 따라 같은 데이터를 읽는 속도가 어떻게 달라지는지 보기 위한 공용 구조다.
type sharedState struct {
	vals []int
}

// mutexState는 sync.Mutex로 공유 상태를 보호한다.
// 읽기 전용 경로라도 쓰기 잠금과 같은 직렬 구간을 만든다.
type mutexState struct {
	mu    sync.Mutex
	state *sharedState
}

// rwMutexState는 sync.RWMutex로 공유 상태를 보호한다.
// 읽기 전용 경로라면 RLock을 쓰는 것이 자연스러워 보인다.
type rwMutexState struct {
	mu    sync.RWMutex
	state *sharedState
}

func newSharedState(size int) *sharedState {
	s := &sharedState{vals: make([]int, size)}
	for i := range s.vals {
		s.vals[i] = i + 1
	}
	return s
}

func newMutexState(size int) *mutexState {
	return &mutexState{state: newSharedState(size)}
}

func newRWMutexState(size int) *rwMutexState {
	return &rwMutexState{state: newSharedState(size)}
}

// readSumMutex는 짧은 읽기 구간을 sync.Mutex로 보호하는 올바른 선택이다.
// 실제 계산이 몇 개의 정수를 더하는 정도라면, Mutex의 빠른 CAS 경로가
// RWMutex의 readerCount 원자 연산보다 오버헤드가 작을 수 있다.
func (m *mutexState) readSumMutex() int {
	m.mu.Lock()
	sum := 0
	for _, v := range m.state.vals {
		sum += v
	}
	m.mu.Unlock()
	return sum
}

// readSumRWMutex는 RLock을 쓰지만, 짧은 구간에서는 오히려 느릴 수 있다.
// RLock은 단순 읽기처럼 보여도 readerCount를 원자적으로 증가시키고,
// 모든 reader goroutine이 같은 캐시 라인을 무효화시키기 때문이다.
func (r *rwMutexState) readSumRWMutex() int {
	r.mu.RLock()
	sum := 0
	for _, v := range r.state.vals {
		sum += v
	}
	r.mu.RUnlock()
	return sum
}

// runReadBenchmark는 정해진 시간 동안 읽기 전용 작업을 반복 실행하고
// 완료한 연산 수를 반환한다. 시간 기반 측정은 시연용이며 테스트에는 쓰지 않는다.
func runReadBenchmark(label string, workers, dataSize int, useRWMutex bool, duration time.Duration) uint64 {
	var ops atomic.Uint64
	var wg sync.WaitGroup

	deadline := time.Now().Add(duration)

	if useRWMutex {
		r := newRWMutexState(dataSize)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(deadline) {
					r.readSumRWMutex()
					ops.Add(1)
				}
			}()
		}
	} else {
		m := newMutexState(dataSize)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(deadline) {
					m.readSumMutex()
					ops.Add(1)
				}
			}()
		}
	}

	wg.Wait()
	fmt.Printf("%-20s dataSize=%4d workers=%2d ops=%d ops/sec=%.0f\n",
		label, dataSize, workers, ops.Load(), float64(ops.Load())/duration.Seconds())
	return ops.Load()
}

// findBoundary는 짧은/중간/긴 읽기 구간을 함께 측정해
// 어떤 지점부터 RWMutex가 Mutex를 이기는지 관찰하게 해준다.
func findBoundary(duration time.Duration) {
	workers := runtime.GOMAXPROCS(0)
	fmt.Printf("읽기 전용 작업 처리량 비교 (GOMAXPROCS=%d)\n", workers)

	// dataSize가 작을수록 임계 구간이 짧다.
	// 4와 128은 잠금 오버헤드가 실제 계산보다 큰 영역이다.
	// 4096은 계산 시간이 충분히 길어져 reader 병렬성이 살아나는 영역이다.
	for _, size := range []int{4, 128, 4096} {
		fmt.Printf("\n[dataSize=%d]\n", size)
		runReadBenchmark("Mutex.Lock", workers, size, false, duration)
		runReadBenchmark("RWMutex.RLock", workers, size, true, duration)
	}
}

func main() {
	// 500ms는 시연 시간을 짧게 유지하면서도 경향을 보기에 충분하다.
	findBoundary(500 * time.Millisecond)
}