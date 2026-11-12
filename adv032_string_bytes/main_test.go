package main

import (
	"io"
	"strings"
	"testing"
	"unsafe"
)

// TestConversionCopies 는 string 과 []byte 사이의 변환이 "그 시점의 스냅숏"임을 검증한다.
// 이 불변식이 깨지면 map 키 해시, 문자열 상수, 부분 문자열 공유가 한꺼번에 무너지므로
// 언어 차원에서 반드시 지켜져야 하는 성질이다.
func TestConversionCopies(t *testing.T) {
	b := []byte("hello")
	s := string(b)
	b[0] = 'H'
	if s != "hello" {
		t.Fatalf("string(b) 가 백킹 배열을 공유했다: %q", s)
	}
	if string(b) != "Hello" {
		t.Fatalf("원본 슬라이스 내용이 예상과 다르다: %q", string(b))
	}

	// payload 는 패키지 변수라 정적 바이트로 접히지 않는다. 그래서 이 대입은 안전하다.
	c := []byte(payload)
	c[0] = 'W'
	if payload != "hello" {
		t.Fatalf("[]byte(s) 가 원본 문자열을 바꿨다: %q", payload)
	}
}

// TestByteViewAliases 는 unsafe.String 판이 정말 같은 메모리를 보는지, 그리고
// 그 대가로 무엇을 잃는지(불변성)를 확인한다. 복사가 사라지면 별칭이 생기고,
// 별칭은 map 키를 조용히 망가뜨릴 수 있다는 사실을 코드로 고정해 둔다.
func TestByteViewAliases(t *testing.T) {
	b := []byte("alpha")
	s := byteView(b)
	if s != "alpha" {
		t.Fatalf("내용이 다르다: %q", s)
	}
	if unsafe.StringData(s) != unsafe.SliceData(b) {
		t.Fatalf("복사가 일어났다: 문자열이 다른 배열을 가리킨다")
	}

	// 계약 위반을 일부러 재현한다. 문자열이 살아 있는데 백킹 배열을 고친다.
	b[0] = 'o'
	if s != "olpha" {
		t.Errorf("별칭 관계가 관찰되지 않았다: %q", s)
	}

	// 빈 입력 경계: nil 과 길이 0 은 모두 빈 문자열이어야 한다.
	if got := byteView(nil); got != "" {
		t.Errorf("nil 슬라이스 결과가 빈 문자열이 아니다: %q", got)
	}
	if got := byteView([]byte{}); got != "" {
		t.Errorf("빈 슬라이스 결과가 빈 문자열이 아니다: %q", got)
	}
}

// TestAllocElision 은 복사가 지워지는 조건을 검증한다. 시간이 아니라 할당 횟수를 재고,
// 절대값이 아니라 "escape 하는 변환보다 적다"는 상대 비교를 써서 부하에 흔들리지 않게 한다.
func TestAllocElision(t *testing.T) {
	b := []byte("hello")
	m := map[string]int{"hello": 1}

	escaping := measureEscapingString(b, 100)
	compareOnly := measureCompareString(b, 100)
	mapKey := measureMapLookup(b, m, 100)

	if escaping <= 0 {
		t.Fatalf("escape 하는 변환이 할당 0 으로 측정됐다. 측정 자체가 잘못됐다")
	}
	if compareOnly >= escaping {
		t.Errorf("비교용 변환이 복사를 피하지 못했다: compare=%v escaping=%v", compareOnly, escaping)
	}
	if mapKey >= escaping {
		t.Errorf("map 조회용 변환이 복사를 피하지 못했다: map=%v escaping=%v", mapKey, escaping)
	}
}

// TestStackBufferCliff 는 비탈출 []byte(s) 가 32바이트까지는 스택 버퍼를 쓰고
// 그보다 크면 힙으로 넘어간다는 경계를 검증한다. 이 경계를 모르면
// "왜 어떤 호출은 할당이 없고 어떤 호출은 있는지" 설명할 수 없다.
func TestStackBufferCliff(t *testing.T) {
	small := measureSliceLocal(strings.Repeat("a", 32), 100)
	big := measureSliceLocal(strings.Repeat("a", 33), 100)
	if small != 0 {
		t.Errorf("32바이트 이하인데 힙 할당이 있다: %v", small)
	}
	if !(small < big) {
		t.Errorf("33바이트에서 힙으로 넘어가지 않았다: 32=%v 33=%v", small, big)
	}
}

// TestWritePathsAgree 는 io.WriteString 과 Write 가 같은 바이트 수를 쓰는지 본다.
// 복사 회피용 인터페이스가 결과를 바꾸지 않는다는 것을 확인하는 것이 목적이다.
func TestWritePathsAgree(t *testing.T) {
	w := &countingWriter{}
	s := "hello"
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatalf("Write 실패: %v", err)
	}
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("WriteString 실패: %v", err)
	}
	if w.bytes != 2*len(s) {
		t.Errorf("쓴 바이트 수가 다르다: got=%d want=%d", w.bytes, 2*len(s))
	}
}

// BenchmarkStringFromBytesEscaping 은 결과가 함수 밖으로 나가는 변환의 비용을 잰다.
// b.ReportAllocs 와 b.ResetTimer 를 준비 작업 뒤에 두어야 측정이 오염되지 않는다.
func BenchmarkStringFromBytesEscaping(b *testing.B) {
	raw := []byte("hello world, a payload long enough to matter")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = string(raw)
	}
}

// BenchmarkMapLookupBytesKey 는 map 조회 문맥에서 복사가 사라지는지 잰다.
func BenchmarkMapLookupBytesKey(b *testing.B) {
	key := []byte("hello world, a payload long enough to matter")
	m := map[string]int{string(key): 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkInt += m[string(key)]
	}
}