package main

import (
	"fmt"
	"iter"
)

// 정수 시퀀스를 생성하는 커스텀 이터레이터 함수
// iter.Seq[T]는 func(yield func(T) bool) 타입의 별칭입니다.
func countSeq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			// yield 함수는 값을 전달하고 루프를 계속할지 여부를 반환합니다.
			// false가 반환되면 range 루프가 즉시 중단됩니다.
			if !yield(i) {
				return
			}
		}
	}
}

// 조건에 따라 값을 필터링하는 이터레이터 함수
func evenSeq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if i%2 == 0 {
				if !yield(i) {
					return
				}
			}
		}
	}
}

// 키-값 페어 시퀀스를 생성하는 커스텀 이터레이터
// iter.Seq2[K, V]는 func(yield func(K, V) bool) 타입의 별칭입니다.
func pairSeq(items []string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		for idx, val := range items {
			if !yield(idx, val) {
				return
			}
		}
	}
}

func main() {
	fmt.Println("=== 1. 기본 시퀀스 이터레이션 ===")
	// countSeq 함수를 호출하면 iter.Seq[int] 함수가 반환됩니다.
	// for range는 이 함수를 자동으로 호출하고 yield 콜백을 처리합니다.
	for num := range countSeq(5) {
		fmt.Printf("카운트: %d\n", num)
	}

	fmt.Println("\n=== 2. 조건부 필터링 이터레이션 ===")
	// evenSeq는 짝수만 yield합니다.
	for num := range evenSeq(10) {
		fmt.Printf("짝수: %d\n", num)
	}

	fmt.Println("\n=== 3. 키-값 쌍 이터레이션 (Seq2) ===")
	// iter.Seq2를 사용하여 인덱스와 값을 동시에 처리합니다.
	data := []string{"go", "java", "python"}
	for idx, lang := range pairSeq(data) {
		fmt.Printf("인덱스 %d: %s\n", idx, lang)
	}

	fmt.Println("\n=== 4. 중첩 범위 및 중단 제어 ===")
	// 내장 slices 패키지의 함수도 동일하게 range with function으로 작동합니다.
	// break 문을 사용하여 중첩 범위에서도 안전하게 중단할 수 있습니다.
	for num := range countSeq(10) {
		if num >= 5 {
			break
		}
		fmt.Printf("조기 중단 테스트: %d\n", num)
	}
}