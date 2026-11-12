package main

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

// sink* 변수는 컴파일러에게 "이 값은 실제로 쓰인다"고 알려 주는 용도다.
// 이게 없으면 escape analysis 가 변환 결과를 스택에 가둬 버리거나
// 아예 변환 자체를 지워 버려서, 우리가 무엇을 재고 있는지 알 수 없게 된다.
var (
	sinkStr  string
	sinkByte []byte
	sinkInt  int
)

// payload 를 상수 리터럴 대신 패키지 변수로 둔 이유:
// 상수 문자열에 대한 []byte("...") 변환은 컴파일러가 정적 바이트 배열로
// 접어 버릴 수 있어서, 할당 측정이 0 으로 나오고 "복사가 없다"고 잘못 읽기 쉽다.
var payload = "hello"

// byteView 는 b 의 바이트를 복사 없이 string 으로 본다.
//
// 계약: 돌려준 문자열이 살아 있는 동안 b 의 내용은 절대 바뀌면 안 된다.
// Go 의 string 은 불변이라는 전제 위에 map 해싱, 부분 문자열 공유, 상수 풀이
// 얹혀 있으므로, 이 계약을 깨면 "분명히 넣었는데 찾을 수 없는 키"가 만들어진다.
// 그래서 이 함수는 복사 비용을 없애는 대신 안전망을 전부 프로그래머에게 넘긴다.
func byteView(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// ---------------------------------------------------------------------------
// 측정 도구: 모두 "힙 할당 횟수"를 센다. 시간이 아니라 개수라서 부하에 흔들리지 않는다.
// ---------------------------------------------------------------------------

// measureEscapingString 은 변환 결과가 함수 밖으로 나가는 경우를 잰다.
// 결과를 전역에 대입하므로 escape analysis 가 반드시 힙 할당을 강요한다.
func measureEscapingString(b []byte, runs int) float64 {
	return testing.AllocsPerRun(runs, func() { sinkStr = string(b) })
}

// measureCompareString 은 결과가 비교에만 쓰이는 경우를 잰다.
// 비교가 끝나면 문자열이 필요 없으므로 컴파일러가 임시 문자열로 처리할 수 있다.
func measureCompareString(b []byte, runs int) float64 {
	return testing.AllocsPerRun(runs, func() {
		if string(b) == "hello" {
			sinkInt++
		}
	})
}

// measureMapLookup 은 결과가 map 조회 키로만 쓰이는 경우를 잰다.
// map 은 조회 중에만 키를 읽고 저장하지 않으므로 임시 문자열로 충분하다.
func measureMapLookup(b []byte, m map[string]int, runs int) float64 {
	return testing.AllocsPerRun(runs, func() { sinkInt += m[string(b)] })
}

// measureSliceEscapingBytes 는 []byte(s) 의 결과가 밖으로 나가는 경우를 잰다.
func measureSliceEscapingBytes(s string, runs int) float64 {
	return testing.AllocsPerRun(runs, func() { sinkByte = []byte(s) })
}

// measureSliceLocal 은 []byte(s) 의 결과가 함수 안에 머무는 경우를 잰다.
// bytes.IndexByte 로 슬라이스를 실제로 읽게 만들어, 컴파일러가 변환을
// 통째로 지워 버리지 못하게 한다.
func measureSliceLocal(s string, runs int) float64 {
	return testing.AllocsPerRun(runs, func() {
		b := []byte(s)
		sinkInt += bytes.IndexByte(b, 'z')
	})
}

// ---------------------------------------------------------------------------
// 시연
// ---------------------------------------------------------------------------

// demoCopySemantics 는 변환이 "그 시점의 스냅숏"임을 보여 준다.
func demoCopySemantics() {
	s := payload
	b := []byte(s)
	b[0] = 'H'
	fmt.Printf("[1] string → []byte : 원본 %q, 사본을 고친 뒤 %q\n", s, string(b))

	raw := []byte(payload)
	snap := string(raw)
	raw[0] = 'W'
	fmt.Printf("[1] []byte → string : 스냅숏 %q, 이후 원본 %q\n", snap, string(raw))
}

// demoMapKeySnapshot 은 map 에 키를 넣는 식으로 변환하면 반드시 복사된다는 것을 보여 준다.
// 조회와 달리 삽입은 키를 맵 안에 보관해야 하므로 임시 문자열을 쓸 수 없다.
func demoMapKeySnapshot() {
	m := map[string]int{}
	key := []byte("alpha")
	m[string(key)] = 1
	key[0] = 'o'
	fmt.Printf("[2] map[\"alpha\"]=%d, map[string(key)] (key=%q)=%d\n",
		m["alpha"], string(key), m[string(key)])
}

// demoAllocElision 은 escape 여부와 문맥에 따라 할당이 사라지는 것을 수치로 보여 준다.
func demoAllocElision() {
	b := []byte("hello")
	m := map[string]int{"hello": 1}
	fmt.Printf("[3] string(b) 결과가 escape       allocs/op = %.1f\n", measureEscapingString(b, 200))
	fmt.Printf("[3] string(b) 비교에만 사용        allocs/op = %.1f\n", measureCompareString(b, 200))
	fmt.Printf("[3] map[string(b)] 조회            allocs/op = %.1f\n", measureMapLookup(b, m, 200))
	fmt.Printf("[3] []byte(s) 결과가 escape       allocs/op = %.1f\n", measureSliceEscapingBytes(payload, 200))
}

// demoStackBufferCliff 는 스택 임시 버퍼의 크기 경계를 드러낸다.
// 런타임의 tmpBuf 는 32바이트라서, 32 이하는 할당이 없고 33부터는 힙으로 간다.
func demoStackBufferCliff() {
	fmt.Println("[4] 비탈출 []byte(s) 의 할당 — 경계 관찰")
	for _, n := range []int{1, 8, 32, 33, 100} {
		s := strings.Repeat("a", n)
		fmt.Printf("[4]   len(s)=%3d → allocs/op = %.1f\n", n, measureSliceLocal(s, 200))
	}
}

// demoUnsafeString 은 unsafe.String 이 정말 같은 메모리를 보는지,
// 그리고 계약을 어겼을 때 무슨 일이 벌어지는지 보여 준다.
func demoUnsafeString() {
	b := []byte("alpha")
	s := byteView(b)
	same := unsafe.StringData(s) == unsafe.SliceData(b)
	fmt.Printf("[5] byteView(b)=%q, 같은 백킹 배열인가 = %v\n", s, same)

	m := map[string]int{s: 1}
	b[0] = 'o' // 계약 위반: 문자열이 살아 있는데 백킹 배열을 고쳤다
	_, found := m[s]
	fmt.Printf("[5] 백킹 배열 수정 후: s=%q, len(m)=%d, m[s] 발견=%v\n", s, len(m), found)
	fmt.Printf("[5]   → 해시는 삽입 당시의 \"alpha\" 기준, 내용은 지금 \"olpha\"\n")

	if got := byteView(nil); got != "" {
		fmt.Printf("[5] 예상 밖: nil 슬라이스 결과 %q\n", got)
	}
}

// demoSubstringAliasing 은 반대 방향 함정을 보여 준다.
// 부분 문자열은 헤더만 자르므로, 작은 조각이 거대한 배열을 붙잡아 둔다.
func demoSubstringAliasing() {
	big := strings.Repeat("x", 1<<16)
	small := big[1000:1010]
	diff := uintptr(unsafe.Pointer(unsafe.StringData(small))) -
		uintptr(unsafe.Pointer(unsafe.StringData(big)))
	fmt.Printf("[6] big.data=%p small.data=%p 차이=%d 바이트\n",
		unsafe.StringData(big), unsafe.StringData(small), diff)
	fmt.Printf("[6]   → small 은 10바이트지만 65536바이트 배열을 계속 살려 둔다\n")
}

// countingWriter 는 io.StringWriter 를 구현한 최소한의 writer 다.
// 이 인터페이스가 존재하는 이유가 바로 []byte 변환 회피다.
type countingWriter struct{ bytes int }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.bytes += len(p)
	return len(p), nil
}

func (w *countingWriter) WriteString(s string) (int, error) {
	w.bytes += len(s)
	return len(s), nil
}

// demoBuilderAndWriter 는 unsafe 없이 복사를 피하는 표준 라이브러리 경로를 보여 준다.
func demoBuilderAndWriter() {
	var sb strings.Builder
	sb.WriteString(payload)
	sb.WriteString(", world")
	builderAllocs := testing.AllocsPerRun(200, func() { sinkStr = sb.String() })

	w := &countingWriter{}
	writeAllocs := testing.AllocsPerRun(200, func() { _, _ = w.Write([]byte(payload)) })
	writeStringAllocs := testing.AllocsPerRun(200, func() { _, _ = io.WriteString(w, payload) })

	fmt.Printf("[7] strings.Builder.String()      allocs/op = %.1f\n", builderAllocs)
	fmt.Printf("[7] w.Write([]byte(s))            allocs/op = %.1f\n", writeAllocs)
	fmt.Printf("[7] io.WriteString(w, s)          allocs/op = %.1f\n", writeStringAllocs)
}

func main() {
	fmt.Printf("== Go %s ==\n", runtime.Version())
	demoCopySemantics()
	demoMapKeySnapshot()
	demoAllocElision()
	demoStackBufferCliff()
	demoUnsafeString()
	demoSubstringAliasing()
	demoBuilderAndWriter()
	fmt.Println("== 끝 ==")
}