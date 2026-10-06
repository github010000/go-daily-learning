## 한 줄 요약

context는 부모에서 자식 방향으로만 취소를 전파하는 트리를 만들어, 요청 단위로 묶인 모든 goroutine이 하나의 Done 채널을 보고 정리되도록 한다. 그 핵심은 `cancelCtx.children` 맵과 `propagateCancel`의 분기 처리다.

## 왜 이런 설계인가

서버 프로그램은 한 요청을 처리할 때 여러 goroutine을 만들고, 클라이언트가 끊거나 타임아웃이 나면 그 요청에 딸린 작업을 모두 중단해야 한다. OS 스레드는 이 문제를 풀지 못한다. goroutine은 OS 스레드와 1:1로 묶이지 않고, 특정 스레드를 강제로 종료해도 그 스레드 위에서 돌던 다른 goroutine까지 죽을 수 있다. 또한 블록된 시스템 콜을 스레드 취소로 중단시킬 수 있는 보장도 없다. 그래서 Go는 스레드를 직접 제어하는 대신, 논리적인 요청 단위에 취소 신호를 전달하는 구조가 필요했다.

가장 단순한 대안은 `done chan struct{}`를 모든 함수에 인자로 넘기는 것이다. 그러나 호출 트리가 깊어지면 채널을 주고받는 시그니처가 지저분해지고, 이미 표준 라이브러리와 수많은 서드파티 라이브러리가 서로 다른 종료 패턴을 쓰게 된다. 하나의 인터페이스로 묶고, 읽기 전용 신호와 그 신호를 언제 닫을지 결정하는 cancel 함수를 함께 반환하는 API가 필요했다. context는 `Done()`이라는 공통 메서드로 그 문제를 정리했다.

context의 취소 전파는 부모에서 자식으로만 흐른다. 자식 하나를 취소한다고 부모나 형제를 취소하지 않는다. 이 선택이 중요한 이유는, 부모는 요청 전체의 생명주기를 대표하고 자식은 그 요청의 일부 작업이기 때문이다. 자식 하나가 끝났다고 전체 요청을 중단시키면 곤란한 경우가 대부분이다. 그래서 `WithCancel`으로 만든 자식은 부모의 children 맵에 등록하지만, 부모는 자식에게 자신을 등록하지 않는다.

취소를 부르지 않으면 어떤 일이 생기는지도 이 설계에서 설명된다. 표준 `cancelCtx` 부모를 가진 자식 context는 부모의 children 맵에 등록된다. 자식 cancel 함수를 잃어버려도 부모가 살아 있는 한 자식 context와 관련 객체가 메모리에 남는다. 이것은 goroutine 누수가 아니라 참조 누수다. goroutine은 이미 끝났을 수 있어도, context 객체와 그 children 맵이 계속 쌓이면 GC가 수거하지 못해 힙이 자란다. 이 차이를 아는 것이 중요하다.

`WithTimeout`은 `WithDeadline`과 함께 timerCtx를 만든다. 마감 시각에 cancel을 자동으로 호출하기 위해 `time.Timer`를 내부에 들고 있는데, 마감 전에 요청이 끝났다면 이 타이머를 반드시 멈춰야 한다. cancel을 호출하지 않으면 타이머가 마감 시각까지 살아 있고, 수천 개가 쌓이면 타이머 힙과 context 객체가 메모리를 점유한다. 이 때문에 `WithTimeout`은 반환된 cancel 함수를 `defer cancel()`로 부르는 패턴을 강하게 권장한다.

## 어떻게 동작하는가

`context.WithCancel(parent)`은 `cancelCtx`를 만들고 `propagateCancel(parent, c)`를 호출한다. `cancelCtx`에는 실제 자료구조로 `done chan struct{}`, `err error`, `children map[canceler]struct{}`이 있다. `done`은 취소되면 닫히는 채널이고, `err`은 `context.Canceled` 또는 `context.DeadlineExceeded`를 저장한다. `children`은 부모가 직접 취소할 때 신호를 내려보낼 자식 canceler들을 담은 맵이다.

`propagateCancel`은 세 갈래로 나뉜다. 첫째, 부모의 `Done()`이 nil이면 부모가 절대 취소되지 않는다는 뜻이므로 자식 등록을 생략한다. `context.Background()`와 `context.TODO()`가 여기에 해당한다. 둘째, 부모가 표준 `cancelCtx` 또는 `timerCtx`이면 부모를 잠근 뒤 부모가 이미 취소되었는지 확인하고, 이미 취소되었다면 자식을 즉시 취소한다. 아니라면 부모의 `children` 맵에 자식을 추가한다. 실제 코드는 `context/cancelCtx.go`가 아니라 `context/context.go` 안에 있다.

셋째, 부모가 `Done()`은 있지만 표준 cancelCtx가 아닌 커스텀 context라면, `propagateCancel`은 goroutine을 하나 띄워 `parent.Done()`과 `child.Done()`를 동시에 감시한다. 부모가 먼저 취소되면 자식을 취소하고, 자식이 먼저 취소되면 goroutine이 끝난다. 이 경로에서는 cancel을 부르지 않고 부모가 오래 살아 있으면 정말 goroutine 누수가 생길 수 있다. 다만 표준 `WithCancel`, `WithTimeout`, `WithDeadline`으로 만든 부모라면 `parentCancelCtx`가 내부 `cancelCtx`를 찾아 맵 등록을 하므로 goroutine은 필요 없다.

cancel이 실제로 불리면 `cancelCtx.cancel`은 mutex를 잡고 `err`이 이미 설정돼 있는지 확인한다. 이미 취소됐다면 바로 반환해서 이중 호출에도 안전하다. 처음 취소될 때는 `done` 채널을 닫고, `children` 맵을 순회하면서 각 자식의 `cancel(false, err)`을 호출한다. 자식 cancel은 다시 그 자식의 children을 취소하므로 재귀적으로 아래 방향 전파가 일어난다. 마지막으로 `children` 맵을 nil로 만들어 자식 참조를 끊는다. 자식 cancel은 `removeFromParent` 인자가 false이므로 부모의 children에서 스스로를 제거하지 않는다. 부모가 자식을 제거하는 구조다.

`WithTimeout`은 `timerCtx`를 만든다. `timerCtx`는 `cancelCtx`를 임베딩하고 `timer *time.Timer`를 추가로 갖는다. 마감 시각이 되면 타이머 콜백이 cancel을 호출한다. cancel이 마감 전에 호출되면 `timerCtx.cancel`이 `timer.Stop()`을 부르고, 이미 타이머 콜백이 실행됐다면 기다렸다가 타이머 자원을 정리한다. 이 Stop 호출이 없으면 타이머가 남아 context와 관련 메모리를 계속 붙잡는다. `WithDeadline`도 같은 `timerCtx`를 사용한다.

## 돌려보기

이 디렉토리에서 다음 명령을 순서대로 실행하면 된다.

```bash
go vet ./...                 # lostcancel 검사가 모든 cancel 함수가 사용되는지 확인
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 취소 전파, 참조 보유, 타이머 보유를 출력
go test -v ./...             # observeCancel과 WithTimeout cancel 동작 검증
go test -race ./...          # goroutine과 채널 사용에 data race가 없는지 검사
```

`go vet`에서는 `context.WithCancel`, `WithTimeout`이 반환한 cancel 함수가 버려지지 않고 사용되는지 검사한다. 이 프로젝트는 고의로 cancel을 미루는 부분을 `defer`로 구현해 vet를 통과한다. `go run .`을 보면 child 취소는 아래로만, parent 취소는 전부 퍼지는 결과가 나온다. HeapObjects 숫자는 실행 환경마다 다를 수 있으므로 절대값보다는 두 비교 대상 사이의 차이에 주목해야 한다.

주제를 더 깊이 관찰하고 싶다면 `GODEBUG=schedtrace=1000 go run .`처럼 실행하면 1초마다 스케줄러 상태가 stderr에 출력된다. 다만 이 예제에서는 context 참조 누수를 보여주는 것이 목적이므로 goroutine 수 변화가 크지 않을 수 있다. `go test -race`는 `observeCancel`이 만든 감시 goroutine들이 채널을 통해 안전하게 통신하는지 검증한다.

## 코드로 확인하기

`observeCancel(true)`는 child를 취소한다. 출력에서 `parent: false, child: true, grandchild: true`가 나와야 한다. parent가 false인 이유는 `childCancel()`이 오직 child의 done 채널만 닫고, child의 자식인 grandchild에게는 child.cancel이 전파되기 때문이다. grandchild는 child의 children에 들어 있었으므로 같이 취소된다. 부모는 자식을 취소하는 경로가 없으므로 parent는 살아 있다.

`observeCancel(false)`는 parent를 취소한다. 출력에서 모든 값이 true여야 한다. parentCancel이 parent의 children 맵을 따라 child를 취소하고, child가 다시 grandchild를 취소하기 때문이다. doneCh에서 신호를 받는 순서는 스케줄러에 따라 다를 수 있기 때문에 코드에서 switch로 이름을 확인해 판정한다. 테스트도 순서가 아니라 각 boolean의 최종 상태만 검증한다.

`measureLeakedChildren`은 5000개의 자식 context를 cancel 없이 만든 뒤 GC를 돌리고 HeapObjects 증가량을 출력한다. 이 증가량은 부모의 children 맵과 defer로 보관된 canceler들이 자식 context를 붙잡고 있다는 증거다. defer가 없으면 cancel 함수를 잃어버린 코드가 되어 vet가 불만을 낸다. 여기서는 함수가 반환될 때 모든 cancel이 실행되므로 측정 시점만 취소 전으로 두고 나중에 정리된다. 실제 응용 코드에서는 defer 없이 cancel을 잃어버리면 이 증가량만 남는다.

`measureWithTimeoutRetention`은 cancelEach가 true일 때와 false일 때를 비교한다. true는 즉시 `cancel()`을 호출해 타이머를 멈추므로 GC 후 HeapObjects 증가가 작거나 0에 가깝다. false는 `defer cancel()`로 취소를 함수 끝까지 미루므로, GC 시점에는 수천 개의 timerCtx와 time.Timer가 살아 있어 증가량이 크게 나온다. 이 차이가 `WithTimeout`에서 cancel을 부르지 않으면 타이머가 정리되지 않는다는 사실을 보여준다.

`main_test.go`의 첫 두 테스트는 `observeCancel`이 만들어 낸 boolean 결과를 검증한다. 세 번째 테스트는 `WithTimeout`의 cancel을 마감 전에 호출하면 Done 채널이 즉시 닫히고 Err()가 context.Canceled임을 확인한다. 모든 테스트는 시간에 의존하지 않으며, goroutine 순서 대신 닫힘 상태와 개수 같은 불변식으로 판정한다.

## 모르면 겪는 일

장시간 실행되는 worker에서 요청마다 `context.WithCancel`로 자식을 만들고 cancel을 부르지 않으면, 부모 context가 살아 있는 동안 자식 context가 계속 누적된다. goroutine 프로파일에는 보이지 않고, 힙 프로파일에는 `context.cancelCtx`, `context.children` 맵, 관련 `sync.Mutex` 같은 객체가 남는다. 메모리 누수처럼 보이지만 goroutine count는 늘지 않아서 원인을 찾기 어렵다.

HTTP 미들웨어에서 `context.WithTimeout(r.Context(), 3*time.Second)`를 만들고 cancel을 부르지 않으면, 요청이 빨리 끝나도 3초 동안 수천 개의 타이머가 쌓일 수 있다. p99 지연 시간이 GC 주기마다 튀는데 CPU 프로파일에는 요청 처리 로직이 거의 안 보이는 상황이 된다. timer heap이 커지면 GC가 context와 timer 객체를 더 자주 스캔해야 하므로 부하가 늘어난다.

커스텀 context를 부모로 쓰는 라이브러리를 사용할 때는 `propagateCancel`이 표준 맵 최적화를 쓰지 못하고 goroutine을 띄운다. 이 부모가 오래 살고 자식을 cancel하지 않으면 goroutine 누수가 생긴다. `pprof`의 goroutine 프로파일에서 `context.propagateCancel.func1`이 select에서 멈춰 있는 모습이 반복적으로 보인다면 이 경로를 의심해야 한다. 표준 context만 쓰면 이 문제는 자주 생기지 않지만, 커스텀 Done을 반환하는 구현이나 다른 래퍼 context를 쓸 때는 가능하다.

또한 context를 구조체에 저장해 두고 새 요청마다 같은 부모로부터 파생 context를 만들되 cancel을 보관하지 않는 코드가 있다. 이 경우 부모가 취소되어야 모든 자식이 정리되는데, 부모가 프로세스 수명과 같으면 프로그램이 끝날 때까지 자식 객체가 남는다. 테스트에서는 짧게 돌아서 안 보이지만, 실제 부하에서 HeapObjects가 선형적으로 증가한다.

## 언제 신경 쓰고 언제 무시하나

짧게 한 번 실행하고 끝나는 CLI 프로그램에서는 cancel을 부르지 않아도 프로세스가 곧 종료되므로 신경 쓸 필요가 없다. 소규모 배치나 스크립트 수준에서는 defer cancel을 습관화하는 것으로 충분하다. 하지만 HTTP 서버, gRPC 서버, 백그라운드 worker처럼 요청 단위로 context를 만들고 오래 실행되는 프로세스에서는 반드시 cancel을 정리해야 한다.

일반적으로 부모 context를 함수에서 받아 자식을 만들었다면 `defer cancel()`을 바로 붙이는 것이 답이다. `context.WithTimeout`을 만들었을 때도 동일하다. 다만 부모가 `context.Background()`처럼 절대 취소되지 않는 경우에는 `propagateCancel`이 자식 등록을 생략하므로, cancel을 잃어버려도 실제 참조 누수는 생기지 않는다. 그래도 코드 일관성을 위해 cancel을 부르는 것이 좋다.

부모가 커스텀 context인데 그 Done 채널이 절대 닫히지 않는 상황이 아니라면, 그리고 자식을 만드는 횟수가 초당 수천 개가 아니라면 cancel 누수가 금방 드러나지 않는다. 그러나 서비스가 트래픽을 받으면 하루 만에 힙이 수백 MB씩 자라거나 goroutine 수가 천천히 늘어날 수 있다. 문제는 보통 p99 GC 지연이나 알 수 없는 메모리 증가로 나타나므로, 이런 지식이 필요해지는 시점은 서비스 규모가 아니라 실행 시간이 길어지는 시점이다.

## 더 파보기

- [context/context.go 소스 코드](https://go.dev/src/context/context.go) — `propagateCancel`, `cancelCtx.cancel`, `WithTimeout`이 실제로 구현된 파일
- [context 패키지 문서](https://pkg.go.dev/context) — 공식 API 문서와 예제
- [Go Concurrency Patterns: Context](https://go.dev/blog/context) — context가 도입된 배경과 사용 패턴
- [Go 이슈에서 context leak 검색](https://github.com/golang/go/issues?q=context+leak) — context와 관련된 누수 및 설계 논의