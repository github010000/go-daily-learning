package main

import (
	"strings"
	"testing"
	"time"
)

// 덤프 파싱이 틀리면 누수 탐지 전체가 조용히 거짓 음성을 낸다.
// 실제 런타임 덤프는 실행마다 내용이 달라지므로, 파서만 고정 텍스트로 따로 검증한다.
// 여기서 보는 것은 세 가지다: id 추출, 대기 상태 추출(꼬리 제거 포함), Raw 보존.
func TestParseStacks(t *testing.T) {
	const dump = `goroutine 7 [chan send]:
main.leakChannelSend.func1()
	/tmp/main.go:20 +0x2c
created by main.leakChannelSend in goroutine 1
	/tmp/main.go:18 +0x40

goroutine 12 [select, 5 minutes]:
main.idleLoop()
	/tmp/main.go:33 +0x11

goroutine 1 [running]:
main.main()
	/tmp/main.go:9 +0x1c
`
	got := parseStacks(dump)
	if len(got) != 3 {
		t.Fatalf("goroutine 3개를 기대했지만 %d개 나왔습니다: %+v", len(got), got)
	}
	// 주의: 순서는 등장 순서다. id 정렬이 아니다.
	if got[0].ID != 7 || got[0].State != "chan send" || got[0].TopFunc != "main.leakChannelSend.func1" {
		t.Errorf("첫 항목이 틀렸습니다: %+v", got[0])
	}
	if !strings.Contains(got[0].Raw, "created by main.leakChannelSend") {
		t.Errorf("Raw 에 created by 줄이 보존되지 않았습니다: %q", got[0].Raw)
	}
	// "5 minutes" 같은 대기 시간 꼬리는 상태 분류에서 잘라내야 한다.
	if got[1].ID != 12 || got[1].State != "select" || got[1].TopFunc != "main.idleLoop" {
		t.Errorf("둘째 항목이 틀렸습니다: %+v", got[1])
	}
	if got[2].ID != 1 || got[2].State != "running" || got[2].TopFunc != "main.main" {
		t.Errorf("셋째 항목이 틀렸습니다: %+v", got[2])
	}
}

// 탐지기가 실제로 "채널 송신에서 멈춘 goroutine"을 찾아내는지 확인한다.
// 찾아낸 뒤 release 로 정리까지 되어야 다음 테스트의 기준선이 오염되지 않는다.
// release 는 멱등이라 t.Cleanup 과 직접 호출이 겹쳐도 안전하다.
func TestDetectsChannelSendLeak(t *testing.T) {
	lc := NewLeakChecker()
	started, release := leakChannelSend()
	t.Cleanup(release)

	// go 문은 goroutine 을 런큐에 넣고 즉시 반환한다. 시작 신호를 기다려야
	// "아직 시작도 안 한 상태"를 보고 0개라고 착각하지 않는다.
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("워커 goroutine 이 시작 신호를 보내지 않았습니다")
	}

	var leaked []GoroutineInfo
	ok := waitFor(5*time.Second, func() bool {
		for _, g := range lc.Report() {
			// 상태 이름과 스택 내용을 함께 본다. 상태만 보면 다른 goroutine 이
			// 우연히 chan send 일 때 거짓 양성이 난다.
			if g.State == "chan send" && strings.Contains(g.Raw, "main.leakChannelSend") {
				leaked = []GoroutineInfo{g}
				return true
			}
		}
		return false
	})
	if !ok {
		t.Fatalf("chan send 상태의 goroutine 을 찾지 못했습니다. 보고된 목록: %+v", lc.Report())
	}
	if len(leaked) != 1 {
		t.Fatalf("정확히 1개를 기대했습니다: %+v", leaked)
	}

	release()
	if !waitFor(5*time.Second, func() bool { return len(lc.Report()) == 0 }) {
		t.Fatalf("release 후에도 goroutine 이 남아 있습니다: %+v", lc.Report())
	}
}

// 고친 버전은 함수가 반환할 때 워커 goroutine 도 함께 끝나야 한다.
// 이 테스트가 깨지면 "고쳤다고 생각한 코드"가 여전히 새고 있다는 뜻이다.
// 고정 sleep 대신 조건 대기(waitFor)를 쓰는 이유는 부하 걸린 CI 에서도
// 같은 판정이 나오게 하기 위해서다.
func TestFixedVersionsLeaveNoGoroutine(t *testing.T) {
	base := takeSnapshot()

	fixedChannelSend()
	fixedRangeClose()
	fixedContextCancel()
	fixedLoopStop()

	if !waitFor(5*time.Second, func() bool { return len(newSince(base)) == 0 }) {
		t.Fatalf("고친 버전 실행 후에도 goroutine 이 남아 있습니다: %+v", newSince(base))
	}
}

// 거짓 양성을 내는 탐지기는 테스트에 넣는 순간 아무도 안 쓰게 된다.
// 깨끗한 코드에 대해서는 아무것도 보고하지 않아야 한다.
// 그리고 앞선 테스트가 남긴 goroutine 이 기준선에 섞여 들어가지 않는지도
// 함께 확인된다 (NewLeakChecker 가 지금 시점 스냅숏을 뜨므로).
func TestLeakCheckerReportsNothingOnCleanCode(t *testing.T) {
	lc := NewLeakChecker()

	fixedChannelSend()
	fixedRangeClose()

	if !waitFor(5*time.Second, func() bool { return len(lc.Report()) == 0 }) {
		t.Fatalf("깨끗한 코드인데 누수로 보고했습니다: %+v", lc.Report())
	}
}

// 누수 탐지는 모든 goroutine 의 스택을 긁어 텍스트를 만든다.
// 핫 패스에서 부르면 안 되는 이유를 수치로 보여준다.
func BenchmarkTakeSnapshot(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = takeSnapshot()
	}
}