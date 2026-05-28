package main

import "fmt"

// Node는 제네릭 연결 리스트의 노드를 나타냅니다.
type Node[T any] struct {
	Value T
	Next  *Node[T]
}

// LinkedList는 제네릭 연결 리스트 구조체입니다.
type LinkedList[T any] struct {
	Head   *Node[T]
	Length int
}

// NewLinkedList는 새 제네릭 연결 리스트 인스턴스를 생성합니다.
func NewLinkedList[T any]() *LinkedList[T] {
	return &LinkedList[T]{}
}

// PushBack은 리스트 끝에 새 값을 추가합니다.
func (ll *LinkedList[T]) PushBack(value T) {
	newNode := &Node[T]{Value: value}
	if ll.Head == nil {
		ll.Head = newNode
	} else {
		curr := ll.Head
		for curr.Next != nil {
			curr = curr.Next
		}
		curr.Next = newNode
	}
	ll.Length++
}

// PopBack은 리스트 끝의 값을 제거하고 반환합니다.
// 값이 없다면 제네릭 기본값(zero value)과 false를 반환합니다.
func (ll *LinkedList[T]) PopBack() (T, bool) {
	var zero T
	if ll.Head == nil {
		return zero, false
	}
	if ll.Head.Next == nil {
		value := ll.Head.Value
		ll.Head = nil
		ll.Length--
		return value, true
	}
	curr := ll.Head
	for curr.Next.Next != nil {
		curr = curr.Next
	}
	value := curr.Next.Value
	curr.Next = nil
	ll.Length--
	return value, true
}

// PeekFront은 리스트 앞의 값을 제거하지 않고 확인합니다.
func (ll *LinkedList[T]) PeekFront() (T, bool) {
	var zero T
	if ll.Head == nil {
		return zero, false
	}
	return ll.Head.Value, true
}

// Len은 현재 리스트에 저장된 요소의 개수를 반환합니다.
func (ll *LinkedList[T]) Len() int {
	return ll.Length
}

// ForEach는 연결 리스트의 모든 노드를 순회하며 콜백 함수를 실행합니다.
func (ll *LinkedList[T]) ForEach(fn func(T)) {
	for curr := ll.Head; curr != nil; curr = curr.Next {
		fn(curr.Value)
	}
}

func main() {
	// 1. 정수(int) 타입 연결 리스트 생성 및 사용
	intList := NewLinkedList[int]()
	intList.PushBack(10)
	intList.PushBack(20)
	intList.PushBack(30)

	fmt.Println("=== 정수형 연결 리스트 ===")
	fmt.Printf("현재 길이: %d\n", intList.Len())
	fmt.Print("순회 출력: ")
	intList.ForEach(func(v int) { fmt.Printf("%d ", v) })
	fmt.Println()

	top, ok := intList.PopBack()
	fmt.Printf("PopBack: %d (성공: %v)\n", top, ok)

	front, ok := intList.PeekFront()
	fmt.Printf("PeekFront: %d (성공: %v)\n", front, ok)
	fmt.Println()

	// 2. 문자열(string) 타입 연결 리스트 생성 및 사용
	// 제네릭을 사용하면 타입만 바꾸면 동일한 로직을 재사용할 수 있습니다.
	strList := NewLinkedList[string]()
	strList.PushBack("Hello")
	strList.PushBack("Go")
	strList.PushBack("Generics")

	fmt.Println("=== 문자열형 연결 리스트 ===")
	fmt.Printf("현재 길이: %d\n", strList.Len())
	fmt.Print("순회 출력: ")
	strList.ForEach(func(v string) { fmt.Printf("%s ", v) })
	fmt.Println()

	last, ok := strList.PopBack()
	fmt.Printf("PopBack: %s (성공: %v)\n", last, ok)
}