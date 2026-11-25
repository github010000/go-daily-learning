package main

import (
	"fmt"
	"sync"
	"testing"
)

// 고친 카운터(Mutex, atomic)가 여러 규모에서 항상 정확한 합을 내는지 확인합니다.
// 실수 버전의 결과는 매번 달라서 단정하지 않습니다.
func TestCountersExact(t *testing.T) {
	cases := []struct {
		workers, per int
	}{
		{1, 10},
		{8, 500},
		{50, 200},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%dx%d", tc.workers, tc.per), func(t *testing.T) {
			want := tc.workers * tc.per
			if got := RunSafe(tc.workers, tc.per); got != want {
				t.Errorf("RunSafe = %d, want %d", got, want)
			}
			if got := RunAtomic(tc.workers, tc.per); got != int64(want) {
				t.Errorf("RunAtomic = %d, want %d", got, want)
			}
		})
	}
}

// RWMutex 캐시의 Incr 가 읽기-수정-쓰기를 원자적으로 처리하는지 확인합니다.
func TestCacheIncr(t *testing.T) {
	cases := []struct {
		name         string
		workers, per int
	}{
		{"작게", 2, 100},
		{"많이", 40, 250},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.workers * tc.per
			if got := RunCacheIncr(tc.workers, tc.per, false); got != want {
				t.Errorf("got %d, want %d", got, want)
			}
		})
	}
}

// Get/Set 의 기본 동작과, 읽기·쓰기 고루틴이 섞여도 -race 에서 깨끗한지 확인합니다.
func TestCacheGetSetConcurrent(t *testing.T) {
	c := NewCache()
	if _, ok := c.Get("none"); ok {
		t.Fatal("없는 키가 있다고 나옴")
	}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.Set(fmt.Sprint("k", i), i)
		}()
		go func() {
			defer wg.Done()
			c.Get(fmt.Sprint("k", i))
		}()
	}
	wg.Wait()
	for i := range 20 {
		v, ok := c.Get(fmt.Sprint("k", i))
		if !ok || v != i {
			t.Errorf("k%d = %d, %v", i, v, ok)
		}
	}
}

// 잠금을 쥔 채 내부 함수를 부르는 패턴이 교착 없이 끝나고 값이 맞는지 확인합니다.
func TestIncTwice(t *testing.T) {
	c := &SafeCounter{}
	c.IncTwice()
	c.Inc()
	if got := c.Value(); got != 3 {
		t.Errorf("Value = %d, want 3", got)
	}
}