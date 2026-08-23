## 한 줄 요약

Go 스케줄러는 OS 스레드마다 글로벌 큐 하나를 쓰는 대신, P(processor)마다 로컬 런큐를 두고, 한 P 가 자기 큐를 비우면 다른 P 의 큐에서 절반을 훔치는 work stealing 으로 부하를 분산한다. runnext 슬롯은 새로 만든 goroutine 이 곧바로 같은 P 에서 실행되도록 보호해서, 다른 P 에 빼앗기지 않게 하고 캐시 지역성과 지연 시간을 개선한다. 훔치기에 실패하면 전역 런큐, 네트워크 폴러, 스핀, sleep 순으로 내려가며 일을 찾는다.

## 왜 이런 설계인가

OS 스레드 하나당 하나의 글로벌 큐를 두고 모든 goroutine 이 거기서 일을 꺼내는 구조는 간단하지만, 스레드 수가 늘어나면 큐 잠금 경합이 폭발한다. 수백 개 스레드가 매번 같은 뮤텍스를 잡고 작업을 넣고 빼면, 실제 CPU 작업 시간보다 잠금 대기와 캐시 라인 경합이 더 커진다. 또한 한 스레드가 자주 사용하던 데이터가 다른 스레드로 넘어가면 CPU 캐시가 무효화되어 캐시 미스가 급증한다. Go 는 goroutine 을 수십만 개까지 실행해야 하므로 이 비용은 감당할 수 없다.

그래서 Go 런타임은 GOMAXPROCS 개수만큼의 P 를 만들고, P 마다 runq(로컬 런큐)를 준다. 자기 P 의 큐에서 task 를 꺼내면 잠금이 필요 없고, 다른 P 와 충돌하지 않는다. 한 P 가 일이 없을 때만 다른 P 의 큐를 훔치므로, 잠금 경합이 드물게 발생한다. 이것이 work stealing 의 핵심 동기다. 훔칠 때도 큐 전체가 아니라 절반만 가져가서, 훔친 P 와 원래 P 사이에 부하가 더 고르게 퍼진다.

runnext 슬롯은 work stealing 의 단점을 보완한다. go 문으로 새 goroutine 을 만들면, 그것을 만든 P 가 그 goroutine 을 가장 먼저 실행할 가능성이 높다. 만약 runnext 없이 그냥 runq 에 넣으면, 다른 P 가 즉시 훔쳐가서 생성자 P 가 아닌 엉뚱한 P 에서 실행될 수 있다. 그러면 방금 생성자 P 가 사용하던 데이터가 다른 P 의 캐시로 옮겨가야 해서 캐시 미스가 발생하고, 생성자 P 는 그 goroutine 의 완료를 기다리면서 손해를 본다. runnext 는 "방금 만든 goroutine 은 내가 바로 실행한다"는 의도를 표현해, 훔치기 대상에서 제외한다.

대안으로는 "글로벌 큐만 쓰기", "큐마다 잠금을 아주 작게 나누기", "작업을 직접 지정한 스레드에 보내기" 등이 있었다. 글로벌 큐는 경합, 잠금 세분화는 관리 복잡도, 직접 지정은 부하 불균형을 만든다. work stealing 은 구현이 비교적 단순하면서도 잠금을 드물게 만들고, 훔치는 과정에서 자연스럽게 부하 균형이 맞는 장점이 있어 채택됐다.

## 어떻게 동작하는가

Go 런타임의 스케줄러 소스는 `runtime/proc.go` 에 있다. P 구조체는 `runq` 라는 크기 256 인 원형 큐와 `runnext` 라는 단일 포인터 슬롯을 가진다. `runqput` 함수는 새 goroutine 을 넣을 때 `runnext` 가 비어 있으면 `runnext` 에 넣고, 이미 차 있으면 기존 `runnext` 를 `runq` 로 밀어내고 새 goroutine 을 `runnext` 에 넣는다. 이렇게 하면 가장 최근에 만든 goroutine 이 항상 P 의 맨 앞에 있게 된다. `runqget` 은 반대로 `runnext` 를 먼저 확인하고, 없으면 `runq` 에서 꺼낸다. `runq` 는 원형 큐라서 배열 인덱스 계산만으로 push/pop 이 이루어지고, 락을 쓰지 않는다.

work stealing 은 `findrunnable` 이라는 함수에서 일어난다. P 가 실행할 goroutine 이 없으면 먼저 전역 런큐를 확인하고, 그다음 다른 P 의 로컬 큐를 훔친다. 훔치는 함수는 `runqgrab` 인데, 대상 P 의 `runq` 에서 절반을 가져와 자신의 `runq` 에 넣는다. 이때 `runnext` 슬롯은 절대로 훔치지 않는다. `runnext` 는 생성자 P 만 접근할 수 있는 일종의 전용 슬롯이다. 훔친 여러 goroutine 중 첫 번째는 곧바로 실행하고, 나머지는 로컬 큐에 쌓아둔다.

훔치기가 실패하면 어디로 가는가? `findrunnable` 은 다음 순서로 일을 찾는다. 첫째, 자신의 `runnext` 와 `runq`. 둘째, 전역 런큐. 셋째, 네트워크 폴러(netpoll) 깨우기. 넷째, 다른 P 의 `runq` 훔치기. 다섯째, 일정 시간 스핀(spin). 여섯째, OS 스레드를 잠든 상태로 전환(stopm)한다. 이렇게 단계적으로 시도하기 때문에, 잠깐 일이 없어도 바로 잠들지 않고 바쁜 대기로 지연을 줄인다. 실제로 `findrunnable` 안에는 `spinning`, `nmspinning`, `sched.nmspinning` 같은 변수들이 등장한다.

본 저장소의 `main.go` 는 이 구조를 교육용으로 축소해 놓았다. `processor` 는 `runq` 슬라이스와 `runnext` 포인터를 가지고, `stealFrom` 은 다른 P 의 `runq` 절반을 가져오되 `runnext` 는 건드리지 않는다. `findRunnable` 은 `runqget` → 전역 큐 → `stealFrom` 순으로 일을 찾는다. 실제 런타임은 락프리 원형 큐와 정교한 스핀 카운트를 쓰지만, 핵심적인 제어 흐름은 동일하다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령을 순서대로 실행해 보자.

```bash
go vet ./...                 # 정적 검사: 시뮬레이션 코드의 동시성 문제와 잘못된 API 사용을 걸러낸다
go build -o /dev/null ./...  # 컴파일 확인: 타입 오류와 누락된 임포트를 잡는다
go run .                     # 시연 실행: work stealing 과 runnext affinity 지표를 출력한다
go test -v ./...             # 테스트: runnext 우선 실행, 훔치기 제외, 분배 불변식을 검증한다
go test -race ./...          # 동시성 주제이므로 반드시: 뮤텍스 누락이나 data race 를 검사한다
GODEBUG=schedtrace=1000 go run .  # 실제 런타임 스케줄러 추적: P 별 runqueue 길이와 idle 상태가 주기적으로 표준 에러에 출력된다
```

`go run .` 출력에서 `P별 처리 수`를 보면, P0 에만 task 를 몰아넣었는데도 다른 P 들이 일정량을 처리한 것을 확인할 수 있다. 이 숫자들이 work stealing 이 실제로 일어났다는 증거다. `GODEBUG=schedtrace=1000 go run .` 명령은 이 프로그램을 실행하면서 동시에 실제 Go 런타임의 스케줄러 상태를 주기적으로 보여준다. 출력에서 `P0: ...` 줄마다 `runnable` goroutine 수가 표시되는데, 시간이 지나면서 여러 P 사이에 숫자가 비슷해지는 것을 관찰할 수 있다.

## 코드로 확인하기

`main.go` 는 두 가지를 시연한다. 첫째, `runWorkStealingDemo` 는 task 20000개를 P0 의 로컬 큐에만 넣고 P 4개로 실행한다. 출력에서 `runnext 사용 O` 일 때 P0 이 처리한 수를 보면, 전체의 절반 이상이 P0 에서 실행된 것을 확인할 수 있다. `runnext 사용 X` 일 때는 P0 의 처리 비율이 뚝 떨어진다. 왜냐하면 runnext 가 없으면 P0 의 runq 에 쌓인 task 들이 다른 P 에 의해 절반씩 훔쳐가기 때문이다. runnext 슬롯은 새로 만든 task 가 P0 에 남아 즉시 실행되도록 지켜준다.

둘째, `runNextAffinityDemo` 는 같은 P 에서 실행되는 비율을 직접 수치로 보여준다. runnext 사용 O 일 때 `samePWith` 가 runnext 사용 X 일 때보다 훨씬 크다. 이는 캐시 지역성에 중요한 의미를 가진다. P0 에서 만든 task 가 P0 에서 실행되면 그 task 가 접근하는 데이터가 이미 P0 을 도는 OS 스레드의 CPU 캐시에 있을 가능성이 높다. 다른 P 로 넘어가면 그 캐시 라인을 무효화하고 다시 가져와야 하므로, 실제 CPU 바운드 작업에서 수십 퍼센트의 성능 차이가 날 수 있다.

셋째, `measureRunNextLatency` 는 runqput/runqget 왕복 시간을 비교한다. runnext 는 단일 포인터 슬롯이라 잠금 없이 O(1)로 접근되지만, runq 슬라이스는 append 와 슬라이싱이 일어나서 아주 약간 느리다. 이 차이는 마이크로초 단위라 작아 보이지만, 실제 런타임에서 수십만 goroutine 이 반복적으로 큐를 조작하면 누적 비용과 캐시 미스가 커진다.

`main_test.go` 는 세 가지를 검증한다. `TestRunqputRunNext` 는 runnext 가 비어 있을 때 `runqput` 이 runnext 에 넣고, `runqget` 이 그 task 를 우선 반환하는지 확인한다. `TestStealFromDoesNotTouchRunnext` 는 다른 P 가 훔칠 때 `runnext` 슬롯의 task 는 절대 가져가지 않고, `runq` 에서만 절반을 훔치는지 확인한다. `TestWorkStealingDistribution` 은 불균등하게 넣은 task 가 모든 P 에 분산되어 처리되고, 전체 처리 수가 정확히 일치하는지 확인한다. 이 테스트들은 `-race` 플래그로도 통과하며, 시간에 의존하는 단정을 사용하지 않는다.

## 모르면 겪는 일

work stealing 과 runnext 를 모르면 실제 서비스에서 미묘한 성능 문제를 겪는다. 가장 흔한 증상은 CPU 코어가 여러 개인데도 한 코어만 100% 가까이 사용되고 나머지는 놀고 있는 경우다. CPU 프로파일을 떠 보면 특정 함수가 오래 걸리는 것처럼 나오지만, 실제 원인은 스케줄러가 아니라 작업 분배 구조에 있다. 예를 들어 글로벌 큐에 뮤텍스를 걸고 모든 goroutine 이 거기서 일을 가져가면, 처음에는 잘 돌아가다가 동시성 수치가 올라가면 p99 지연이 주기적으로 튄다. 이때 CPU 프로파일에는 뮤텍스 대기 시간이 잡히지 않아 원인을 찾기 어렵다.

runnext 를 모르면 "왜 내가 만든 goroutine 이 엉뚱한 스레드에서 실행되지?"라는 의문을 갖게 된다. 특히 채널로 완료 신호를 주고받는 패턴에서, `go func(){ ch <- 1 }()` 직후 `<-ch` 로 기다리는 코드가 다른 P 로 넘어가면 왕복 지연이 커진다. CPU 캐시를 공유해야 하는 연산에서 goroutine 이 다른 P 로 훔쳐지면 캐시 미스가 발생해 처리율이 떨어진다. 벤치마크를 잘못 짜서 GOMAXPROCS=1 로 돌리면 work stealing 이 일어나지 않는데도 "병렬 처리가 효과 없다"고 오해할 수도 있다.

이 지식을 모르면 잘못된 최적화를 하기 쉽다. 예를 들어 goroutine 수를 줄이기 위해 작업을 글로벌 큐에 직접 넣는 코드를 작성하면, 오히려 뮤텍스 경합이 생겨서 더 느려진다. 반대로 runnext 의 존재를 모르고 "왜 이 goroutine 이 예상보다 빨리 실행되지?"라고 생각해 `runtime.Gosched` 를 과도하게 호출하면, runnext 의 이점을 스스로 없애고 오히려 스케줄링 오버헤드만 키운다. Go 런타임이 이미 잘 해주는 일을 사용자가 다시 구현하려다 성능을 깎아먹는 셈이다.

## 언제 신경 쓰고 언제 무시하나

goroutine 수가 수천 개 이하이거나, 작업이 대부분 IO 바운드(네트워크, 디스크, 채널 대기)라면 work stealing 과 runnext 는 거의 신경 쓸 필요가 없다. IO 대기 중인 goroutine 은 P 를 떠나 있고, 실행 가능한 goroutine 수가 적어서 훔치기 자체가 드물게 일어난다. 이 경우에는 코드 가독성과 유지보수성이 더 중요하다. GOMAXPROCS 를 늘려도 성능이 나아지지 않는 대표적인 경우가 바로 IO 바운드 워크로드다.

CPU 바운드 작업이 많고 goroutine 수가 수만 개를 넘어가면 이 지식이 중요해진다. 특히 p99 지연이 GC 주기마다 튀거나, 특정 코어만 과부하되는 증상이 보이면 스케줄러 동작을 의심해야 한다. `GODEBUG=schedtrace=1000` 으로 P 별 runqueue 길이를 관찰하면, 어느 P 에 일이 몰리는지, 훔치기가 얼마나 자주 일어나는지 알 수 있다. runnext 의 효과를 보려면 CPU 프로파일과 하드웨어 성능 카운터로 캐시 미스를 함께 봐야 한다.

초당 수십만 개의 짧은 goroutine 을 생성하는 서버라면 runnext 의 우선 실행이 응답 지연에 직접 영향을 준다. 이런 극단적인 경우가 아니라면, "go 문으로 만들고 채널로 통신하는" 표준 패턴을 그대로 쓰는 것이 최선이다. 런타임이 이미 최적화해 놓은 work stealing 과 runnext 위에서 동작하도록 두는 것이 과최적화를 피하는 길이다.

## 더 파보기

- [runtime/proc.go 소스](https://go.dev/src/runtime/proc.go) — `runqput`, `runqget`, `runqgrab`, `findrunnable`, `stealWork` 가 실제 구현이다. 특히 `findrunnable` 의 주석을 읽으면 스핀과 sleep 조건이 자세히 설명돼 있다.
- [Go 스케줄러 공식 문서](https://go.dev/s/go15gcpacing) — Go 1.5 에서 GOMAXPROCS 가 CPU 수로 기본 설정된 이유와 work stealing 설계 배경을 설명한다.
- [The Go scheduler (Ardan Labs 블로그)](https://www.ardanlabs.com/blog/2015/02/scheduler-tracing-in-go.html) — GODEBUG=schedtrace 출력을 해석하는 방법과 P, M, G 관계를 그림으로 보여준다. 런타임 내부에 처음 입문할 때 좋다.
- [Go issue 10077: work stealing starves runnext](https://github.com/golang/go/issues/10077) — runnext 가 훔치기에서 제외되면서 생긴 부하 불균형 문제와 해결 과정을 볼 수 있는 실제 이슈다.
- [runtime/proc.go 의 findrunnable 주석](https://go.dev/src/runtime/proc.go#L2556) — 훔치기 실패 시 전역 큐, netpoll, spin, stopm 순서를 코드와 함께 확인할 수 있다.