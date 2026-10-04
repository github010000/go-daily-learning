## 한 줄 요약

Go 런타임의 `selectgo`는 소스 코드에 적힌 case 순서대로 채널을 검사하지 않습니다. 참여하는 모든 channel을 주소 순서로 잠근 뒤, 준비된 case를 `pollorder`라는 무작위 순열로 검사해서 첫 번째로 준비된 case를 선택합니다. 이렇게 하면 특정 channel이 항상 먼저 선택되어 다른 channel이 굶는 결정적 기아를 피할 수 있습니다. `default`가 있으면 준비된 case가 하나도 없을 때 대기열에 들어가지 않고 즉시 반환하므로, non-blocking 검사를 빠르게 수행할 수 있습니다.

여기서 말하는 "공정성"은 동시에 준비된 case들 사이에서 각 case가 선택될 기대 확률이 같아진다는 뜻입니다. 엄격한 round-robin이나 가중치 공정성은 아닙니다. 실행 횟수가 적으면 어느 한 case가 연속으로 선택될 수도 있고, 오래 실행했을 때 각 case의 선택 횟수가 확률적으로 균등해질 뿐입니다.

## 왜 이런 설계인가

네트워크 서버는 보통 수많은 연결, 타이머, 종료 신호, 작업 큐를 동시에 기다려야 합니다. OS 스레드는 스택 크기가 크고 문맥 전환 비용이 비싸서 연결마다 하나씩 만들기 어렵습니다. Go는 goroutine과 channel을 조합해 이 문제를 해결했지만, 한 goroutine이 여러 channel 중 하나를 기다리려면 `select`가 필요합니다. 문제는 여러 channel이 동시에 준비된 경우 무엇을 선택할지 정하는 규칙이 없다면, 가장 단순한 규칙인 "소스 코드 순서대로 검사해서 처음 준비된 것을 선택"은 특정 channel을 영원히 우선하게 만든다는 점입니다.

예를 들어 `select { case <-ctrl: ...; case <-data: ... }`에서 `ctrl`이 자주 준비되어 있으면 `data`는 거의 처리되지 않습니다. 제어 평면 트래픽이 데이터 평면 트래픽보다 적더라도, `ctrl`이 준비되는 순간마다 항상 먼저 검사되면 데이터 경로가 굶을 수 있습니다. OS 스케줄러의 starvation 문제와 비슷한데, 언어 차원의 select는 별도 우선순위 큐를 두기보다는 간단한 무작위화를 선택했습니다.

대안으로는 round-robin을 생각할 수 있습니다. 그러나 round-robin은 select 호출 지점마다 이전에 어느 case를 선택했는지 상태를 유지해야 합니다. select는 언어 문법이자 런타임 함수인데, 모든 호출 위치에 숨은 상태를 두면 지역적이지 않고, 재귀 호출이나 인터페이스 뒤에서 호출될 때 추론이 어려워집니다. 또한 각 case가 준비되는 빈도 자체가 다르기 때문에, 단순 round-robin은 실제 트래픽 패턴을 반영하지 못할 수 있습니다. 무작위 순서는 상태가 없으면서도 특정 case가 결정적으로 우선되는 것을 막아줍니다.

또 하나 중요한 설계 문제는 deadlock입니다. `select`가 여러 channel을 잠가야 한다면, 두 goroutine이 서로 다른 소스 순서로 lock을 잡으면 서로를 기다리는 교착 상태가 생길 수 있습니다. Go 런타임은 이 문제를 피하려고 channel 자체의 메모리 주소를 기준으로 `lockorder`를 정렬합니다. 따라서 어떤 소스 순서로 select를 작성했든 런타임은 항상 같은 전역 순서로 channel을 lock합니다. 이는 다중 잠금 문제에서 번호 순서로 lock을 잡는 고전적 규율과 같습니다.

`default`가 없는 blocking select는 준비된 case가 없으면 현재 goroutine을 각 channel의 wait queue에 넣고 `gopark`로 재웁니다. 이 경로는 wait queue 조작, scheduler 호출, 이후 깨어났을 때 여러 channel에서 자기 자신을 제거하는 cleanup까지 포함해서 비쌉니다. 반면 `default`가 있으면 준비된 case가 없을 때 그 자리에서 반환할 수 있습니다. 특히 `select { case v := <-ch: default: }`처럼 case가 하나뿐이면 컴파일러가 `selectnbrecv`나 `selectnbsend`라는 전용 non-blocking 함수를 호출해서 전체 `selectgo` 경로를 타지 않습니다. 이것이 "default가 있을 때의 빠른 경로"입니다.

## 어떻게 동작하는가

`select`가 실행되면 컴파일러는 각 case를 런타임의 `scase` 구조체 배열로 만듭니다. `runtime/select.go`에 정의된 `scase`는 대상 channel을 가리키는 `c *hchan`, 송수신할 데이터의 포인터 `elem`, case 종류를 나타내는 `kind`를 가집니다. `kind`는 `caseRecv`, `caseSend`, `caseDefault`, `caseNil` 중 하나입니다. `hchan`은 `runtime/chan.go`에 정의된 channel 런타임 구조체로, 수신 대기열 `recvq`, 송신 대기열 `sendq`, 버퍼에 들어 있는 원소 개수 `qcount`, 버퍼 크기 `dataqsiz` 같은 필드를 가집니다.

일반적인 다중 case select는 `runtime.selectgo`로 진입합니다. 먼저 `fastrandn`을 이용해 case 인덱스를 섞은 `pollorder`를 만듭니다. `pollorder`는 Fisher-Yates shuffle입니다. 동시에 channel을 잠그는 순서인 `lockorder`도 만드는데, 이때는 channel의 주소를 비교하는 `sortkey`를 사용해 정렬합니다. `lockorder`는 교착 상태를 막기 위한 것이고, `pollorder`는 공정성을 위한 것입니다. 같은 channel이 select에 여러 번 등장하면 `lockorder`에서 tie가 생기는데, 이때는 `pollorder`를 이용해 순서를 섞습니다.

이후 `sellock`이 `lockorder` 순서대로 모든 channel을 잠급니다. 모든 channel을 잠그는 이유는 준비 상태를 일관되게 읽기 위해서입니다. 한 channel만 먼저 확인하고 다른 channel을 확인하는 사이에 다른 goroutine이 상태를 바꾸면, select의 결정이 원자적이지 않게 됩니다. 예를 들어 A와 B를 확인하는 사이에 A가 준비되지 않았는데 갑자기 준비될 수도 있고, 두 channel에 모두 값을 보내는 것처럼 보이는 모순된 상황이 생길 수 있습니다.

준비 상태 검사는 `pollorder` 순서로 진행됩니다. receive case라면 `c.sendq.dequeue()`로 대기 중인 송신자를 꺼내거나, `c.qcount > 0`이면 버퍼에서 값을 가져오거나, channel이 닫혔으면 zero value를 만듭니다. send case라면 `c.recvq.dequeue()`로 대기 중인 수신자를 꺼내거나, `c.qcount < c.dataqsiz`이면 버퍼에 값을 넣거나, 닫힌 channel이면 panic을 일으킵니다. 이 과정에서 `default` case를 만나면 그 위치를 기억해 둡니다. 만약 `pollorder`를 끝까지 돌았는데 준비된 case가 하나도 없고 `default`가 있다면, 그 즉시 lock을 풀고 `default`의 인덱스를 반환합니다.

준비된 case도 없고 `default`도 없다면 select는 두 번째 단계로 넘어가 현재 goroutine을 각 channel의 `sendq`나 `recvq`에 넣고 `gopark`로 잠듭니다. 나중에 어느 한 channel에서 송수신이 발생하면 깨어나는데, 이때 다른 channel의 wait queue에 남아 있는 자기 자신을 모두 제거해야 합니다. 이 enqueue/dequeue와 lock traffic 때문에 blocking select는 참여하는 channel 수가 많을수록 비용이 커집니다.

공정성의 정확한 범위는 "어떤 select 호출에서 여러 case가 동시에 준비되어 있을 때, 각 준비된 case가 선택될 확률이 같다"는 것입니다. `pollorder`가 균등 무작위 순열이므로 준비된 case들 사이에서 어느 case가 가장 먼저 나올 확률은 모두 같습니다. 그러나 짧은 구간에서는 편차가 클 수 있고, 연속해서 같은 case가 선택될 수도 있습니다. 또 어떤 case가 select가 시작된 이후에야 준비되면 그 호출에서는 고려되지 않습니다. blocking select가 깨어나는 시점의 순서는 channel wait queue와 실제 송신 순서에 따라 달라질 수 있어서, select 자체가 goroutine 간의 엄격한 queueing fairness를 보장하지는 않습니다.

## 돌려보기

이 디렉토리에서 아래 명령을 순서대로 실행하면 됩니다.

```bash
go vet ./...                 # 정적 검사: 잘못된 문법, 버그 가능성 확인
go build -o /dev/null ./...  # 컴파일 확인: 실행 파일은 남기지 않음
go run .                     # 시연 실행: 고정 순서 vs select 공정성, default 경로 출력
go run . 5000                # 시행 횟수를 5000으로 줄여 빠르게 확인
go test -v ./...             # 테스트 실행: non-blocking, 합계 불변식, 고정 순서 기아 검증
go test -race ./...          # race detector로 동시성 안전성 확인
go test -bench=. -benchmem ./...  # default 단일 수신의 non-blocking 벤치마크 확인
```

`go run .`에서는 먼저 고정 순서 검사의 잘못된 결과가 나옵니다. 0번 case가 100%에 가깝고 나머지는 0이어야 합니다. 그다음 `select` 공정성 결과가 각각 약 25%로 퍼지는 것을 확인할 수 있습니다. `go test -bench=. -benchmem`에서는 `NonBlockingReceive`가 channel에 값이 없어도 블록하지 않고 매번 default 경로를 타는 속도를 볼 수 있습니다.

## 코드로 확인하기

`main.go`의 `FixedOrderDemo`는 select를 사용하지 않고, 0번 channel부터 차례로 `NonBlockingReceive`를 호출합니다. 모든 채널을 매 시행 전에 가득 채웠기 때문에 0번 channel이 항상 준비되어 있습니다. 따라서 출력은 case 0이 시행 횟수의 100%를 차지하고 case 1, 2, 3은 0이 됩니다. 이는 select가 소스 코드 순서로 동작한다고 잘못 가정했을 때 어떤 channel이 굶게 되는지를 보여줍니다.

`SelectFairnessDemo`는 동일하게 모든 channel을 매번 준비 상태로 만들고 실제 `select`를 사용합니다. 런타임은 `pollorder`를 무작위로 섞기 때문에 각 case가 약 25%씩 선택됩니다. 예를 들어 20000회 실행하면 5000 근처에서 흔들리는데, 표준 편차는 대략 70 정도이므로 대부분 4800~5200 사이에 들어옵니다. 출력에서 네 case의 비율이 극단적으로 치우치지 않는 것을 확인해야 합니다. 정확히 25%가 아닌 이유는 의도된 무작위성 때문입니다.

`DefaultPathDemo`는 unbuffered channel에 송신 goroutine이 8개의 값을 보내는 동안 main goroutine이 `select`와 `default`로 계속 검사합니다. 출력에서 `received=8`은 보장되지만 `missed` 값은 실행할 때마다 달라집니다. unbuffered channel은 송신자와 수신자가 실제로 만나야만 송수신이 성공하므로, main goroutine이 송신자가 준비되기 전까지는 계속 default로 빠집니다. `missed`가 크게 나오는 것은 default 경로가 블록하지 않고 즉시 반환된다는 증거입니다. 실제 프로그램에서 이렇게 busy-wait하면 CPU를 낭비하므로, 잠시 멈추거나 blocking goroutine을 쓰는 것이 필요합니다.

`main_test.go`에서는 세 가지를 검증합니다. `TestNonBlockingReceive`는 빈 채널에서 default가 선택되고, 값이 들어 있는 채널에서는 실제 값이 수신되는지 확인합니다. `TestSelectFairnessDemoCountsSum`은 무작위 분포를 직접 검증하는 대신, 반환된 count의 합이 시행 횟수와 정확히 일치하는 불변식만 확인합니다. 이렇게 하면 CI에서 확률 때문에 flaky test가 생기는 것을 막을 수 있습니다. `TestFixedOrderDemoStarvesOthers`는 고정 순서 검사가 0번 channel만 100번 선택하고 나머지는 굶긴다는 것을 결정적으로 검증합니다. `BenchmarkNonBlockingReceive`는 default가 포함된 단일 수신이 매번 블로킹 없이 끝나는 경로의 성능을 측정합니다.

## 모르면 겪는 일

select가 소스 코드 순서대로 동작한다고 생각하고 제어 채널과 데이터 채널을 함께 쓰면, 제어 채널이 조금만 자주 준비되어도 데이터 채널이 굶을 수 있습니다. 증상은 CPU가 놀고 있는데도 특정 작업 큐만 계속 밀리는 형태로 나타납니다. 로그에는 control 이벤트는 잘 처리되는데 data 파이프라인의 p99 latency만 비정상적으로 높아지고, 큐 깊이는 점점 증가합니다. 코드 리뷰에서 select case 순서를 바꿔보기 전에는 원인을 찾기 어렵습니다.

`default` 경로가 wait queue에 들어가지 않는다는 사실을 모르고 non-blocking select를 사용하면, CPU 사용률이 치솟거나 반대로 이벤트를 놓칠 수 있습니다. 예를 들어 `for { select { case v := <-ch: ...; default: time.Sleep(time.Millisecond) } }` 같은 코드를 작성하면, sleep이 너무 짧으면 대부분의 시간을 헛돌고, 너무 길면 channel에 값이 들어와도 바로 처리하지 못합니다. 이는 select 자체가 queueing을 해주지 않기 때문입니다. default가 있으면 준비되지 않았을 때 즉시 돌아오므로, 호출자가 직접 backoff나 큐를 관리해야 합니다.

여러 channel을 직접 lock해서 select 비슷한 동작을 만들 때, lock 순서를 소스 코드 순서대로 잡으면 두 goroutine이 서로 반대 순서로 lock을 잡아 deadlock이 발생할 수 있습니다. Go 런타임은 `lockorder`를 channel 주소로 정렬해서 이 문제를 피합니다. 이 내부 설계를 모르면 select를 직접 재구현하려다가 교착 상태를 만들 수 있고, 반대로 select가 느릴 거라고 지레짐작하고 unsafe한 자체 구현을 선택할 수도 있습니다.

select가 공정하다는 말을 엄격한 queue fairness로 오해하면, 여러 producer goroutine이 하나의 consumer select로 합류하는 지점에서 특정 producer만 자주 선택되는 현상을 보고 "runtime 버그"로 오해할 수 있습니다. 실제로는 select가 준비된 case 중에서 무작위로 고를 뿐이므로, 어떤 producer가 더 자주 준비 상태가 되거나 CPU 스케줄링을 더 자주 받으면 그 producer가 더 자주 선택될 수 있습니다. 엄격한 producer별 공정성이 필요하면 select의 무작위 공정성만 믿지 말고 별도의 큐나 가중치를 도입해야 합니다.

## 언제 신경 쓰고 언제 무시하나

대부분의 소규모 프로그램에서는 select의 내부 동작을 깊이 알 필요가 없습니다. 초당 수천 건 미만의 select라면 `pollorder` 생성, `lockorder` 정렬, channel lock 비용은 전체 처리 시간에서 거의 드러나지 않습니다. 이 경우에는 코드의 가독성과 case 순서의 의미 전달이 더 중요합니다. case 순서를 바꾼다고 실행 의미가 크게 달라지지 않으므로, 도메인 관례에 따라 읽기 쉽게 배치하면 됩니다.

단일 non-blocking case + default는 컴파일러가 전용 경로로 바꿔주므로 특히 신경 쓸 것이 없습니다. 예를 들어 `select { case ch <- task: default: }`는 `selectnbsend`로 바뀌어 `selectgo`의 full path보다 훨씬 가볍습니다. 이런 패턴은 worker가 busy할 때 task를 버리는 load shedding에 유용하며, 내부적으로 wait queue에 들어가지 않는다는 점만 기억하면 됩니다.

select 공정성이 중요해지는 조건은 많은 case가 자주 동시에 준비되는 fan-in 지점입니다. 예를 들어 수십 개 연결에서 데이터가 몰려오고, 동시에 타이머나 종료 신호가 항상 준비되어 있을 수 있습니다. 이때 무작위 `pollorder`가 아니었다면 한쪽 연결이 계속 밀릴 수 있습니다. 초당 수만 회 이상 select를 실행하는 고부하 서버에서는 `runtime.selectgo`나 `sellock`이 프로파일 상위에 올라올 수 있습니다. 이때는 select case 수를 줄이거나, 재사용 가능한 worker 구조로 바꾸거나, batch channel을 도입하는 편이 낫습니다.

반대로 엄격한 fair queueing이 필요한 경우에는 select의 무작위 공정성에 의존하면 안 됩니다. producer별 가중치, 우선순위, 최소 보장량이 필요하다면 select 앞에 자체 큐를 두거나, 두 단계로 나눠 먼저 큐에서 작업을 고르고 그 작업의 channel만 select하는 방식이 맞습니다. 이 지식을 과최적화에 쓰지 말고, "select는 결정적 기아를 막기 위한 무작위 선택"이라는 본래 목적에 맞게 사용하는 것이 중요합니다.

## 더 파보기

- Go 런타임 select 구현: https://go.dev/src/runtime/select.go
- channel 런타임 구현: https://go.dev/src/runtime/chan.go
- Go 언어 명세의 select statements: https://go.dev/ref/spec#Select_statements
- Go 블로그의 파이프라인 패턴과 fan-in/fan-out: https://go.dev/blog/pipelines
```