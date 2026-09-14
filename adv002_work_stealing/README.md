## 한 줄 요약

Go 런타임은 각 P마다 지역 runq와 별도의 runnext 슬롯을 두어, 방금 실행 가능해진 goroutine을 같은 P에서 즉시 실행시켜 cache locality와 wakeup 지연을 줄인다. P가 할 일이 없으면 다른 P의 runq에서 절반을 훔쳐 오는데, 이때 head 쪽에서 훔쳐 최근 작업은 원래 P에 남긴다.

이 글은 runtime/proc.go의 work stealing과 runnext가 왜 이렇게 설계됐는지, 실제 런타임이 어떤 순서로 훔치는지, 그리고 이 구조가 없으면 어떤 지연이 생기는지를 관찰 가능한 Go 코드와 함께 설명한다.

## 왜 이런 설계인가

Go는 OS 스레드 하나에 goroutine 여러 개를 얹는 M:N 스케줄러를 쓴다. OS 스레드는 생성·문맥 전환 비용이 크고 커널 자원을 차지하므로, goroutine마다 OS 스레드를 하나씩 만들 수는 없었다. 그래서 여러 goroutine을 소수의 OS 스레드에 multiplexing해야 했고, 이때 핵심은 "어느 goroutine을 어느 스레드에서 언제 실행할 것인가"를 빠르게 정하는 것이다.

초기 Go 스케줄러에는 전역 run queue 하나가 있었다. 모든 P가 그곳에서 작업을 꺼내다 보니 CPU 코어가 늘어날수록 전역 큐에 lock 경합이 심해졌다. 이를 해결하기 위해 P마다 지역 runq를 두었고, 지역 runq는 대부분 lock 없이 동작하게 만들었다. 하지만 지역 runq만 두면 어떤 P는 일이 많고 다른 P는 놀 수 있다. 이 불균형을 해결하는 메커니즘이 work stealing이다. P가 자기 runq를 비우면 다른 P의 runq에서 작업을 훔쳐 온다.

그런데 지역 runq를 단순 FIFO로만 두면 "방금 실행 가능해진 goroutine"이 불리해진다. 예를 들어 goroutine A가 channel에 값을 보내서 goroutine B를 깨웠다면, B는 A와 같은 P에서 실행되는 것이 cache hit에 유리하다. 하지만 FIFO runq라면 B는 tail에 붙어 앞에 쌓인 오래된 작업들이 모두 처리된 뒤에야 실행된다. 이 문제를 풀기 위해 runnext라는 단일 슬롯을 runq와 분리한 것이다.

runnext는 전체를 LIFO로 바꾸지 않고 딱 한 개만 최근 작업에 우선권을 준다. 전체 LIFO면 오래된 작업이 계속 밀려 starvation이 생길 수 있고, FIFO면 wakeup locality가 나빠진다. runnext는 이 둘 사이의 절충이다. 방금 만들어진 작업은 자기 P에서 즉시 실행되고, 나머지 오래된 작업은 runq에서 FIFO 순서를 유지하며 다른 P가 훔쳐 갈 수 있게 한다.

work stealing의 방향도 중요하다. P가 다른 P의 runq에서 작업을 훔칠 때 tail이 아니라 head에서 훔친다. head에서 훔치면 오래된 작업이 이동하고, 최근에 만들어진 cache-hot한 작업은 원래 P에 남는다. tail에서 훔치면 방금 만든 goroutine을 다른 P로 보내 cache locality를 파괴한다. 이렇게 head steal과 runnext는 서로 맞물려 locality와 fairness를 동시에 확보한다.

## 어떻게 동작하는가

Go 런타임의 핵심 자료구조는 runtime/runtime2.go의 `type p struct`에 있다. P는 `runq [256]guintptr`, `runqhead uint32`, `runqtail uint32`, `runnext guintptr` 필드를 가진다. `runq`는 256개짜리 원형 큐이고, `runqhead`와 `runqtail`은 lock-free 소비·생산을 위한 인덱스다. `runnext`는 runq보다 먼저 확인하는 별도 슬롯이다.

goroutine이 실행 가능해지면 runtime/proc.go의 `runqput`이 호출된다. `runqput`의 두 번째 인자 `next`가 true이면 runnext 슬롯에 넣는다. 이미 runnext에 값이 있으면 기존 값을 runq tail에 밀어 넣고 새 goroutine을 runnext에 둔다. runq가 가득 차면 `runqputslow`가 전역 runq로 보낸다. `next`는 channel send, mutex unlock, 새 goroutine 생성처럼 현재 goroutine이 방금 다른 goroutine을 깨운 경우 주로 true다.

P가 다음 goroutine을 고를 때 `runqget`을 호출한다. `runqget`은 먼저 `runnext`를 확인해 값이 있으면 CAS로 nil로 바꾸고 반환한다. runnext가 없으면 `runqhead`와 `runqtail`을 읽어 runq에서 하나 꺼낸다. 즉 실행 순서는 runnext가 runq보다 항상 우선한다. 이 때문에 방금 깨어난 goroutine은 runq의 오래된 작업들을 제치고 즉시 실행될 수 있다.

P가 자기 runq와 runnext를 모두 비우면 `findRunnable`은 먼저 전역 runq, netpoll, timer 등을 확인한 뒤 `stealWork`를 호출한다. `stealWork`는 최대 4번 시도하고, 각 시도마다 `stealOrder`라는 랜덤 시작 순서로 모든 P를 순회한다. 희생 P에 작업이 있으면 `runqsteal`이 불린다. `runqsteal`은 `runqgrab`으로 희생 P의 runq에서 `n = t - h; n = n - n/2` 공식대로 올림 절반을 가져온다. 훔친 batch 중 하나는 즉시 실행하고 나머지는 도둑 P의 runq tail에 붙인다.

`runqgrab`은 `stealRunNextG`가 true인 마지막 시도에서만 runq가 비어 있을 경우 runnext를 훔칠 수 있다. runnext에 담긴 가장 최근의 goroutine을 다른 P로 보내면 locality가 깨지므로, 정말 다른 일이 없을 때만 최후의 수단으로 훔치는 것이다. 이 모든 과정은 runtime/proc.go의 `findRunnable`, `stealWork`, `runqsteal`, `runqgrab`에 실제 코드로 존재한다.

## 돌려보기

```bash
go vet ./...                 # 시뮬레이션 코드와 테스트의 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행: FIFO vs runnext의 hot task tick 차이를 본다
go test -v ./...             # 테스트: runnext 불변식, 도둑질 배치, 지연 단축 검증
go test -race ./...          # 동시성 이슈가 없음을 -race로 확인
GODEBUG=schedtrace=1000 go run .  # 실제 런타임 스케줄러 trace와 함께 실행
```

`go run .`에서는 FIFO 모드에서 hot task가 tick=5에 실행되고 runnext 모드에서는 tick=1에 실행되는 로그가 나온다. `GODEBUG=schedtrace=1000` 명령은 우리 모델 출력 뒤에 Go 런타임의 `SCHED` trace를 주기적으로 출력한다. 이 데모는 시뮬레이션이라 P가 대부분 idle이지만, `P` 상태와 `IDLE` 표시가 어떻게 나오는지 보면 스케줄러가 어떤 P를 쉬게 하는지 감을 잡을 수 있다.

## 코드로 확인하기

main.go의 `runHotLatency`는 P0 runq에 task 1부터 10까지 넣고 시작한다. tick 0에서 P0이 task 1을 실행하면서 hot task 999를 `enqueue`한다. P1은 idle이라 P0의 runq에서 올림 절반을 훔쳐 첫 task를 즉시 실행한다. 이때 FIFO 모드라면 hot task가 P0의 runq tail에 남아 앞에 남은 7,8,9,10 뒤에 서기 때문에 tick=5에 실행된다. runnext 모드라면 hot task가 P0의 runnext에 들어가므로 다음 tick에서 P0이 runq보다 먼저 runnext를 꺼내 tick=1에 실행된다.

출력에서 `[FIFO mode]`와 `[runnext mode]`의 `hot task(#999) executed at tick=...` 부분을 비교하면 runnext가 준 지연 단축을 수치로 확인할 수 있다. `steal+run first, stole 5` 같은 이벤트는 runqgrab의 올림 절반 공식(여기서는 9개 중 5개, 10개 중 5개)을 반영한다.

main_test.go는 세 가지를 검증한다. 첫 번째 테스트는 runnext에 새 작업이 들어오면 기존 runnext가 runq tail로 밀려나는지, `popLocal`이 runnext를 먼저 반환하는지를 확인한다. 두 번째 테스트는 동일한 시나리오에서 runnext 모드의 hot task tick이 FIFO 모드보다 작은지 검증한다. 세 번째 테스트는 `stealHalf`가 runq head에서 올림 절반을 정확히 가져가는지 확인한다.

이 시뮬레이션은 실제 Go 런타임을 완전히 재현한 것은 아니다. 실제 런타임에는 전역 runq, netpoll, timer, sysmon, random victim 선택, 4번의 steal 시도, CAS 기반 lock-free 연산이 더 있다. 하지만 runq와 runnext가 지연에 미치는 영향은 이 결정적 모델로도 충분히 관찰 가능하다.

## 모르면 겪는 일

runnext를 모르면 channel close나 mutex unlock 직후의 goroutine wakeup 지연을 오해할 수 있다. 예를 들어 어떤 P에 오래된 작업이 10개 쌓여 있고, 현재 실행 중인 goroutine이 channel을 닫아서 대기 중이던 goroutine을 깨운다. runnext가 있으면 깨어난 goroutine이 즉시 실행되지만, FIFO만 생각하고 디버깅하면 "왜 깨어난 goroutine이 queue 앞으로 점프하지 못하고 한참 뒤에 실행되지?"라는 혼란이 생긴다.

자신이 직접 work stealing 비슷한 구조를 짜면 tail에서 훔치기 쉽다. tail에서 훔치면 cache locality가 무너지고 p99 latency가 GC 주기나 cache miss와 섞여 튀는데 CPU 프로파일에는 명확한 hot spot이 안 잡힌다. 이런 증상은 "코드는 단순한데 왜 캐시 미스가 많지?"로 이어진다. Go 런타임이 head에서 훔치는 이유를 모르면 같은 실수를 반복한다.

또한 runq와 runnext가 분리돼 있다는 것을 모르면 `GODEBUG=schedtrace=1000`이나 `go tool trace`에서 goroutine이 예상과 다른 P에서 실행되는 모습을 보고 race나 버그로 오판할 수 있다. 실제로는 스케줄러가 work stealing으로 합법적으로 P를 옮긴 것이며, 이때 runnext가 있으면 옮겨지기 전에 같은 P에서 최대한 실행된다는 것을 이해해야 정확한 프로파일 해석이 가능하다.

## 언제 신경 쓰고 언제 무시하나

대부분의 Go 애플리케이션 코드는 이 내부를 몰라도 잘 동작한다. goroutine 수가 수백 개 수준이고 CPU 사용률이 낮으면 runnext와 work stealing은 눈에 띄는 차이를 만들지 않는다. 이 수준까지 신경 쓰는 것은 과최적화다. 코드를 단순하게 유지하고 표준 channel, sync 패키지를 쓰는 편이 낫다.

이 지식이 중요해지는 때는 P 수보다 훨씬 많은 goroutine이 지속적으로 wakeup/sleep을 반복하는 고성능 서비스다. 예를 들어 HTTP 서버에서 channel로 요청을 넘기거나, 많은 goroutine이 하나의 mutex를 기다리는 구조라면 runnext가 p99 wakeup latency에 영향을 준다. `go tool trace`에서 goroutine이 P 사이를 자주 이동하거나, 한 P만 바쁘고 다른 P가 놀고 있다면 work stealing과 runnext 동작을 의심해 봐야 한다.

자신의 코드가 특정 P에 작업을 몰아 넣고 있다면 sharding이나 batch 전략을 바꾸는 것이 scheduler 튜닝보다 효과적이다. runnext는 자동으로 동작하므로 직접 조작할 수 없고, GOMAXPROCS나 channel buffer 크기, shard 개수 같은 상위 설계를 조정하는 것이 실제 개선으로 이어진다.

## 더 파보기

- runtime/proc.go: runqput, runqget, runqgrab, runqsteal, stealWork, findRunnable 구현
  https://go.dev/src/runtime/proc.go
- runtime/runtime2.go: p 구조체의 runq, runqhead, runqtail, runnext 필드 정의
  https://go.dev/src/runtime/runtime2.go
- Go 스케줄러 디자인 문서 (Go 1.1 이전 전역 큐에서 현재 work stealing까지)
  https://go.dev/s/go11sched
- Go 런타임 트레이스와 스케줄러 상태를 보는 공식 문서
  https://go.dev/doc/diagnostics