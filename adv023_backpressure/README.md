## 한 줄 요약

큐 깊이는 마음대로 정하는 것이 아니라 `L = λW` 로 추정해야 하며, 무한 버퍼는 지연을 숨기고 결국 메모리를 태우기 때문에 bounded queue, drop, load shedding 중 하나로 배압을 만들어야 한다.

## 왜 이런 설계인가

큐는 producer 와 consumer 의 속도 차이를 흡수하기 위해 존재한다. OS thread 기반 서버에서는 스레드 생성 비용이 커서 요청 수를 제한하거나 스레드 풀을 써야 했다. goroutine 은 스레드보다 훨씬 가벼워서 producer 와 consumer 를 수십만 개 띄울 수 있지만, 이 사실이 "큐를 무한히 쌓아도 된다"는 의미는 아니다. producer 가 초당 1000개를 넣고 consumer 가 초당 800개를 처리한다면, 큐를 아무리 크게 잡아도 시간이 지나면 큐는 계속 증가한다. 큐는 처리 능력 자체를 늘리지 않는다.

Go 의 channel 은 `make(chan T, cap)` 으로 용량을 정한 뒤 절대 늘지 않는다. 이것은 우연한 선택이 아니라 메모리 유한성과 배압을 강제하기 위한 설계다. `runtime/chan.go` 의 `hchan` 구조체는 `dataqsiz` 라는 고정 용량과 `qcount` 라는 현재 원소 수를 따로 가진다. `qcount == dataqsiz` 가 되면 sender 는 `sendq` 에서 대기하거나, select default 가 있으면 즉시 포기한다. 이렇게 해서 overload 상태가 producer 에게 되돌아간다.

무한 버퍼는 이 고리를 끊는다. producer 는 append 만 하면 되므로 절대 기다리지 않는다. 문제는 consumer 가 느리다는 사실이 latency 로 드러나지 않고 큐 깊이로만 쌓인다는 점이다. 나중에 consumer 가 정상화되면 그동안 쌓인 오래된 요청을 처리하느라 p99 latency 가 수백 밀리초에서 수 초로 튄다. 이때 CPU 프로파일에는 consumer 의 병목만 보이고 producer 는 그냥 append 만 했기 때문에 원인이 잘 안 보인다.

Little's Law 는 이 큐 깊이를 정량화한다. `L = λW` 에서 `λ` 는 초당 도착률, `W` 는 요청이 시스템에 머무는 시간(큐 대기 + 서비스 시간)이다. 1000 rps 에 W 가 20ms 라면 평균 20개가 시스템에 있어야 한다. 목표 p99 latency 가 50ms 라면 W 를 50ms 로 보고 큐 깊이를 50개 아래로 잡아야 한다. 그래서 큐 깊이는 "넉넉하게 10000"이 아니라 `λW` 로 계산해서 정해야 한다.

선택지는 크게 세 가지다. 첫째, bounded blocking queue 는 데이터를 버리지 않지만 큐가 차면 producer goroutine 이 park 되어 응답 지연이 늘어난다. 둘째, drop queue 는 큐가 찼을 때 요청을 즉시 버려서 메모리와 지연을 제한하지만 데이터 손실을 감수한다. 셋째, load shedding 은 큐 깊이나 추정 대기 시간을 보고 아예 입구에서 거절한다. drop 은 load shedding 의 단순한 구현이고, load shedding 은 Little's Law 를 런타임에 적용한 것이다.

## 어떻게 동작하는가

Go channel 의 내부 구조는 `runtime/chan.go` 에 있다. `hchan` 은 `qcount`(현재 버퍼에 들어 있는 원소 수), `dataqsiz`(`make` 로 정한 버퍼 총량), `buf`(실제 원소 저장 공간), `sendq`(송신 대기 sudog 리스트), `recvq`(수신 대기 sudog 리스트)를 가진다. sender 가 `ch <- v` 를 실행하면 `runtime.chansend` 는 먼저 `qcount < dataqsiz` 인지 확인하고, 여유가 있으면 `buf` 에 넣고 끝낸다. 여유가 없으면 `block` 플래그에 따라 두 갈래로 나뉜다. 일반 send 는 `gopark` 로 자신을 park 시킨 뒤 `sendq` 에 넣고, select default 가 있는 `select { case ch <- v: default: }` 는 park 하지 않고 즉시 default 분기를 실행한다.

이 default 분기가 바로 drop 이다. `DropQueue.TryPush` 는 `select` 로 send 를 시도하되 full 이면 `default` 에서 `dropped` 를 증가시킨다. 이 방식은 별도의 lock 없이 channel 자체가 제공하는 원자적 send-or-default 를 사용하므로 race 없이 부하 차단을 구현할 수 있다. 반면 `BlockingQueue.Push` 는 `q.ch <- v` 만 실행하므로 큐가 차 있으면 goroutine 이 park 된다. park 된 goroutine 은 consumer 가 값을 꺼낼 때까지 `sendq` 에 남아 있으며, 빈 자리가 생기면 `recv` 쪽에서 깨워서 send 를 재개한다.

무한 버퍼는 보통 `sync.Mutex` 로 보호된 slice 로 만든다. `Push` 는 `append` 만 하고, `Pop` 은 첫 원소를 꺼낸다. 이 구조에는 `dataqsiz` 같은 상한이 없고 park 도 없기 때문에 producer 는 절대 멈추지 않는다. 큐가 커지면 slice 의 용량이 두 배씩 증가하면서 allocation 이 발생하고, 원소가 포인터라면 GC 가 모든 객체를 mark 해야 한다. 따라서 CPU 프로파일에는 producer 의 `append` 만 보이고, 메모리 프로파일에는 `[]*Request` 가 가득 찬 모습이 보인다.

본 코드는 이 세 가지를 `UnboundedQueue`, `DropQueue`, `BlockingQueue` 로 구현했다. `unboundedMemoryDemo` 는 100만 개를 push 하면서 `runtime.MemStats.HeapAlloc` 을 측정한다. `runDropScenario` 는 cap 30, 작업 300개, 서비스 시간 2ms 로 consumer 를 돌리며 producer 가 모든 요청을 시도한다. drop 은 용량 30을 넘는 순간 남은 요청을 버리므로 elapsed 가 짧다. blocking 은 같은 조건에서 producer 가 30개 지점에서 park 되기 시작해 모든 요청을 다 넣을 때까지 약 600ms 가 걸린다.

큐 깊이를 정할 때 `littleQueueDepth` 는 도착률과 허용 latency 를 받아 `ceil(λ * W)` 를 반환한다. 예를 들어 `littleQueueDepth(1200, 25*time.Millisecond)` 는 30을 반환한다. 이 30은 평균적으로 버퍼에 있어야 하는 원소 수이지 정확한 상한은 아니다. 트래픽 burst 가 있다면 그 burst 크기를 더해야 하고, consumer 가 일시적으로 멈출 수 있다면 그 멈춤 시간도 W 에 포함해야 한다. 공식은 완벽한 정답이 아니라 "최소한 이 정도는 버퍼로 잡아야 한다"는 출발점이다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령은 아래와 같다.

```bash
go vet ./...                 # 정적 검사: 사용하지 않는 import, 잘못된 복사 등을 잡는다
go build -o /dev/null ./...  # 컴파일 확인: main.go 와 main_test.go 모두 빌드된다
go run .                     # 시연 실행: Little's Law, 무한 버퍼 메모리, drop vs blocking 을 출력한다
go test -v ./...             # 테스트: 큐 깊이 수식, FIFO, drop 불변식, blocking 보존을 검증한다
go test -race ./...          # 동시성 주제이므로 race detector 로 channel/cond/atomic 사용을 확인한다
GODEBUG=gctrace=1 go run .   # GC trace 로 무한 버퍼가 만드는 힙 증가를 관찰한다
```

`go run .` 에서는 먼저 Little's Law 계산 결과가 나오고, 이어서 100만 개 push 후 `HeapAlloc` 증가량이 출력된다. `DROP` 과 `BLOCK` 시나리오에서는 accepted, dropped, elapsed 가 출력된다. `go test -v ./...` 에서는 `TestLittleQueueDepthMatchesLittleLaw` 부터 `TestBlockingQueuePreservesAllWork` 까지 순서대로 실행 결과가 찍힌다. `-race` 는 이 테스트 전체를 race detector 로 검사하므로 goroutine 이 있는 `TestBlockingQueuePreservesAllWork` 도 커버한다.

## 코드로 확인하기

`main.go` 의 `littleQueueDepth(1200, 25*time.Millisecond)` 는 `1200 * 0.025 = 30` 이므로 30을 출력한다. 이 값은 "초당 1200개가 들어오고 하나가 25ms 머문다면 평균 30개가 in-flight 상태"라는 의미다. 큐를 30보다 작게 만들면 burst 가 없어도 원하는 latency 를 지키기 어렵다.

`unboundedMemoryDemo(1_000_000)` 는 consumer 를 전혀 돌리지 않고 100만 개를 push 한다. 출력의 `queue_depth=1000000` 은 producer 가 한 번도 멈추지 않았다는 뜻이다. `heap_alloc before` 와 `after` 의 delta 는 대략 8 MiB 정도로 보이는데, 100만 개 int 가 8바이트씩이므로 순수 데이터만 8 MB 이다. slice append 과정에서 capacity 가 두 배로 늘어나면 더 많은 할당이 생기므로 delta 는 8 MiB 보다 클 수 있다. 핵심은 요청 수가 늘어날수록 메모리가 선형으로, 아니 append 증분 때문에 계단형으로 늘어난다는 점이다.

`runDropScenario(300, 30, 2*time.Millisecond)` 는 cap 이 30이므로 대부분의 요청이 즉시 drop 된다. 출력은 보통 `accepted=30 dropped=270 elapsed=...` 근처가 나온다. consumer 가 동시에 돌기 때문에 scheduler 타이밍에 따라 accepted 가 30보다 약간 클 수는 있지만, queue `Len()` 은 30을 넘지 않는다. 이는 큐 깊이가 bounded 라는 불변식을 그대로 보여준다. 반면 `runBlockingScenario(300, 30, 2*time.Millisecond)` 는 `accepted=300` 을 출력하고 elapsed 는 약 `600ms` 가량 나온다. 모든 일을 처리했으므로 데이터 손실은 없지만, 그 대가로 producer latency 가 30개 cap 지점부터 계속 증가한다.

`main_test.go` 는 시간 의존 단정 없이 불변식만 검증한다. `TestUnboundedQueueGrowsWithoutBackpressure` 는 10000개 push 후 `Len` 이 정확히 10000이고 pop 순서가 FIFO 임을 확인한다. `TestDropQueueBoundsQueueDepth` 는 capacity 16에 100개를 넣으면 accepted 가 정확히 16이고 dropped 가 84인 것, 그리고 pop 순서가 0..15 인 것을 검증한다. `TestBlockingQueuePreservesAllWork` 는 capacity 2에 3개를 push 하되 세 번째 push goroutine 이 두 번째 pop 이후 완료되는 순서를 확인해 blocking 이 데이터를 버리지 않음을 보여준다.

## 모르면 겪는 일

이 디자인을 모르고 무한 버퍼를 쓰면, consumer 가 잠깐 느려지는 순간 메모리가 선형으로 올라가고 결국 OOM kill 을 당한다. 증상은 메모리 그래프가 천천히 올라가다가 갑자기 떨어지는 triangular wave 가 되고, heap profile 에는 `[]*Request` 같은 큰 객체밖에 안 보인다. CPU profile 에는 appender 만 보이고 실제 병목인 느린 consumer 는 겉으로 티가 안 난다.

blocking bounded queue 를 썼는데 drop 이 없으면, 큐가 찬 순간부터 producer goroutine 이 `runtime/chan.go` 의 `sendq` 에 쌓인다. HTTP 서버라면 request handler goroutine 이 모두 send 에서 park 되어 accept loop 까지 멈춘다. 새 커넥션은 accept 되지 못하고 기존 요청은 아무도 consume 하지 못하는 deadlock 모양이 된다. 이때 p99 latency 는 치솟는데 goroutine 프로파일을 보지 않으면 reason 이 안 보인다.

drop queue 를 쓰면서도 요청이 멱등인지 확인하지 않으면, client 재시도와 겹쳐서 같은 일이 여러 번 실행된다. 예를 들어 결제 요청을 멱등 키 없이 drop 했다가 client 가 retry 를 보내면 이중 결제가 될 수 있다. drop 은 telemetry, 로그, 캐시워밍, 멱등 요청처럼 버려도 되는 일에만 써야 한다.

이 모든 문제는 큐 깊이를 추정하지 않아서 생긴다. Little's Law 를 몰라도 경험적으로 1000개 버퍼를 만들기도 하는데, 서비스 시간이 10ms 고 도착률이 2000 rps 라면 큐는 평균 20개만 있으면 된다. 1000개 버퍼는 p99 latency 를 500ms 로 만들어도 도착률이 2000이고 서비스 시간이 10ms 라면 큐 깊이는 20이므로, 실제로는 980개를 더 받을 시간 여유를 만들어 주는 것이 아니라 그만큼 오래 묵힌 요청을 만들 뿐이다.

## 언제 신경 쓰고 언제 무시하나

로컬 CLI 도구나 배치 작업처럼 입력 크기가 고정되어 있고 consumer 가 producer 와 같은 프로세스에서 금방 따라잡는다면 unbounded queue 를 써도 무방하다. 어차피 전체 데이터를 메모리에 들고 있어야 하는 작업이라면 `[]string` 에 append 하는 것과 다르지 않다. 이 경우 오히려 bounded channel + blocking 은 파이프라인이 교착할 가능성을 만들어 복잡도만 올린다.

초당 수십 개 이하의 낮은 요청률, consumer 서비스 시간이 수 ms 로 짧고 트래픽이 예측 가능한 internal 서비스도 무한 버퍼가 큰 문제를 만들지 않는다. GC 가 100만 개 객체를 mark 하지 않는 한 메모리 증가가 무시할 수준이기 때문이다. 다만 이때도 "오늘은 10 rps" 라고 무한 버퍼를 썼다가 이벤트로 5000 rps 가 몰리면 바로 OOM 이 온다는 사실은 기억해야 한다.

반대로 공용 서비스, cloud cgroup memory limit 이 있는 컨테이너, p99 SLO 가 있는 API, 느린 DB 나 external HTTP 를 consumer 로 두는 경우에는 반드시 bounded queue 와 drop/load shedding 중 하나를 골라야 한다. 먼저 Little's Law 로 필요한 깊이를 추정하고, 그 위에 burst tolerance 를 더한 capacity 를 정한다. 그 다음 모든 큐에 `Len`, `Dropped` 같은 메트릭을 붙여서 queue depth 가 capacity 근처에 있는지 관찰해야 한다.

하지만 아직 트래픽이 작고 요구사항이 명확하지 않은 초기 단계에서 load shedder 를 미리 다 설계하는 것은 과최적화다. drop 은 어디서 언제 데이터를 버렸는지, client 가 그걸 어떻게 처리하는지까지 알아야 제대로 쓸 수 있다. 규모가 작을 때는 일단 bounded channel 과 blocking 으로 시작하고, p99 latency 가 예산을 넘거나 queue depth 가 지속적으로 capacity 에 붙는 순간 drop/load shedding 을 도입하는 것이 안전하다. 최적화는 문제가 관찰된 뒤에 하는 것이지, 추측으로 시스템을 복잡하게 만드는 것이 아니다.

## 더 파보기

- [Go runtime channel source](https://go.dev/src/runtime/chan.go): `hchan`, `sendq`, `recvq`, `chansend` 의 실제 구현
- [Little's Law](https://en.wikipedia.org/wiki/Little%27s_law): 큐 깊이, 도착률, 체류 시간의 관계
- [Backpressure](https://en.wikipedia.org/wiki/Backpressure): producer-consumer 시스템에서 흐름을 제어하는 일반 개념
- [Go blog: pipelines and cancellation](https://go.dev/blog/pipelines): channel 로 파이프라인을 구성할 때 bounded queue 와 배압이 왜 필요한지
- [runtime package docs](https://pkg.go.dev/runtime): `MemStats` 와 `ReadMemStats` 로 메모리 관찰하는 방법