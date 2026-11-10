package main

import "testing"

// TestValueReceiverDoesNotModifyOriginal: 값 리시버 메서드는 원본을 변경하지 않음을 검증한다.
// 이는 "실수 버전" GrowByValue()가 기대하는 동작과 일치한다 (변경 안 됨).
// 포인터 리시버 Grow()는 변경됨을 별도로 검증한다.
func TestValueReceiverDoesNotModifyOriginal(t *testing.T) {
	r := Rectangle{Width: 10, Height: 5}
	r.GrowByValue(2.0) // 값 리시버 메서드 호출
	if r.Width != 10 || r.Height != 5 {
		t.Errorf("값 리시버 메서드는 원본을 변경해서는 안 된다. got: Width=%.2f, Height=%.2f", r.Width, r.Height)
	}
}

// TestPointerReceiverModifiesOriginal: 포인터 리시버 메서드는 원본을 변경함을 검증한다.
func TestPointerReceiverModifiesOriginal(t *testing.T) {
	r := Rectangle{Width: 10, Height: 5}
	r.Grow(2.0) // 포인터 리시버 메서드 호출
	if r.Width != 20 || r.Height != 10 {
		t.Errorf("포인터 리시버 메서드는 원본을 변경해야 한다. got: Width=%.2f, Height=%.2f", r.Width, r.Height)
	}
}

// TestNilReceiverSafety: nil 포인터 리시버 메서드가 panic 없이 안전히 0을 반환함을 검증한다.
func TestNilReceiverSafety(t *testing.T) {
	var c *Circle
	if c.SafeArea() != 0 {
		t.Errorf("nil 포인터 리시버 메서드는 0을 반환해야 한다.")
	}
}

// TestMethodSetOnInterface: 값 리시버 메서드만 구현해도 값과 포인터 모두 interface를 만족함을 검증한다.
func TestMethodSetOnInterface(t *testing.T) {
	var r Rectangle = Rectangle{Width: 10, Height: 5}
	var c Circle = Circle{Radius: 7}

	// 값 타입이 interface를 만족하는지 확인
	var _ interface {
		Area() float64
		Name() string
	} = r
	var _ interface {
		Area() float64
		Name() string
	} = c

	// 포인터 타입도 interface를 만족하는지 확인
	var _ interface {
		Area() float64
		Name() string
	} = &r
	var _ interface {
		Area() float64
		Name() string
	} = &c
}