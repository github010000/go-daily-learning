package main

import (
	"fmt"
)

// 포인터의 개념을 시연하는 메인 함수
func main() {
	fmt.Println("=====================================================")
	fmt.Println("          Day 10: 포인터(Pointers) 학습          ")
	fmt.Println("=====================================================")

	// 1. 기본 변수 선언 및 값 확인
	originalValue := 100 // 원본 값
	fmt.Printf("1. 원본 변수 (originalValue) 값: %d
", originalValue)
	fmt.Printf("   &originalValue (주소): %p

", &originalValue) // 주소 연산자(&) 사용

	// 2. 포인터 변수 선언 (int 포인터 타입) 및 주소 저장
	var ptr *int
	ptr = &originalValue // originalValue의 메모리 주소를 ptr에 저장

	fmt.Printf("2. 포인터 변수 (ptr)에 저장된 주소: %p
", ptr)
	// 역참조(*)를 사용하여 주소가 가리키는 값에 접근
	fmt.Printf("   *ptr (역참조된 값): %d

", *ptr) 

	// 3. 값 전달 vs 포인터 전달 (함수 호출을 통한 비교)
	fmt.Println("-----------------------------------------------------")	
	// --- A. 값 전달 (Pass by Value) 테스트 ---
	valueCopy := originalValue // 값 복사
	fmt.Printf("3-A. 함수 호출 전 값 (valueCopy): %d
", valueCopy)
	
	// 값 전달은 원본에 영향을 주지 않음
	incrementByValue(valueCopy)
	fmt.Printf("3-A. 함수 호출 후 값 (valueCopy): %d (변화 없음)
", valueCopy)
	fmt.Printf("3-A. 원본 값 (originalValue): %d (변화 없음)

", originalValue)

	fmt.Println("-----------------------------------------------------")
	
	// --- B. 포인터 전달 (Pass by Pointer) 테스트 ---
	// 포인터 변수를 사용하여 원본 주소를 함수에 전달
	fmt.Printf("3-B. 함수 호출 전 원본 값 (originalValue): %d
", originalValue)
	
	// &originalValue (주소)를 &원래포인터변수 (포인터의 주소)가 아닌, 포인터 자체를 전달해야 함
	// 여기서는 &originalValue를 포인터 변수로 사용
	incrementByPointer(&originalValue) 
	
	fmt.Printf("3-B. 함수 호출 후 원본 값 (originalValue): %d (값이 변경됨)
", originalValue)
	fmt.Println("=====================================================")

}

// incrementByValue: 값 복사본을 받기 때문에 원본 값을 변경할 수 없음 (Pass by Value)
func incrementByValue(x int) {
	fmt.Println("   [함수 내부] valueCopy가 받은 값:", x)
	fmt.Println("   [함수 내부] valueCopy 증가 시도...")
	// 이 변경은 함수 스택 내의 복사본에만 영향을 줌
	x = x + 10
	fmt.Println("   [함수 내부] 변경된 valueCopy 값:", x) 
}

// incrementByPointer: 포인터를 받기 때문에 원본 메모리 위치의 값을 변경할 수 있음 (Pass by Reference via Pointer)
func incrementByPointer(p *int) {
	fmt.Println("   [함수 내부] 포인터를 통해 접근하는 원본 값: