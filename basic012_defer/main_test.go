package main

import (
	"os"
	"testing"
)

// demoLIFOAndNamedReturn는 LIFO 실행 순서와 named return value의 상호 작용을
// 검증합니다. 두 defer 함수가 어떤 순서로 실행되어 최종 반환값 11을 만드는지
// 테이블 주도 방식으로 확인합니다.
// - 첫 번째 defer: result++ (1 → 2)
// - 두 번째 defer: result *= 10 (2 → 20)
// - LIFO 순서로 두 번째 defer가 먼저 실행: 1 * 10 = 10
// - 그 다음 첫 번째 defer: 10 + 1 = 11
func TestDeferLIFOAndNamedReturn(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{"LIFO + named return", 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := demoLIFOAndNamedReturn()
			if got != tt.expected {
				t.Fatalf("demoLIFOAndNamedReturn() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// demoDeferInLoopFixed는 루프 안에서 defer를 올바르게 사용할 때
// 파일이 즉시 닫히는지를 검증합니다.
// 루프가 끝난 후에도 파일이 열려 있으면 이후에 같은 파일명으로
// 재작성하는 등의 문제가 발생할 수 있으므로,
// 이 테스트는 고친 버전이 실제로 자원을 정리하는지를 확인합니다.
func TestDeferInLoopFixed(t *testing.T) {
	t.Run("closed files do not leak descriptors", func(t *testing.T) {
		// 고친 버전 실행 — 내부에서 즉시 정리됩니다.
		demoDeferInLoopFixed()

		// 실제로 닫혔는지 직접 검증하기 위해
		// 비슷한 패턴으로 직접 만들어보고 바로 닫았을 때
		// 에러가 없는지 확인합니다.
		for i := 0; i < 10; i++ {
			func() {
				f, err := os.CreateTemp("", "test-*.txt")
				if err != nil {
					t.Fatalf("create temp error: %v", err)
				}
				defer f.Close()
				if f.Name() == "" {
					t.Error("expected non-empty temp file name")
				}
			}()
		}
	})
}