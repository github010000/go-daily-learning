package main

import (
	"sync"
	"testing"
)

// TestVictimCacheLifecycle 는 객체가 GC 1회에는 victim cache 덕분에 재사용되고,
// GC 2회에는 비워져 새 객체가 만들어지는지 확인한다.
func TestVictimCacheLifecycle(t *testing.T) {
	reused, cleared, calls := victimCacheResults()
	if !reused {
		t.Errorf("1회 GC 후에는 victim cache 에서 객체가 재사용되어야 한다")
	}
	if !cleared {
		t.Errorf("2회 GC 후에는 victim cache 가 비워져 새 객체여야 한다")
	}
	if calls != 2 {
		t.Fatalf("New 호출 수 = %d, want 2", calls)
	}
}

// TestAllocationCounts 는 작은 객체에서 pool 의 자료형 선택이 allocation 횟수에
// 미치는 영향을 검증한다. 포인터를 넣으면 할당이 크게 줄어들고, 값을 넣으면
// boxing allocation 때문에 직접 할당과 비슷하거나 더 많은 할당이 남아야 한다.
func TestAllocationCounts(t *testing.T) {
	n := 1000
	direct, ptr, value, _, _, _ := allocationCounts(n)

	if ptr >= direct {
		t.Errorf("포인터 pool 은 직접 할당보다 allocation 이 적어야 함: ptr=%d, direct=%d", ptr, direct)
	}
	if value < direct {
		t.Errorf("값 타입 pool 은 boxing allocation 때문에 직접 할당보다 allocation 이 줄지 않아야 함: value=%d, direct=%d", value, direct)
	}
}

// BenchmarkSmallObjectDirect 는 작은 객체를 매번 new 로 할당하는 기준선이다.
func BenchmarkSmallObjectDirect(b *testing.B) {
	for i := 0; i < b.N; i++ {
		x := new(byte32)
		x.b[0] = byte(i)
		sinkPtr = x
	}
}

// BenchmarkSmallObjectPoolPointer 는 *byte32 를 pool 에 넣는 올바른 방식이다.
// warm up 후 측정해 pool 내부 구조 초기화 비용을 제외한다.
func BenchmarkSmallObjectPoolPointer(b *testing.B) {
	p := sync.Pool{New: func() any { return new(byte32) }}
	x := p.Get().(*byte32)
	p.Put(x)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := p.Get().(*byte32)
		x.b[0] = byte(i)
		p.Put(x)
		sinkPtr = x
	}
}

// BenchmarkSmallObjectPoolValue 는 byte32 값을 pool 에 넣는 잘못된 방식이다.
// boxing allocation 이 성능을 갉아먹는지 확인하기 위한 것이다.
func BenchmarkSmallObjectPoolValue(b *testing.B) {
	p := sync.Pool{New: func() any { return byte32{} }}
	x := p.Get().(byte32)
	p.Put(x)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := p.Get().(byte32)
		x.b[0] = byte(i)
		p.Put(x)
		sinkVal = x
	}
}