package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("=== 1. 기본 for 루프 ===")
	showBasicFor()

	fmt.Println("\n=== 2. 조건만 있는 for (while 스타일) ===")
	showWhileStyleFor()

	fmt.Println("\n=== 3. 무한 루프 ===")
	showInfiniteLoop()

	fmt.Println("\n=== 4. range: 슬라이스 ===")
	showRangeSlice()

	fmt.Println("\n=== 5. range: 문자열 (rune 단위) ===")
	showRangeString()

	fmt.Println("\n=== 6. 레이블과 break (중첩 루프 한 번에 탈출) ===")
	showLabelBreak()

	fmt.Println("\n=== 7. 레이블과 continue (외부 루프 건너뛰기) ===")
	showLabelContinue()

	fmt.Println("\n=== 8. Go 1.22: 변수 선언 없이 range ===")
	showRangeNoVar()
}

// showBasicFor: C/Java 스타일의 기본 for 루프
// - 초기화; 조건; 증감 순서
// - Go에서는 세 부분 모두 선택적
func showBasicFor() {
	fmt.Print("  1부터 5까지: ")
	for i := 1; i <= 5; i++ {
		fmt.Print(i, " ")
	}
	fmt.Println()

	// 실수 버전: 증감식에서 ++ 대신 += 1을 쓰는 것은 허용되지만
	// 관례적으로 ++를 사용
	fmt.Println("  (참고) i++ 와 i += 1 은 모두 작동")
}

// showWhileStyleFor: 조건만 있는 for 루프
// - Go에는 while이 없음. for로만 작성
func showWhileStyleFor() {
	count := 0
	fmt.Print("  카운트 0부터 2까지: ")
	for count <= 2 {
		fmt.Print(count, " ")
		count++
	}
	fmt.Println()
}

// showInfiniteLoop: 무한 루프
// - break로 탈출해야 함
func showInfiniteLoop() {
	fmt.Println("  무한 루프에서 3회 후 탈출:")
	i := 0
	for {
		if i >= 3 {
			break
		}
		fmt.Printf("    i=%d\n", i)
		i++
	}
}

// showRangeSlice: range 기반 슬라이스 순회
// - 인덱스와 값을 동시에 받음
func showRangeSlice() {
	names := []string{"홍길동", "김영희", "이순신"}
	fmt.Println("  이름 목록:")
	for i, name := range names {
		fmt.Printf("    인덱스 %d: %s\n", i, name)
	}

	// 실수: 인덱스는 필요 없지만 변수로 받으면 컴파일 오류
	fmt.Println("  (실수) 인덱스를 받지 않는데 변수로 받으면:")
	fmt.Println("    컴파일 오류: i declared and not used")
	fmt.Println("    (고침) 인덱스를 무시하려면 _ 사용: for _, name := range names")
}

// showRangeString: 문자열 range 루프 (UTF-8 코드포인트 순회)
// - byte 단위가 아닌 rune(유니코드 코드포인트) 단위로 순회
func showRangeString() {
	text := "안녕👋"
	fmt.Printf("  문자열: %s\n", text)
	fmt.Printf("  rune 수: %d\n", len([]rune(text)))
	fmt.Println("  range로 순회 (rune 단위):")
	for i, r := range text {
		fmt.Printf("    위치 %d: %c (U+%04X)\n", i, r, r)
	}

	// 실수 버전: byte로 순회하면 UTF-8이 깨짐
	fmt.Println("  byte로 순회 (틀린 방법):")
	for i, b := range []byte(text) {
		fmt.Printf("    위치 %d: %d (0x%02X) -> %q\n", i, b, b, string(b))
	}
	fmt.Println("    ↑ 한국어/이모지가 깨져서 나옵니다")
}

// showLabelBreak: 레이블과 break로 중첩 루프 한 번에 탈출
// - break는 가장 안쪽 루프만 탈출
// - 레이블을 쓰면 중첩 루프 전체에서 탈출 가능
func showLabelBreak() {
	matrix := [][]int{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}

	fmt.Println("  2D 슬라이스에서 5를 찾으면 탈출:")

	// 실수 버전: break만 쓰면 안쪽 루프만 탈출, 바깥 루프는 계속 돎
	fmt.Println("  [실수] break만 사용 (안쪽만 탈출):")
	found := false
	for i, row := range matrix {
		for j, val := range row {
			if val == 5 {
				found = true
				break // ← 이 break는 안쪽 루프만 탈출!
			}
			fmt.Printf("    확인: (%d,%d)=%d\n", i, j, val)
		}
		if found {
			fmt.Println("    5를 찾았음 (바깥 루프도 종료)")
		} else {
			fmt.Printf("    행 %d 계속 탐색\n", i)
		}
	}

	// 고친 버전: 레이블과 break로 한 번에 탈출
	fmt.Println("  [고침] 레이블과 break 사용 (전체 탈출):")
searchLoop:
	for i, row := range matrix {
		for j, val := range row {
			fmt.Printf("    확인: (%d,%d)=%d\n", i, j, val)
			if val == 5 {
				fmt.Println("    5를 찾았음!")
				break searchLoop // ← 이 break는 searchLoop에서 탈출!
			}
		}
	}
	fmt.Println("  (이곳으로 바로 이동)")
}

// showLabelContinue: 레이블과 continue로 외부 루프 건너뛰기
func showLabelContinue() {
	fmt.Println("  2의 배수가 아닌 행은 건너뛰기:")
	matrix := [][]int{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
	}

	// 실수: continue만 쓰면 안쪽 루프만 건너뜀
	fmt.Println("  [실수] continue만 사용:")
	for i, row := range matrix {
		if len(row) > 0 && row[0]%2 != 0 { // 첫 번째 원소가 홀수
			fmt.Printf("    행 %d 건너뛰기 (첫 번째 원소 %d이므로)\n", i, row[0])
			// continue는 여기에서 안쪽 루프만 건너뜀 (행은 이미 건너뜀)
		}
		for j, val := range row {
			fmt.Printf("      (%d,%d)=%d\n", i, j, val)
		}
	}

	// 고침: 레이블과 continue로 전체 행 건너뛰기
	fmt.Println("  [고침] 레이블과 continue 사용:")
outerLoop:
	for i, row := range matrix {
		if len(row) > 0 && row[0]%2 != 0 {
			fmt.Printf("    행 %d 건너뛰기 (첫 번째 원소 %d이므로)\n", i, row[0])
			continue outerLoop // ← 이 continue는 outerLoop의 다음 반복으로
		}
		for j, val := range row {
			fmt.Printf("      (%d,%d)=%d\n", i, j, val)
		}
	}
}

// showRangeNoVar: Go 1.22+에서 변수 선언 없이 range 사용
// - 인덱스나 값이 필요 없으면 변수 선언 불필요
func showRangeNoVar() {
	values := []int{10, 20, 30}

	fmt.Println("  값만 필요할 때 (변수 선언 생략):")
	for _, v := range values {
		fmt.Printf("    %d\n", v)
	}

	fmt.Println("  인덱스만 필요할 때 (변수 선언 생략):")
	for i := range values {
		fmt.Printf("    인덱스 %d\n", i)
	}

	// Go 1.22+: 인덱스와 값을 모두 받고 싶지 않으면 변수 선언 없이도 가능
	fmt.Println("  무조건 반복만 (변수 선언 없이):")
	for range values {
		fmt.Println("    반복!")
	}
}