package main

import (
	"fmt"
	"cmp"
	"slices"
	"maps"
)

func main() {
	// 1. slices 패키지: 슬라이스 정렬, 포함 여부 확인, 역순 정렬
	fmt.Println("=== slices 패키지 예제 ===")
	numbers := []int{5, 2, 8, 1, 9, 3}
	fmt.Printf("원본 숫자 슬라이스: %v\n", numbers)

	// slices.Sort 사용 (오름차순 정렬, 원본 슬라이스 수정)
	slices.Sort(numbers)
	fmt.Printf("slices.Sort 후: %v\n", numbers)

	// slices.Contains 확인
	fmt.Printf("8이 포함되어 있나요? %v\n", slices.Contains(numbers, 8))
	fmt.Printf("10이 포함되어 있나요? %v\n", slices.Contains(numbers, 10))

	// slices.Reverse 사용 (역순 정렬, 원본 슬라이스 수정)
	slices.Reverse(numbers)
	fmt.Printf("slices.Reverse 후: %v\n", numbers)

	// 2. maps 패키지: 키와 값 추출
	fmt.Println("\n=== maps 패키지 예제 ===")
	studentScores := map[string]int{
		"철수": 95,
		"영희": 88,
		"민수": 92,
	}

	// maps.Keys: 모든 키 슬라이스 생성 (정렬하여 출력 순서 고정)
	keys := maps.Keys(studentScores)
	slices.Sort(keys)
	fmt.Printf("학생 키 목록: %v\n", keys)

	// maps.Values: 모든 값 슬라이스 생성
	values := maps.Values(studentScores)
	fmt.Printf("점수 값 목록: %v\n", values)

	// 3. cmp 패키지: 타입 비교 및 Ordered 제약 조건
	fmt.Println("\n=== cmp 패키지 예제 ===")

	// cmp.Compare: 두 값을 비교하여 -1, 0, 1 반환
	fmt.Printf("cmp.Compare(3, 5) 결과: %d (음수면 첫 인자가 작음)\n", cmp.Compare(3, 5))
	fmt.Printf("cmp.Compare(5, 3) 결과: %d (양수면 첫 인자가 큼)\n", cmp.Compare(5, 3))
	fmt.Printf("cmp.Compare(4, 4) 결과: %d (0이면 같음)\n", cmp.Compare(4, 4))

	// cmp.Ordered 제약을 사용하는 제네릭 함수 예시
	// 정수 슬라이스의 총합을 float64로 반환
	total := sumOrdered(numbers)
	fmt.Printf("숫자 슬라이스의 총합: %.0f\n", total)
}

// sumOrdered는 cmp.Ordered 제약을 받는 제네릭 함수
// 슬라이스 내 모든 요소의 합을 float64로 반환
func sumOrdered[T cmp.Ordered](s []T) float64 {
	var sum float64
	for _, v := range s {
		sum += float64(v)
	}
	return sum
}