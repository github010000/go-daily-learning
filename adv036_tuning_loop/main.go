package main

import (
	"flag"
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// 이 파일은 "측정 -> 가설 -> 반증 -> 판정" 루프를 코드로 보여준다.
// 대상 작업: "k=v;k=v;..." 형태의 로그 줄들에서 "k=v" 쌍의 빈도를 센다.

// 벤치마크 결과가 dead code 로 제거되지 않도록 붙잡아 두는 전역 변수.
// 지역 변수에 받으면 컴파일러가 호출 자체를 지울 수 있어 측정이 무의미해진다.
var (
	sinkMap     map[string]int
	scratchSink []byte
)

// genInput 은 결정적(deterministic) 입력을 만든다.
// 난수를 쓰지 않는 이유: 실행마다 입력이 다르면 "코드가 바뀌어서"인지
// "입력이 바뀌어서"인지 분리할 수 없다. 측정에서 변인 통제의 첫걸음이다.
func genInput(lines, pairs int) []string {
	out := make([]string, 0, lines)
	for l := 0; l < lines; l++ {
		parts := make([]string, 0, pairs)
		for p := 0; p < pairs; p++ {
			parts = append(parts, "k"+strconv.Itoa((l+p)%5)+"=v"+strconv.Itoa((l*p)%7))
		}
		out = append(out, strings.Join(parts, ";"))
	}
	return out
}

// aggBaseline: 출발점. 프로파일을 보기 전까지는 "그냥 읽기 쉬운 코드"다.
// 최적화는 반드시 이런 기준선이 있어야 시작할 수 있다.
func aggBaseline(lines []string) map[string]int {
	m := map[string]int{}
	for _, line := range lines {
		for _, pair := range strings.Split(line, ";") {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				continue
			}
			m[fmt.Sprintf("%s=%s", kv[0], kv[1])]++
		}
	}
	return m
}

// aggPresize: "map 이 느릴 것 같다"는 추측에서 나온 수정.
// 프로파일 없이 직감으로 고친 전형적인 예이며, 뒤에서 반증된다.
// 고정 크기 1024 는 입력이 작을 때 오히려 큰 메모리를 먼저 잡아먹는다.
func aggPresize(lines []string) map[string]int {
	m := make(map[string]int, 1024)
	for _, line := range lines {
		for _, pair := range strings.Split(line, ";") {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				continue
			}
			m[fmt.Sprintf("%s=%s", kv[0], kv[1])]++
		}
	}
	return m
}

// aggFixed: alloc 프로파일에서 Split 과 Sprintf 가 할당 대부분을 차지한다는
// 가설을 세우고 그 두 곳만 고친 버전. strings.Cut 은 슬라이스를 만들지 않고,
// 키 "k=v" 는 pair 자체와 같으므로 Sprintf 가 필요 없다.
func aggFixed(lines []string) map[string]int {
	m := map[string]int{}
	for _, line := range lines {
		rest := line
		for rest != "" {
			var pair string
			pair, rest, _ = strings.Cut(rest, ";")
			_, v, found := strings.Cut(pair, "=")
			if !found || strings.Contains(v, "=") {
				continue // baseline 의 len(kv) != 2 조건과 동일한 의미를 유지한다
			}
			if c, exists := m[pair]; exists {
				m[pair] = c + 1
			} else {
				// pair 는 line 의 부분 문자열이다. 그대로 키로 저장하면 map 이
				// 원본 줄 전체를 메모리에 붙잡는다. 처음 넣을 때만 복사한다.
				m[strings.Clone(pair)] = 1
			}
		}
	}
	return m
}

// aggScratch: "버퍼를 미리 잡아두면 빠를 것"이라는 과최적화.
// 큰 입력에서는 티가 안 나고 작은 입력에서는 호출마다 64KB 를 낭비한다.
// 전역에 대입하므로 escape analysis 가 이를 힙 할당으로 만든다.
func aggScratch(lines []string) map[string]int {
	scratchSink = make([]byte, 0, 1<<16)
	return aggFixed(lines)
}

// Measure 는 한 워크로드에서의 한 번의 측정 결과다.
type Measure struct {
	Workload    string
	NsPerOp     float64
	BytesPerOp  int64
	AllocsPerOp int64
}

// measure 는 testing.Benchmark 를 main 에서 직접 호출한다.
// 별도 도구 없이도 b.N 자동 조절과 ReportAllocs 를 그대로 쓸 수 있다.
func measure(workload string, f func([]string) map[string]int, in []string) Measure {
	r := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sinkMap = f(in)
		}
	})
	return Measure{workload, float64(r.NsPerOp()), r.AllocedBytesPerOp(), r.AllocsPerOp()}
}

// bytesPerCall 은 시간이 아닌 "할당 바이트"를 잰다. 시간과 달리
// 이 값은 거의 결정적이라 테스트에서 단정으로 쓸 수 있다.
func bytesPerCall(f func([]string) map[string]int, in []string, runs int) uint64 {
	sinkMap = f(in) // 워밍업: 처음 한 번의 초기화 비용을 제외한다
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	for i := 0; i < runs; i++ {
		sinkMap = f(in)
	}
	runtime.ReadMemStats(&b)
	return (b.TotalAlloc - a.TotalAlloc) / uint64(runs)
}

// Verdict 는 최적화를 유지할지 되돌릴지에 대한 판정이다.
type Verdict struct {
	Keep    bool
	Reasons []string
}

func rel(base, cand float64) float64 {
	d := base
	if d < 1 {
		d = 1
	}
	return (cand - base) / d
}

// Decide 는 "한 군데라도 tol 이상 나빠지면 되돌린다, 어디서도 tol 이상
// 좋아지지 않으면 되돌린다"는 규칙이다. 평균 하나로 판단하지 않고
// 워크로드별로 보는 이유는, 큰 입력의 이득이 작은 입력의 손해를 가리기 때문이다.
func Decide(base, cand []Measure, tol float64) Verdict {
	v := Verdict{Keep: true}
	byName := map[string]Measure{}
	for _, c := range cand {
		byName[c.Workload] = c
	}
	improved := false
	for _, b := range base {
		c, ok := byName[b.Workload]
		if !ok {
			continue
		}
		metrics := []struct {
			name string
			r    float64
		}{
			{"ns/op", rel(b.NsPerOp, c.NsPerOp)},
			{"B/op", rel(float64(b.BytesPerOp), float64(c.BytesPerOp))},
			{"allocs/op", rel(float64(b.AllocsPerOp), float64(c.AllocsPerOp))},
		}
		for _, m := range metrics {
			if m.r > tol {
				v.Keep = false
				v.Reasons = append(v.Reasons, fmt.Sprintf("%s 워크로드의 %s 가 %+.0f%% 나빠짐", b.Workload, m.name, m.r*100))
			} else if m.r < -tol {
				improved = true
			}
		}
	}
	if !improved {
		v.Keep = false
		v.Reasons = append(v.Reasons, "어떤 지표도 노이즈 허용치 이상 좋아지지 않음: 복잡도만 늘었다")
	}
	return v
}

func sameResult(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

type variant struct {
	name string
	fn   func([]string) map[string]int
}

func printRow(name string, ms []Measure) {
	for _, m := range ms {
		fmt.Printf("  %-10s %-6s %12.0f ns/op %10d B/op %8d allocs/op\n",
			name, m.Workload, m.NsPerOp, m.BytesPerOp, m.AllocsPerOp)
	}
}

func main() {
	// 벤치마크를 기본 1초 대신 짧게 돌려 시연 시간을 줄인다.
	testing.Init()
	_ = flag.Set("test.benchtime", "200ms")

	small, large := genInput(4, 6), genInput(2000, 6)
	variants := []variant{
		{"baseline", aggBaseline}, {"presize", aggPresize},
		{"fixed", aggFixed}, {"scratch", aggScratch},
	}

	fmt.Println("[0] 정합성: 최적화 전에 결과가 같은지부터 확인한다")
	ref := aggBaseline(large)
	for _, v := range variants {
		fmt.Printf("  %-10s baseline 과 동일 결과: %v\n", v.name, sameResult(ref, v.fn(large)))
	}

	fmt.Println("\n[1] 노이즈 바닥 측정: 같은 코드를 두 번 재서 흔들림을 본다")
	n1 := measure("large", aggBaseline, large)
	n2 := measure("large", aggBaseline, large)
	spread := math.Abs(n1.NsPerOp-n2.NsPerOp) / math.Min(n1.NsPerOp, n2.NsPerOp)
	tol := math.Max(0.10, 2*spread)
	fmt.Printf("  동일 코드 두 번: %.0f vs %.0f ns/op, 차이 %.1f%% -> 판정 허용치 %.0f%%\n",
		n1.NsPerOp, n2.NsPerOp, spread*100, tol*100)

	fmt.Println("\n[2] 변형별 측정 (small=4줄, large=2000줄)")
	results := map[string][]Measure{}
	for _, v := range variants {
		ms := []Measure{measure("small", v.fn, small), measure("large", v.fn, large)}
		results[v.name] = ms
		printRow(v.name, ms)
	}

	fmt.Println("\n[3] 판정: baseline 대비 유지할 것인가")
	for _, v := range variants[1:] {
		d := Decide(results["baseline"], results[v.name], tol)
		verdict := "되돌림(REVERT)"
		if d.Keep {
			verdict = "유지(KEEP)"
		}
		fmt.Printf("  %-10s %s\n", v.name, verdict)
		for _, r := range d.Reasons {
			fmt.Printf("      - %s\n", r)
		}
	}
}