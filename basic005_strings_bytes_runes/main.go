package main

import (
	"fmt"
	"unicode/utf8"
)

// printHex는 문자열의 각 바이트를 16진수로 출력합니다.
// 한글 '한글'이 6바이트인 이유를 시각적으로 확인하기 위함입니다.
func printHex(s string) {
	fmt.Printf("%q -> ", s)
	for i := 0; i < len(s); i++ {
		fmt.Printf("%02x ", s[i])
	}
	fmt.Println()
}

// unsafeByteAccess는 초보자가 흔히 하는 실수입니다.
// 문자열의 byte 인덱스를 그대로 사용해 '글'을 자르려 합니다.
// '한'의 UTF-8 바이트열은 3바이트이므로, 인덱스 2는 '한'의 중간에 해당합니다.
// 이 코드는 실행은 되지만, 결과물인 s는 유효한 UTF-8 문자열이 아닙니다.
func unsafeByteAccess() string {
	s := "한글"
	// '글'의 시작은 인덱스 3입니다. 하지만 2를 사용하면 '한'의 3번째 바이트와
	// '글'의 첫 번째 바이트가 합쳐져 깨집니다.
	return s[2:]
}

// safeByteAccess는 올바른 방법으로 문자열을 자릅니다.
// utf8.DecodeRuneInString을 사용해 문자 하나( rune )의 바이트 길이를 계산한 후
// 그 지점부터 슬라이싱해야 합니다.
func safeByteAccess() string {
	s := "한글"
	// r, size := utf8.DecodeRuneInString(s)
	// 실제로는 첫 번째 문자 '한'을 건너뛰고 싶으므로 두 번째부터 시작합니다.
	// 또는 단순히 utf8.RuneCountInString을 이용해 위치를 찾을 수도 있습니다.
	_, i := utf8.DecodeRuneInString(s) // 첫 번째 문자 '한'의 길이 계산
	return s[i:]
}

// byteSliceToRuneSlice는 []byte를 []rune으로 변환합니다.
// UTF-8 바이트열을 코드로포인트(rune) 목록으로 바꿉니다.
func byteSliceToRuneSlice(b []byte) []rune {
	return []rune(string(b))
}

// runeSliceToByteSlice는 []rune을 []byte로 변환합니다.
// 코드로포인트 목록을 UTF-8 바이트열로 인코딩합니다.
func runeSliceToByteSlice(r []rune) []byte {
	return []byte(string(r))
}

// iterateBytes는 문자열을 byte 단위로 반복합니다.
// 한글 1글자가 3개의 byte로 쪼개지는 것을 확인합니다.
func iterateBytes() {
	s := "안"
	fmt.Println("byte 단위 반복:")
	for i := 0; i < len(s); i++ {
		fmt.Printf("  index %d: byte %d (%02x)\n", i, s[i], s[i])
	}
}

// iterateRunes는 문자열을 rune 단위로 반복합니다.
// 'for range'를 사용하면 자동으로 UTF-8을 해석해 한글 1글자를 떼어줍니다.
func iterateRunes() {
	s := "안"
	fmt.Println("rune 단위 반복:")
	for i, r := range s {
		// i는 rune이 시작된 위치의 byte 인덱스입니다.
		fmt.Printf("  byte index %d: rune '%c' (U+%04X)\n", i, r, r)
	}
}

func main() {
	fmt.Println("=== UTF-8과 한글: 문자열의 실제 모습 ===")
	
	// 1. 문자열은 그냥 바이트열이다
	printHex("한글")
	fmt.Println("  -> '한글'은 2글자이지만, len()은 6을 반환합니다. 각 한자 글자가 UTF-8로 3바이트이기 때문입니다.")
	fmt.Println()

	// 2. 인덱스로 접근할 때의 함정
	fmt.Println("=== 인덱싱 함정 ===")
	fmt.Println("[실수 버전] 인덱스 2부터 잘라내기:")
	fmt.Printf("  결과: %q (이건 '글'이 아닙니다! 깨진 문자열입니다)\n", unsafeByteAccess())

	fmt.Println("[고친 버전] 올바른 UTF-8 경계에서 자르기:")
	fmt.Printf("  결과: %q (올바르게 '글'만 잘렸습니다)\n", safeByteAccess())
	fmt.Println()

	// 3. byte vs rune 반복
	fmt.Println("=== 반복(iteration) ===")
	iterateBytes()
	fmt.Println()
	iterateRunes()
	fmt.Println("  -> for range는 자동으로 UTF-8 디코딩을 해줍니다. byte 인덱스와 rune 값을 함께 얻을 수 있습니다.")
	fmt.Println()

	// 4. 타입 변환 비용
	fmt.Println("=== 타입 변환 비용 ===")
	fmt.Println("[]byte <-> []rune 변환은 매번 문자열을 새로 만들어내므로 비용이 발생합니다.")
	fmt.Println("  -> 자주 변환해야 하는 경우, string 변환 없이 []rune으로 미리 바꾸거나,")
	fmt.Println("     utf8.RuneCountInString 등을 사용해 바이트 단위로 처리하는 것이 효율적일 수 있습니다.")
}