package main

import (
	"errors"
	"os"
	"testing"
)

// TestSentinelErrorComparison는 errors.Is가 센티널 에러를 올바르게 찾는지 검증합니다.
// == 비교는 래핑된 에러에서 실패하지만, errors.Is는 성공해야 합니다.
func TestSentinelErrorComparison(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		wantIsNotFound bool
		wantIsPerm     bool
	}{
		{"정상 사용자", 10, false, false},
		{"존재하지 않음", 999, true, false},
		{"권한 없음", 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := getUser(tt.id)
			if tt.wantIsNotFound && !errors.Is(err, ErrNotFound) {
				t.Errorf("errors.Is(err, ErrNotFound) = false, want true")
			}
			if tt.wantIsPerm && !errors.Is(err, ErrPermissionDenied) {
				t.Errorf("errors.Is(err, ErrPermissionDenied) = false, want true")
			}
			// == 비교는 래핑된 에러에서 항상 실패한다 (이것이 버그의 원인)
			if err != nil && err == ErrNotFound {
				t.Errorf("err == ErrNotFound should be false for wrapped error")
			}
		})
	}
}

// TestWrappedErrorIs는 %w로 래핑된 에러가 errors.Is로 찾아지는지 검증합니다.
// os.ErrNotExist는 io/fs package의 센티널 에러입니다.
func TestWrappedErrorIs(t *testing.T) {
	err := openFileFixed("nonexistent.txt")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("errors.Is(err, os.ErrNotExist) = false, want true")
	}
	// %v 래핑은 찾아지지 않아야 한다 (버그 시나리오)
	errBuggy := openFileBuggy("nonexistent.txt")
	if errors.Is(errBuggy, os.ErrNotExist) {
		t.Errorf("errors.Is on %v-wrapped error should be false")
	}
}

// TestCustomErrorAs는 errors.As가 구체 에러 타입을 찾고 값을 추출하는지 검증합니다.
func TestCustomErrorAs(t *testing.T) {
	err := lookupUser(-5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var notFound *CustomNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatal("errors.As should find *CustomNotFoundError")
	}
	if notFound.ID != -5 {
		t.Errorf("ID = %d, want -5", notFound.ID)
	}
	if notFound.Msg == "" {
		t.Error("Msg should not be empty")
	}
}

// TestNilSafeAs는 errors.As가 nil 에러에서도 panic하지 않고 false를 반환하는지 검증합니다.
func TestNilSafeAs(t *testing.T) {
	err := lookupUser(10) // nil 반환
	var notFound *CustomNotFoundError
	if errors.As(err, &notFound) {
		t.Error("errors.As on nil error should return false")
	}
	// notFound는 nil이어야 한다
	if notFound != nil {
		t.Error("notFound should be nil when error is nil")
	}
}