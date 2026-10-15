## 한 줄 요약

errgroup은 여러 goroutine의 반환 오류를 하나로 모으고, 첫 non-nil error가 발생하면 파생 context를 즉시 취소한다. SetLimit은 동시에 실행 중인 goroutine 수를 int 값으로 제한한다. 표준 library만 쓰는 이 예제에서 x/sync/errgroup의 핵심 구조를 직접 재현해 보여준다.

## 왜 이런 설계인가

WaitGroup만 쓰면 오류를 전달할 통로가 없다. `chan error`를 만들면 누가 언제 닫을지, 버퍼 크기를 몇으로 할지, 소비자를 어디 둘지 모두 사용자가 결정해야 한다. unbuffered channel에 goroutine이 `errCh <- err`를 보내고 main이 `wg.Wait()` 다음에 수신하려고 하면, 그 goroutine은 채널에 보내는 순간 블록되어 `wg.Done()`에 도달하지 못하고 교착 상태가 된다. 이 버그는 겉으로 드러나지 않고 `SIGQUIT` 덤프를 봐야만 보인다.

fan-out 패턴에서는 하나의 RPC가 실패했을 때 나머지 요청을 계속 보내 봐야 CPU와 네트워크만 낭비한다. 상위 context를 취소하면 이미 실행 중인 하위 작업들도 `ctx.Done()`을 받아 빠르게 탈출할 수 있다. WaitGroup은 실행 취소를 전혀 도와주지 않으므로, 작업 하나가 실패해도 나머지 수십 개가 전부 끝날 때까지 기다려야 한다.

초기 errgroup은 첫 번째 오류만 반환했다. 여러 오류를 보고 싶으면 `errors.Join`이나 별도 collection 로직을 써야 했다. 그래서 많은 팀이 errgroup 대신 수동 WaitGroup + channel 패턴을 고수했는데, 그 패턴은 매번 닫는 위치와 버퍼 크기를 실수하기 쉽다. SetLimit이 추가되면서 worker pool의 semaphore와 error propagation이 한 곳에 합쳐졌고, 이제는 굳이 직접 제한 채널을 만들 필요가 없어졌다.

참고로 `SetLimit`은 `int` 파라미터를 받는다. `SetLimit(1.20)`처럼 float 상수를 넣으면 컴파일되지 않는다. 아마 Go 1.20 버전 언급에서 혼동이 온 것일 수 있는데, 실제 API는 `SetLimit(n int)`이고 n은 최대 동시 실행 goroutine 수를 뜻한다.

## 어떻게 동작하는가

`NewGroup(ctx)`는 parent에서 `context.WithCancelCause`로 자식 ctx를 만들고 cancel cause function을 저장한다. parent는 절대 취소되지 않는다. 실행할 작업은 `g.ctx`를 받고, 첫 번째 오류가 나면 `g.cancel(err)`가 불린다. 이 구조 때문에 작업 입장에서는 `ctx.Done()`만 보면 되고, parent를 오염시키지 않는다.

`Group`은 `wg sync.WaitGroup`, `errOnce sync.Once`, `err error`, `sem chan struct{}`를 가진다. `SetLimit(n)`은 크기 n의 buffered channel을 만든다. 이 channel은 goroutine 실행 slot을 나타내는 token으로 쓰인다. `Go(f)` 안에서는 `select`로 `g.sem <- struct{}{}`를 먼저 시도한다. slot이 없으면 goroutine을 만들지 않고 그 자리에서 block한다. context가 이미 취소됐다면 조용히 return한다.

token을 얻은 뒤에만 `wg.Add(1)`를 부르고 goroutine을 시작한다. goroutine은 `defer g.wg.Done()`을 부르고, sem을 사용했다면 `defer`로 token을 하나 꺼낸다. f가 non-nil error를 반환하면 `errOnce.Do` 안에서 `g.err`에 첫 오류를 기록하고 `g.cancel(err)`을 호출한다. `errOnce` 때문에 첫 오류가 아니라 두 번째 이후의 오류는 context 취소를 다시 시도하지 않는다.

`Wait`는 `wg.Wait()`로 모든 goroutine이 끝나기를 기다린 뒤에 `g.cancel(g.err)`를 부른다. 성공해도 `g.err`가 nil이므로 `cancel(nil)`이 되어 derived context는 `context.Canceled` 원인으로 취소된다. 이 동작은 x/sync/errgroup과 동일하며, 성공 후에도 child ctx가 취소 상태라는 점을 모르면 다음 작업에서 이미 취소된 context를 재사용하게 된다.

x/sync/errgroup의 실제 source file은 `errgroup.go`이며, `Group` 구조체와 `Go`, `Wait`, `SetLimit`가 같은 방식으로 구현돼 있다. 최근 버전은 `errors.Join`을 사용해 여러 오류를 합칠 수 있게 확장됐지만, 기본 골격은 여전히 semaphore token과 `sync.Once`로 유지된다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사: atomic 다루기, context 반환, unreachable code 확인
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행: 취소 전파, SetLimit, 수동 WaitGroup 패턴 비교
go test -v ./...             # 테스트: 취소 전파와 limit 최대 동시성, 성공 시 context 취소 검증
go test -race ./...          # 동시성 race 검사: limit 테스트와 취소 테스트를 race detector로 확인
```

`go run .`에서는 세 블록이 출력된다. 첫 블록에서 `first error from group`과 `derived ctx after wait`가 같은 cause를 가리키는지 본다. 둘째 블록에서 `observed maxActive`가 정확히 2인지 본다. 셋째 블록에서 수동 WaitGroup + buffered channel 방식이 errgroup보다 얼마나 verbose한지 비교한다.

## 코드로 확인하기

`runCancellationDemo`는 worker one이 즉시 `errors.New("worker one failed")`를 반환하고, worker two는 `ctx.Done()`을 기다렸다가 `ctx.Err()`와 `context.Cause(ctx)`를 출력한다. 출력에서 `worker two stopped: context canceled, cause: worker one failed`가 나온다. `g.Wait()` 후에는 `derived ctx after wait: context canceled`와 `cause: worker one failed`가 찍힌다. parent는 취소되지 않았으므로 `<nil>`이 나온다.

`runLimitDemo`는 `SetLimit(2)`를 건 뒤 6개의 task를 돌린다. 각 task는 active를 1 늘리고 40ms 동안 slot을 잡고 있다가 active를 줄인다. 실행 결과 `observed maxActive: 2`가 된다. 만약 SetLimit을 빼면 maxActive가 6까지 튈 수 있다. 여기서 token은 goroutine 시작 전에 얻으므로, 실행 중인 goroutine 수만 제한하는 게 아니라 시작 속도 자체도 제한한다.

`runWaitGroupManualDemo`는 직접 `sync.WaitGroup`과 `chan error`를 써서 오류를 모은다. 채널은 꼭 buffered로 만든다. 주석에도 적었듯 unbuffered로 만들면 첫 번째 goroutine이 채널에 보내는 순간 블록되어 `wg.Done()`을 못 부르고 `wg.Wait()`가 풀리지 않는다. errgroup은 이 함정을 API 안에 가둬서 사용자가 채널 버퍼를 고민하지 않게 만든다.

`main_test.go`의 `TestGroupCancelsContextOnFirstError`는 첫 오류가 context cause로 기록되고 derived ctx가 취소되며 parent는 살아 있는지 검증한다. `TestGroupSetLimitKeepsMaxActive`는 시간 sleep 없이 channel과 semaphore만으로 active 수가 limit를 넘지 않는지를 결정적으로 확인한다. `TestGroupWaitCancelsEvenWithoutError`는 성공 후에도 child ctx가 취소된다는 흔한 함정을 검증한다.

## 모르면 겪는 일

unbuffered error channel과 `WaitGroup`을 같이 쓰면 교착 상태가 난다. CPU profile에는 `futex`에서 멈춘 goroutine만 보이고, 상위 로직은 아무 반응이 없다. `go test -race`로는 잡히지 않으므로 재현도 어렵다. 채널 버퍼를 여유 있게 주면 증상이 사라져서 원인을 오래 찾게 된다.

첫 오류 발생 후에도 context를 취소하지 않으면, 이미 실패한 fan-out 작업의 나머지 요청이 전부 완료될 때까지 기다린다. 예를 들어 100개 요청 중 하나가 1초에 실패했지만 나머지 요청이 30초씩 timeout 걸린다면, 전체 작업은 30초가 지나야 끝난다. p99가 외부 서비스의 worst case로 튀는데, CPU profile에는 아무것도 안 보이고 네트워크 대기만 보인다.

SetLimit 없이 수만 개 goroutine을 만들어 외부 API rate limit을 맞추려 하면 메모리와 file descriptor가 먼저 터진다. 반대로 SetLimit을 잘못 설정하면 limit 앞에서 goroutine이 block되어 context cancel 신호를 못 받을 수 있다. `Go` 안의 select에서 token과 ctx.Done을 함께 기다리므로, context가 이미 취소된 뒤에는 Go가 조용히 return한다. 호출자는 "시작했다"고 생각한 작업이 실제로는 실행 안 된 경우를 인지하지 못할 수 있다.

`Wait` 후에 inherited context가 살아 있다고 생각하고 재사용하면, 이미 취소된 context를 받아서 다음 작업이 전부 즉시 실패한다. x/sync/errgroup은 성공해도 child context를 취소한다. 이 특성을 모르고 같은 derived context를 여러 단계에서 쓰면 하위 단계가 전혀 시작되지 않는 이상한 버그가 된다.

## 언제 신경 쓰고 언제 무시하나

동시 작업 수가 수십 이상이고, 한 작업이 실패했을 때 전체를 조기 종료해야 할 때 errgroup이 빛을 발한다. 특히 외부 RPC, DB query, HTTP fan-out처럼 작업 하나가 오래 걸리고 실패 가능성이 높은 경우에는 첫 오류 취소가 p99 latency를 크게 줄여준다.

작업이 짧고 오류가 거의 없으며 goroutine 수가 몇 개라면 `WaitGroup` + buffered channel이 충분하다. errgroup도 결국 그 위에 얹은 것이므로, 이 규모에서는 API 간결함 외에는 큰 차이가 없다. 복잡한 오류 조합 없이 로그만 남기면 된다면 수동 패턴이 오히려 단순할 수 있다.

SetLimit은 외부 API 호출, DB connection pool, file open처럼 자원 제한이 명확할 때만 효과가 있다. CPU-bound 작업은 `GOMAXPROCS`가 이미 실행 병목을 제한하므로 SetLimit이 스케줄러를 더 도와주지 않는다. 오히려 limit로 인해 goroutine이 semaphore 앞에 대기하면서 컨텍스트 취소를 늦게 받을 수 있으니, I/O 대기 없이 CPU만 쓰는 작업에는 무시하는 편이 낫다.

정확한 cause 전파가 중요한 경우에만 `context.Cause`를 본다. 단지 첫 오류 메시지만 필요하면 `Wait()`가 반환하는 error를 그대로 쓰면 된다. `context.WithCancelCause`까지 써서 원인을 포장하는 것은 나중에 디버깅할 때 원인을 더 명확히 추적하고 싶을 때만 가치가 있다.

## 더 파보기

- [errgroup 공식 문서](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [x/sync errgroup.go source](https://cs.opensource.google/go/x/sync/+/master:errgroup/errgroup.go)
- [Go 블로그: Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [context.WithCancelCause 문서](https://pkg.go.dev/context#WithCancelCause)