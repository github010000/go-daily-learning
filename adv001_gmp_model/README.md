## 한 줄 요약

Go runtime scheduler는 G(goroutine), M(OS thread), P(logical processor)의 3층 구조를 쓴다. G는 실행 흐름, M은 실제 OS 스레드, P는 scheduler context이자 지역 run queue를 가진 실행 슬롯이다. P를 G와 M 사이에 끼워 넣어 전역 run queue 경합을 줄이고, OS 스레드 수를 GOMAXPROCS 단위로 제한하면서, syscall 중에도 실행 자원을 안전하게 넘길 수 있게 만들었다.

## 왜 이런 설계인가

Go가 처음 나온 시절에는 오늘날 같은 GMP 구조가 없었다. 초기 Go scheduler는 G와 M만 있었고, 실행 가능한 G는 모두 하나의 global run queue에 보관됐다. 그 큐는 mutex로 보호됐는데, CPU 코어가 늘어나고 goroutine 수가 많아지면 모든 M이 같은 큐에서 G를 꺼내려고 경합했다. 특히 CPU-bound goroutine 수천 개가 동시에 돌면 mutex contention이 치솟았고, G를 꺼내는 시간이 실제 작업 시간보다 커지는 경우도 있었다. 사용자 입장에서는 goroutine이 아무리 가벼워도 scheduler가 bottleneck이 되면 아무 의미가 없었다.

두 번째 문제는 thread explosion이었다. M은 OS thread이므로 syscall에서 blocking되면 그 M은 한동안 아무것도 하지 못한다. 초기 구조에서는 M이 syscall에 들어가면 그 M에 붙어 있던 G도 전부 멈춰 보였고, 이를 만회하려고 런타임이 새 M을 계속 만들었다. 명시적인 상한이 없었기 때문에 blocking syscall이 많은 서버는 OS thread가 수백 개, 수천 개로 늘어났고, 커널 스택과 context switch 비용이 급증했다. 이는 goroutine을 가볍게 쓰려는 Go의 핵심 가치를 훼손하는 지점이었다.

Go 1.1에서 P를 도입한 이유가 여기에 있다. P는 M과 분리된 scheduler context로서 지역 run queue와 runnext slot, 상태, 통계를 가진다. GOMAXPROCS는 P의 개수를 의미하고, P 하나가 곧 동시에 실행될 수 있는 G 하나의 슬롯이다. M이 blocking syscall에 들어가면 P가 그 M에서 떨어져 idle P pool로 이동하고, 다른 M이 그 P를 acquirep하여 남은 G를 계속 실행한다. 이렇게 하면 M의 수는 동적이되 P의 수는 고정되어, OS thread가 무한정 늘어나는 것을 막으면서도 syscall 중에 실행 자원이 방치되지 않는다.

P를 굳이 M에 직접 묶지 않고 분리한 이유는 M이 오래 block되거나 park될 수 있기 때문이다. 만약 지역 run queue를 M에 묶었다면 M이 syscall에 들어가는 순간 그 큐에 있던 G는 모두 멈춰야 하고, M이 돌아올 때까지 실행 기회를 잃는다. P는 이런 OS 자원의 불확실성에서 queue를 보호하는 계층이다. 어떤 의미에서 P는 core를 쓸 자격을 나타내고, M은 그 자격을 실제로 실행하는 수단이다. 이 3층 분리가 없으면 G와 M을 직접 묶으면서 생기는 캐시 지역성 저하와 thread 관리를 동시에 해결하기 어렵다.

## 어떻게 동작하는가

핵심 자료구조는 `src/runtime/runtime2.go`에 정의돼 있다. `type g struct`는 stack, sched 정보, atomicstatus, lockedm 등을 가진다. `type m struct`는 `p *p`, `nextp`, `oldp`, `schedlink`, `mcache` 등을 가진다. `type p struct`는 지역 run queue 관련 필드를 직접 보유하는데, `runq [256]guintptr`, `runnext guintptr`, `runqhead uint32`, `runqtail uint32`가 대표적이다. 전역 run queue는 같은 파일의 `schedt` 구조체에 `runqhead`, `runqtail`, `runqsize`로 존재하며 `sched`라는 전역 변수로 관리된다. 이 필드 이름이 실제 코드에 그대로 등장한다.

P의 지역 run queue는 크기가 256으로 고정된 circular array다. `src/runtime/proc.go`의 `runqput` 함수는 새 G를 지역 run queue에 넣을 때 먼저 `runnext` slot에 넣는다. `runnext`는 LIFO로 동작하는 1칸 slot인데, 방금 만든 goroutine이 같은 데이터를 다시 쓰는 캐시 친화적 패턴을 활용하기 위해 존재한다. 이미 `runnext`에 G가 있다면 기존 G는 지역 run queue tail로 밀려난다. 지역 run queue가 256개로 가득 찼다면 `runqputslow`가 호출된다. `runqputslow`는 runq의 앞쪽 절반인 128개와 밀려난 G 하나를 묶어 batch로 만들고 `lock(&sched.lock)`을 잡은 뒤 `globrunqputbatch`로 전역 run queue에 넣는다. 그 다음 head를 128 증가시켜 지역 run queue에는 128개만 남긴다.

이렇게 절반만 전역으로 보내는 이유는 한 P가 너무 많은 G를 독점하지 않게 하면서도, 완전히 지역성을 잃지 않게 하려는 절충이다. 전역 run queue는 모든 P가 공유하므로 크기가 커질수록 mutex 경합과 cache miss가 늘어난다. 반면 지역 run queue가 비어 있는 P는 `findrunnable`에서 `globrunqget`으로 전역 큐를 확인하거나, 다른 P의 지역 run queue에서 `runqsteal`로 절반을 훔쳐 온다. `runqsteal`도 절반만 가져오는 것은 victim P가 계속할 수 있게 남겨 두기 위한 균형이다.

P는 고정된 상태 기계를 가진다. `_Pidle`, `_Prunning`, `_Psyscall`, `_Pgcstop`, `_Pdead`가 대표적이다. M이 syscall에 들어가면 `reentersyscall`이 P를 `_Psyscall`로 바꾸고, syscall이 blocking이라면 P를 idle pool로 반환한다. syscall에서 돌아온 M은 `exitsyscall`에서 다시 P를 얻으려 시도하고, 없으면 M을 park한다. 이 흐름이 M 증가를 제한하면서 P를 계속 활용하는 핵심이다. `schedule`은 이 state transition을 계속 돌면서 G를 실행한다.

## 코드로 확인하기

`main.go`는 크게 세 가지를 출력한다. 처음에는 `runtime.GOMAXPROCS(0)`으로 P 수를, `runtime.NumGoroutine()`으로 G 수를, `pprof.Lookup("threadcreate").Count()`로 M 수의 근사치를 보여준다. 보통 초기 상태에서는 P 수가 M 수보다 크다. P는 실행 슬롯이므로 여러 개가 존재하지만, 실제로 OS thread는 G가 병렬로 돌기 전까지는 많이 필요하지 않기 때문이다. 이 시점에서 이미 GMP의 역할 분리를 확인할 수 있다.

다음으로 P를 1개로 제한하고 300개의 goroutine을 만든다. 각 goroutine은 생성 index를 받고, 실행 순서를 atomic counter로 기록한다. 주석에도 적었듯이 GOMAXPROCS=1이므로 한 번에 하나의 G만 실행된다. 출력에서 `생성 index 299 -> 실행 순서 1`을 볼 수 있다. 이는 마지막으로 만든 G가 P의 `runnext`에 있기 때문이다. `runqget`은 지역 runq보다 `runnext`를 먼저 꺼내므로 가장 최근에 만든 goroutine이 가장 먼저 실행된다.

`생성 index 128 -> 실행 순서 2`는 왜 나오는가? 300개를 만들다 보면 지역 runq가 256개로 가득 차는 시점이 온다. 이때 `runqputslow`가 runq의 앞쪽 절반인 0~127을 전역으로 보내고 head가 128로 이동한다. 따라서 지역 runq에 남은 첫 번째 G는 index 128이 된다. 그 뒤로는 129, 130, ... 순서로 실행된다. `생성 index 255 -> 실행 순서 129`는 128로 시작해 128개가 순서대로 실행됐을 때의 마지막인 255가 그 위치라는 뜻이다.

`생성 index 0 -> 실행 순서 172`는 전역 run queue에 갔던 batch의 head가 G0이기 때문이다. 지역 run queue가 모두 소진된 후 P가 `globrunqget`으로 전역 큐에서 꺼내면서 G0부터 실행된다. `생성 index 256 -> 실행 순서 300`은 전역 batch의 마지막이 G256이라는 사실을 보여준다. 전체 실행 순서가 생성 순서와 완전히 다르다는 점이 scheduler의 내부 동작을 관찰 가능하게 만든다.

마지막으로 P를 2개로 올리고 CPU-bound goroutine 2개를 동시에 실행한다. `osThreadCount`가 버스트 직전보다 직후에 증가하는 것을 볼 수 있다. 기존 M 하나로는 P 두 개의 병렬 실행 요구를 만족할 수 없으므로 런타임이 새 M을 만들어 두 번째 P를 acquirep한다. 이 출력은 G 자체가 스레드가 아니라 P가 병렬 실행의 단위이며, M은 필요할 때만 늘어나는 실제 OS 자원이라는 점을 직접 보여준다.

## 모르면 겪는 일

goroutine 생성 순서가 실제 실행 순서를 보장한다고 착각하면 재현하기 어려운 버그를 만든다. 특히 한 P에서 연속으로 goroutine을 만들면 마지막에 만든 G가 `runnext`로 들어가 LIFO로 실행된다. 예를 들어 요청을 채널로 보내고 그 순서대로 처리될 것이라고 기대했는데, GOMAXPROCS=1에서도 뒤에 도착한 요청이 먼저 실행되어 응답 순서가 뒤집힐 수 있다. 로그에는 요청 도착 순서와 응답 순서가 어긋나 보이지만 scheduler 문제인지 몰라 오래 헤매는 경우가 많다.

CPU-bound 작업이 섞인 서비스에서 GOMAXPROCS를 잘못 설정하면 p99 latency가 GC 주기마다 튀는데 CPU profile에는 user time만 보이고 scheduler time은 보이지 않는다. GOMAXPROCS=1로 두면 모든 G가 코어 하나에서 순서를 기다리므로 나머지 코어가 놀고, 반대로 코어 수보다 훨씬 크게 잡으면 P가 많아져 지역 run queue가 자주 비고, P끼리 steal을 많이 하면서 cache miss와 scheduler overhead가 늘어난다. 이 오버헤드는 profile에서 잘 안 보이기 때문에 원인을 파악하기 어렵다.

한 P에서 256개 이상의 G를 burst로 만들면 절반이 global run queue로 이동한다. global run queue는 lock으로 보호되므로 여러 코어에서 이벤트가 동시에 몰리는 fan-out/fan-in 패턴에서 bottleneck이 된다. 특히 네트워크 수신 goroutine 하나가 수천 개의 작업을 만들어 내는 구조에서는 지역 run queue overflow가 전역 큐 경합을 유발하고, 각 작업이 다른 P로 흩어지면서 캐시 지역성이 깨진다. goroutine을 아주 잘게 쪼개서 동시성을 높이면 오히려 느려지는 이유가 여기에 있다.

`LockOSThread`를 남발하면 G와 M이 강하게 묶이면서 P의 장점이 퇴색된다. lock을 건 G는 특정 M에서만 실행되어야 하므로, 그 M이 syscall이나 park 상태에 들어가도 재사용되지 못하고 다른 P가 실행되지 못하는 상황이 생길 수 있다. 너무 많은 G에 `LockOSThread`를 걸면 M 수가 비정상적으로 늘어나고 OS 스레드 context switch 비용이 커진다. `UnlockOSThread`를 빼먹으면 M이 재사용되지 못해 자원 누수처럼 보이기도 한다.

## 언제 신경 쓰고 언제 무시하나

일반적인 서비스에서는 이 지식을 몰라도 잘 돌아간다. CPU-bound 작업이 지배적이지 않고 동시 실행 goroutine 수가 수백 개 미만이라면 지역 run queue가 256개를 넘어 전역으로 넘어갈 일도 적고, work stealing이 성능에 직접 보일 만큼 일어나지도 않는다. GOMAXPROCS를 기본값인 `runtime.NumCPU()`로 두고 goroutine을 너무 잘게 쪼개지 않는 선에서 큰 문제 없이 동작한다.

신경 써야 하는 조건은 명확하다. CPU-bound goroutine이 코어 수보다 훨씬 많거나, 특정 이벤트 루프 하나가 한 번에 수백 개 이상의 G를 만들어 내는 경우에는 GMP의 queue 구조가 성능에 영향을 준다. 이때는 worker pool로 동시 실행 수를 GOMAXPROCS 부근으로 제한하거나, batch 크기를 조절해 지역 run queue가 계속 overflow되지 않도록 만드는 편이 낫다. GOMAXPROCS는 CPU-bound 작업에서는 코어 수와 비슷하게, blocking I/O가 많은 작업에서는 조금 더 크게 주는 것이 일반적이다.

이 주제를 과하게 신경 쓰면 금방 과최적화에 빠진다. 아직 프로파일에 scheduler time이나 blocking time이 잡히지 않는데 G 수를 수동으로 배치하거나 `GOMAXPROCS`를 미세 조정하는 것은 거의 의미가 없다. 병목이 DB 쿼리나 I/O wait라면 scheduler를 만져도 성능은 그대로다. `GODEBUG=schedtrace=1000`, `GODEBUG=scheddetail=1`, `go tool trace`로 scheduler 동작을 관찰할 수 있지만, 측정 없이 이 값들만 보면서 판단하지 말아야 한다.

## 더 파보기

- https://go.dev/src/runtime/proc.go — runqput, runqget, runqputslow, findrunnable, work stealing이 구현된 핵심 파일
- https://go.dev/src/runtime/runtime2.go — g, m, p, schedt 자료구조 정의
- https://go.dev/src/runtime/trace.go — scheduler trace 포인트와 실행 흐름
- https://go.dev/doc/go1.1#runtime — Go 1.1 release note에서 P 도입과 scheduler 개편 언급
- https://morsmachine.dk/go-scheduler — Go scheduler 내부를 초보자 관점에서 설명한 글