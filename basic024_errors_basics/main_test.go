package main

import (
	"errors"
	"strings"
	"testing"
)

// TestGreetingBad은 greetingBad 함수가 잘못된 입력에 대해 올바른 에러를 반환하는지 검증합니다.
// 빈 이름에는 ErrNameEmpty를, 짧은 이름에는 ErrNameTooShort를 반환해야 합니다.
func TestGreetingBad(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errIs     error
		errContains string
	}{
		{
			name:    "정상적인 이름",
			input:   "고고",
			wantErr: false,
		},
		{
			name:    "빈 이름",
			input:   "",
			wantErr: true,
			errIs:   ErrNameEmpty,
		},
		{
			name:      "짧은 이름",
			input:     "김",
			wantErr:   true,
			errIs:     ErrNameTooShort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := greetingBad(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("에러가 발생해야 하지만 nil이 반환되었습니다. 입력: %q", tt.input)
				}
				// sentinel error인지 직접 비교
				if tt.errIs != nil && err != tt.errIs {
					t.Errorf("에러가 %v여야 하지만, %v 반환", tt.errIs, err)
				}
				// 에러 메시지에 특정 문자열이 포함되는지 확인
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("에러 메시지에 %q가 포함되어야 합니다. 실제: %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("예상치 못한 에러: %v", err)
				}
				if msg == "" {
					t.Errorf("메시지가 비어있어서는 안 됩니다. 입력: %q", tt.input)
				}
			}
		})
	}
}

// TestErrorWrapping은 wrapDBError가 에러를 올바르게 감싸고,
// errors.Is로 감싸진 에러 체인에서 원본 에러(ErrDBDown)를 찾을 수 있는지 검증합니다.
func TestErrorWrapping(t *testing.T) {
	// wrapDBError("user123")는 ErrDBDown을 %w로 감싸서 반환합니다.
	err := wrapDBError("user123")

	if err == nil {
		t.Fatal("에러가 발생해야 합니다")
	}

	// errors.Is는 감싸진 에러 체인에서 원본 에러를 찾습니다.
	if !errors.Is(err, ErrDBDown) {
		t.Errorf("에러가 ErrDBDown을 포함해야 하지만, %v", err)
	}

	// 에러 메시지에 컨텍스트 정보가 포함되어야 합니다.
	if !strings.Contains(err.Error(), "사용자 user123 처리 중 오류") {
		t.Errorf("에러 메시지에 컨텍스트가 포함되어야 합니다. 실제: %q", err.Error())
	}
}

// TestSentinelErrors는 sentinel error 값들이 올바르게 정의되었는지 검증합니다.
// nil이 아니고, 서로 다른 에러이며, errors.Is로 자신의 에러를 찾을 수 있어야 합니다.
func TestSentinelErrors(t *testing.T) {
	if ErrNameEmpty == nil {
		t.Error("ErrNameEmpty는 nil이면 안 됩니다")
	}
	if ErrNameTooShort == nil {
		t.Error("ErrNameTooShort는 nil이면 안 됩니다")
	}
	if ErrDBDown == nil {
		t.Error("ErrDBDown는 nil이면 안 됩니다")
	}

	// 서로 다른 에러인지 확인
	if ErrNameEmpty == ErrNameTooShort {
		t.Error("ErrNameEmpty와 ErrNameTooShort는 달라야 합니다")
	}
	if ErrNameEmpty == ErrDBDown {
		t.Error("ErrNameEmpty와 ErrDBDown는 달라야 합니다")
	}

	// errors.Is로 자신의 에러를 찾을 수 있어야 함 (기본 동작)
	if !errors.Is(ErrNameEmpty, ErrNameEmpty) {
		t.Error("에러는 자신의 에러와 errors.Is로 비교되어야 합니다")
	}
}