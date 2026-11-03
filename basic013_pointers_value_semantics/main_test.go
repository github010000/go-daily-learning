package main

import "testing"

// TestSwapValueVsPointer는 값 전달과 포인터 전달의 차이를 검증합니다.
// swapVal은 복사본만 바꾸므로 원본이 변하지 않아야 하고,
// swapPtr은 역참조를 통해 원본을 바꾸어야 합니다.
func TestSwapValueVsPointer(t *testing.T) {
	t.Run("값 전달은 원본을 바꾸지 않는다", func(t *testing.T) {
		a, b := 10, 20
		swapVal(a, b)
		if a != 10 {
			t.Errorf("a는 10이어야 함. got %d", a)
		}
		if b != 20 {
			t.Errorf("b는 20이어야 함. got %d", b)
		}
	})

	t.Run("포인터 전달은 원본을 바꾼다", func(t *testing.T) {
		c, d := 10, 20
		swapPtr(&c, &d)
		if c != 20 {
			t.Errorf("c는 20이어야 함. got %d", c)
		}
		if d != 10 {
			t.Errorf("d는 10이어야 함. got %d", d)
		}
	})
}

// TestNilDerefPanics는 nil 포인터 역참조 시 함수가 panic을 recover하여
// 정상 종료되는지를 검증합니다. panic이 잡히지 않으면 테스트가 실패합니다.
func TestNilDerefPanics(t *testing.T) {
	nilDeref() // recover가 있으면 정상 종료, 없으면 panic으로 테스트 실패
}

// TestSliceAppend는 badAppend(반환값 무시)와 goodAppend(반환값 사용)의 차이를 검증합니다.
func TestSliceAppend(t *testing.T) {
	t.Run("badAppend는 원본을 바꾸지 않는다", func(t *testing.T) {
		s := []int{1, 2, 3}
		badAppend(s)
		if len(s) != 3 {
			t.Errorf("s의 길이는 3이어야 함. got %d", len(s))
		}
	})

	t.Run("goodAppend는 반환값을 받아야 한다", func(t *testing.T) {
		s := []int{1, 2, 3}
		s = goodAppend(s)
		if len(s) != 4 {
			t.Errorf("s의 길이는 4이어야 함. got %d", len(s))
		}
		if s[3] != 99 {
			t.Errorf("s[3]는 99이어야 함. got %d", s[3])
		}
	})
}

// TestSliceElementMutation은 슬라이스 헤더는 복사되지만 underlying 배열은 공유되므로,
// 요소 값 수정은 원본에 반영되는지를 검증합니다.
func TestSliceElementMutation(t *testing.T) {
	s := []int{1, 2, 3}
	setFirst(s, 99)
	if s[0] != 99 {
		t.Errorf("s[0]는 99이어야 함. got %d", s[0])
	}
}

// TestStructValueVsPointer는 값 복사 구조체 함수와 포인터 구조체 함수의 차이를 검증합니다.
func TestStructValueVsPointer(t *testing.T) {
	t.Run("값 복사는 원본을 바꾸지 않는다", func(t *testing.T) {
		p := Person{Name: "Alice", Age: 30}
		copyPerson(p)
		if p.Age != 30 {
			t.Errorf("p.Age는 30이어야 함. got %d", p.Age)
		}
	})

	t.Run("포인터 전달은 원본을 바꾼다", func(t *testing.T) {
		p := Person{Name: "Bob", Age: 25}
		ptrPerson(&p)
		if p.Age != 99 {
			t.Errorf("p.Age는 99이어야 함. got %d", p.Age)
		}
	})
}