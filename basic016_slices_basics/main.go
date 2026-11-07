package main

import (
	"fmt"
)

// sliceByVal: 슬라이스는 값(포인터, len, cap)으로 전달됩니다.
// 함수 안에서 append를 사용하면, 내부 배열이 부족할 때 **새 배열을 할당**할 수 있습니다.
// 이때 함수 밖의 원본 슬라이스는 여전히 옛날 배열을 가리키므로 변화가 없습니다.
// 이것이 "슬라이스를 함수로 보내면 바뀐다"는 오해가 생기는 이유입니다.
func sliceByVal(src []int, val int) []int {
	fmt.Printf("  [함수 안에서] src: %v, addr: %p, len: %d, cap: %d\n", src, src, len(src), cap(src))
	return append(src, val)
}

// modifyInPlace: 슬라이스가 가리키는 '배열'의 요소는 직접 건드릴 수 있습니다.
// 슬라이스 헤더(포인터)를 바꾸지 않고 내용만 바꾸므로 원본에 영향이 미칩니다.
func modifyInPlace(src []int, idx, val int) {
	if idx < 0 || idx >= len(src) {
		fmt.Println("  [인덱스 범위 초과]")
		return
	}
	src[idx] = val
}

// growSlice: 슬라이스를 넘겨받고 append로 늘리는 시연입니다.
func growSlice(src []int, val int) []int {
	return append(src, val)
}

// sliceOverlapping: 슬라이싱으로 만든 새 슬라이스는 원본과 배열을 공유합니다.
// 이 공유가 '실수'로 이어질 수 있는 경우를 보여줍니다.
func sliceOverlapping() {
	// 10, 20, 30, 40, 50 이 저장된 배열
	original := []int{10, 20, 30, 40, 50}

	// original의 1번째부터 3번째까지(20, 30, 40)만 담는 부분 슬라이스
	// 이 '부분 슬라이스'는 original과 같은 배열을 보고 있습니다.
	sub := original[1:4]

	fmt.Printf("슬라이싱 전 original: %v\n", original)
	fmt.Printf("슬라이싱 전 sub: %v\n", sub)

	// 실수: sub의 요소를 바꾸면 original의 중간 값도 바뀝니다.
	// 개발자는 sub만 건드렸는데 original이 왜? 하고 당황합니다.
	sub[0] = 99

	fmt.Printf("슬라이싱 후 original: %v (의도치 않게 변함!)\n", original)
	fmt.Printf("슬라이싱 후 sub:      %v\n", sub)
}

// nilVsEmpty: nil 슬라이스와 빈 슬라이스([]int{})는 append 동작은 같지만
// JSON 직렬화 등에서는 결과가 다릅니다.
func nilVsEmpty() {
	var nilSlice []int         // len: 0, cap: 0, 포인터: nil
	emptySlice := []int{}      // len: 0, cap: 0, 포인터: (배열 주소)

	fmt.Printf("nilSlice append 결과: %v\n", append(nilSlice, 1, 2))
	fmt.Printf("emptySlice append 결과: %v\n", append(emptySlice, 1, 2))
	// 위 두 결과는 같습니다. 하지만 JSON에서는 nil은 null, 빈 슬라이스는 []가 됩니다.
}

func main() {
	fmt.Println("=== 1. 슬라이스 전달 시 append 동작 ===")
	nums := []int{1, 2, 3}
	fmt.Printf("원본 nums: %v, addr: %p\n", nums, nums)

	result := sliceByVal(nums, 4)

	// 실수: 원본 nums가 변했다고 생각했지만, append로 인해 내부 배열이 재할당될 경우
	// 원본은 여전히 옛날 배열을 가리킵니다.
	fmt.Printf("함수 호출 후 원본 nums: %v (변함 없음)\n", nums)
	fmt.Printf("함수 반환값 result:    %v\n", result)

	fmt.Println("\n=== 2. 원본 배열 직접 수정 ===")
	nums2 := []int{10, 20, 30}
	fmt.Printf("원본 nums2: %v\n", nums2)
	modifyInPlace(nums2, 1, 99)
	// 슬라이스 헤더는 안 바뀌었지만, 가리키는 배열의 요소는 바꿨습니다.
	fmt.Printf("함수 호출 후 nums2: %v (변경됨!)\n", nums2)

	fmt.Println("\n=== 3. 슬라이싱과 배열 공유 (흔한 실수) ===")
	sliceOverlapping()
	// 원본 슬라이스를 그대로 쓰는 함수를 만들 때, 부분 슬라이스를 넘기면
	// 원본 배열이 쓰레기 데이터로 가득 차는 '메모리 누수' 현상이 발생할 수도 있습니다.

	fmt.Println("\n=== 4. nil vs 빈 슬라이스 ===")
	nilVsEmpty()
}