package main

import "testing"

// TestStructLiteralsAndInitialization: 구조체 리터럴 초기화 검증.
// 필드명 생략, 부분 초기화, 익명 구조체 리터럴이 올바르게 생성되는지 확인합니다.
func TestStructLiteralsAndInitialization(t *testing.T) {
	tests := []struct {
		name     string
		point    Point
		expected Point
	}{
		{
			name:     "필드 순서만 맞춘 리터럴",
			point:    Point{3, 4},
			expected: Point{X: 3, Y: 4},
		},
		{
			name:     "부분 초기화 (나머지 zero value)",
			point:    Point{X: 5},
			expected: Point{X: 5, Y: 0},
		},
		{
			name:     "Color 리터럴",
			point:    Point{10, 20},
			expected: Point{X: 10, Y: 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.point != tt.expected {
				t.Errorf("구조체 불일치: got %+v, want %+v", tt.point, tt.expected)
			}
		})
	}

	// 익명 구조체 리터럴 생성 확인
	config := struct {
		Debug   bool
		Version string
	}{
		Debug:   true,
		Version: "1.0",
	}
	if config.Version != "1.0" {
		t.Errorf("익명 구조체 Version 필드 오류: got %s", config.Version)
	}
	if !config.Debug {
		t.Error("익명 구조체 Debug 필드 오류: true여야 함")
	}
}

// TestStructComparison: comparable struct 비교 동작 검증.
// slices/maps가 없는 struct만 == 비교가 가능해야 합니다.
func TestStructComparison(t *testing.T) {
	p1 := Point{X: 1, Y: 2}
	p2 := Point{X: 1, Y: 2}
	p3 := Point{X: 3, Y: 4}

	if p1 != p2 {
		t.Error("같은 필드값을 가진 struct는 == 비교에서 true여야 함")
	}
	if p1 == p3 {
		t.Error("다른 필드값을 가진 struct는 == 비교에서 false여야 함")
	}

	per1 := Person{Name: "Alice", Age: 30}
	per2 := Person{Name: "Alice", Age: 30}
	per3 := Person{Name: "Bob", Age: 25}

	if per1 != per2 {
		t.Error("같은 값의 Person struct는 == 비교에서 true여야 함")
	}
	if per1 == per3 {
		t.Error("다른 값의 Person struct는 == 비교에서 false여야 함")
	}
}

// TestStructMutationFix: 구조체 수정 시 반환값 사용 검증.
// struct는 값 타입이므로, 반환값을 사용하지 않으면 변경 사항이 반영되지 않아야 합니다.
func TestStructMutationFix(t *testing.T) {
	// Point 수정 테스트
	p := Point{X: 1, Y: 2}
	fixedP := FixPoint(p)

	if fixedP.X != 100 || fixedP.Y != 200 {
		t.Errorf("FixPoint 반환값 오류: got %+v", fixedP)
	}
	if p.X != 1 || p.Y != 2 {
		t.Errorf("원본 Point는 변하지 않아야 함: got %+v", p)
	}

	// Person 이름 수정 테스트
	per := Person{Name: "Dave", Age: 25}
	updatedPer := FixPersonName(per, "David")

	if updatedPer.Name != "David" {
		t.Errorf("FixPersonName 반환값 오류: got %+v", updatedPer)
	}
	if per.Name != "Dave" {
		t.Errorf("원본 Person은 변하지 않아야 함: got %+v", per)
	}
}

// TestSliceDeepCopy: 슬라이스 포함 구조체의 깊은 복사 검증.
// 얕은 복사와 달리, 깊은 복사는 원본 슬라이스를 보존해야 합니다.
func TestSliceDeepCopy(t *testing.T) {
	ps := Person{
		Name: "Original",
		Tags: []string{"a", "b"},
	}

	// 원본 Tags 확인
	originalTags := make([]string, len(ps.Tags))
	copy(originalTags, ps.Tags)

	// FixSliceInStruct 호출
	fixedPs := FixSliceInStruct(ps)

	// 변경된 Tags 확인
	if fixedPs.Tags[0] != "mutated" {
		t.Errorf("변경된 Tags[0] 오류: got %s, want mutated", fixedPs.Tags[0])
	}

	// 원본 Tags가 변하지 않았는지 확인
	if ps.Tags[0] != originalTags[0] {
		t.Errorf("원본 Tags[0]이 변했습니다: got %s, want %s", ps.Tags[0], originalTags[0])
	}
}

// TestPublicFields: 공개 필드 접근 검증.
// 대문자로 시작하는 필드만 접근 가능해야 합니다.
func TestPublicFields(t *testing.T) {
	alice := Person{Name: "Alice", Age: 25}

	if alice.Name != "Alice" {
		t.Errorf("Name 필드 접근 오류: got %s", alice.Name)
	}
	if alice.Age != 25 {
		t.Errorf("Age 필드 접근 오류: got %d", alice.Age)
	}
}