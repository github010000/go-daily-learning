// package main으로 시작하여, 외부 테스트 패키지가 없어도 실행 가능한 완전한 예제입니다.
// 이 코드는 Go의 testing 패키지에서 제공하는 Benchmark의 내부 동작 원리를
// fmt와 time 패키지를 사용하여 직접 재현한 것입니다.
package main

import (
	"fmt"
	"strings"
	"time"
)

// targetFunction: 벤치마크할 대상 함수입니다.
// 문자열을 대문자로 변환하는 단순한 로직입니다.
func targetFunction(s string) string {
	return strings.ToUpper(s)
}

// benchSimulator: Go의 testing 패키지가 제공하는 b.N 루프 구조를 시뮬레이션합니다.
// 이 함수는 targetFunction의 성능을 측정하고 결과를 출력합니다.
func benchSimulator(benchName string, f func(string) string, input string) {
	fmt.Printf("--- %s ---\n", benchName)
	
	// Go의 testing 패키지는 정확한 벤치마킹을 위해 여러 번 실행한 후
	// 최적의 실행 시간을 찾아 b.N을 동적으로 조정합니다.
	// 여기서는 학습을 위해 b.N을 100, 1000, 10000으로 고정하여 보여줍니다.
	tiers := []int{100, 1000, 10000}
	
	for _, n := range tiers {
		startTime := time.Now()
		
		// b.N 루프: 테스트할 함수를 b.N번 반복 실행
		for i := 0; i < n; i++ {
			f(input)
		}
		
		// 경과 시간 측정
		elapsed := time.Since(startTime)
		
		// 연산 횟수(n)로 시간을 나누어 단일 연소당 평균 시간 계산
		// 단위: 나노초/ns/op
		avgPerOp := float64(elapsed.Nanoseconds()) / float64(n)
		
		fmt.Printf("\t%7d\t%10.2f ns/op\n", n, avgPerOp)
	}
	fmt.Println()
}

// exampleSimulator: Go의 Example 테스트를 시뮬레이션합니다.
// Example 함수는 함수의 사용법을 설명하고, 코드가 예시대로 동작하는지 검증합니다.
func exampleSimulator() {
	fmt.Println("=== Example Simulation ===")
	fmt.Println("Input:  \"hello world\"")
	
	// Example 함수의 핵심은 '출력 주석'입니다.
	// 실제 테스트에서는 코드의 실행 결과가 출력 주석과 일치하는지 검증합니다.
	output := targetFunction("hello world")
	
	fmt.Printf("Expected: \"HELLO WORLD\"\n")
	fmt.Printf("Actual:   \"%s\"\n", output)
	
	if output == "HELLO WORLD" {
		fmt.Println(">> [PASS] Example test passed! (Output matches comment)")
	} else {
		fmt.Println(">> [FAIL] Example test failed!")
	}
	fmt.Println()
}

func main() {
	// 1. 벤치마크 시뮬레이션
	// Go에서는 'go test -bench=.' 명령어로 실행되지만, 여기서는 main에서 직접 호출합니다.
	fmt.Println(">>> Benchmark Simulation")
	benchSimulator("BenchmarkTargetFunction", targetFunction, "Hello World")

	// 2. 테스트 가능한 예제 (Testable Example) 시뮬레이션
	fmt.Println(">>> Testable Example Simulation")
	exampleSimulator()

	fmt.Println("=== Simulation Completed ===")
}