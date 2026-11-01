package main

import "testing"

// TestHybridBarrierNeverLosesLiveObject 는 Go 1.8 부터 쓰는 hybrid 배리어가
// 다섯 시나리오 전부에서 살아있는 객체를 하나도 잃지 않는지 검증한다.
// 이 불변식이 깨지면 실제 런타임에서는 use-after-free 로 나타나므로,
// 여기서 lost 가 0 이라는 것이 이 주제의 가장 중요한 주장이다.
func TestHybridBarrierNeverLosesLiveObject(t *testing.T) {
	for _, sc := range scenarios() {
		m := sc.run(barrierHybrid)
		v := m.verify()
		if v.lost != 0 {
			t.Errorf("%s: hybrid 배리어인데 살아있는 객체 %d개를 잃었다 (%s)", sc.name, v.lost, dumpColors(m))
		}
		if len(m.work) != 0 {
			t.Errorf("%s: 마킹이 끝났는데 작업 목록에 %d개가 남았다", sc.name, len(m.work))
		}
		for _, o := range m.objs {
			if o.color == grey {
				t.Errorf("%s: 마킹이 끝났는데 회색 객체가 남았다: %s", sc.name, o.tag)
			}
		}
		if v.lost == 0 && v.stackScans != 1 {
			t.Errorf("%s: hybrid 는 스택을 한 번만 훑어야 하는데 %d회 훑었다", sc.name, v.stackScans)
		}
	}
}

// TestBarrierModesDifferAtTheExpectedScenario 는 각 배리어가 "어디서" 실패하는지를
// 고정한다. 이 표가 이 주제의 핵심 주장이므로 시나리오별 기대값을 숫자로 못박는다.
func TestBarrierModesDifferAtTheExpectedScenario(t *testing.T) {
	byName := make(map[string]scenario, len(scenarios()))
	for _, sc := range scenarios() {
		byName[sc.name] = sc
	}
	cases := []struct {
		scenario string
		mode     barrierMode
		wantLost int
	}{
		{"S2", barrierNone, 1}, // 배리어가 없으면 힙 안에서만 옮겨도 잃는다
		{"S2", barrierInsert, 0},
		{"S2", barrierDelete, 0},
		{"S3", barrierDelete, 1}, // 회색 스택: 삭제 배리어는 스택 쓰기를 못 본다
		{"S3", barrierInsert, 0},
		{"S3", barrierHybrid, 0},
		{"S4", barrierInsert, 1}, // 검은 스택: 삽입 배리어는 힙에서의 삭제를 못 본다
		{"S4", barrierDelete, 0},
		{"S4", barrierHybrid, 0},
		{"S4", barrierInsertRescan, 0},
	}
	for _, c := range cases {
		sc, ok := byName[c.scenario]
		if !ok {
			t.Fatalf("시나리오 %s 를 찾지 못했다", c.scenario)
		}
		v := sc.run(c.mode).verify()
		if v.lost != c.wantLost {
			t.Errorf("%s / %v: lost=%d, 기대값 %d", c.scenario, c.mode, v.lost, c.wantLost)
		}
	}
}

// TestRescanCostsAnExtraStackScan 은 insert+rescan 이 정확하지만 스택을 두 번
// 훑는다는 사실을 검증한다. Go 1.8 이 제거한 것이 바로 이 두 번째 스캔이고,
// 그것이 hybrid 배리어를 도입한 이유다.
func TestRescanCostsAnExtraStackScan(t *testing.T) {
	for _, sc := range scenarios() {
		h := sc.run(barrierHybrid).verify()
		r := sc.run(barrierInsertRescan).verify()
		if h.stackScans != 1 {
			t.Errorf("%s: hybrid 의 스택 스캔 %d회, 기대값 1", sc.name, h.stackScans)
		}
		if r.stackScans != 2 {
			t.Errorf("%s: insert+rescan 의 스택 스캔 %d회, 기대값 2", sc.name, r.stackScans)
		}
		if r.lost != 0 {
			t.Errorf("%s: insert+rescan 인데 살아있는 객체 %d개를 잃었다", sc.name, r.lost)
		}
	}
}

// TestAllocDuringMarkIsBlack 은 마킹 중 할당된 객체가 즉시 검은색이 되는지
// 확인한다(runtime.gcmarknewobject 와 같은 동작). 그래서 할당 경로에는 배리어가
// 필요 없고, 대신 그 객체는 이번 사이클에 절대 회수되지 않는다.
func TestAllocDuringMarkIsBlack(t *testing.T) {
	m := newMachine(barrierHybrid)
	m.startMark()
	if got := m.alloc("fresh").color; got != black {
		t.Fatalf("마킹 중 할당한 객체의 색이 %v, 기대값은 검정", got)
	}
	m2 := newMachine(barrierHybrid)
	if got := m2.alloc("cold").color; got != white {
		t.Fatalf("마킹 전 할당한 객체의 색이 %v, 기대값은 흰색", got)
	}
}

// TestDeadBlackObjectSurvivesOneCycle 은 삭제 배리어가 만드는 "이번 사이클에
// 회수되지 않는 쓰레기"를 검증한다. 정확성의 대가로 지불하는 메모리 비용이고,
// 컨테이너 메모리 한도에서 RSS 가 튀는 원인이다.
func TestDeadBlackObjectSurvivesOneCycle(t *testing.T) {
	m := newMachine(barrierHybrid)
	root := m.alloc("root")
	a := m.alloc("a")
	b := m.alloc("b")
	root.slots[0] = a
	a.slots[0] = b
	m.globals = append(m.globals, root)
	m.stack = append(m.stack, nil)

	m.startMark()
	m.scanStack()
	m.drainAll()           // 전부 검정
	m.storeHeap(a, 0, nil) // b 를 버린다
	m.finishMark()

	v := m.verify()
	if v.lost != 0 {
		t.Fatalf("살아있는 객체를 잃었다: lost=%d", v.lost)
	}
	if v.floating != 1 {
		t.Fatalf("b 는 죽었지만 이번 사이클에 회수되면 안 된다: floating=%d", v.floating)
	}
}

// BenchmarkPointerStore 는 힙 슬롯에 포인터를 쓰는 비용을 잰다.
// GC 가 마킹 중이면 이 루프의 모든 저장에 배리어가 붙는다.
func BenchmarkPointerStore(b *testing.B) {
	dst := make([]*cell, 1024)
	src := &cell{}
	b.ResetTimer()
	storePointerLoop(dst, src, b.N)
}

// BenchmarkWordStore 는 같은 모양의 정수 쓰기를 잰다. 배리어가 붙지 않으므로
// 위 벤치마크와의 차이가 배리어 및 GC 압력의 몫이다.
func BenchmarkWordStore(b *testing.B) {
	dst := make([]uintptr, 1024)
	b.ResetTimer()
	storeWordLoop(dst, 1, b.N)
}