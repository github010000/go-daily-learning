package main

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// tryReceive 와 trySend 가 default 덕분에 막히지 않고
// 상태에 맞는 결과를 주는지 테이블로 검증한다.
func TestNonBlocking(t *testing.T) {
	t.Run("tryReceive", func(t *testing.T) {
		tests := []struct {
			name   string
			vals   []int
			wantV  int
			wantOK bool
		}{
			{"빈 채널", nil, 0, false},
			{"값 있음", []int{5}, 5, true},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				v, ok := tryReceive(filled(tc.vals, false))
				if v != tc.wantV || ok != tc.wantOK {
					t.Fatalf("got (%d,%v), want (%d,%v)", v, ok, tc.wantV, tc.wantOK)
				}
			})
		}
	})
	t.Run("trySend", func(t *testing.T) {
		ch := make(chan int, 1)
		if !trySend(ch, 1) {
			t.Fatal("빈 버퍼에는 보낼 수 있어야 한다")
		}
		if trySend(ch, 2) {
			t.Fatal("가득 찬 버퍼에는 false 여야 한다")
		}
	})
}

// recvWithTimeout 이 값이 있으면 값을, 없으면 ErrTimeout 을 주는지 검증한다.
// 시간 길이는 단정하지 않고 결과만 본다.
func TestRecvWithTimeout(t *testing.T) {
	v, err := recvWithTimeout(filled([]int{9}, false), time.Second)
	if err != nil || v != 9 {
		t.Fatalf("got (%d,%v)", v, err)
	}
	_, err = recvWithTimeout(make(chan int), 10*time.Millisecond)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("ErrTimeout 이어야 하는데 %v", err)
	}
}

// 닫힌 채널을 nil 로 바꾸는 방식이 두 채널을 모두 비우고 끝나는지 검증한다.
func TestDrainBoth(t *testing.T) {
	got := drainBoth(filled([]int{1, 3, 5}, true), filled([]int{2, 4}, true))
	want := []int{1, 2, 3, 4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// firstOf 가 준비된 쪽 채널의 값을 고르는지 검증한다.
func TestFirstOf(t *testing.T) {
	a := make(chan string)
	b := make(chan string, 1)
	b <- "hi"
	if got := firstOf(a, b); got != "hi" {
		t.Fatalf("got %q", got)
	}
}

// 고친 버전이 채널이 닫히면 마감 없이, 닫히지 않으면 마감으로 끝나는지 검증한다.
func TestConsumeWithDeadline(t *testing.T) {
	tests := []struct {
		name         string
		closeIt      bool
		wantTimedOut bool
	}{
		{"닫힌 채널", true, false},
		{"닫히지 않은 채널", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n, timedOut := consumeWithDeadline(filled([]int{1, 2, 3}, tc.closeIt), 20*time.Millisecond)
			if n != 3 || timedOut != tc.wantTimedOut {
				t.Fatalf("got (%d,%v)", n, timedOut)
			}
		})
	}
}