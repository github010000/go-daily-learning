package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Produce는 vals를 out으로 보내지만 ctx가 취소되면 즉시 중단한다.
// ctx가 이미 취소된 채로 호출되면 goroutine을 시작하지 않고 out을 닫는다.
// 이 초기 확인이 있으면 "이미 취소됨" 경로가 결정적이 되어,
// 소비자가 받을 준비를 하기 전에 취소된 경우에도 값이 새어 나가지 않는다.
func Produce(ctx context.Context, vals ...int) <-chan int {
	out := make(chan int)
	if ctx.Err() != nil {
		close(out)
		return out
	}

	go func() {
		defer close(out)
		for _, v := range vals {
			select {
			case <-ctx.Done():
				return
			case out <- v:
			}
		}
	}()
	return out
}

// Square는 입력 한 개를 받아 제곱을 출력하는 fan-out worker 한 개를 만든다.
// out은 이 worker만 보내므로 이 함수가 소유하고, worker가 끝날 때 닫는다.
func Square(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				// 두 번째 select가 중요하다. v를 받은 직후에는 downstream 수신자가
				// 없을 수 있고, 그 순간 취소가 오면 worker가 send에서 영원히
				// 블록될 수 있다. 취소와 send를 한 select에 묶어야 안전하다.
				select {
				case <-ctx.Done():
					return
				case out <- v * v:
				}
			}
		}
	}()
	return out
}

// Merge는 여러 worker 출력을 하나의 out으로 fan-in한다.
// Merge만이 out을 닫을 수 있다. 각 worker가 out을 닫으면
// close of closed channel panic이 날 수 있다.
// WaitGroup으로 모든 worker가 끝난 뒤 한 번만 close한다.
func Merge(ctx context.Context, cs ...<-chan int) <-chan int {
	var wg sync.WaitGroup
	out := make(chan int)

	output := func(c <-chan int) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-c:
				if !ok {
					return
				}
				// 여기서도 send 전에 취소를 확인해야 한다.
				// 그래야 소비자가 사라진 뒤에도 output goroutine이
				// out <- v에서 멈추지 않는다.
				select {
				case <-ctx.Done():
					return
				case out <- v:
				}
			}
		}
	}

	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// FindFirst는 소비자가 먼저 끝나는 올바른 구현이다.
// target을 찾으면 cancel을 호출해 upstream 생산자에게 취소를 전파한다.
// 소비자는 데이터 channel out을 닫지 않고, 취소 신호만 보낸다.
func FindFirst(ctx context.Context, cancel context.CancelFunc, out <-chan int, target int) (int, bool) {
	for v := range out {
		// 파이프라인이 이미 종료 중이면 target과 경쟁해서 성공으로
		// 잘못 보고하지 않도록 먼저 취소를 확인한다.
		select {
		case <-ctx.Done():
			return 0, false
		default:
		}

		if v == target {
			cancel()
			return v, true
		}
	}
	return 0, false
}

// WrongFindFirstLeak는 잘못된 패턴을 시연한다.
// target을 찾아도 cancel하지 않고 즉시 반환한다.
// 그러면 upstream goroutine들은 아무도 읽지 않는 channel에 send하려고
// 블록된 채 남는다. 실제 코드에서 이 함수가 early return하는 순간 leak가 생긴다.
func WrongFindFirstLeak(out <-chan int, target int) (int, bool) {
	for v := range out {
		if v == target {
			// BUG: 취소도, drain도 없다. 시연용으로만 남겨 둔다.
			return v, true
		}
	}
	return 0, false
}

// ProduceWithDone은 context 대신 plain done channel을 쓰는 변형이다.
// deadline, 취소 이유, error 전파가 필요 없고 지역 정지만 필요할 때 사용한다.
// done은 소비자 또는 부모 조정자만 닫아야 한다. 생산자는 데이터 channel만 닫는다.
func ProduceWithDone(done <-chan struct{}, vals ...int) <-chan int {
	out := make(chan int)

	// done이 이미 닫혔다면 goroutine을 만들지 않고 즉시 out을 닫는다.
	select {
	case <-done:
		close(out)
		return out
	default:
	}

	go func() {
		defer close(out)
		for _, v := range vals {
			select {
			case <-done:
				return
			case out <- v:
			}
		}
	}()
	return out
}

func main() {
	fmt.Println("=== fan-out/fan-in consumer-dies-first demo ===")

	// 잘못된 패턴: 소비자가 target을 찾고 cancel 없이 반환한다.
	base := runtime.NumGoroutine()
	ctxWrong, cancelWrong := context.WithCancel(context.Background())
	inWrong := Produce(ctxWrong, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

	w1 := Square(ctxWrong, inWrong)
	w2 := Square(ctxWrong, inWrong)
	w3 := Square(ctxWrong, inWrong)
	outWrong := Merge(ctxWrong, w1, w2, w3)

	foundWrong, _ := WrongFindFirstLeak(outWrong, 16)
	// 차단된 sender들이 보이도록 잠깐 scheduler를 돌린다.
	// 실제 프로그램에서는 이 goroutine들이 밀리초가 아니라 평생 남는다.
	time.Sleep(20 * time.Millisecond)
	fmt.Printf("wrong: found=%d, goroutines=%d (base %d)\n", foundWrong, runtime.NumGoroutine(), base)

	// 데모 프로세스가 끝나도록 정리한다. 실무에서 이 cancel을 빼먹는 것이 곧 leak다.
	cancelWrong()
	for range outWrong {
		// cancel 덕분에 upstream goroutine이 빠져나간다.
	}
	time.Sleep(10 * time.Millisecond)
	fmt.Printf("wrong cleanup done: goroutines=%d (base %d)\n", runtime.NumGoroutine(), base)

	// 올바른 패턴: 소비자가 끝나자마자 context를 cancel한다.
	base2 := runtime.NumGoroutine()
	ctxRight, cancelRight := context.WithCancel(context.Background())
	inRight := Produce(ctxRight, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

	c1 := Square(ctxRight, inRight)
	c2 := Square(ctxRight, inRight)
	c3 := Square(ctxRight, inRight)
	outRight := Merge(ctxRight, c1, c2, c3)

	foundRight, _ := FindFirst(ctxRight, cancelRight, outRight, 16)
	fmt.Printf("right: found=%d, immediate goroutines=%d (base %d)\n", foundRight, runtime.NumGoroutine(), base2)

	for range outRight {
		// 이 for range가 끝난다는 것은 모든 upstream이 취소를 관찰하고
		// outRight가 정상적으로 close되었다는 뜻이다.
	}
	time.Sleep(10 * time.Millisecond)
	fmt.Printf("right cleanup done: goroutines=%d (base %d)\n", runtime.NumGoroutine(), base2)

	fmt.Println("=== done channel with pre-closed done ===")
	done := make(chan struct{})
	close(done)
	outDone := ProduceWithDone(done, 7, 8, 9)
	var got []int
	for v := range outDone {
		got = append(got, v)
	}
	fmt.Printf("pre-closed done produced: %v\n", got)
}