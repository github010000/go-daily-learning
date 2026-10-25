package main

import "fmt"

// ─────────────────────────────────────────────────────────────────────────────
// 상수 · iota · 타입 없는 상수 데모
// ─────────────────────────────────────────────────────────────────────────────

// DayOfWeek 는 요일을 나타내는 열거형 타입입니다.
// iota 를 사용해 0=Sunday, 1=Monday, ..., 6=Saturday 로 정의합니다.
type DayOfWeek int

const (
	Sunday    DayOfWeek = iota // 0
	Monday                     // 1
	Tuesday                    // 2
	Wednesday                  // 3
	Thursday                   // 4
	Friday                     // 5
	Saturday                   // 6
)

// String 은 요일을 한국어 이름으로 반환합니다.
func (d DayOfWeek) String() string {
	switch d {
	case 0:
		return "일"
	case 1:
		return "월"
	case 2:
		return "화"
	case 3:
		return "수"
	case 4:
		return "목"
	case 5:
		return "금"
	case 6:
		return "토"
	default:
		return "알 수 없음"
	}
}

// UntypedConstFlexibility 는 타입 없는 상수가 변수에 할당되는 순간
// 타입이 결정되는 것을 보여줍니다.
func UntypedConstFlexibility() {
	const limit = 100
	var b byte = byte(limit)
	var i int = limit
	var u uint = uint(limit)

	fmt.Println("=== 타입 없는 상수 유연성 ===")
	fmt.Printf("limit=100 -> byte: %d, int: %d, uint: %d\n", b, i, u)

	// 타입 없는 상수는 타입에 상관없이 쓸 수 있습니다.
	sum := limit + 50 // 타입 없는 상수 + 정수 리터럴 -> 타입 없는 상수
	fmt.Printf("limit + 50 = %d (타입 없는 상수)\n", sum)
	fmt.Println()
}

// IotaResetsPerBlock 은 iota 가 const 블록이 열릴 때마다 0 으로 리셋되는 것을 보여줍니다.
func IotaResetsPerBlock() {
	const (
		a = iota // 0
		b        // 1
	)

	const (
		c = iota // 0 — 리셋!
		d        // 1
	)

	fmt.Println("=== iota: 블록당 리셋 ===")
	fmt.Printf("a=%d, b=%d, c=%d, d=%d\n", a, b, c, d)
	fmt.Println("iota 는 const 블록 안에서만 0, 1, 2... 로 올라갑니다.")
	fmt.Println()
}

// BitFlagPattern 은 1 << iota 패턴으로 비트 플래그를 만드는 예를 보여줍니다.
func BitFlagPattern() {
	type Color int
	const (
		Red   Color = 1 << iota // 1
		Green                   // 2
		Blue                    // 4
	)

	fmt.Println("=== 1 << iota 로 비트 플래그 만들기 ===")
	fmt.Printf("Red   = %b (%d)\n", Red, Red)
	fmt.Printf("Green = %b (%d)\n", Green, Green)
	fmt.Printf("Blue  = %b (%d)\n", Blue, Blue)

	combined := Red | Green
	fmt.Printf("Red|Green = %b (%d)\n", combined, combined)
	fmt.Printf("Red|Green has Red? %v\n", combined&Red != 0)
	fmt.Printf("Red|Green has Green? %v\n", combined&Green != 0)
	fmt.Printf("Red|Green has Blue? %v\n", combined&Blue != 0)
	fmt.Println()
}

// GoodDayOfWeek 는 0 을 '알 수 없음'으로 비워두는 올바른 예시입니다.
// var DayOfWeek{} 를 써도 0(알 수 없음)이 됩니다.
func GoodDayOfWeek() {
	var today DayOfWeek
	fmt.Println("=== 0 을 '알 수 없음'으로 비워둔 예 ===")
	fmt.Printf("var DayOfWeek{} = %d (Sunday=알 수 없음)\n", today)
	fmt.Println("0 은 '미정의 상태'. new() 로 만들어도 안전한 시작점입니다.")
	fmt.Println()

	// 실제로 쓰는 요일 값들
	fmt.Printf("Monday=%d, Wednesday=%d, Saturday=%d\n", Monday, Wednesday, Saturday)
	fmt.Printf("Monday.String()=%s, Saturday.String()=%s\n", Monday.String(), Saturday.String())
}

// BadDayOfWeek 는 0 을 '월요일'로 할당해 잘못된 예시입니다.
// 0 을 '미정의'가 아닌 '월요일'로 쓰면, var BadDayOfWeek{} 로 선언해도 월요일이 됩니다.
// Go 의 영값(zero value)이 0 인 점을 무시한 실수입니다.
func BadDayOfWeek() {
	fmt.Println("=== 0 을 '월요일'로 잘못 쓴 예 ===")

	// BadDayOfWeek 는 BadMonday=0, ..., BadSunday=6 으로 시작합니다.
	type BadDayOfWeek int
	const (
		BadMonday    BadDayOfWeek = iota // 0
		BadTuesday                         // 1
		BadWednesday                       // 2
		BadThursday                        // 3
		BadFriday                          // 4
		BadSaturday                        // 5
		BadSunday                          // 6
	)

	// var 로 선언하면 0(영값)이 됩니다.
	var today BadDayOfWeek
	fmt.Printf("var BadDayOfWeek{} = %d (BadMonday)\n", today)
	fmt.Println("0 = BadMonday 인데, new() 로 만들면 월요일이 됩니다.")
	fmt.Println("의도하지 않은 초기값이 생깁니다.")
	fmt.Println()

	_ = BadMonday
	_ = BadTuesday
	_ = BadWednesday
	_ = BadThursday
	_ = BadFriday
	_ = BadSaturday
	_ = BadSunday
}

// UntypedStringConst 는 타입 없는 문자열 상수도 유연하게 쓸 수 있는 것을 보여줍니다.
func UntypedStringConst() {
	const greeting = "안녕하세요"
	var s string = greeting
	var b []byte = []byte(greeting)

	fmt.Println("=== 타입 없는 문자열 상수 ===")
	fmt.Printf("string: %s, bytes: %v\n", s, b)
	fmt.Println()
}

func main() {
	UntypedConstFlexibility()
	IotaResetsPerBlock()
	BadDayOfWeek()
	GoodDayOfWeek()
	BitFlagPattern()
	UntypedStringConst()
}