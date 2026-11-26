package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 마감 안에 끝나면 nil, 넘기면 DeadlineExceeded 가 나오는지 테이블로 검증한다.
// 시간 차이를 크게 잡아(수 ms 대 수 초) 느린 환경에서도 결과가 뒤집히지 않게 했다.
func TestRunWithTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		work    time.Duration
		want    error
	}{
		{"마감 초과", 20 * time.Millisecond, 5 * time.Second, context.DeadlineExceeded},
		{"마감 안에 완료", 5 * time.Second, time.Millisecond, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runWithTimeout(tc.timeout, tc.work)
			if !errors.Is(got, tc.want) && got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// 부모를 취소하면 부모는 Canceled, 마감이 1시간인 자식도 Canceled 가 되는지 확인한다.
func TestPropagate(t *testing.T) {
	p, c := propagate()
	if !errors.Is(p, context.Canceled) {
		t.Errorf("부모 오류 = %v", p)
	}
	if !errors.Is(c, context.Canceled) {
		t.Errorf("자식 오류 = %v", c)
	}
}

// ctx 를 지키는 worker 는 취소 뒤에 반드시 끝난다. 대기 시간은 넉넉히 두고 끝났는지 여부만 본다.
func TestRespectingWorkerStops(t *testing.T) {
	if !stoppedQuickly(respectingWorker, 5*time.Second) {
		t.Fatal("취소 후에도 worker 가 멈추지 않았다")
	}
}

// 전용 키 타입으로 넣은 값은 읽히고, 넣지 않은 ctx 에서는 ok 가 false 인지 검증한다.
func TestRequestID(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		wantID string
		wantOK bool
	}{
		{"값 있음", WithRequestID(context.Background(), "abc"), "abc", true},
		{"값 없음", context.Background(), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := RequestID(tc.ctx)
			if id != tc.wantID || ok != tc.wantOK {
				t.Fatalf("got (%q, %v), want (%q, %v)", id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}