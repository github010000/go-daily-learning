## 한 줄 요약

fan-out/fan-in pipeline에서 소비자가 먼저 필요를 마쳤을 때, 데이터 channel을 닫으려 하지 말고 context 또는 done channel로 취소를 전파해야 한다. channel은 송신자만 닫아야 하며, fan-in 단계에서는 여러 송신자가 끝났는지를 WaitGroup으로 확인한 뒤 한 번만 닫아야 한다.

## 왜 이런 설계인가

Go는 goroutine이 가벼워서 stage마다 goroutine을 여러 개 띄우는 fan-out/fan-in 구조를 쉽게 만든다. 그런데 이 가벼움 때문에 전통적인 스레드 모델보다 block이 덜 두드러진다. OS 스레드는 수천 개가 동시에 block되면 스레드 생성 실패나 자원 고갈로 곧바로 드러나지만, goroutine은 수만 개가 channel send에서 대기해도 프로세스는 조용히 계속 돌아간다. 이 주제가 어려운 이유는 실패가 즉시 panic이나 에러로 오지 않고, 고루틴 개수와 메모리만 천천히 올라가는 형태로 오기 때문이다.

파이프라인에서 소비자가 먼저 죽는 상황은 특히 위험하다. 생산자와 worker는 upstream에서 열심히 값을 보내고 있는데, 최종 소비자가 target을 찾았다거나 에러가 났다거나 deadline이 지났다는 이유로 `return` 해버리면, 아무도 읽지 않는 unbuffered channel에 `send`하려는 goroutine들이 영원히 대기한다. unbuffered channel은 receiver와 sender가 동시에 만나야 값을 전달하기 때문에, receiver가 사라진 뒤의 sender는 block에서 빠져나올 방법이 없다.

이 문제를 해결하기 위해 channel을 닫는 방법을 생각해 볼 수 있지만, 이는 잘못된 접근이다. Go의 channel은 송신자만 닫아야 한다는 원칙이 있다. 소비자가 받기 전용 channel `<-chan int`를 닫을 수도 없고, 설사 닫을 수 있다고 해도 fan-in 구조에서는 여러 worker가 같은 channel에 send하기 때문에 `send on closed channel` panic이 난다. 또한 close는 취소 신호가 아니라 "더 이상 보낼 값이 없다"는 신호다. 따라서 데이터 경로와 취소 경로를 분리해야 한다.

취소 경로로는 context와 done channel이 있다. context는 deadline, cancellation reason, parent-child 전파가 필요할 때 쓴다. done channel은 그런 정보 없이 지역적으로 stop만 알릴 때 쓴다. 둘 다 goroutine에 `select`의 case로 넣어서 channel send를 기다리는 동안에도 취소를 관찰할 수 있게 만든다. 만약 done channel만으로 외부 API에 deadline을 전달하려 들면 context가 가진 `Deadline()`과 `Err()`을 다시 흉내 내야 하는 지저분한 코드가 생긴다. 반대로 context 값 전달이 필요 없는 내부 정지 신호에 context를 쓰면 코드 전반에 불필요한 인터페이스와 cancel 함수가 번진다.

## 어떻게 동작하는가

이 코드의 핵심은 모든 channel에 소유자가 하나씩 있다는 것이다. `Produce`는 `out`을 만들고, 그 `out`에 send하는 유일한 goroutine을 실행하며, 그 goroutine이 끝날 때 `defer close(out)`으로 닫는다. `Square`도 마찬가지로 자신의 `out`을 만들고, 그 자신만 send하며, 끝날 때 닫는다. `Merge`는 fan-in으로 여러 worker가 send하는 공동 channel `out`을 만든다. 따라서 아무 worker도 `out`을 닫으면 안 되고, 모든 worker가 끝났음을 아는 조정자만 `out`을 닫아야 한다.

`Merge`가 모든 worker의 종료를 아는 방법은 `sync.WaitGroup`이다. `wg.Add(len(cs))`로 worker 개수를 등록하고, 각 output goroutine이 `defer wg.Done()`을 실행하도록 한다. 별도의 goroutine이 `wg.Wait()` 후 `close(out)`을 호출한다. 이렇게 하면 여러 goroutine이 같은 channel을 닫는 `close of closed channel` panic을 피하면서도, 모든 송신자가 끝난 뒤 channel이 닫힌다.

context 취소는 `context/context.go`의 `cancelCtx`에서 `Done()`이 돌려주는 channel을 닫는 방식으로 동작한다. `cancel()`은 `done` channel을 한 번 닫고, 그 context의 자식들에게도 취소를 전파한다. `ctx.Err()`는 취소 이유를 돌려준다. 이 구조 때문에 여러 goroutine이 `<-ctx.Done()`을 기다리고 있어도 안전하게 broadcast된다. done channel은 이보다 단순해서 close 한 번이 곧 stop 신호다.

`Produce`와 `Square`, `Merge`가 모두 `select`를 쓰는 이유는 취소가 어느 순간에 도착해도 send와 receive를 하나의 결정으로 만들기 위해서다. 예를 들어 `Produce`가 `out <- v`에서 block하고 있을 때 `cancel()`이 호출되면, select는 `ctx.Done()` case를 선택해 그 자리에서 빠져나올 수 있다. `Square`는 receive 단계와 send 단계가 분리되어 있기 때문에 select가 두 번 필요하다. 첫 select는 `in`에서 값을 받다가 취소를 보고 빠져나오는 단계이고, 두 번째 select는 받은 `v`를 downstream에 보내다가 취소가 오면 빠져나오는 단계다. 두 번째 select가 없으면 worker가 값을 받은 직후 downstream receiver가 사라졌을 때 그대로 block된다.

주의할 점은 `select`에서 여러 case가 동시에 준비되면 Go가 그중 하나를 의사 난수로 선택한다는 것이다. 즉 `ctx.Done()`이 준비되어 있고 동시에 `out <- v`의 receiver도 준비되어 있다면, 취소가 이미 일어났어도 send가 선택될 수 있다. 따라서 cancel 후 "값이 절대로 하나도 더 오지 않는다"고 단정하면 안 된다. 우리가 보장할 수 있는 것은 "취소를 관찰한 goroutine은 더 이상 send를 시작하지 않으며, 결국 pipeline이 종료된다"는 것뿐이다. `Produce`가 시작 전에 `ctx.Err()`를 확인하는 것은 이미 취소된 상태에서 goroutine을 만들지 않도록 하는 최적화이자 결정적 동작을 위한 장치다.

## 돌려보기

이 디렉토리에서 다음을 순서대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사: 잘못된 API 사용이나 fmt 문제 확인
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 잘못된 패턴과 올바른 패턴의 goroutine 수 차이 확인
go test -v ./...             # 테스트 3개가 순서대로 통과하는지 확인
go test -count=10 ./...      # 반복 실행으로 select 난수 선택 때문에 깨지는 테스트가 없는지 확인
go test -race ./...          # 동시성 race 검사, concurrency 주제이므로 반드시
GOMAXPROCS=4 go run .        # CPU 4개에서도 잘못된 패턴의 leak와 올바른 패턴의 정리 확인
```

`go run .`에서는 `wrong` 항목의 goroutine 수가 `base`보다 높게 찍히는 것을 봐야 한다. `right cleanup done`에서 goroutine 수가 다시 `base` 근처로 내려가면 cancel이 정상적으로 fan-out/fan-in 전체에 전파된 것이다. `go test -race`가 통과해야 `context` 취소와 channel send/close 사이에 데이터 경쟁이 없다고 말할 수 있다.

## 코드로 확인하기

`main.go`는 잘못된 소비자와 올바른 소비자를 나란히 실행한다. 먼저 `WrongFindFirstLeak`를 사용한다. 이 함수는 `target`인 16을 찾으면 cancel 없이 반환한다. 그 시점에 upstream의 `Square` worker 세 개와 `Merge`의 output goroutine들은 아직 보낼 값이 남아 있고, 최종 소비자가 사라졌기 때문에 unbuffered channel send에서 block된다. `runtime.NumGoroutine()`을 찍으면 base보다 몇 개 늘어난 수가 보인다. 정확한 숫자는 스케줄러가 어느 goroutine을 먼저 실행했는지에 따라 달라지지만, base보다 높게 나오는 것은 항상 같다. 그 후 `cancelWrong()`을 호출하고 `outWrong`을 drain하면 close가 되어 다시 base 근처로 내려간다.

다음 `FindFirst`는 target을 찾으면 `cancel()`을 호출한다. cancel은 `Produce`, `Square`, `Merge`의 모든 `select`에서 `ctx.Done()` case를 준비시킨다. 이후 `for range outRight`가 끝나는 것은 모든 upstream goroutine이 취소를 관찰하고 `Merge`가 `sync.WaitGroup`을 통과해 `close(outRight)`을 호출했다는 뜻이다. `immediate goroutines` 수는 아직 일부 goroutine이 빠져나가는 중이라 base보다 잠깐 높을 수 있지만, drain이 끝나면 base 근처로 돌아온다.

`ProduceWithDone`은 취소 신호로 `done channel`을 사용하는 변형이다. main에서 done을 미리 `close`해 두고 호출하면 `ProduceWithDone`은 goroutine을 만들지 않고 이미 닫힌 out을 돌려준다. 그래서 `pre-closed done produced: []`가 출력된다. 이는 context가 해주는 `Err()`나 deadline 전파가 필요 없을 때 done channel만으로도 같은 취소 전파를 만들 수 있음을 보여준다.

테스트는 세 가지를 검증한다. `TestProducerStopsWhenContextCanceled`는 cancel이 Produce보다 먼저 일어난 경로에서 값이 한 개도 새지 않는지를 본다. 이전 구현에서는 goroutine을 무조건 띄운 뒤 select를 돌렸기 때문에 receiver가 준비되면 `out <- v`가 이겨서 10 같은 값이 나갈 수 있었다. `TestFindFirstCancelsContext`는 cancel이 없으면 `for range merged`가 hang 하는 구조로 만들어서 close 여부를 검증한다. `TestMergeFansInAllValues`는 worker 세 개가 fan-in하더라도 입력 1,2,3,4,5에 대해 제곱 1,4,9,16,25가 정확히 한 번씩만 나오는지 확인한다.

## 모르면 겪는 일

이 주제를 모르는 상태에서 소비자가 먼저 return 하는 fan-out/fan-in을 짜면 가장 흔한 증상은 goroutine 수가 계속 증가하는 것이다. CPU 프로파일에는 아무것도 안 보이는데 메모리는 계속 올라가고, goroutine profile을 떠 보면 `chan send`에서 block된 goroutine이 수백, 수천 개 보인다. 이들은 GC되지 않는다. goroutine은 stack을 몇 KB씩 차지하고, block된 채 channel과 컨텍스트를 물고 있으므로 관련 객체도 해제되지 못한다.

테스트 코드에서도 증상이 나타난다. `for range merged`가 어떤 순간부터 끝나지 않아 테스트가 hang 하고, CI에서는 timeout으로 찍힌다. 시간 제한을 두지 않는 테스트라면 로컬에서 몇 분씩 멈춰 있다가 사람이 수동으로 중단하게 된다. 반대로 `Merge`에서 worker마다 `close(out)`을 하도록 잘못 짜면 `close of closed channel` panic이 발생한다. 이 panic은 실행 순서에 따라 나왔다 안 나왔다 할 수 있어 디버깅하기 어렵다.

HTTP 서버 같은 long-running 서비스라면 요청별로 pipeline을 만들고 클라이언트가 먼저 끊을 때 내부 pipeline이 취소되지 않으면, 커넥션은 이미 끊겼는데 내부 worker들은 계속 결과를 만들려고 대기한다. 겉으로 보기에는 p99 latency가 GC 주기나 CPU 사용률과 무관하게 뛰는 것처럼 보이지만, 실제로는 goroutine leak로 인한 scheduler 부담과 메모리 증가가 원인일 수 있다. CPU 프로파일에는 나타나지 않기 때문에 goroutine profile을 함께 봐야 원인이 보인다.

또 하나 조심할 것은 cancel 이후에도 값이 몇 개 더 도착할 수 있다는 사실이다. 소비자가 cancel을 호출한 직후에도 이미 select 단계에서 send를 선택한 goroutine이 있을 수 있다. 따라서 cancel 후에는 "더 이상 값이 없을 것"이라고 가정하고 side effect를 중복 수행하지 않도록, cancel 후 수신한 값은 무시하거나 idempotent하게 처리해야 한다. 이 한계를 이해하지 못하면 드물게 중복 주문이나 중복 집계 같은 버그가 생긴다.

## 언제 신경 쓰고 언제 무시하나

이 지식은 long-running 서비스, 요청별 pipeline, 동적으로 크기가 변하는 fan-out에서 중요하다. 특히 소비자가 항상 전체 channel을 끝까지 읽는 구조가 아니라 `break`, `return`, `if 조건`으로 중간에 빠져나올 수 있다면 반드시 cancel 또는 done을 설계해야 한다. 외부 API에 deadline이나 cancellation reason을 전달해야 한다면 done channel이 아니라 context를 써야 한다.

반면 한 번 실행되고 곧 종료되는 CLI 도구라면 goroutine leak가 있어도 프로세스가 끝나면서 OS가 모든 자원을 회수하므로 실질적인 문제가 되지 않을 수 있다. fan-out이 아주 작고 소비자가 항상 끝까지 drain하며, early return 경로가 없다면 복잡한 context 전파를 넣지 않는 것이 더 읽기 좋다. 이 경우에도 테스트에서 `for range`가 닫히는지 정도는 확인해 두는 것이 좋다.

최적화 측면에서도 "모든 pipeline에 무조건 context를 넣자"는 과한 선택이 될 수 있다. 규모가 작은 배치 작업이나, 프로그램 시작 시 한 번만 도는 pipeline에서는 goroutine 하나가 block되는 것보다 코드가 단순한 것이 더 낫다. 하지만 goroutine은 프로세스가 살아 있는 동안 계속 남으므로, "지금은 괜찮다"와 "영원히 괜찮다"는 다르다. 요청 수가 늘어나면 그동안 무시했던 leak가 갑자기 튀어나온다.

## 더 파보기

- Go 블로그 Pipeline 패턴: https://go.dev/blog/pipelines
- context 패키지 문서: https://pkg.go.dev/context
- Go 언어 명세의 close와 receive 규칙: https://go.dev/ref/spec#Close
- Go 런타임 channel 구현 `runtime/chan.go`: https://github.com/golang/go/blob/master/src/runtime/chan.go
- context 패키지 소스 `context/context.go`: https://github.com/golang/go/blob/master/src/context/context.go