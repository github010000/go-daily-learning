package main

import (
	"fmt"
	"sort"
)

func main() {
	fmt.Println("=== 맵(map) 완벽 가이드 ===")

	demonstrateNilMap()
	fmt.Println()
	demonstrateCommaOk()
	fmt.Println()
	demonstrateMapIteration()
	fmt.Println()
	demonstrateMapValueModification()
	fmt.Println()
	demonstrateDeleteAndLen()
}

// demonstrateNilMap 은 nil 맵에 쓰면 panic 하고, make 로 초기화하면 안전함을 보여줍니다.
func demonstrateNilMap() {
	fmt.Println("[1] nil 맵 vs 초기화 맵")

	// 실수: 선언만 하고 쓰면 panic
	fmt.Println("실수 버전: 선언만 한 nil 맵에 쓰기")
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("  panic 발생: %v\n", r)
			}
		}()
		var m map[string]int
		m["key"] = 1 // panic!
	}()

	// 고친 버전: make 로 초기화
	fmt.Println("고친 버전: make 로 초기화 후 쓰기")
	m := make(map[string]int)
	m["key"] = 1
	fmt.Printf("  m['key'] = %d\n", m["key"])

	// 읽기는 nil 맵에서도 안전 (zero value 반환)
	var nilMap map[string]int
	fmt.Printf("  nil 맵 읽기: m['key'] = %d (panic 없음)\n", nilMap["key"])
}

// demonstrateCommaOk 은 키가 존재하는지 확인하는 패턴을 보여줍니다.
func demonstrateCommaOk() {
	fmt.Println("[2] comma-ok 패턴: 값 존재 여부 확인")

	scores := map[string]int{
		"홍길동": 85,
		"김철수": 90,
	}

	// 존재하는 키
	if score, ok := scores["홍길동"]; ok {
		fmt.Printf("  '홍길동' 점수: %d (존재함)\n", score)
	}

	// 존재하지 않는 키
	if score, ok := scores["박영희"]; ok {
		fmt.Printf("  '박영희' 점수: %d\n", score)
	} else {
		fmt.Printf("  '박영희'는 맵에 없음 (default: 0)\n")
	}

	// 실수: comma-ok 없이 읽으면 0 반환으로 구별 불가
	fmt.Println("실수 버전: comma-ok 없이 읽기")
	score := scores["박영희"] // 0 반환 - 이게 '없는 값'인지 '0점'인지 모른다
	fmt.Printf("  '박영희' 점수: %d (0 인데, 없는 건지 0 점인지?)\n", score)

	// 고친 버전: comma-ok 사용
	fmt.Println("고친 버전: comma-ok 사용")
	if _, ok := scores["박영희"]; ok {
		fmt.Println("  '박영희' 있음")
	} else {
		fmt.Println("  '박영희' 없음 (명확하게 구분 가능)")
	}
}

// demonstrateMapIteration 은 맵 순회가 매번 다름을 보여줍니다.
func demonstrateMapIteration() {
	fmt.Println("[3] 맵 순회 순서: 무작위")

	m := map[string]int{
		"apple":  1,
		"banana": 2,
		"cherry": 3,
		"date":   4,
	}

	fmt.Println("순회 결과 (실행할 때마다 순서 다름):")
	for i := 0; i < 3; i++ {
		fmt.Printf("  [%d] ", i)
		for k := range m {
			fmt.Printf("%s ", k)
		}
		fmt.Println()
	}

	// 정렬 필요시: 키를 슬라이스로 꺼내서 sort
	fmt.Println("정렬된 순서로 보고 싶을 때:")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s: %d\n", k, m[k])
	}
}

// demonstrateMapValueModification 은 맵 값의 필드를 직접 수정하는 방법을 보여줍니다.
func demonstrateMapValueModification() {
	fmt.Println("[4] 맵 값의 필드 수정")

	type Person struct {
		Name string
		Age  int
	}

	// 실수: 값 타입으로 저장하면 직접 필드 수정 불가
	fmt.Println("실수 버전: 값 타입으로 저장")
	m1 := map[string]Person{
		"kim": {Name: "김철수", Age: 30},
	}
	// m1["kim"].Age = 31 // 컴파일 오류: assignment not allowed
	fmt.Println("  m1['kim'].Age = 31 → 컴파일 오류 (직접 수정 불가)")

	// 고친 버전 1: 전체 재할당
	fmt.Println("고친 버전 1: 전체 재할당")
	m1["kim"] = Person{Name: "김철수", Age: 31}
	fmt.Printf("  수정 후: %+v\n", m1["kim"])

	// 고친 버전 2: 포인터 사용 (권장)
	fmt.Println("고친 버전 2: 포인터 사용")
	m2 := map[string]*Person{
		"kim": {Name: "김철수", Age: 30},
	}
	m2["kim"].Age = 31
	fmt.Printf("  수정 후: %+v\n", *m2["kim"])

	// nil 체크 필요 (포인터일 때)
	fmt.Println("nil 체크 필요:")
	m3 := map[string]*Person{
		"lee": nil,
	}
	if p := m3["lee"]; p != nil {
		fmt.Printf("  lee: %+v\n", *p)
	} else {
		fmt.Println("  lee: nil (안전하게 처리)")
	}
}

// demonstrateDeleteAndLen 은 delete 와 len 동작을 보여줍니다.
func demonstrateDeleteAndLen() {
	fmt.Println("[5] delete 와 len")

	m := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	fmt.Printf("  초기: len=%d, 맵=%v\n", len(m), m)

	// 존재하는 키 삭제
	delete(m, "b")
	fmt.Printf("  'b' 삭제 후: len=%d, 맵=%v\n", len(m), m)

	// 존재하지 않는 키 삭제 (panic 없음)
	delete(m, "z")
	fmt.Printf("  'z' 삭제 후: len=%d, 맵=%v (panic 없음)\n", len(m), m)
}