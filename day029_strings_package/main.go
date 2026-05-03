package main

import (
	"fmt"
	"strconv"
	"strings"
)

func main() {
	// 1. strings 패키지 기본 조작 예제
	fmt.Println("=== strings 패키지 예제 ===")
	original := "  Go Programming Language  "
	fmt.Println("원본 문자열:", original)

	// TrimSpace: 시작과 끝의 공백 및 개행 문자 제거
	cleaned := strings.TrimSpace(original)
	fmt.Println("TrimSpace 적용:", cleaned)

	// Split: 문자열을 구분자 기준으로 분할하여 []string 생성
	parts := strings.Split(cleaned, " ")
	fmt.Println("Split 결과:", parts)

	// Join: []string 요소를 구분자로 연결하여 문자열로 합침
	combined := strings.Join(parts, "-")
	fmt.Println("Join 결과:", combined)

	// ReplaceAll: 모든 일치하는 부분 문자열을 다른 문자열로 교체
	replaced := strings.ReplaceAll(combined, "Go", "Golang")
	fmt.Println("ReplaceAll 결과:", replaced)

	// Contains: 특정 부분 문자열이 포함되어 있는지 확인 (bool 반환)
	hasGolang := strings.Contains(replaced, "Golang")
	fmt.Println("Contains 결과 (Golang 포함?):", hasGolang)

	// 2. strings.Builder: 효율적인 문자열 누적
	fmt.Println("\n=== strings.Builder 예제 ===")
	var builder strings.Builder
	builder.WriteString("안녕하세요.")
	builder.WriteString(" Go ")
	builder.WriteString("언어는")
	builder.WriteString(" 컴파일 속도가 빠릅니다!")
	fmt.Println("Builder 최종 결과:", builder.String())

	// 3. strconv 패키지: 문자열과 기본 타입 상호 변환
	fmt.Println("\n=== strconv 패키지 예제 ===")

	// Atoi: 문자열을 정수(int)로 변환
	strNum := "1024"
	num, err := strconv.Atoi(strNum)
	if err != nil {
		fmt.Println("Atoi 변환 실패:", err)
	} else {
		fmt.Println("Atoi 결과:", num, "(타입:", fmt.Sprintf("%T", num), ")")
	}

	// Itoa: 정수를 문자열로 변환
	strResult := strconv.Itoa(2048)
	fmt.Println("Itoa 결과:", strResult, "(타입:", fmt.Sprintf("%T", strResult), ")")

	// ParseFloat: 문자열을 float64로 변환 (소수점 자릿수 지정)
	strFloat := "3.14159265"
	f, err := strconv.ParseFloat(strFloat, 64)
	if err != nil {
		fmt.Println("ParseFloat 변환 실패:", err)
	} else {
		fmt.Println("ParseFloat 결과:", f)
	}

	// FormatBool: bool 값을 문자열("true" 또는 "false")로 변환
	isValid := true
	boolStr := strconv.FormatBool(isValid)
	fmt.Println("FormatBool 결과:", boolStr)
}