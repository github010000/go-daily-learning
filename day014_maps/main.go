package main

import (
	"fmt"
)

func main() {
	// 1. 맵 리터럴을 이용한 초기화 (Map Literal)
	// 선언과 동시에 값을 할당할 수 있습니다.
	studentScores := map[string]int{
		"Alice": 90,
		"Bob":   85,
		"Charlie": 95,
	}
	fmt.Println("1. 초기 학생 점수:", studentScores)

	// 2. make 함수를 이용한 맵 초기화
	// 빈 맵을 만들 때는 반드시 make를 사용해야 합니다. 
	// 선언만 하고 초기화하지 않은 맵(nil map)에 값을 넣으려고 하면 런타임 에러가 발생합니다.
	grades := make(map[string]int)
	grades["David"] = 88
	grades["Eve"] = 92
	fmt.Println("2. make로 생성된 점수:", grades)

	// 3. 값 추가 및 수정
	// 기존에 있는 키에 값을 넣으면 수정(Update)되고, 없는 키라면 추가(Insert)됩니다.
	studentScores["Alice"] = 95 // 수정
	studentScores["Frank"] = 70  // 추가
	fmt.Println("3. Alice 점수 수정 및 Frank 추가:", studentScores)

	// 4. 키 존재 여부 확인 (Comma ok idiom)
	// 맵에서 값을 꺼낼 때 두 번째 반환값(ok)을 통해 키가 존재하는지 확인할 수 있습니다.
	searchKey := "Bob"
	score, ok := studentScores[searchKey]
	if ok {
		fmt.Printf("4. %s의 점수는 %d점입니다.\n", searchKey, score)
	} else {
		fmt.Printf("4. %s의 점수 정보가 없습니다.\n", searchKey)
	}

	missingKey := "Grace"
	_, ok = studentScores[missingKey]
	if !ok {
		fmt.Printf("4. %s는 명단에 존재하지 않습니다.\n", missingKey)
	}

	// 5. 요소 삭제 (delete)
	// delete 함수를 사용하여 특정 키와 그에 해당하는 값을 삭제합니다.
	// 존재하지 않는 키를 삭제하려고 해도 에러가 발생하지 않고 아무 일도 일어나지 않습니다.
	fmt.Println("5. 삭제 전:", studentScores)
	delete(studentScores, "Bob")
	fmt.Println("   Bob 삭제 후:", studentScores)

	// 6. 맵 순회 (Iteration)
	// for range 문을 사용하여 맵의 모든 키-값 쌍을 순회할 수 있습니다.
	// 주의: 맵의 순회 순서는 매번 달라질 수 있습니다(Random order).
	fmt.Println("6. 전체 학생 명단 순회:")
	for name, score := range studentScores {
		fmt.Printf("   - 이름: %s, 점수: %d\n", name, score)
	}
}