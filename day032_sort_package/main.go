package main

import (
	"fmt"
	"sort"
)

// 구조체 정의 (sort.Slice 및 sort.Interface 예제용)
type Student struct {
	Name  string
	Score int
}

// sort.Interface를 구현하기 위한 구조체
type StudentByScore struct {
	Students []Student
}

// Len, Less, Swap 메서드를 구현하여 sort.Interface 만족
func (s StudentByScore) Len() int      { return len(s.Students) }
func (s StudentByScore) Less(i, j int) bool { return s.Students[i].Score < s.Students[j].Score }
func (s StudentByScore) Swap(i, j int) { s.Students[i], s.Students[j] = s.Students[j], s.Students[i] }

func main() {
	// 1. 기본 슬라이스 정렬 (Ints, Strings)
	nums := []int{3, 1, 4, 1, 5, 9, 2, 6}
	fmt.Println("1. 정수 슬라이스 정렬 전:", nums)
	sort.Ints(nums)
	fmt.Println("1. 정수 슬라이스 정렬 후:", nums)

	words := []string{"banana", "apple", "cherry"}
	fmt.Println("2. 문자열 슬라이스 정렬 전:", words)
	sort.Strings(words)
	fmt.Println("2. 문자열 슬라이스 정렬 후:", words)

	// 2. sort.Slice을 이용한 구조체 슬라이스 정렬
	students := []Student{
		{"Alice", 85},
		{"Bob", 92},
		{"Charlie", 78},
	}
	fmt.Println("3. 구조체 슬라이스 정렬 전:", students)
	// 점수가 높은 순으로 정렬 (내림차순)
	sort.Slice(students, func(i, j int) bool {
		return students[i].Score > students[j].Score
	})
	fmt.Println("3. 구조체 슬라이스 정렬 후 (점수 내림차순):", students)

	// 3. sort.Interface 구현을 통한 커스텀 정렬
	customStudents := StudentByScore{
		Students: []Student{
			{"Dave", 88},
			{"Eve", 95},
			{"Frank", 80},
		},
	}
	fmt.Println("4. sort.Interface 적용 전:", customStudents.Students)
	sort.Sort(customStudents) // Interface 메서드를 이용해 정렬
	fmt.Println("4. sort.Interface 적용 후 (점수 오름차순):", customStudents.Students)

	// 4. sort.Search를 이용한 이진 탐색
	// 이진 탐색은 이미 정렬된 슬라이스에서만 동작하므로 주의
	sortedNums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	target := 5
	idx := sort.Search(len(sortedNums), func(i int) bool {
		return sortedNums[i] >= target
	})
	fmt.Println("5. 이진 탐색 결과 (인덱스):", idx, "값:", sortedNums[idx])

	// 탐색 범위를 넘거나 조건을 만족하지 않는 경우 처리 예시
	target2 := 10
	idx2 := sort.Search(len(sortedNums), func(i int) bool {
		return sortedNums[i] >= target2
	})
	if idx2 < len(sortedNums) && sortedNums[idx2] == target2 {
		fmt.Println("5. target2 찾음:", idx2)
	} else {
		fmt.Println("5. target2 없음 (인덱스:", idx2, ")")
	}
}