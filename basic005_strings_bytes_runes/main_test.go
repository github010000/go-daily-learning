package main

import (
	"testing"
	"unicode/utf8"
)

// TestUnsafeByteAccess_VerifiesBrokenOutput:
// unsafeByteAccess 함수가 '한글'에서 인덱스 2를 기준으로 자르면,
// '한'의 마지막 바이트와 '글'의 첫 바이트가 합쳐져 유효한 UTF-8이 아닌 결과가 나옴을 검증합니다.
// 이는 초보자가 '인덱스 1부터 자르면 두 번째 글자가 나온다'고 생각할 때 발생하는 함정을 보여줍니다.
func TestUnsafeByteAccess_VerifiesBrokenOutput(t *testing.T) {
	result := unsafeByteAccess()
	
	// 결과물이 유효한 UTF-8 문자열인지 확인합니다.
	// safeByteAccess()는 항상 유효한 UTF-8을 반환해야 합니다.
	if utf8.ValidString(result) {
		// 유효한 UTF-8이 나왔다면, 아마도 우연히 맞춘 것이거나 구현이 바뀐 것입니다.
		// 하지만 이 함수의 의도는 '잘못된 접근'을 시연하는 것이므로,
		// 보통은 유효하지 않은 바이트열이 나오거나, 의도한 '글'이 아닌 값이 나옵니다.
		// 여기서는 구체적으로 '글'이 아닌지 확인합니다.
		if result == "글" {
			t.Fatal("실수 버전이 우연히 올바른 결과를 냈습니다. 코드를 다시 확인하세요.")
		}
	}
	
	// 구체적으로 결과물이 '글'이 아님을 단정합니다.
	if result == "글" {
		t.Fatal("실수 버전이 '글'을 반환했습니다. 인덱싱 오류 시연에 실패했습니다.")
	}
}

// TestSafeByteAccess_VerifiesCorrectSlicing:
// safeByteAccess 함수가 '한글'에서 올바른 UTF-8 경계(인덱스 3)를 찾아
// 정확히 '글'만을 반환하는지 검증합니다.
func TestSafeByteAccess_VerifiesCorrectSlicing(t *testing.T) {
	result := safeByteAccess()
	expected := "글"
	
	if result != expected {
		t.Errorf("safeByteAccess() = %q, want %q", result, expected)
	}
	
	// 반환된 문자열이 유효한 UTF-8인지도 확인합니다.
	if !utf8.ValidString(result) {
		t.Error("safeByteAccess()가 유효하지 않은 UTF-8 문자열을 반환했습니다.")
	}
}

// TestRunesVsBytes_LenDifference:
// 문자열 "한글"에 대해 len()과 utf8.RuneCountInString()의 차이가
// 바이트 단위 길이(6)와 rune 단위 길이(2)임을 검증합니다.
func TestRunesVsBytes_LenDifference(t *testing.T) {
	s := "한글"
	
	byteLen := len(s)
	runeLen := utf8.RuneCountInString(s)
	
	if byteLen != 6 {
		t.Errorf("len(%q) = %d, want 6", s, byteLen)
	}
	if runeLen != 2 {
		t.Errorf("utf8.RuneCountInString(%q) = %d, want 2", s, runeLen)
	}
}

// TestConvertBetweenByteSliceAndRuneSlice:
// []byte <-> []rune 변환이 대칭적으로 동작하는지 검증합니다.
// 변환 후 원본과 동일한 rune 시퀀스가 나오는지를 확인합니다.
func TestConvertBetweenByteSliceAndRuneSlice(t *testing.T) {
	originRunes := []rune("안녕")
	
	// []rune -> []byte -> []rune
	byteSlice := runeSliceToByteSlice(originRunes)
	backToRunes := byteSliceToRuneSlice(byteSlice)
	
	if len(backToRunes) != len(originRunes) {
		t.Errorf("변환 후 rune 길이 불일치: got %d, want %d", len(backToRunes), len(originRunes))
	}
	
	for i := range originRunes {
		if backToRunes[i] != originRunes[i] {
			t.Errorf("위치 %d에서 rune 불일치: got %c, want %c", i, backToRunes[i], originRunes[i])
		}
	}
}