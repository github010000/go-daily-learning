package main

import (
	"fmt"
)

func main() {
	// 1. 슬라이스의 기본 선언 및 배열과의 차이
	// 배열은 크기가 고정되어 있지만, 슬라이스는 크기를 변경할 수 있는 가변적인 구조입니다.
	arr := [5]int{1, 2, 3, 4, 5} // 크기가 5인 배열
	slice := arr[1:4]           // 배열의 인덱스 1부터 3까지를 참조하는 슬라이스

	fmt.Println("--- 1. 기본 슬라이스 및 배열 참조 ---")
	fmt.Printf("배열: %v, 슬라이스: %v\n", arr, slice)
	fmt.Printf("슬라이스의 요소 변경 시 배열도 변경됨: %v\n", arr)

	// 2. make 함수를 이용한 슬라이스 생성
	// make(type, length, capacity)
	// length: 슬라이스의 현재 길이, capacity: 슬라이스가 확장될 수 있는 최대 용량
	s2 := make([]int, 3, 5)
	fmt.Println("\n--- 2. make를 이용한 생성 ---")
	fmt.Printf("슬라이스: %v, 길이(len): %d, 용량(cap): %d\n", s2, len(s2), cap(s2))

	// 3. append를 이용한 요소 추가 및 용량 변화
	fmt.Println("\n--- 3. append를 통한 요소 추가 ---")
	s2 = append(s1, 10) // s1은 정의되지 않았으므로 s2를 사용
	s2 = append(s2, 20, 30)
	fmt.Printf("추가 후 슬라이스: %v, 길이: %d, 용량: %d\n", s2, len(s2), cap(s2))

	// 4. copy 함수를 이용한 슬라이스 복사
	// copy(dest, src)는 src의 내용을 dest로 복사합니다. (dest의 길이만큼만 복사됨)
	src := []int{1, 2, 3, 4, 5}
	dst := make([]int, 3)
	copy(dst, src)
	fmt.Println("\n--- 4. copy를 이용한 슬라이스 복사 ---")
	fmt.Printf("원본: %v, 복사본(dst): %v\n", src, dst)

	// 5. nil 슬라이스 vs 빈 슬라이스
	fmt.Println("\n--- 5. nil 슬라이스 vs 빈 슬라이스 ---")
	var nilSlice []int          // nil 슬라이스 (초기화되지 않음)
	emptySlice := []int{}       // 빈 슬라이스 (길이 0, 용량 0, 하지만 메모리 할당됨)

	fmt.Printf("nil 슬라이스: %v, len: %d, cap: %d, is nil: %t\n", nilSlice, len(nilSlice), cap(nilSlice), nilSlice == nil)
	fmt.Printf("빈 슬라이스: %v, len: %d, cap: %d, is nil: %t\n", emptySlice, len(emptySlice), cap(emptySlice), emptySlice == nil)
}

// s1 선언을 위해 main 함수 상단에 추가적인 변수 처리가 필요함 (위 코드의 로직 흐름을 위해 수정)
var s1 = []int{1, 2, 3}