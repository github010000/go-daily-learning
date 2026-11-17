## 한 줄 요약

`go tool pprof` 는 "CPU 를 실제로 쓴 곳"만 보여준다. 지연의 절반 이상은 CPU 를 쓰지 않고 보내는 시간이고, 그 시간은 다시 실행 중(running), 실행 대기(runnable), 블록(blocked: 채널, mutex, syscall), GC 로 성격이 갈린다. execution tracer 는 이 네 가지를 goroutine 별 타임라인으로 보여주는 유일한 기본 도구다. 이 자료는 같은 CPU 작업을 조건만 바꿔 네 번 돌려서 wall 시간이 어디로 새는지를 `runtime/metrics` 로 수치화하고, `runtime/trace` 로 파일에 남겨 `go tool trace` 에서 눈으로 확인하게 만든다.

## 왜 이런 설계인가

CPU 프로파일러의 원리는 단순하다. 100Hz 로 SIGPROF 을 던지고, 그 순간 CPU 위에서 돌고 있던 스택을 모은다. 그래서 이름이 "on-CPU profile" 이다. 이 설계의 대가가 명확하다. 프로그램이 CPU 를 안 쓰고 보내는 시간은 표본이 아예 잡히지 않는다. CPU 사용률 15퍼센트인데 p99 가 2초인 서비스에서 프로파일을 열면 화면이 조용하다. 이것은 pprof 의 버그가 아니라 설계다. 샘플링 프로파일러는 "느린 함수"를 찾는 도구이지 "느린 시간"을 찾는 도구가 아니다. 그리고 실무에서 p99 를 망치는 범인은 대개 함수가 아니라 시간이다.

Go 에서 goroutine 의 상태가 여러 개로 쪼개지는 이유는 스케줄러가 M 대 N 이기 때문이다. goroutine 은 G, OS 스레드는 M, 실행 컨텍스트는 P 다. G 가 실제로 명령을 실행하려면 P 를 손에 쥐어야 하고, P 의 개수는 GOMAXPROCS 로 고정된다. 그래서 "일할 준비가 다 됐는데 P 가 없어서 못 하는 시간"이 구조적으로 생긴다. OS 스레드 모델에서는 각 스레드가 커널 스케줄러에게서 CPU 를 받아가므로 이 지연이 사용자 공간에서 관측되지 않는다. Go 는 이 지연을 런타임이 직접 만들어내기 때문에 직접 측정할 수 있고, 동시에 직접 책임져야 한다. runnable 큐 대기는 Go 고유의 비용이고, Go 고유의 도구로만 보인다.

execution tracer 는 샘플링을 포기하고 이벤트 로깅을 택했다. 런타임 안의 상태 전이 지점, 즉 goroutine 이 생성되는 순간, P 에 올라가는 순간, 블록되는 순간, syscall 로 나가는 순간마다 기록을 남긴다. 정확도는 표본 확률이 아니라 이벤트 단위다. 대신 비용이 있다. 처리량이 높은 서버에서는 초당 수십만 건의 전이가 생기고, 그 전부를 기록하면 오버헤드가 무시할 수 없으며 버퍼가 차면 이벤트를 버린다. 그래서 상시 켜두는 도구가 아니다. Go 1.21 부터는 flight recorder 가 들어와서 "평소에는 조용히 돌다가 사고 순간의 최근 몇 초만 덤프"하는 운영 방식이 가능해졌다.

대안도 있다. 코드에 타이머를 직접 심는 계측은 정확하지만 커버리지가 좁고 프로덕션 코드에 흔적을 남긴다. 반대편에는 `runtime/metrics` 가 있다. 이쪽은 집계값이라 개별 goroutine 은 못 보지만 오버헤드가 사실상 없어서 상시 켜둘 수 있다. `/sched/latencies:seconds` 히스토그램이 대표적이다. 실무의 정석 조합은 이렇다. 상시 감시는 metrics 로 하고, 지표가 튀는 순간에만 tracer 를 짧게 켜서 원인을 분해한다. 이 자료의 코드가 정확히 그 구조를 따라간다.

## 어떻게 동작하는가

트레이서의 자료구조는 P 마다 붙은 `traceBuf` 다. 이벤트는 `traceEvent` 라는 고정 크기 레코드로 인코딩되어 per-P 버퍼에 배치로 쌓이고, 버퍼가 차면 중앙 저장소로 flush 된다. Go 1.22 이후에는 세대(gen) 개념이 들어가서, 읽는 쪽이 쓰는 쪽의 버퍼를 직접 건드리지 않는다. `runtime/trace/trace.go` 와 `runtime/trace/tracebuf.go` 를 보면 이 구조가 그대로 나온다. 트레이스를 끄면 `traceAdvance` 가 모든 P 의 버퍼를 비우고 파일을 마감한다.

기록되는 이벤트 이름은 상태 기계와 일대일이다. `GoCreate`, `GoStart`, `GoEnd`, `GoSched`, `GoPreempt`, `GoBlock`, `GoUnblock`, `GoBlockSync`, `GoBlockRecv`, `GoSysCall`, `GoSysExit`, `GoSysBlock`, `GCStart`, `GCDone`, `STWStart`, `STWDone`. 이 이름들을 알면 `go tool trace` 화면에서 회색 막대가 왜 생겼는지 스스로 읽어낼 수 있다. 회색이 `GoSysCall` 이면 커널에 있다 나온 것이고, 주황색이 `GoBlockSync` 면 mutex 대기다.

`/sched/latencies:seconds` 는 조금 다르게 만들어진다. goroutine 이 `ready()` 로 런 큐에 들어간 시각과 `execute()` 로 P 위에 올라간 시각의 차이를 P 별 히스토그램에 넣는다. 즉 이 지표 하나가 "스케줄 지연"의 분포 전체다. `runtime/metrics.go` 에서 노출하고, `runtime/proc.go` 의 `ready` 와 `execute` 주변에서 표본이 만들어진다. CPU 시간 쪽은 `/cpu/classes/user:cpu-seconds`, `/cpu/classes/gc/total:cpu-seconds`, `/cpu/classes/idle:cpu-seconds` 같은 클래스로 나뉘어 있다.

`go tool trace` 화면에서 가장 중요한 탭이 Goroutine analysis 다. 시작 스택이 같은 goroutine 들을 묶어서, 그 그룹이 평균적으로 실행에 얼마, runnable 대기에 얼마, 블록에 얼마, GC 에 얼마를 썼는지 표로 보여준다. 이 표의 행을 누르면 해당 구간으로 화면이 점프한다. pprof 에서는 절대 볼 수 없는 각도이고, "이 요청 처리 경로가 느리다"를 "이 요청 처리 경로가 어디서 기다린다"로 바꿔주는 지점이다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사. metrics 샘플 이름 오타도 여기서 걸러진다
go build -o /dev/null ./...  # 컴파일 확인. trace API 시그니처 변경을 잡는다
go run .                     # 시연 실행. 표 네 줄과 trace.out 이 생긴다
go test -v ./...             # 테스트. 분위수, 히스토그램 차분, 순서 불변식 검증
go test -race ./...          # 동시성 주제라면 반드시. convoy 단계가 race 없이 도는지
```

여기에 주제별 플래그를 더한다.

```bash
GODEBUG=schedtrace=1000 go run .   # 1초마다 런큐 길이와 P 상태를 텍스트로 덤프
go tool trace trace.out            # 실행 후 생성된 트레이스를 브라우저 UI 로 연다
go test -run=^$ -bench=. -benchmem ./...  # measure harness 자체의 오버헤드 측정
```

`GODEBUG=schedtrace=1000` 은 표준출력에 `SCHED` 줄을 뿌린다. `runqueue` 값이 계속 0 보다 크면 runnable 대기가 있다는 뜻이고, 표의 대기 열과 같은 현상을 다른 각도에서 보는 것이다. `go tool trace` 는 로컬 HTTP 서버를 띄우므로 브라우저가 필요하다. 서버 환경이라면 파일만 남겨두고 개발자 PC 로 가져가서 열어도 된다.

## 코드로 확인하기

표의 첫 줄은 기준선이다. P 가 충분할 때는 wall 이 짧고 `점유P` 가 GOMAXPROCS 에 가깝다. 두 번째 줄은 같은 CPU 작업을 P=2 로 줄여서 돌린 결과다. 여기서 봐야 할 것은 세 가지다. 첫째, `userCPU` 가 1단계와 비슷하다. 일의 양이 같기 때문이다. 둘째, `wall` 이 늘었다. 셋째, `대기p99` 가 1단계보다 커졌다. 늘어난 wall 은 일이 아니라 대기였고, 대기의 정체를 `대기p99` 열이 밀리초 단위로 알려준다. CPU 프로파일은 이 상황에서 여전히 "CPU 100퍼센트 사용 중"이라고 말한다. 프로파일만 보면 아무 문제가 없어 보인다.

세 번째 줄은 mutex 경합이다. `wall` 은 오히려 2단계보다 클 수 있는데 `대기p99` 는 작다. 이 조합이 진단의 핵심이다. 기다리는 goroutine 이 runnable 이 아니라 blocked 이기 때문이다. Go 의 mutex 는 경합이 심해지면 `semacquire` 로 goroutine 을 재우고, 깨어날 때 handoff 를 한다. 그 시간은 `GoBlockSync` 로 기록되고 `sched latencies` 에는 안 들어간다. 즉 "대기 지표는 조용한데 wall 이 크다"는 것은 runnable 큐가 아니라 락이나 채널이나 syscall 을 봐야 한다는 신호다.

네 번째 줄은 할당 폭풍이다. `GCCPU` 열이 wall 중 GC 가 가져간 몫이다. 이 값이 커지면 CPU 프로파일 상위에 `runtime.gcBgMarkWorker` 나 `runtime.mallocgc` 가 올라오는데, 그걸 보고 "런타임이 느리다"고 결론 내리면 틀린다. 내 코드의 할당량이 만든 결과이므로 처방은 할당 줄이기다. `샘플` 열은 그 구간에 몇 개의 runnable 대기 표본이 있었는지다. 표본이 0 이면 p50 도 p99 도 0 으로 나오는데, 이건 "대기가 없었다"는 뜻이지 "측정 실패"가 아니다. 이 구분을 못 하면 표를 오독한다.

`main_test.go` 는 네 가지를 본다. 분위수 계산이 손으로 계산한 값과 일치하는지, 누적 히스토그램 차분이 맞는지, fanout 이 모든 인덱스를 정확히 한 번씩 실행하는지, measure 의 출력이 항상 지켜야 하는 순서(p99 가 p50 이상, max 가 p99 이상)를 만족하는지. 절대 시간은 기계마다 다르므로 단정하지 않는다. 대신 구조적으로 참인 관계만 검사해서 CI 부하와 무관하게 통과한다.

## 모르면 겪는 일

CPU 프로파일이 깨끗한데 p99 만 튀는 상황을 만난다. 프로파일을 열면 상위 함수가 전부 5퍼센트 미만이고 총합도 낮다. 여기서 "특별한 핫스팟이 없으니 코드는 문제 없다"고 결론 내리면 원인을 영원히 못 찾는다. 실제로는 goroutine 이 runnable 큐에서 줄 서 있거나 GC assist 로 멈춰 있는 것이고, 그 시간은 CPU 프로파일에 존재하지 않는다.

`GOMAXPROCS` 를 컨테이너 CPU limit 에 맞춰 2 로 줄였더니 처리량이 반으로 줄고 p99 가 세 배가 되는 경우가 있다. 이때 `대기p99` 는 급증하는데 CPU 사용률 지표는 오히려 내려간다. 노드의 실제 코어는 남아 있는데 Go 런타임이 스스로 P 를 2개로 제한했기 때문이다. 반대 방향의 사고도 있다. 컨테이너에 8 코어를 줬는데 quota 를 2 코어로 걸어두면, `GOMAXPROCS=8` 인 런타임이 8 개의 P 를 열심히 돌리다가 커널에 의해 쓰로틀링된다. 이때 트레이스에는 실행 구간이 정상으로 보이는데 벽시계만 늘어나는, 트레이서에도 안 잡히는 구간이 생긴다.

mutex 를 `sync.RWMutex` 로 바꿨는데 더 느려지는 경우도 흔하다. 쓰기 쪽 convoy 가 읽기 쪽으로 옮겨간 것뿐인데, 지표상으로는 "락 대기"가 그대로다. convoy 는 락 종류를 바꿔서 해결되는 문제가 아니라 임계 구역을 쪼개거나 큐로 바꿔야 하는 문제다. 세 번째 시연 단계가 이 형태를 그대로 재현한다.

syscall 이 많은 서비스에서 goroutine 수는 적은데 지연이 큰 경우도 있다. `GoSysCall` 구간은 트레이스에는 보이지만 CPU 프로파일에는 안 잡히고, `/cpu/classes/user` 에도 일부만 반영된다. 파일 쓰기나 DNS 조회처럼 커널에서 시간을 보내는 작업이 요청 경로에 끼어 있으면, 코드를 아무리 최적화해도 p99 는 안 움직인다. 트레이스에서 회색 막대가 요청 구간의 절반을 차지하는 것을 보고 나서야 원인이 보인다.

## 언제 신경 쓰고 언제 무시하나

지연 목표가 밀리초 단위이고 p99 가 그 목표를 넘나들 때만 이 도구를 꺼내라. 하루에 몇 번 도는 배치 작업이나, p99 가 수백 밀리초여도 되는 내부 관리 도구라면 스케줄 지연 몇 밀리초는 아무 의미가 없다. 그런 곳에 트레이서를 붙이면 오버헤드만 늘고 얻는 것이 없다.

판단 기준은 `점유P` 다. 측정 구간의 평균 점유 P 가 GOMAXPROCS 의 절반도 안 되는데 wall 이 길다면, 코어가 남는데 goroutine 이 못 도는 상태이므로 스케줄 지연을 볼 가치가 있다. 반대로 점유 P 가 GOMAXPROCS 에 붙어 있다면 기계를 실제로 다 쓰고 있는 것이므로 트레이서가 아니라 알고리즘이나 할당을 봐야 한다. 이 한 줄로 도구 선택이 갈린다.

표본 수(`샘플` 열)도 같이 봐야 한다. 측정 구간이 짧으면 runnable 대기 표본이 한 자릿수로 나오고, 그 위에서 계산한 p99 는 통계가 아니라 잡음이다. 표본이 수백 개 이상 쌓이도록 구간을 잡거나, 그냥 `GODEBUG=schedtrace` 로 런큐 길이 추세만 보는 편이 낫다.

마지막으로, 이 모든 것은 한 번의 최적화 라운드에서만 유효하다. 병목을 하나 없애면 다음 병목이 드러나고 지표 모양이 완전히 달라진다. 대기 지표가 낮아졌다고 끝난 것이 아니라, 그때 표를 다시 뽑아서 어느 열이 새로 커졌는지 확인해야 한다.

## 더 파보기

- Go 공식 진단 가이드: https://go.dev/doc/diagnostics
- `runtime/trace` 패키지 문서와 flight recorder: https://pkg.go.dev/runtime/trace
- `runtime/metrics` 지표 전체 목록, `/sched/latencies:seconds` 포함: https://pkg.go.dev/runtime/metrics
- Go 블로그, execution trace 의 2024 년 개편: https://go.dev/blog/execution-traces-2024
- 트레이서 내부 구현: `src/runtime/trace.go`, `src/runtime/trace/tracebuf.go`, `src/runtime/metrics.go`, `src/runtime/proc.go` 의 `ready` 와 `execute`