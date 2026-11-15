package main

import (
	"bytes"
	"compress/gzip"
	"testing"
	"time"
)

// pbVarint / pbTag / pbBytes / pbUint 는 손으로 profile 메시지를 인코딩하는
// 최소 도구다. 실제 프로파일 파일에 기대지 않고 디코더와 flat/cum 계산을
// 결정적으로 검증하려고 쓴다. 표본 개수가 항상 같은 입력을 만들 수 있다.
func pbVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func pbTag(dst []byte, field, wire int) []byte {
	return pbVarint(dst, uint64(field)<<3|uint64(wire))
}

func pbBytes(dst []byte, field int, body []byte) []byte {
	dst = pbTag(dst, field, 2)
	dst = pbVarint(dst, uint64(len(body)))
	return append(dst, body...)
}

func pbUint(dst []byte, field int, v uint64) []byte {
	dst = pbTag(dst, field, 0)
	return pbVarint(dst, v)
}

// buildSyntheticProfile 은 다음 스택 하나가 3번 잡힌 가짜 프로파일을 만든다.
//
//	hotLeaf <- hotTop   (표본 3개)
//
// 기대: Total=3, hotLeaf flat=3 cum=3, hotTop flat=0 cum=3.
// location_id 와 value 를 packed(wire 2) 형식으로 써서 실제 Go 런타임이
// 내보내는 인코딩과 같은 모양을 만든다.
func buildSyntheticProfile() []byte {
	strs := []string{"", "hotLeaf", "hotTop"}

	fn1 := pbUint(pbUint(nil, 1, 1), 2, 1) // Function{id:1, name:1}
	fn2 := pbUint(pbUint(nil, 1, 2), 2, 2) // Function{id:2, name:2}

	line1 := pbUint(nil, 1, 1)                   // Line{function_id:1}
	line2 := pbUint(nil, 1, 2)                   // Line{function_id:2}
	loc1 := pbUint(pbBytes(nil, 4, line1), 1, 1) // Location{line:[line1], id:1}
	loc2 := pbUint(pbBytes(nil, 4, line2), 1, 2) // Location{line:[line2], id:2}

	packedLocs := pbVarint(pbVarint(nil, 1), 2) // [1, 2] leaf 먼저
	sample := pbBytes(nil, 1, packedLocs)       // Sample{location_id:[1,2]}
	sample = pbBytes(sample, 2, []byte{3})      // Sample{value:[3]}

	var msg []byte
	for _, s := range strs {
		msg = pbBytes(msg, 6, []byte(s))
	}
	msg = pbBytes(msg, 5, fn1)
	msg = pbBytes(msg, 5, fn2)
	msg = pbBytes(msg, 4, loc1)
	msg = pbBytes(msg, 4, loc2)
	msg = pbBytes(msg, 2, sample)

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(msg)
	_ = zw.Close()
	return buf.Bytes()
}

// 손으로 만든 프로파일로 flat/cum 계산이 맞는지 본다. 표본 수가 고정이라
// 시간이나 머신 상태에 흔들리지 않는다. 검증하는 불변식은 두 가지다.
// flat 은 leaf 에만 붙고, cum 은 스택에 올라온 모든 프레임에 붙는다.
// 이걸 뒤집으면 driver 함수를 최적화하는 헛수고를 하게 된다.
func TestSyntheticProfileFlatAndCum(t *testing.T) {
	sum, err := decodeCPUProfile(buildSyntheticProfile())
	if err != nil {
		t.Fatalf("프로파일 디코딩 실패: %v", err)
	}
	if sum.Total != 3 {
		t.Fatalf("표본 총합 = %d, 기대 3", sum.Total)
	}
	leaf := statOf(sum, "hotLeaf")
	if leaf == nil {
		t.Fatalf("hotLeaf 가 프로파일에 없다")
	}
	if leaf.Flat != 3 || leaf.Cum != 3 {
		t.Errorf("hotLeaf flat=%d cum=%d, 기대 flat=3 cum=3", leaf.Flat, leaf.Cum)
	}
	top := statOf(sum, "hotTop")
	if top == nil {
		t.Fatalf("hotTop 이 프로파일에 없다")
	}
	if top.Flat != 0 {
		t.Errorf("hotTop flat=%d, 기대 0 (자기 몸통에서는 아무 일도 안 한다)", top.Flat)
	}
	if top.Cum != 3 {
		t.Errorf("hotTop cum=%d, 기대 3 (스택에 항상 올라와 있다)", top.Cum)
	}
}

// 실제로 뜬 프로파일이 만족해야 하는 구조적 불변식만 검사한다.
// 표본 개수 자체는 머신과 부하에 따라 달라지므로 절대값을 단정하지 않고
// flat <= cum <= total, 그리고 "CPU 를 태운 함수가 표본의 절반 이상"을 본다.
func TestProfileInvariants(t *testing.T) {
	sum, _, err := runProfile(func() { burnCPU(250 * time.Millisecond) })
	if err != nil {
		t.Fatalf("프로파일 실행 실패: %v", err)
	}
	// 아주 붐비는 CI 에서 표본이 0 일 수 있다. 그건 코드 버그가 아니다.
	if sum.Total == 0 {
		t.Skip("이 환경에서 CPU 표본이 하나도 잡히지 않음")
	}

	sumFlat := 0
	for name, st := range sum.Funcs {
		if st.Flat < 0 || st.Cum < 0 {
			t.Errorf("%s: 음수 표본 flat=%d cum=%d", shortName(name), st.Flat, st.Cum)
		}
		if st.Flat > st.Cum {
			t.Errorf("%s: flat=%d > cum=%d (flat 은 cum 의 부분집합이어야 한다)",
				shortName(name), st.Flat, st.Cum)
		}
		if st.Cum > sum.Total {
			t.Errorf("%s: cum=%d > 전체=%d", shortName(name), st.Cum, sum.Total)
		}
		sumFlat += st.Flat
	}
	if sumFlat > sum.Total {
		t.Errorf("flat 합 %d > 전체 표본 %d", sumFlat, sum.Total)
	}
	if sumFlat == 0 {
		t.Errorf("flat 표본이 하나도 없다: leaf 를 못 찾고 있다는 뜻")
	}

	tf, tc := flatCum(sum, "hotTop")
	if tc == 0 {
		t.Fatalf("hotTop 이 프로파일에 없다: CPU 를 태운 함수가 안 보인다")
	}
	if tc*2 < sum.Total {
		t.Errorf("hotTop cum=%d 가 전체 %d 의 절반도 안 된다", tc, sum.Total)
	}
	if tf > tc/2 {
		t.Errorf("hotTop flat=%d 가 cum=%d 에 비해 너무 크다 (call overhead 만 있는 함수)", tf, tc)
	}
	_, lc := flatCum(sum, "hotLeaf")
	if lc != tc {
		t.Errorf("hotLeaf cum=%d, hotTop cum=%d: 같은 스택인데 다르다", lc, tc)
	}
}

// syscall 안에서 놀고 있는 시간은 CPU 표본에 안 잡힌다는 것을 절대값이
// 아니라 두 구간의 비교로 확인한다. "몇 ms 안에" 같은 시간 단정은 쓰지 않아
// CI 부하에서도 깨지지 않는다.
func TestBlockedSyscallInvisible(t *testing.T) {
	blocked, _, err := runProfile(func() {
		if err := blockOnSyscall(250 * time.Millisecond); err != nil {
			t.Errorf("파이프 실패: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("블로킹 구간 프로파일 실패: %v", err)
	}
	cpu, _, err := runProfile(func() { burnCPU(250 * time.Millisecond) })
	if err != nil {
		t.Fatalf("CPU 구간 프로파일 실패: %v", err)
	}
	if cpu.Total == 0 {
		t.Skip("이 환경에서 CPU 표본이 하나도 잡히지 않음")
	}
	if blocked.Total >= cpu.Total {
		t.Errorf("블로킹 구간 표본 %d >= CPU 구간 표본 %d: syscall 대기가 프로파일에 새어 들어왔다",
			blocked.Total, cpu.Total)
	}
}

// 짧은 함수를 여러 번 부르는 경로 자체의 비용을 잰다. 이 벤치마크는
// pprof 와 무관하게 "표본에 안 잡히는 코드"의 실제 속도를 재는 용도다.
// x 를 밖에서 선언하고 마지막에 확인하는 이유는 루프 전체가 죽지 않게 하려는 것.
func BenchmarkHotPath(b *testing.B) {
	x := uint64(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x = hotTop(x)
	}
	if x == 0 {
		b.Fatalf("최적화로 루프가 사라졌다")
	}
}