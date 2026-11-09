package main

import (
	"testing"
)

// TestNilMapWithMake 은 make 로 초기화한 맵에 쓰기·읽기가 정상 동작함을 검증합니다.
// nil 맵에 쓰는 것은 panic 이므로 테스트로 직접 검증하지 않고, main.go 출력으로 확인합니다.
func TestNilMapWithMake(t *testing.T) {
	m := make(map[string]int)
	m["key"] = 42

	if got := m["key"]; got != 42 {
		t.Errorf("m['key'] = %d, want 42", got)
	}
}

// TestCommaOk 는 존재하는 키와 존재하지 않는 키를 구분하여 처리하는 로직을 검증합니다.
func TestCommaOk(t *testing.T) {
	scores := map[string]int{
		"홍길동": 85,
	}

	t.Run("존재하는 키", func(t *testing.T) {
		score, ok := scores["홍길동"]
		if !ok {
			t.Error("expected key to exist")
		}
		if score != 85 {
			t.Errorf("expected 85, got %d", score)
		}
	})

	t.Run("존재하지 않는 키", func(t *testing.T) {
		score, ok := scores["박영희"]
		if ok {
			t.Error("expected key to not exist")
		}
		if score != 0 {
			t.Errorf("expected default 0, got %d", score)
		}
	})
}

// TestMapValueModificationViaPointer 는 맵에 포인터를 저장하면 필드 직접 수정이 가능함을 검증합니다.
func TestMapValueModificationViaPointer(t *testing.T) {
	m := map[string]*struct {
		Name string
		Age  int
	}{
		"kim": {Name: "김철수", Age: 30},
	}

	m["kim"].Age = 31

	if m["kim"].Age != 31 {
		t.Errorf("expected Age 31, got %d", m["kim"].Age)
	}
	if m["kim"].Name != "김철수" {
		t.Errorf("expected Name '김철수', got %s", m["kim"].Name)
	}
}

// TestMapIterationRandomness 는 맵 순회가 무작위임을 통계적으로 검증합니다.
// 여러 번 순회하면서 각 키가 최소 한 번은 첫 번째로 등장하는지 확인합니다.
func TestMapIterationRandomness(t *testing.T) {
	m := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	firstKeys := make(map[string]int)
	for i := 0; i < 100; i++ {
		for k := range m {
			firstKeys[k]++
			break
		}
	}

	// 맵 순회가 무작위이므로, 세 키 모두 첫 번째로 등장해야 함
	if len(firstKeys) != 3 {
		t.Errorf("expected all 3 keys to appear first, got %d unique first keys: %v", len(firstKeys), firstKeys)
	}
}

// TestMapWithNilPointer 는 맵에 nil 포인터가 있을 때 nil 체크를 해야 함을 검증합니다.
func TestMapWithNilPointer(t *testing.T) {
	m := map[string]*struct {
		Name string
	}{
		"empty": nil,
	}

	t.Run("nil 체크 없이 접근 시 panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic when accessing nil map value")
			}
		}()
		_ = m["empty"].Name // nil dereference panic
	})

	t.Run("nil 체크 후 접근", func(t *testing.T) {
		if v := m["empty"]; v != nil {
			t.Error("expected nil value")
		}
	})
}

// TestDeleteAndLen 은 delete 와 len 동작을 검증합니다.
func TestDeleteAndLen(t *testing.T) {
	m := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	// 초기 크기 검증
	if len(m) != 3 {
		t.Errorf("expected len 3, got %d", len(m))
	}

	// 존재하는 키 삭제
	delete(m, "b")
	if len(m) != 2 {
		t.Errorf("expected len 2 after delete, got %d", len(m))
	}
	if _, ok := m["b"]; ok {
		t.Error("expected 'b' to be deleted")
	}

	// 존재하지 않는 키 삭제 (panic 없음)
	delete(m, "z")
	if len(m) != 2 {
		t.Errorf("expected len still 2 after deleting non-existent key, got %d", len(m))
	}
}