package main

import (
	"fmt"
	"os"
	"strconv"
)

const demoChannels = 4

// NonBlockingReceive는 select + default의 단일 수신 사례를 감쌉니다.
// 이 형태는 컴파일러가 selectnbrecv 호출로 바꿔서, 블로킹 없이
// 채널이 준비되었는지만 검사합니다.
func NonBlockingReceive(ch <-chan int) (int, bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		return 0, false
	}
}

// FixedOrderDemo는 select 대신 고정된 순서로 채널을 검사하는 잘못된
// 방식을 보여줍니다. 모든 채널이 항상 준비되어 있을 때는 0번 채널만
// 계속 선택되어 나머지가 굶습니다.
//
// select의 무작위 pollorder가 왜 필요한지를 이 함수와 SelectFairnessDemo의
// 출력을 비교해서 확인할 수 있습니다.
func FixedOrderDemo(trials int) []int {
	chans := make([]chan int, demoChannels)
	for i := range chans {
		chans[i] = make(chan int, 1)
	}

	// 모든 채널을 가득 채워 "항상 준비된" 상태로 만듭니다.
	for i := 0; i < demoChannels; i++ {
		chans[i] <- i
	}

	counts := make([]int, demoChannels)
	for t := 0; t < trials; t++ {
		chosen := -1

		// 잘못된 가정: 첫 번째로 준비된 케이스만 보면 공정할 것 같다?
		// 실제 select는 이런 식으로 동작하지 않습니다.
		for i := 0; i < demoChannels; i++ {
			if v, ok := NonBlockingReceive(chans[i]); ok {
				chosen = v
				counts[chosen]++
				chans[chosen] <- chosen // 다음 시행을 위해 다시 채웁니다.
				break
			}
		}

		if chosen < 0 {
			panic("fixed order: no channel was ready")
		}
	}
	return counts
}

// SelectFairnessDemo는 런타임 select가 준비된 케이스를 무작위 순서로
// 검사하기 때문에 선택 빈도가 고르게 퍼지는 모습을 보여줍니다.
//
// 모든 채널이 매 시행 전에 가득 차 있으므로, 매번 4개 케이스 모두
// 준비된 상태입니다. 런타임은 pollorder라는 무작위 순열을 만들고
// 그 순서대로 첫 번째 준비된 케이스를 고릅니다.
func SelectFairnessDemo(trials int) []int {
	chans := make([]chan int, demoChannels)
	for i := range chans {
		chans[i] = make(chan int, 1)
	}
	for i := 0; i < demoChannels; i++ {
		chans[i] <- i
	}

	counts := make([]int, demoChannels)
	for t := 0; t < trials; t++ {
		var chosen int
		select {
		case v := <-chans[0]:
			chosen = v
		case v := <-chans[1]:
			chosen = v
		case v := <-chans[2]:
			chosen = v
		case v := <-chans[3]:
			chosen = v
		}
		counts[chosen]++
		chans[chosen] <- chosen // 선택된 채널만 비었으므로 다시 채웁니다.
	}
	return counts
}

// DefaultPathDemo는 default가 있을 때 select가 블록하지 않는 경로를
// 보여줍니다. main 고루틴은 값이 없으면 default로 계속 돌아가고,
// 송신 고루틴이 채널에 실제로 붙었을 때만 수신합니다.
func DefaultPathDemo(n int) (received, missed int) {
	ch := make(chan int) // 버퍼가 없으면 송신자와 수신자가 만날 때만 성공
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < n; i++ {
			ch <- i
		}
	}()

	for received < n {
		select {
		case <-ch:
			received++
		default:
			missed++
		}
	}
	<-done
	return received, missed
}

func printCounts(title string, counts []int, trials int) {
	fmt.Printf("%s (trials=%d)\n", title, trials)
	for i, c := range counts {
		pct := float64(c) / float64(trials) * 100.0
		fmt.Printf("  case %d: %d  (%.1f%%)\n", i, c, pct)
	}
	ideal := trials / demoChannels
	fmt.Printf("  ideal per case ~ %d\n", ideal)
}

func main() {
	trials := 20000
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil {
			trials = v
		}
	}

	fmt.Println("=== 잘못된 방식: 고정 순서 검사 ===")
	fixed := FixedOrderDemo(trials)
	printCounts("fixed order", fixed, trials)

	fmt.Println()
	fmt.Println("=== select의 준비된 케이스 무작위 검사 ===")
	fair := SelectFairnessDemo(trials)
	printCounts("select fairness", fair, trials)

	fmt.Println()
	fmt.Println("=== default가 있을 때의 non-blocking 경로 ===")
	received, missed := DefaultPathDemo(8)
	fmt.Printf("received=%d missed=%d\n", received, missed)
}