package main

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrTimeout 은 기다리던 값이 제한 시간 안에 오지 않았음을 뜻한다.
// 센티널 에러로 두면 호출하는 쪽이 errors.Is 로 구분할 수 있다.
var ErrTimeout = errors.New("시간 초과")

// filled 는 값이 미리 들어 있는 버퍼 채널을 만든다.
// closeIt 이 false 이면 닫지 않으므로, 비면 수신자는 영원히 기다리게 된다.
func filled(vals []int, closeIt bool) chan int {
	ch := make(chan int, len(vals))
	for _, v := range vals {
		ch <- v
	}
	if closeIt {
		close(ch)
	}
	return ch
}

// firstOf 는 두 채널 중 먼저 값을 주는 쪽의 값을 돌려준다.
// 둘 다 준비되어 있으면 Go 가 무작위로 고르므로 순서를 기대하면 안 된다.
func firstOf(a, b <-chan string) string {
	select {
	case s := <-a:
		return s
	case s := <-b:
		return s
	}
}

// tryReceive 는 default 덕분에 기다리지 않고 "지금 있나?" 만 확인한다.
func tryReceive(ch <-chan int) (int, bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		return 0, false
	}
}

// trySend 는 버퍼가 가득 차 있으면 막히지 않고 false 를 돌려준다.
// 로그나 메트릭처럼 "못 보내면 버려도 되는" 상황에서 쓴다.
func trySend(ch chan<- int, v int) bool {
	select {
	case ch <- v:
		return true
	default:
		return false
	}
}

// recvWithTimeout 은 NewTimer + Stop 으로 타이머를 직접 정리한다.
// 값이 일찍 오면 타이머가 남아 있을 이유가 없기 때문이다.
func recvWithTimeout(ch <-chan int, d time.Duration) (int, error) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case v := <-ch:
		return v, nil
	case <-timer.C:
		return 0, ErrTimeout
	}
}

// drainBoth 는 두 채널이 모두 닫힐 때까지 읽는다.
// 닫힌 채널 변수를 nil 로 바꾸면 select 가 그 case 를 영원히 고르지 않는다.
func drainBoth(a, b <-chan int) []int {
	var out []int
	for a != nil || b != nil {
		select {
		case v, ok := <-a:
			if !ok {
				a = nil
				continue
			}
			out = append(out, v)
		case v, ok := <-b:
			if !ok {
				b = nil
				continue
			}
			out = append(out, v)
		}
	}
	sort.Ints(out) // 도착 순서는 무작위이므로 정렬해서 돌려준다
	return out
}

// consumeResetting 은 실수 버전이다.
// 반복마다 time.After 를 새로 만들므로 "마지막 수신 후 idle" 만 재고,
// 전체 마감은 지켜지지 않는다.
func consumeResetting(ch <-chan int, idle time.Duration) (int, bool) {
	count := 0
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return count, false
			}
			count++
		case <-time.After(idle):
			return count, true
		}
	}
}

// consumeWithDeadline 은 고친 버전이다.
// 타이머를 반복문 밖에서 한 번만 만들어야 전체 마감이 된다.
func consumeWithDeadline(ch <-chan int, total time.Duration) (int, bool) {
	timer := time.NewTimer(total)
	defer timer.Stop()
	count := 0
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return count, false
			}
			count++
		case <-timer.C:
			return count, true
		}
	}
}

// slowProducer 는 gap 간격으로 n 개를 보낸다.
// done 이 닫히면 보내다 말고 끝나므로 고루틴이 남지 않는다.
func slowProducer(n int, gap time.Duration, done <-chan struct{}) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := range n {
			select {
			case out <- i:
			case <-done:
				return
			}
			time.Sleep(gap)
		}
	}()
	return out
}

func demoBasic() {
	a := make(chan string, 1)
	b := make(chan string, 1)
	b <- "b 채널의 값"
	fmt.Println("firstOf:", firstOf(a, b))
	fmt.Println("drainBoth:", drainBoth(filled([]int{1, 3}, true), filled([]int{2, 4}, true)))
}

func demoDefault() {
	ch := make(chan int, 1)
	_, ok := tryReceive(ch)
	fmt.Println("빈 채널 tryReceive 성공?", ok)
	fmt.Println("첫 trySend:", trySend(ch, 1))
	fmt.Println("가득 찬 뒤 trySend:", trySend(ch, 2))
}

func demoTimeout() {
	empty := make(chan int)
	_, err := recvWithTimeout(empty, 30*time.Millisecond)
	fmt.Println("아무도 안 보낼 때:", err, "| 타임아웃인가?", errors.Is(err, ErrTimeout))
	v, err := recvWithTimeout(filled([]int{7}, false), time.Second)
	fmt.Println("값이 있을 때:", v, err)
}

func demoLoop() {
	// 20ms 마다 10개, 즉 전체 약 200ms 가 걸리는 생산자
	done := make(chan struct{})
	n, timedOut := consumeResetting(slowProducer(10, 20*time.Millisecond, done), 50*time.Millisecond)
	close(done)
	fmt.Println("실수 버전(반복마다 time.After): 받은 개수", n, "마감 발동", timedOut)

	done = make(chan struct{})
	n, timedOut = consumeWithDeadline(slowProducer(10, 20*time.Millisecond, done), 100*time.Millisecond)
	close(done)
	fmt.Println("고친 버전(전체 100ms 마감): 받은 개수", n, "마감 발동", timedOut, "| 10개 다 받지 못함:", n < 10)
}

func main() {
	demoBasic()
	demoDefault()
	demoTimeout()
	demoLoop()
}