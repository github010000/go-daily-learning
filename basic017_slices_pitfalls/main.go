package main

import (
	"fmt"
)

func main() {
	fmt.Println("=== 슬라이스 함정 시연 ===")
	fmt.Println()

	showAppendCapacityTrap()
	showSharedSliceBugs()
	showMemoryLeak()
	showCopyRules()
}

// showAppendCapacityTrap는 append가 원본 배열을 때때로 갱신하는 이유를 보여줍니다.
// Go는 슬라이스가 "배열 포인터 + 길이 + 용량" 3종 세트로 동작합니다.
func showAppendCapacityTrap() {
	fmt.Println("--- 1. append: 용량(cap)에 따른 동작 차이 ---")
	fmt.Println()

	fmt.Println("【실수 버전】용량을 미리 확보하지 않아 append 후 원본이 깨집니다")
	original := []int{1, 2}
	fmt.Printf("  원본 슬라이스 (len=%d, cap=%d): %v\n", len(original), cap(original), original)

	// 용량이 부족하면 append는 새 배열을 할당할 수 있지만,
	// 우리는 'original'을 계속 참조하므로 새 배열을 잃어버립니다.
	// 이 코드는 실제로는 안전해 보이지만, 용량이 부족할 때 새 배열을 반환합니다.
	// 여기서 중요한 함정은 "하위 슬라이스를 만들고 append하는" 경우입니다.
	base := []int{10, 20, 30}
	// 하위 슬라이스: [20] (len=1, cap=2)
	sub := base[1:2]
	fmt.Printf("  base  : %v (len=%d, cap=%d)\n", base, len(base), cap(base))
	fmt.Printf("  sub   : %v (len=%d, cap=%d)\n", sub, len(sub), cap(sub))

	// sub의 용량은 2이지만, base는 3입니다.
	// sub에 3개 원소를 추가하면 base의 3번째 원소(30)를 덮어씁니다!
	sub = append(sub, 99, 88)
	fmt.Printf("  subappend 후 sub: %v\n", sub)
	fmt.Printf("  subappend 후 base: %v  ← base가 변했습니다!\n", base)
	fmt.Println("  원인: sub와 base가 같은 underlying array를 공유하고, cap이 부족해")
	fmt.Println("        base의 여유 공간(30)을 sub가 사용했기 때문입니다.")
	fmt.Println()

	fmt.Println("【고친 버전】append 결과를 새 변수에 받고, 필요시 깊은 복사 사용")
	base2 := []int{10, 20, 30}
	sub2 := base2[1:2]
	original2 := make([]int, len(sub2))
	copy(original2, sub2) // 깊은 복사: 원본과 독립적인 슬라이스 생성

	original2 = append(original2, 99, 88)
	fmt.Printf("  base2 : %v (변경 없음)\n", base2)
	fmt.Printf("  original2: %v  ← base2와 독립적입니다\n", original2)
	fmt.Println()
}

// showSharedSliceBugs는 하위 슬라이스 공유로 인한 함정을 보여줍니다.
func showSharedSliceBugs() {
	fmt.Println("--- 2. 공유 슬라이스: 하위 배열이 서로 영향을 줍니다 ---")
	fmt.Println()

	fmt.Println("【실수 버전】함수가 받은 슬라이스를 내부에서 변경합니다")
	data := []int{100, 200, 300, 400}
	fmt.Printf("  호출 전 data : %v\n", data)

	modified := mutateSlice(data)
	fmt.Printf("  호출 후 data : %v  ← mutateSlice가 data를 갱신했습니다!\n", data)
	fmt.Printf("  함수 반환값  : %v\n", modified)
	fmt.Println("  원인: 슬라이스는 배열을 '참조'합니다. 함수에 슬라이스를 넘기면")
	fmt.Println("        함수가 같은 underlying array를 수정할 수 있습니다.")
	fmt.Println()

	fmt.Println("【고친 버전】인자를 깊은 복사한 뒤 작업합니다")
	data2 := []int{100, 200, 300, 400}
	fmt.Printf("  호출 전 data2: %v\n", data2)

	modified2 := mutateSliceSafe(data2)
	fmt.Printf("  호출 후 data2: %v  ← 원본이 보존됩니다\n", data2)
	fmt.Printf("  함수 반환값  : %v\n", modified2)
	fmt.Println()
}

// mutateSlice는 슬라이스를 받아 첫 번째 원소를 0으로 만듭니다.
// 이 함수는 callers의 데이터를 변조합니다 — 의도치 않은 side effect입니다.
func mutateSlice(s []int) []int {
	if len(s) > 0 {
		s[0] = 0 // callers의 배열을 직접 수정
	}
	return s
}

// mutateSliceSafe는 슬라이스를 받아 깊은 복사 후 작업합니다.
func mutateSliceSafe(s []int) []int {
	// 깊은 복사: callers의 배열과 무관한 새 배열을 만듭니다.
	copied := make([]int, len(s))
	copy(copied, s)

	if len(copied) > 0 {
		copied[0] = 0
	}
	return copied
}

// showMemoryLeak는 하위 슬라이스가 전체 배열을 메모리에 고정시키는 현상을 보여줍니다.
func showMemoryLeak() {
	fmt.Println("--- 3. 메모리 누수: 하위 슬라이스가 큰 배열을 붙잡습니다 ---")
	fmt.Println()

	// 큰 배열을 simulated 합니다. 실제론 100만 원소로 하면 더 명확합니다.
	largeArray := make([]byte, 1_000_000)
	for i := range largeArray {
		largeArray[i] = byte(i % 256)
	}
	fmt.Printf("  largeArray 크기: %d bytes (약 %.1f MB)\n", len(largeArray), float64(len(largeArray))/1024/1024)

	// 하위 슬라이스: 첫 10바이트만 필요하지만...
	tinySlice := largeArray[:10]
	fmt.Printf("  tinySlice len  : %d\n", len(tinySlice))
	fmt.Printf("  tinySlice cap  : %d  ← cap이 전체 배열을 가리킵니다!\n", cap(tinySlice))

	// largeArray를 nil로 만들어도...
	largeArray = nil
	fmt.Println("  largeArray를 nil로 만들었습니다")
	fmt.Println("  하지만 tinySlice가 그 배열을 계속 참조하므로 GC가 회수하지 못합니다")
	fmt.Println("  → tinySlice 하나 때문에 1MB를 잡아먹는 메모리 누수가 발생합니다")
	fmt.Println()

	fmt.Println("【고친 버전】copy로 독립적인 작은 슬라이스를 만듭니다")
	largeArray2 := make([]byte, 1_000_000)
	for i := range largeArray2 {
		largeArray2[i] = byte(i % 256)
	}

	// copy: 데이터를 새 메모리로 이동시킵니다.
	safeTiny := make([]byte, 10)
	copy(safeTiny, largeArray2[:10])
	largeArray2 = nil // 이제 GC가 1MB를 회수할 수 있습니다

	fmt.Printf("  safeTiny len : %d, cap: %d\n", len(safeTiny), cap(safeTiny))
	fmt.Println("  largeArray2는 nil이 되었으며, GC가 1MB를 회수할 수 있습니다")
	fmt.Println("  safeTiny는 독립된 10바이트만 가리킵니다")
	fmt.Println()
}

// showCopyRules는 copy의 길이 규칙을 보여줍니다.
func showCopyRules() {
	fmt.Println("--- 4. copy: 길이 규칙과 반환값 ---")
	fmt.Println()

	src := []int{1, 2, 3, 4, 5}
	dst := make([]int, 3)

	n := copy(dst, src)
	fmt.Printf("  src : %v (len=%d)\n", src, len(src))
	fmt.Printf("  dst : %v (len=%d) ← copy 전\n", dst, len(dst))
	fmt.Printf("  copy(dst, src) 반환값: %d\n", n)
	fmt.Printf("  dst : %v (len=%d) ← copy 후\n", dst, len(dst))
	fmt.Println("  copy는 min(len(dst), len(src))만큼 복사하고 그 수를 반환합니다")
	fmt.Println()

	// dst가 더 긴 경우
	src2 := []int{1, 2}
	dst2 := make([]int, 5)
	n2 := copy(dst2, src2)
	fmt.Printf("  src2: %v (len=%d)\n", src2, len(src2))
	fmt.Printf("  dst2: %v (len=%d) ← copy 전\n", dst2, len(dst2))
	fmt.Printf("  copy(dst2, src2) 반환값: %d\n", n2)
	fmt.Printf("  dst2: %v (len=%d) ← copy 후 (남는 곳은 0으로 초기화됨)\n", dst2, len(dst2))
	fmt.Println("  dst가 더 길면 남은 위치는 0으로 채워집니다")
	fmt.Println()

	// 오버랩 복사 (동일 슬라이스 내에서 이동)
	overlapSrc := []int{1, 2, 3, 4, 5}
	// [2, 3, 4, 5, ?]로 왼쪽으로 한 칸 이동
	copy(overlapSrc[:4], overlapSrc[1:])
	fmt.Printf("  오버랩 복사: %v  ← src[1:]를 src[:4]에 복사\n", overlapSrc)
	fmt.Println("  copy는 오버랩을 안전하게 처리합니다 (내부적으로 임시 버퍼 사용)")
	fmt.Println()
}