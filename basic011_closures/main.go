package main

import (
	"fmt"
	"sync"
	"time"
)

// makeIncrementersOldStyle는 Go 1.22 이전의 루프 변수 스코프 문제를 보여줍니다.
// 모든 클로저가 같은 i 변수를 공유하기 때문에, 루프가 끝난 뒤 모든 클로저가 10을 반환합니다.
func makeIncrementersOldStyle() []func() int {
	var incrementers []func() int
	for i := 0; i < 10; i++ {
		// i는 루프 전체에서 하나만 존재하고 모든 클로저가 같은 변수를 참조합니다.
		incrementers = append(incrementers, func() int {
			return i
		})
	}
	return incrementers
}

// makeIncrementersNewStyle은 Go 1.22 이후의 올바른 클로저 작성법입니다.
// 루프 변수가 각 반복마다 새로 생성되어 각 클로저가 독립적인 값을 갖습니다.
func makeIncrementersNewStyle() []func() int {
	var incrementers []func() int
	for i := 0; i < 10; i++ {
		// Go 1.22+: i는 이 반복마다 새로운 스코프에 갇혀 각 클로저가 독립적
		incrementers = append(incrementers, func() int {
			return i
		})
	}
	return incrementers
}

// demonstrateDataRace는 클로저가 공유 변수를 고루틴에서 수정할 때
// 데이터 레이스가 발생하는 것을 시연합니다.
func demonstrateDataRace() {
	counter := 0
	var closures []func()
	for i := 0; i < 5; i++ {
		closures = append(closures, func() {
			counter++
			fmt.Printf("클로저 %d 실행: counter = %d\n", i, counter)
		})
	}

	done := make(chan bool, len(closures))
	for _, fn := range closures {
		go func(f func()) {
			f()
			done <- true
		}(fn)
	}

	timeout := time.After(2 * time.Second)
	for i := 0; i < len(closures); i++ {
		select {
		case <-done:
		case <-timeout:
			fmt.Println("타임아웃: 고루틴이 멈춤")
			return
		}
	}
	fmt.Printf("최종 counter: %d (데이터 레이스로 인해 불확실)\n", counter)
}

// fixDataRace는 데이터 레이스를 mutex로 보호한 버전입니다.
func fixDataRace() {
	var mu sync.Mutex
	counter := 0
	var closures []func()
	for i := 0; i < 5; i++ {
		closures = append(closures, func() {
			mu.Lock()
			counter++
			fmt.Printf("클로저 %d 실행: counter = %d\n", i, counter)
			mu.Unlock()
		})
	}

	done := make(chan bool, len(closures))
	for _, fn := range closures {
		go func(f func()) {
			f()
			done <- true
		}(fn)
	}

	timeout := time.After(2 * time.Second)
	for i := 0; i < len(closures); i++ {
		select {
		case <-done:
		case <-timeout:
			fmt.Println("타임아웃: 고루틴이 멈춤")
			return
		}
	}
	fmt.Printf("최종 counter: %d (안전하게 계산됨)\n", counter)
}

// showClosureReference는 클로저가 변수의 메모리 위치를 참조한다는 점을 보여줍니다.
// 클로저 내부에서 루프 변수를 수정하면 원본 슬라이스에도 영향을 미칩니다.
func showClosureReference() {
	values := []int{1, 2, 3}
	// 클로저가 루프 변수 i와 values 슬라이스를 함께 참조합니다.
	var closures []func()
	for i := range values {
		closures = append(closures, func() {
			values[i] = values[i] * 10
		})
	}
	
	for _, fn := range closures {
		fn()
	}
	fmt.Println("values:", values)
	fmt.Println("이유: 클로저가 values[i]와 i의 메모리 위치를 참조하기 때문입니다.")
}

func main() {
	fmt.Println("=== 1. 루프 변수 스코프: 실수 버전 (공유 변수) ===")
	oldIncrements := makeIncrementersOldStyle()
	for idx, inc := range oldIncrements {
		fmt.Printf("인덱스 %d: %d\n", idx, inc())
	}
	fmt.Println("이유: 모든 클로저가 같은 루프 변수 i를 참조하기 때문입니다.")
	fmt.Println()

	fmt.Println("=== 2. 루프 변수 스코프: 고친 버전 (고유 변수) ===")
	newIncrements := makeIncrementersNewStyle()
	for idx, inc := range newIncrements {
		fmt.Printf("인덱스 %d: %d\n", idx, inc())
	}
	fmt.Println("이유: 각 클로저가 각기 다른 루프 변수 인스턴스를 갖기 때문입니다.")
	fmt.Println()

	fmt.Println("=== 3. 데이터 레이스: 클로저가 공유 변수를 고루틴에서 수정할 때 ===")
	demonstrateDataRace()
	fmt.Println()

	fmt.Println("=== 4. 데이터 레이스 수정: mutex로 보호 ===")
	fixDataRace()
	fmt.Println()

	fmt.Println("=== 5. 클로저는 변수 '참조'를 붙잡습니다 ===")
	showClosureReference()
}