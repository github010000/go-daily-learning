package main

import (
	"testing"
)

// TestSplitName은 다중 반환값 (값, error) 패턴을 검증합니다.
// 성공 케이스와 실패 케이스 모두를 테이블 주도 방식으로 테스트합니다.
func TestSplitName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantF    string
		wantL    string
		wantErr  bool
	}{
		{"정상 케이스", "Kim Min-su", "Kim", "Min-su", false},
		{"정상 케이스 2", "Lee Soo-jin", "Lee", "Soo-jin", false},
		{"이름 부족", "Min-su", "", "", true},
		{"빈 문자열", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last, err := splitName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitName(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
				return
			}
			if first != tt.wantF || last != tt.wantL {
				t.Errorf("splitName(%q) = (%q, %q), want (%q, %q)", tt.input, first, last, tt.wantF, tt.wantL)
			}
		})
	}
}

// TestCountWords는 이름 붙은 반환값을 사용하는 함수의 동작을 검증합니다.
// 반환값의 정확성과 함께, 빈 문자열 처리도 확인합니다.
func TestCountWords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCount int
		wantMax   int
	}{
		{"일반 문장", "Go is fast", 3, 4},
		{"빈 문자열", "", 0, 0},
		{"단어 하나", "Hello", 1, 5},
		{"여러 단어", "a bb ccc", 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, maxLen := countWords(tt.input)
			if count != tt.wantCount {
				t.Errorf("countWords(%q) count=%d, want %d", tt.input, count, tt.wantCount)
			}
			if maxLen != tt.wantMax {
				t.Errorf("countWords(%q) maxLen=%d, want %d", tt.input, maxLen, tt.wantMax)
			}
		})
	}
}

// TestDeferInFuncFixed는 defer를 사용할 때 올바른 패턴이 어떻게 동작하는지 검증합니다.
// 이름 붙은 반환값의 함정에 빠지지 않고, 명확한 반환값을 반환하는지 확인합니다.
func TestDeferInFuncFixed(t *testing.T) {
	tests := []struct {
		name    string
		success bool
		wantRes int
		wantErr bool
	}{
		{"성공 케이스", true, 42, false},
		{"실패 케이스", false, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := deferInFuncFixed(tt.success)
			if res != tt.wantRes {
				t.Errorf("deferInFuncFixed(success=%v) result=%d, want %d", tt.success, res, tt.wantRes)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("deferInFuncFixed(success=%v) err=%v, wantErr=%v", tt.success, err, tt.wantErr)
			}
		})
	}
}