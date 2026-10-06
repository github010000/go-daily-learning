package main

import (
	"context"
	"testing"
	"time"
)

// TestObserveCancelChildPropagatesToDescendantButNotParent는
// child를 취소했을 때 grandchild는 취소되고 parent는 취소되지 않는 방향성을 검증한다.
// 시간에 의존하지 않고 Done 채널의 닫힘만으로 판정한다.
func TestObserveCancelChildPropagatesToDescendantButNotParent(t *testing.T) {
	p, c, g := observeCancel(true)
	if p {
		t.Errorf("child 취소로 부모가 취소되면 안 됩니다")
	}
	if !c || !g {
		t.Errorf("child 취소는 child와 grandchild를 취소해야 합니다: child=%v grandchild=%v", c, g)
	}
}

// TestObserveCancelParentPropagatesToAll은
// parent를 취소했을 때 children 맵을 따라 모든 자식이 취소되는지 검증한다.
// 부모-자식-손자 전체 Done 상태를 확인해 순서가 아니라 불변식으로 판정한다.
func TestObserveCancelParentPropagatesToAll(t *testing.T) {
	p, c, g := observeCancel(false)
	if !p || !c || !g {
		t.Fatalf("parent 취소는 모든 자식을 취소해야 합니다: parent=%v child=%v grandchild=%v", p, c, g)
	}
}

// TestCancelWithTimeoutBeforeDeadlineClosesDone은
// WithTimeout의 cancel을 마감 시각 전에 호출하면 Done 채널이 즉시 닫히고
// Err()가 context.Canceled가 되는지 검증한다.
// time.Hour를 쓰지만 cancel을 즉시 호출하므로 테스트는 시간을 기다리지 않는다.
func TestCancelWithTimeoutBeforeDeadlineClosesDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	cancel()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Fatalf("기대한 에러는 context.Canceled지만 실제는 %v", err)
		}
	default:
		t.Fatal("cancel 직후 Done 채널이 닫혀 있어야 합니다")
	}
}