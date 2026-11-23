package main

import (
	"reflect"
	"testing"
	"time"
)

// 버퍼 없는 채널로 값이 넘어가는 기본 동작을 확인한다.
func TestUnbufferedHandoff(t *testing.T) {
	if got := unbufferedHandoff(); got != "안녕" {
		t.Fatalf("got %q", got)
	}
}

// len 은 지금 들어 있는 개수, cap 은 칸 수라는 것과 FIFO 순서를 확인한다.
func TestBufferedBasics(t *testing.T) {
	l, c, got := bufferedBasics()
	if l != 2 || c != 3 {
		t.Fatalf("len=%d cap=%d", l, c)
	}
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("got %v", got)
	}
	if !bufferedIsFull() {
		t.Fatal("가득 찬 채널이어야 한다")
	}
}

// 닫힌 채널 수신은 남은 값 다음에 제로값과 ok=false 를 준다.
func TestReadClosed(t *testing.T) {
	want := []string{"값=7 ok=true", "값=0 ok=false"}
	if got := readClosed(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if got := drainClosedRight(); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("range 결과 %v", got)
	}
}

// 생산자가 close 해야 range 가 끝나 합계가 나온다. 테이블 주도로 여러 크기를 본다.
func TestProducerSum(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{"빈 채널", 0, 0},
		{"하나", 1, 1},
		{"다섯", 5, 15},
		{"백", 100, 5050},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sum(producer(tc.n)); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// 버퍼가 1칸 있으면 아무도 안 받아도 일꾼이 끝나 done 이 닫힌다(누수 없음).
func TestBufferedWorkerNoLeak(t *testing.T) {
	ch, release, done := startWorker(1)
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("일꾼이 끝나지 않았다")
	}
	if v := <-ch; v != "결과" {
		t.Fatalf("got %q", v)
	}
}