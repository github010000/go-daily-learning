package main

import (
	"testing"
)

// TestNilErrorTrap은 nil 포인터를 담은 인터페이스가 nil이 아닌지를 검증합니다.
// 초심자가 자주 겪는 실수를 테스트로 확인합니다.
func TestNilErrorTrap(t *testing.T) {
	// 테이블 주도 테스트: 각 함수의 이름과 예상 결과
	tests := []struct {
		name    string
		fn      func() error
		wantNil bool
	}{
		{
			name:    "okFunc는 정상적으로 nil을 반환한다",
			fn:      okFunc,
			wantNil: true,
		},
		{
			name:    "badFunc는 nil 포인터를 반환하므로 nil이 아니다",
			fn:      badFunc,
			wantNil: false,
		},
		{
			name:    "fixedFunc는 nil 인터페이스를 반환한다",
			fn:      fixedFunc,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			isNil := err == nil
			if isNil != tt.wantNil {
				t.Errorf("nil 여부 불일치: got %v, want %v", isNil, tt.wantNil)
			}
		})
	}
}

// TestErrorHandling은 에러 처리 로직에서 nil 체크가 올바르게 작동하는지를 검증합니다.
func TestErrorHandling(t *testing.T) {
	// okFunc와 fixedFunc는 nil을 반환하므로 에러 처리가 없어야 함
	if err := okFunc(); err != nil {
		t.Errorf("okFunc: 예상치 못한 에러: %v", err)
	}
	if err := fixedFunc(); err != nil {
		t.Errorf("fixedFunc: 예상치 못한 에러: %v", err)
	}

	// badFunc는 nil 포인터를 반환하므로 에러 처리가 발생해야 함
	if err := badFunc(); err == nil {
		t.Error("badFunc: 에러가 발생해야 하지만 nil을 반환했다")
	}
}