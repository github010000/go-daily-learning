package main

import (
	"testing"
)

// TestStructureEmbeddingPromotion: 임베딩된 필드와 메서드가 승격되는지 확인
// Circle이 Shape를 임베딩했으므로, Circle 인스턴스에서 Shape의 필드와 메서드를 직접 접근할 수 있어야 합니다.
func TestStructureEmbeddingPromotion(t *testing.T) {
	tests := []struct {
		name     string
		circle   Circle
		wantName string
		wantX    int
		wantInfo string
	}{
		{
			name:     "기본 Circle",
			circle:   Circle{Shape: Shape{Name: "원", X: 10, Y: 20}, Radius: 5},
			wantName: "원",
			wantX:    10,
			wantInfo: "Shape[원] at (10,20)",
		},
		{
			name:     "원점이 있는 Circle",
			circle:   Circle{Shape: Shape{Name: "단위 원", X: 0, Y: 0}, Radius: 1},
			wantName: "단위 원",
			wantX:    0,
			wantInfo: "Shape[단위 원] at (0,0)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 승격된 필드 접근 검증
			if tt.circle.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", tt.circle.Name, tt.wantName)
			}
			if tt.circle.X != tt.wantX {
				t.Errorf("X = %d, want %d", tt.circle.X, tt.wantX)
			}

			// 승격된 메서드 호출 검증
			if got := tt.circle.Info(); got != tt.wantInfo {
				t.Errorf("Info() = %q, want %q", got, tt.wantInfo)
			}

			// 명시적 내부 구조체 접근도 가능해야 함
			if tt.circle.Shape.Name != tt.wantName {
				t.Errorf("Shape.Name = %q, want %q", tt.circle.Shape.Name, tt.wantName)
			}
		})
	}
}

// TestInterfaceEmbedding: 임베딩을 통한 인터페이스 충족 확인
// PrintableCircle이 Shape를 임베딩하고 Description() 메서드를 가지므로,
// Describable 인터페이스를 자동으로 충족해야 합니다.
func TestInterfaceEmbedding(t *testing.T) {
	pc := PrintableCircle{Shape: Shape{Name: "테스트 원", X: 5, Y: 5}}

	// Describable 인터페이스로 할당 가능해야 함
	var d Describable = pc

	// Description() 메서드가 올바르게 동작하는지 확인
	wantDesc := "테스트 원의 면적은 3.14"
	if got := d.Description(); got != wantDesc {
		t.Errorf("Description() = %q, want %q", got, wantDesc)
	}
}

// TestPointerEmbeddingBehavior: 포인터 임베딩이 참조 동작을 어떻게 바꾸는지 확인
// 값 타입 임베딩: 부모 객체 수정 시 자식 객체에 영향 없음
// 포인터 임베딩: 부모 객체 수정 시 자식 객체에도 영향 있음
func TestPointerEmbeddingBehavior(t *testing.T) {
	// 값 타입 임베딩 테스트
	base := &Shape{Name: "부모", X: 1}
	child := Circle{Shape: *base, Radius: 2}

	originalName := child.Name
	originalX := child.X

	// base 수정
	base.Name = "수정된 부모"
	base.X = 99

	// 값 임베딩이므로 child는 변하지 않음
	if child.Name != originalName {
		t.Errorf("child.Name changed to %q, should remain %q", child.Name, originalName)
	}
	if child.X != originalX {
		t.Errorf("child.X changed to %d, should remain %d", child.X, originalX)
	}

	// 포인터 임베딩 테스트
	basePtr := &Shape{Name: "부모", X: 1}
	childPtr := Circle{Shape: *basePtr, Radius: 2}

	originalNamePtr := childPtr.Name
	originalXPtr := childPtr.X

	// basePtr 수정
	basePtr.Name = "수정된 부모"
	basePtr.X = 99

	// 포인터 임베딩이므로 childPtr도 변함
	if childPtr.Name != basePtr.Name {
		t.Errorf("childPtr.Name should match basePtr.Name, got %q, want %q", childPtr.Name, basePtr.Name)
	}
	if childPtr.X != basePtr.X {
		t.Errorf("childPtr.X should match basePtr.X, got %d, want %d", childPtr.X, basePtr.X)
	}
}

// TestMethodOverride: 메서드 재정의 동작 확인
// DrawCircle이 Circle의 Draw() 메서드를 재정의했는지 확인합니다.
func TestMethodOverride(t *testing.T) {
	dc := DrawCircle{Circle{Shape: Shape{Name: "원", X: 0, Y: 0}, Radius: 5}}

	// DrawCircle의 Draw() 메서드가 재정의된 동작을 해야 함
	wantDraw := "원 그리기: 반지름 5"
	if got := dc.Draw(); got != wantDraw {
		t.Errorf("Draw() = %q, want %q", got, wantDraw)
	}

	// 하지만 Circle의 다른 메서드들은 그대로 사용 가능
	wantInfo := "Shape[원] at (0,0)"
	if got := dc.Info(); got != wantInfo {
		t.Errorf("Info() = %q, want %q", got, wantInfo)
	}
}

// TestRectArea: Rect의 면적 계산 확인
// Rect가 Shape를 임베딩하고 Area() 메서드를 올바르게 구현했는지 확인합니다.
func TestRectArea(t *testing.T) {
	r := Rect{Shape: Shape{Name: "사각형", X: 0, Y: 0}, Width: 4, Height: 5}

	wantArea := 20.0
	if got := r.Area(); got != wantArea {
		t.Errorf("Area() = %f, want %f", got, wantArea)
	}

	// 승격된 필드와 메서드도 확인
	if r.Name != "사각형" {
		t.Errorf("Name = %q, want %q", r.Name, "사각형")
	}
	if r.Info() != "Shape[사각형] at (0,0)" {
		t.Errorf("Info() = %q, want %q", r.Info(), "Shape[사각형] at (0,0)")
	}
}