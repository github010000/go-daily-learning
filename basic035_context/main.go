package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// slowWork 는 d 만큼 걸리는 작업을 흉내 낸다.
// 오래 걸리는 함수는 반드시 ctx.Done() 을 함께 기다려야 한다.
// 그렇지 않으면 호출자가 취소해도 이 함수는 끝까지 달린다.
func slowWork(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// runWithTimeout 은 마감 시간을 두고 작업을 돌린다.
// defer cancel() 은 작업이 일찍 끝나도 타이머 같은 내부 자원을 즉시 풀어 주기 위해 꼭 필요하다.
func runWithTimeout(timeout, work time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return slowWork(ctx, work)
}

// propagate 는 부모를 취소하면 자식도 함께 취소되는 것을 보여 준다.
// 자식에 긴 마감(1시간)을 줘도 부모의 취소가 먼저 전파된다.
func propagate() (parentErr, childErr error) {
	parent, cancelParent := context.WithCancel(context.Background())
	child, cancelChild := context.WithTimeout(parent, time.Hour)
	defer cancelChild()

	cancelParent()
	<-child.Done()
	return parent.Err(), child.Err()
}

// ignoringWorker 는 ctx 를 받지만 쓰지 않는 「실수 버전」이다.
// 시그니처만 ctx 를 받는다고 취소가 지켜지는 것은 아니다.
func ignoringWorker(ctx context.Context, done chan<- struct{}) {
	time.Sleep(150 * time.Millisecond)
	close(done)
}

// respectingWorker 는 반복 중 매번 ctx.Done() 을 확인하는 「고친 버전」이다.
func respectingWorker(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
			// 여기서 한 단위의 일을 한다고 가정
		}
	}
}

// stoppedQuickly 는 worker 를 띄우고 곧바로 취소한 뒤, wait 안에 끝났는지 알려 준다.
func stoppedQuickly(worker func(context.Context, chan<- struct{}), wait time.Duration) bool {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go worker(ctx, done)
	cancel()

	select {
	case <-done:
		return true
	case <-time.After(wait):
		<-done // 시연이 끝난 뒤에 고루틴이 남지 않도록 마저 기다린다
		return false
	}
}

// 실수 버전: string 을 키로 쓰면 다른 패키지가 같은 문자열을 쓰는 순간 서로 덮어쓴다.
func mistakeKeys() string {
	ctx := context.WithValue(context.Background(), "id", "패키지 A 의 값")
	ctx = context.WithValue(ctx, "id", "패키지 B 의 값")
	v, _ := ctx.Value("id").(string)
	return v
}

// 고친 버전: 패키지 안에서만 보이는 빈 구조체 타입을 키로 쓰고 접근 함수를 공개한다.
type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func RequestID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok
}

func demoTimeout() {
	fmt.Println("== 마감 시간 ==")
	err := runWithTimeout(20*time.Millisecond, 2*time.Second)
	fmt.Println("느린 작업:", err, "/ DeadlineExceeded 인가:", errors.Is(err, context.DeadlineExceeded))
	err = runWithTimeout(2*time.Second, time.Millisecond)
	fmt.Println("빠른 작업:", err)
}

func demoPropagate() {
	fmt.Println("== 취소 전파 ==")
	p, c := propagate()
	fmt.Println("부모:", p, "/ 자식:", c)
}

func demoLeak() {
	fmt.Println("== 취소를 무시하는 작업 ==")
	fmt.Println("실수 버전: 30ms 안에 멈췄는가 =", stoppedQuickly(ignoringWorker, 30*time.Millisecond))
	fmt.Println("고친 버전: 30ms 안에 멈췄는가 =", stoppedQuickly(respectingWorker, 30*time.Millisecond))
}

func demoValues() {
	fmt.Println("== context 값 ==")
	fmt.Println("실수 버전(string 키): A 가 읽은 값 =", mistakeKeys())
	ctx := WithRequestID(context.Background(), "req-42")
	id, ok := RequestID(ctx)
	fmt.Println("고친 버전(전용 키 타입):", id, ok)
	_, ok = RequestID(context.Background())
	fmt.Println("값을 넣지 않은 ctx:", ok)
}

func main() {
	demoTimeout()
	demoPropagate()
	demoLeak()
	demoValues()
}