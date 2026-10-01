package main

import (
	"reflect"
	"testing"
)

// unbuffered 송신이 receiver 의 got 이벤트 뒤에 완료됐다고 확정한 순서를 검증한다.
// unbuffered 채널은 수신자와의 직접 만남을 강제하기 때문에
// 송신이 buffer 에 머물지 않고 바로 수신자 스택으로 넘어간다.
func TestUnbufferedHandoffSequence(t *testing.T) {
	events := unbufferedHandoffSequence()
	want := []string{
		"receiver goroutine: blocked before receive",
		"sender goroutine: about to send on unbuffered channel",
		"receiver goroutine: got value 42",
		"sender goroutine: send returned (receiver confirmed got value)",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("event mismatch\n got: %#v\nwant: %#v", events, want)
	}
}

// buffered 채널은 빈 슬롯이 있으면 receiver 없이도 송신이 끝난다.
// 이후 receiver 가 나중에 값을 꺼내는 순서를 검증한다.
func TestBufferedSendSequence(t *testing.T) {
	events := bufferedSendSequence()
	want := []string{
		"sender goroutine: about to send on buffered channel (cap 1)",
		"sender goroutine: send returned even though no receiver yet",
		"receiver goroutine: about to receive",
		"receiver goroutine: got value 42",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("event mismatch\n got: %#v\nwant: %#v", events, want)
	}
}

// buffered 채널의 버퍼가 FIFO 라는 불변식을 확인한다.
// 환형 배열의 인덱스가 돌아도 값 순서가 바뀌지 않는다.
func TestBufferedCircularFIFOSequence(t *testing.T) {
	events := bufferedCircularFIFOSequence()
	if len(events) != 3 {
		t.Fatalf("unexpected event count: %d", len(events))
	}
	if events[0] != "buffered cap 2: send 1 and 2 before any receive" {
		t.Fatalf("setup event mismatch: %q", events[0])
	}
	if events[1] != "first receive got: 1" || events[2] != "second receive got: 2" {
		t.Fatalf("FIFO order broken: %#v", events)
	}
}

// benchmarkChanPair 는 sender 와 receiver 가 한 쌍으로 돌아가는
// 채널 통신 비용을 측정한다. b.ResetTimer 는 receiver goroutine 준비가
// 끝난 뒤 호출해 goroutine 시작 비용이 측정에 섞이지 않게 한다.
func benchmarkChanPair(b *testing.B, mk func() chan int) {
	ch := mk()
	done := make(chan struct{})
	go func() {
		for i := 0; i < b.N; i++ {
			<-ch
		}
		close(done)
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- i
	}
	<-done
}

func BenchmarkUnbufferedChan(b *testing.B) {
	benchmarkChanPair(b, func() chan int { return make(chan int) })
}

func BenchmarkBufferedChanCap1(b *testing.B) {
	benchmarkChanPair(b, func() chan int { return make(chan int, 1) })
}