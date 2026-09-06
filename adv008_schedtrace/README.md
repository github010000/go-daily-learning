## 한 줄 요약

GODEBUG=schedtrace 는 런타임 스케줄러의 전역 상태를 1초 단위로 찍어 주는 창이고, runtime/trace 는 goroutine 한 개 한 개의 상태 전이를 이벤트로 기록해 `go tool trace` 로 분석하는 현미경이다.

## 왜 이런 설계인가

goroutine 이 수만 개가 되어도 OS 스레드는 수십 개뿐이다. 개발자가 `fmt.Println("start goroutine")` 같은 로그를 넣어 보면, 로그 출력 순서는 실제 실행 순서가 아니라 스케줄러가 언제 그 goroutine 을 선택했는지에 따라 뒤섞인다. 어떤 goroutine 이 CPU 를 오래 잡고 있는지, 어떤 goroutine 이 channel 이나 syscall 에서 오래 멈춰 있는지는 로그만으로는 알 수 없다. 스케줄러 문제는 CPU 프로파일에서도 잘 안 보인다. CPU 프로파일은 "어느 함수가 CPU 시간을 많이 썼는가"는 보여 주지만 "goroutine 이 run queue 에서 50ms 대기한 뒤 실행되었다"는 대기 시간을 보여 주지 않는다.

Go 팀은 이 문제를 두 가지 계층으로 풀었다. 하나는 저렴한 주기적 요약이다. `GODEBUG=schedtrace=1000` 을 주면 런타임이 매 초 스케줄러의 전역 상태를 stderr 에 한 줄씩 남긴다. 로그 한 줄은 몇십 바이트에 불과하므로 장시간 운영 중인 프로세스에서도 부담이 거의 없다. 하지만 1초 평균이므로 50ms 만에 사라지는 run queue 급증 같은 짧은 사건은 놓칠 수 있다. 또 goroutine 개별 정체는 전혀 보이지 않는다.

다른 하나는 execution tracer 다. `runtime/trace` 는 goroutine 생성, 블로킹, 언블로킹, syscall 진입/복귀, GC 단계, 프로세서(P) 할당 같은 이벤트를 ring buffer 에 기록한다. 이 데이터는 `go tool trace` 로 시각화하면 goroutine 한 개의 수명 동안 상태가 `Runnable`, `Running`, `Waiting`, `Syscall` 중 어디에 있었는지 시간축으로 그려진다. 단점은 기록량이 많고, 프로세스에 따라 CPU 오버헤드가 몇 퍼센트까지 올라간다는 점이다. 그래서 항상 켜 두는 도구가 아니라, 증상이 보일 때 잠깐 켜서 원인을 찾는 현미경으로 설계되었다.

왜 OS 스레드 풀을 직접 관리하지 않고 이런 관측 도구가 필요할 정도로 복잡한 스케줄러를 가졌는가. OS 스레드 생성은 수 마이크로초에서 수십 마이크로초가 걸리고, 스택은 보통 1MB 이상을 미리 잡는다. 반면 goroutine 은 수 KB 스택으로 시작하고 필요하면 늘어난다. 이렇게 수십만 개의 goroutine 을 만들려면 OS 스레드 하나에 하나씩 매핑하는 1:1 모델은 불가능하다. Go 런타임은 M:N 모델을 쓴다. M은 OS 스레드, P는 CPU 코어마다 하나씩 존재하는 context, G는 goroutine 이다. G 가 실행되려면 P 에 붙어야 하고, P 는 M 에 붙어야 한다. 이런 구조에서 생기는 `runqueue`, `idleprocs`, `spinningthreads` 같은 개념은 OS 스레드만 쓰던 세계에는 없던 것들이다.

## 어떻게 동작하는가

schedtrace 는 `runtime/proc.go` 의 `schedtrace` 함수가 주기적으로 호출되며 출력한다. 이 함수는 `sched` 라는 런타임 전역 구조체에서 `gomaxprocs`, `idleprocs`, `threads`, `spinningthreads`, `needspinning`, `idlethreads`, `runqsize` 를 읽는다. 또 각 P 의 로컬 run queue 길이를 `[0 1 0 ...]` 형태로 출력한다. `GOMAXPROCS` 는 P 의 개수다. `idleprocs` 는 그 시점에 실행 가능한 G 를 하나도 갖고 있지 않은 P 의 개수다. `threads` 는 현재 존재하는 OS 스레드(M)의 수다. `spinningthreads` 는 일감을 찾으려고 CPU 를 쓰며 spin 중인 M 의 수다. `idlethreads` 는 잠들어 있는 M 의 수다. `runqueue` 는 전역 run queue 에 쌓인 G 의 개수인데, per-P run queue 에 비해 접근은 느리지만 P 사이 일감 훔치기(work stealing)와 부하 분산에 쓰인다. 마지막 `[ ]` 는 각 P 의 로컬 run queue 길이를 보여 주는데, 특정 P 만 과부하 상태라면 다른 P 들은 0이고 그 P 만 수십 개가 쌓인다.

execution tracer 는 `runtime/trace.go` 에서 이벤트를 수집한다. 사용자 코드는 `trace.Start(w io.Writer)` 를 호출해 수집을 시작하고 `trace.Stop()` 을 호출해 종료한다. 런타임은 P 별로 trace 버퍼를 두고, GC 나 네트워크 poller 같은 런타임 이벤트까지 event ID 로 기록한다. 중요한 점은 trace.Start 가 성능에 민감한 지역에서 호출되면 안 된다는 것이다. trace 는 사용자 goroutine 상태 전이와 런타임 내부 사건을 모두 기록하므로, trace 파일을 쓰는 I/O 자체가 스케줄링을 왜곡할 수 있다. 짧은 구간만 떠서 관찰해야 원래 동작에 가까운 데이터를 얻는다.

trace 파일은 `go tool trace trace.out` 으로 연다. 브라우저 UI 에서 goroutine analysis, network blocking profile, synchronization blocking profile, scheduler latency profile 을 볼 수 있다. 가장 유용한 화면 중 하나는 timeline viewer 다. 여기서 각 P 와 goroutine 이 시간축 위에 표시된다. `Runnable` 은 P 에 붙기 전 run queue 에 있는 상태, `Running` 은 실제 CPU 를 쓰는 상태, `Waiting` 은 channel 이나 sleep 등에서 멈춘 상태, `Syscall` 은 OS 커널 호출로 넘어간 상태다. 이 상태들이 톱니 모양으로 전이되는 것을 보면 어디서 병목이 생기는지 알 수 있다. schedtrace 는 이런 개별 상태를 주지 않는다. 단지 숫자 요약일 뿐이다.

## 돌려보기

이 디렉토리에서 아래 명령을 순서대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사. parse 함수와 atomic 변수 사용에 의심스러운 점이 있는지 본다
go build -o /dev/null ./...  # 컴파일 확인. runtime/trace import 가 실제로 문제없이 빌드되는지 본다
go run .                     # 시연 실행. 3초 CPU worker 후 channel burst, 마지막에 trace.out 생성
go test -v ./...             # 테스트. schedtrace 파싱과 worker pool 완료 개수 검증
go test -race ./...          # 동시성 코드가 race 없이 동작하는지 검사
```

schedtrace 를 직접 보려면 아래 명령을 실행한다.

```bash
GODEBUG=schedtrace=1000 go run .
```

그러면 `SCHED 1000ms: gomaxprocs=8 idleprocs=4 threads=7 ...` 형태의 줄이 1초마다 stderr 로 출력된다. `1000` 은 밀리초 단위 간격이다. `GOMAXPROCS` 를 바꿔도 보고, `GODEBUG=schedtrace=500` 으로 더 자주 찍어도 된다.

trace 파일을 만든 뒤에는 아래 명령으로 웹 UI 를 연다.

```bash
go tool trace trace.out
```

브라우저가 열리면 goroutine analysis 와 timeline viewer 를 확인한다. `runChannelBurst` 에서 50개 goroutine 이 동시에 waiting 이었다가 close 직후 runnable 로 바뀌는 모습이 보인다.

## 코드로 확인하기

main.go 의 `runCPUWorkers` 는 CPU-bound goroutine 을 GOMAXPROCS 의 두 배만큼 만든다. 예를 들어 8코어 머신이면 16개 goroutine 이 생긴다. P 는 8개뿐이므로 8개는 running, 나머지 8개는 runnable 상태로 run queue 에 머문다. 이때 `GODEBUG=schedtrace=1000` 출력을 보면 `idleprocs` 는 0에 가깝고, `[0 0 0 0 1 2 0 ...]` 같은 per-P run queue 에 0이 아닌 값이 보인다. `runqueue` 는 전역 run queue 이름이지만 실제 goroutine 은 가능하면 로컬 run queue 에 먼저 들어간다. 숫자가 0이 아니라 1, 2 정도로 보이는 이유는 work stealing 이 빠르게 일감을 분배하기 때문이다.

`runChannelBurst` 는 200개 goroutine 이 `start` channel 에서 모두 `<-start` 로 waiting 상태에 들어가게 한다. main 이 `close(start)` 를 호출하면 channel 은 대기자 전원에 닫힘을 알린다. 그러면 200개 goroutine 이 동시에 runnable 이 되고 스케줄러는 하나씩 P 에 올려 실행한다. 이 짧은 폭주는 schedtrace 1초 요약으로는 잘 안 보이지만, execution tracer 파일에서는 명확하게 나타난다. `go tool trace trace.out` 의 timeline viewer 에서 세로로 나란히 `Runnable -> Running -> Done` 으로 이어지는 띠를 볼 수 있다.

parseSchedtraceLine 은 schedtrace 한 줄을 regexp 로 분해해 숫자 필드를 추출한다. `schedTraceLineRe` 는 실제 runtime 출력 형식과 맞아야 하므로 형식이 바뀌면 테스트가 실패한다. main_test.go 의 `TestParseSchedtraceLine` 은 `gomaxprocs=8 idleprocs=4 threads=7` 같은 알려진 줄을 넣고 각 필드가 예상한 정수로 변환되는지 확인한다. `TestRunSchedulerDemoCompletesAllTasks` 는 worker pool 에 100개 task 를 주고 4개 worker 가 전부 처리하면 `Completed == 100` 인지 검증한다. 시간 제한 없이 완료 개수만 보므로 느린 CI 에서도 안전하다.

## 모르면 겪는 일

schedtrace 필드를 모르고 보면 `idleprocs` 가 높은 게 좋은 것인지 나쁜 것인지 혼동한다. 실제로는 `idleprocs` 가 높으면서도 전체 CPU 사용률이 높은 경우, 일감이 고르게 분산되지 않고 특정 P 에만 쏠려 있는 것일 수 있다. 예를 들어 8개 P 중 7개는 `idleprocs=7` 인데 어떤 goroutine 하나가 특정 channel 을 오래 점유하면, 나머지 P 들은 놀고 있는데도 p99 지연은 높게 나온다. CPU 프로파일에는 이 goroutine 이 CPU 를 조금 쓰는 것으로 보이므로 원인이 잘 안 보인다.

execution tracer 없이 로그만 추가하면 재현되지 않는 지연이 자주 생긴다. 예를 들어 특정 요청이 가끔 2초씩 멈추는 장애가 있다고 하자. `log.Println` 을 함수 시작과 끝에 넣으면 로그상으로는 2초 차이가 보이지만, 그 goroutine 이 그 2초 동안 channel 수신에서 waiting 중이었는지, run queue 에서 밀렸는지, syscall 에 있었는지는 구분할 수 없다. trace 를 떠 놓으면 정확히 그 2초 구간의 상태가 `Waiting` 인지 `Runnable` 인지 `Syscall` 인지 보인다. 원인에 따라 해결 방법이 완전히 다르기 때문에 상태 전이를 보지 못하면 엉뚱한 곳을 최적화하게 된다.

trace 를 너무 길게 수집하면 파일이 수 기가바이트가 되고 `go tool trace` 가 열리지 않거나 브라우저가 멈춘다. 운영 중인 프로세스에서 10초만 수집해도 이벤트 수는 수십만 개가 될 수 있다. 대부분의 장애는 짧은 재현 구간에서 포착되므로, trace 는 5초 이내로 제한하는 것이 좋다. 초 단위로 파일 크기를 확인하고, 미리 `debug.SetTraceback` 같은 것은 필요 없다.

race detector 와 execution tracer 는 함께 쓸 수 없다. `go test -race` 로 실행되는 테스트 안에서 `trace.Start` 를 호출하면 runtime panic 이 발생할 수 있다. 따라서 trace 수집 로직은 `-race` 테스트에 노출하지 않는 편이 안전하다. 이 예제에서도 main.go 의 `runTraceDemo` 는 실제 실행에서만 호출하고, `go test -race` 에서는 호출하지 않는다.

## 언제 신경 쓰고 언제 무시하나

goroutine 이 수십 개 수준이고 CPU 코어가 4개 미만인 소규모 서비스에서는 schedtrace 의 숫자를 들여다볼 필요가 거의 없다. 병목이 생겨도 함수 프로파일이나 로그만으로 대부분 원인이 보인다. execution tracer 도 마찬가지다. 간단한 CLI 도구나 배치 프로그램에서 trace 파일을 만드는 것은 시간 낭비다. goroutine 상태 전이를 봐야 하는 문제는 보통 동시 요청 수가 많고, 서로 다른 goroutine 이 channel 과 lock 을 통해 얽히는 시점부터 생긴다.

규모가 커지면 신호가 분명해진다. 같은 worker pool 인데 트래픽이 늘면 p99 가 튀기 시작한다. CPU 사용률은 30%인데 응답은 느리다. 특정 시간대에만 goroutine 수가 수만 개로 늘어난다. 이런 상황에서 schedtrace 의 `runqueue` 와 `idleprocs` 를 보면 일감 분산 문제인지, worker 수 부족인지, 아니면 전혀 다른 I/O 병목인지 1차 분류가 된다. 그 다음 특정 요청 하나를 잡아 trace 로 깊게 들어가는 순서가 실제 디버깅 워크플로다.

schedtrace 는 1초 요약이므로 짧은 폭주가 의심되면 간격을 250ms 나 100ms 로 줄여 볼 수 있다. 하지만 출력량이 늘어난다. 운영 로그 수집기에 보내면 초당 수십 줄에 불과하므로 부담은 작다. trace 는 반대로 짧은 시간 동안 매우 높은 해상도를 제공하지만 오버헤드가 크다. p99 지연 문제를 찾을 때는 재현 환경에서 trace 를 3~5초 수집하는 것을 기본값으로 삼으면 된다. 이 둘은 경쟁 도구가 아니라 망원경과 현미경이다.

## 더 파보기

- https://go.dev/doc/diagnostics — Go 진단 도구 공식 문서. schedtrace, trace, pprof 를 한 번에 훑는다.
- https://pkg.go.dev/runtime/trace — runtime/trace 패키지 문서. trace.Start, Stop, Region, Task 사용법과 주의사항.
- https://github.com/golang/go/blob/master/src/runtime/proc.go — schedtrace 출력과 런타임 스케줄러 구현. `schedtrace` 함수에서 실제 필드 순서를 확인할 수 있다.
- https://github.com/golang/go/blob/master/src/runtime/trace.go — execution tracer 이벤트 기록 구현. trace 버퍼가 P 별로 어떻게 관리되는지 볼 수 있다.
- https://go.dev/s/go11sched — Go 스케줄러 설계 문서. G, M, P, run queue, work stealing 의 배경이 상세히 설명되어 있다.