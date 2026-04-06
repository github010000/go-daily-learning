package main

import "fmt"

func main() {
	// 1. 기본 for 반복문: 초기화, 조건, 증감 표현식을 모두 포함
	fmt.Println("=== 1. 기본 for 반복문 ===")
	for i := 0; i < 5; i++ {
		fmt.Printf("i = %d\n", i)
	}

	// 2. 조건만 있는 for: while 대체 역할
	fmt.Println("\n=== 2. 조건만 있는 for (while 대체) ===")
	count := 3
	for count > 0 {
		fmt.Printf("Countdown: %d\n", count)
		count-- // 감소
	}

	// 3. 무한 루프: 조건 생략 (break 또는 return으로 탈출 필수)
	fmt.Println("\n=== 3. 무한 루프 ===")
	x := 0
	for {
		if x >= 2 {
			break // 탈출 조건
		}
		fmt.Printf("x = %d (무한 루프 내)\n", x)
		x++
	}

	// 4. range 없이 slice를 for로 순회 (index만 사용)
	fmt.Println("\n=== 4. for를 사용한 slice 순회 (index 사용) ===")
	numbers := []int{10, 20, 30}
	for i := 0; i < len(numbers); i++ {
		fmt.Printf("index: %d, value: %d\n", i, numbers[i])
	}

	// 5. range 대신 while 스타일로 조건 기반 반복
	fmt.Println("\n=== 5. while 스타일: 조건에 따른 반복 ===")
	index := 0
	sum := 0
	for sum < 25 { // 조건 기반 탈출
		sum += numbers[index]
		index++
		if index >= len(numbers) {
			break // 안전장치
		}
	}
	fmt.Printf("최종 합계: %d\n", sum)
}
