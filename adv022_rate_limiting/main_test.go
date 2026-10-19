package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestChanSemaphoreLimit은 채널 세마포어가 동시 실행 수를 limit 이하로
// 제한하는지 검증한다. 순서가 아닌 최대 동시 실행 수라는 불변식을 본다.
func TestChanSemaphoreLimit(t *testing.T) {
	const limit = 3
	const jobs = 50
	sem := newChanSemaphore(limit)

	var active, maxActive int64
	var wg sync.WaitGroup

	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.acquire()
			defer sem.release()

			cur := atomic.AddInt64(&active, 1)
			updateMax(&maxActive, cur)
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()

	got := atomic.LoadInt64(&maxActive)
	if got > limit {
		t.Fatalf("동시 실행 수 %d이 limit %d를 초과했습니다", got, limit)
	}
}

// TestWeightedSemaphoreLimit은 weighted semaphore가 가중치 1 요청만으로도
// 동시 실행 수를 limit 이하로 제한하는지 검증한다.
func TestWeightedSemaphoreLimit(t *testing.T) {
	const limit = 3
	const jobs = 50
	sem := newWeightedSemaphore(limit)

	var active, maxActive int64
	var wg sync.WaitGroup

	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem.acquire(1)
			defer sem.release(1)

			cur := atomic.AddInt64(&active, 1)
			updateMax(&maxActive, cur)
			time.Sleep(time.Millisecond)
			atomic.AddInt64(&active, -1)
		}()
	}
	wg.Wait()

	got := atomic.LoadInt64(&maxActive)
	if got > limit {
		t.Fatalf("동시 실행 수 %d이 limit %d를 초과했습니다", got, limit)
	}
}

// TestTokenBucketReserveN은 token bucket의 대기 시간 계산이 정확한지 검증한다.
// 초기화 시각을 tb.last로 고정해 시간 변화에 의존하지 않게 만든다.
func TestTokenBucketReserveN(t *testing.T) {
	tb := newTokenBucket(1, 2) // 초당 1개, 버스트 2개

	now := tb.last // 결정적 테스트를 위해 초기화 시각을 그대로 사용한다.

	// 버킷이 가득 차 있으므로 2개는 즉시 허용되어야 한다.
	if d := tb.reserveN(now, 2); d != 0 {
		t.Fatalf("첫 요청 2개는 즉시 허용되어야 하는데 대기 시간 %v", d)
	}

	// 같은 시각에 1개 더 요청하면 토큰이 없어 1초를 기다려야 한다.
	if d := tb.reserveN(now, 1); d != time.Second {
		t.Fatalf("부족한 토큰 1개의 대기 시간 = %v, 원하는 값 %v", d, time.Second)
	}

	// 1초 뒤에는 토큰이 1개 보충되므로 즉시 허용된다.
	later := now.Add(time.Second)
	if d := tb.reserveN(later, 1); d != 0 {
		t.Fatalf("1초 후 1개 요청은 즉시 허용되어야 하는데 대기 시간 %v", d)
	}

	// 같은 시각에 추가 요청하면 다시 1초를 기다려야 한다.
	if d := tb.reserveN(later, 1); d != time.Second {
		t.Fatalf("추가 요청의 대기 시간 = %v, 원하는 값 %v", d, time.Second)
	}
}

// BenchmarkChanSemaphore는 채널 세마포어의 acquire/release 비용을 측정한다.
// 경합이 없는 조건에서 연산 자체의 오버헤드를 확인하기 위한 벤치마크다.
func BenchmarkChanSemaphore(b *testing.B) {
	sem := newChanSemaphore(100) // 충분히 큰 버퍼로 블로킹 없이 동작하게 한다.
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sem.acquire()
			sem.release()
		}
	})
}