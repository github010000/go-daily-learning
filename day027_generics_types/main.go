package main

import (
	"fmt"
	"golang.org/x/exp/constraints"
)

// 1. 제네릭 구조체: 모든 타입을 허용하는 컨테이너
type Container[T any] struct {
	Value T
}

// 제네릭 메서드: 저장된 값을 반환
func (c Container[T]) GetValue() T {
	return c.Value
}

// 2. 제네릭 구조체: constraints 패키지의 Integer 제약 사용
type NumberContainer[T constraints.Integer] struct {
	Value T
}

// 제네릭 메서드: 제약을 만족하는 타입에서만 컴파일됨
func (nc NumberContainer[T]) Double() T {
	return nc.Value * 2
}

// 3. 제네릭 스택 구현
type Stack[T any] struct {
	items []T
}

// 스택 생성자 (제네릭 함수)
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{items: make([]T, 0)}
}

// 아이템 추가
func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

// 아이템 제거 및 반환, 스택이 비어있으면 zero value와 false 반환
func (s *Stack[T]) Pop() (T, bool) {
	if s.IsEmpty() {
		var zero T
		return zero, false
	}
	index := len(s.items) - 1
	item := s.items[index]
	s.items = s.items[:index]
	return item, true
}

// 스택이 비어있는지 확인
func (s *Stack[T]) IsEmpty() bool {
	return len(s.items) == 0
}

// 4. constraints.Ordered 활용 예시 (제네릭 함수)
func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func main() {
	// --- Container 테스트 ---
	strContainer := Container[string]{Value: "Hello Go Generics"}
	fmt.Println("1. Container Value:", strContainer.GetValue())

	intContainer := Container[int]{Value: 42}
	fmt.Println("1. Container Value:", intContainer.GetValue())

	// --- NumberContainer 테스트 ---
	numContainer := NumberContainer[int]{Value: 5}
	fmt.Println("2. NumberContainer Value:", numContainer.Value, "Doubled:", numContainer.Double())

	// --- Stack 테스트 ---
	strStack := NewStack[string]()
	strStack.Push("A")
	strStack.Push("B")
	fmt.Println("3. Stack Pop:", strStack.Pop())
	fmt.Println("3. Stack Pop:", strStack.Pop())
	fmt.Println("3. Stack Empty:", strStack.IsEmpty())

	intStack := NewStack[int]()
	intStack.Push(100)
	intStack.Push(200)
	fmt.Println("3. Int Stack Pop:", intStack.Pop())
	fmt.Println("3. Int Stack Empty:", intStack.IsEmpty())

	// --- constraints.Ordered 함수 테스트 ---
	fmt.Println("4. Max Integer:", Max(10, 20))
	fmt.Println("4. Max Float:", Max(3.14, 2.71))
}