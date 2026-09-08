package main

import "testing"

// TestPublishWithChannel은 channel close/receive가 happens-before edge를 만들어
// writer가 기록한 값을 reader가 반드시 관찰하는지 검증한다.
func TestPublishWithChannel(t *testing.T) {
	for i := 1; i <= 100; i++ {
		got := publishWithChannel(i)
		want := i * 2
		if got != want {
			t.Fatalf("publishWithChannel(%d) = %d, want %d", i, got, want)
		}
	}
}

// TestPublishWithMutex는 mutex Unlock/Lock이 writer의 ready 표시와 값을
// reader에게 보이게 만드는지 검증한다. 0이 아닌 입력으로 안정적으로 확인한다.
func TestPublishWithMutex(t *testing.T) {
	for i := 1; i <= 100; i++ {
		got := publishWithMutex(i)
		want := i * 2
		if got != want {
			t.Fatalf("publishWithMutex(%d) = %d, want %d", i, got, want)
		}
	}
}

// TestConcurrentOnce는 여러 goroutine이 sync.Once를 동시에 호출해도
// 최초 한 번의 기록만 일어나고 모두 같은 값을 읽는지 검증한다.
func TestConcurrentOnce(t *testing.T) {
	for i := 1; i <= 50; i++ {
		got := concurrentOnce(i)
		want := i * 2
		if got != want {
			t.Fatalf("concurrentOnce(%d) = %d, want %d", i, got, want)
		}
	}
}