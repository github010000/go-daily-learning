## 한 줄 요약

Go 런타임은 blocking syscall 에 들어가기 전에 현재 실행 주체인 goroutine 과 P 의 상태를 바꿔서, 그 syscall 이 오래 걸리더라도 다른 goroutine 이 P 를 계속 쓸 수 있게 만든다. 이 실험은 `syscall.Select` 같은 안전한 syscall wrapper 가 `runtime.entersyscall` 을 호출하고, `sysmon` 이 10ms 뒤 P 를 회수해 `spinCounter` goroutine 이 도는 모습을 보여준다.

반대로 `syscall.RawSyscall6` 로 같은 select syscall 을 직접 호출하면 `runtime.entersyscall` 없이 커널에 들어가므로, P 가 `_Prunning` 상태로 M 에 묶인 채 블로킹된다. 이때는 `sysmon` 도 P 를 회수하지 못하고, GOMAXPROCS=1 이면 CPU 를 쓸 수 있는 goroutine 이 그냥 굶는다.

따라서 "blocking syscall 이네? 무조건 막히니까 다른 goroutine 이 돌겠지"라고 생각하면 안 되고, 정확히는 "Go runtime 이 그 syscall 을 blocking syscall 로 인지했느냐"가 관건이다. `RawSyscall` 은 그 인지 과정을 건너뛴다.

## 왜 이런 설계인가

OS thread 하나에 goroutine 하나를 올리는 1:1 모델은 커널 스케줄링, 스택 할당, 생성 비용 때문에 Go 가 만들고 싶었던 동시성 모델이 아니다. Go 는 M 대 P 대 G 모델에서 M 은 실제 OS thread, P 는 CPU 를 사용할 권리, G 는 실행 단위로 두고, P 개수를 GOMAXPROCS 로 제한해 CPU 개수만큼만 동시 실행되도록 설계했다.

그런데 goroutine 이 syscall 을 호출하면 어쩔 수 없이 커널 모드로 내려가서 M 자체가 블로킹된다. 이때 만약 P 가 M 에 붙은 채로 있으면, 그 syscall 이 끝날 때까지 CPU 하나가 통째로 노는 셈이다. GOMAXPROCS 가 8 이면 8개 P 중 하나가 막혀서 그동안 실제로는 7개 P 만 일하게 된다.

이 문제를 풀기 위한 대안으로 모든 I/O 를 non-blocking 으로 바꾸고 netpoll 이나 이벤트 루프에 맡기는 방법이 있다. 하지만 파일 I/O, 일부 시스템 콜, 커널 sleep 같은 것은 non-blocking 으로 만들 수 없거나 플랫폼마다 정상 동작이 어렵다. 따라서 Go runtime 은 "어쩔 수 없이 막히는 syscall"을 아예 지원하는 방향을 택했다.

이 지원의 핵심이 P handoff 다. syscall 에 들어가기 전에 runtime 이 P 의 상태를 `_Prunning` 에서 `_Psyscall` 로 바꾸고, sysmon 이 일정 시간이 지나면 그 P 를 M 에서 떼어내 다른 M 에 붙여준다. 이렇게 하면 blocking syscall 하나가 P 하나를 계속 점유하지 않게 된다.

이 설계에는 비용도 있다. entersyscall/exitsyscall 전환, sysmon 의 주기적 감시, P 를 다른 M 에 넘기는 경합 등이 매 syscall 마다 작지만 쌓인다. 그런데 이 비용은 "blocking syscall 때문에 CPU 가 놀고 tail latency 가 튀는" 문제보다 훨씬 작고 예측 가능하다.

## 어떻게 동작하는가

`goodBlockingSyscall` 이 `syscall.Select` 를 호출하면, `syscall` 패키지의 `Select` 구현은 `Syscall6(SYS_SELECT, ...)` 같은 내부 wrapper 를 부른다. 이 wrapper 는 `runtime.entersyscall` 을 먼저 호출하고 실제 SYSCALL 명령을 수행한 뒤 `runtime.exitsyscall` 을 호출한다. 관련 어셈블리는 `syscall/asm_linux_amd64.s` 나 플랫폼별 syscall 파일에 있다.

`runtime.entersyscall` 은 `runtime/proc.go` 를 보면 현재 goroutine 의 상태를 `_Grunning` 에서 `_Gsyscall` 로 바꾸고, `m.locks` 를 올리고, P 의 `syscallwhen` 을 현재 시각으로 기록해 둔다. P 의 상태도 `_Prunning` 에서 `_Psyscall` 로 전환된다. 이 상태가 sysmon 이 "P 가 syscall 에 들어가 있다"고 판단하는 근거다.

`sysmon` 은 특별한 백그라운드 스레드로, 자기 자신에게 P 를 붙이지 않고 주기적으로 `retake(now)` 를 실행한다. `retake` 는 모든 P 를 순회하며 `p.status == _Psyscall` 인 경우 `syscallwhen + 10ms` 보다 시간이 더 지났는지 확인한다. 10ms 가 넘었으면 `atomic.Cas` 로 P 를 `_Pidle` 로 바꾸고 `handoffp(p)` 를 호출한다.

`handoffp` 는 `runtime/proc.go` 에 있으며, 해당 P 의 runq 나 전역 runq 에 실행 가능한 goroutine 이 있으면 `startm` 을 호출해 다른 M 을 깨우거나 새 M 을 만든다. 새 M 은 그 P 를 받아서 runq 의 goroutine 을 실행한다. 이 실험에서는 main goroutine 이 syscall 에 들어간 사이에 `spinCounter` goroutine 이 그 P 를 받아 돈다.

`badBlockingSyscall` 은 `syscall.RawSyscall6` 로 같은 `SYS_SELECT` 를 직접 호출한다. `RawSyscall6` 는 entersyscall/exitsyscall 경로를 거치지 않도록 만들어진 raw syscall 진입점이다. 그래서 runtime 은 이 goroutine 이 여전히 실행 중이라고 생각하고, P 상태도 `_Prunning` 으로 남는다.

`retake` 는 `_Prunning` 상태의 P 를 "막혀 있다"고 보지 않는다. 물론 긴 실행 시간에 대한 선점 메커니즘은 있지만, 지금 상황은 goroutine 이 Go 코드를 실행하는 것이 아니라 M 과 함께 커널에서 블로킹된 상태다. 따라서 sysmon 은 이 P 를 정상적인 실행 중인 P 로 보고 그냥 둔다. 결과적으로 GOMAXPROCS=1 이면 P 하나가 통째로 멈춘 것처럼 된다.

## 돌려보기

```bash
go vet ./...                 # raw syscall 트랩 상수와 unsafe 포인터 사용이 대상 플랫폼에서 옳은지 확인한다
go build -o /dev/null ./...  # 패키지가 실제로 컴파일되는지 확인한다
go run .                     # good 은 카운트가 크고 bad 는 0이 되는 것을 관찰한다
GODEBUG=schedtrace=1000,scheddetail=1 go run .  # P 상태가 _Psyscall/_Prunning 으로 갈리는 흔적을 본다
go test -v ./...             # 반환, 카운터 증가, 종료 테스트가 통과하는지 확인한다
go test -count=10 ./...      # 시간에 민감한 단정이 없어 반복 실행에도 안정적인지 확인한다
go test -race ./...          # atomic 카운터와 채널 동기화에 race 가 없는지 확인한다
GOOS=darwin go vet ./...     # macOS syscall 시그니처로 교차 vet 을 통과하는지 확인한다
```

`go run .` 에서 `good` 항목은 0보다 훨씬 큰 값이, `bad` 항목은 0이 나와야 한다.
`GODEBUG=schedtrace=1000,scheddetail=1` 을 주면 각 P 의 상태가 주기적으로 출력되는데, good 경로는 `_Psyscall` 로 잠깐 보인 뒤 다른 M 이 붙는 흔적을 남기고, bad 경로는 `_Prunning` 으로 계속 멈춰 있는 것처럼 보인다.

## 코드로 확인하기

`main.go` 는 `measureProgress` 에서 `runtime.GOMAXPROCS(1)` 로 P 를 하나만 만든다. 그 다음 `spinCounter` goroutine 을 만들어 두고, main goroutine 이 바로 `block(d)` 를 호출한다. worker 를 먼저 실행시키는 `runtime.Gosched` 를 일부러 넣지 않았기 때문에, main 이 syscall 에 들어가기 전에는 worker 가 실행될 기회가 없다.

`goodBlockingSyscall` 의 경우 main 이 `syscall.Select` 에 들어가면서 runtime 이 P 를 `_Psyscall` 로 바꾼다. 10ms 뒤 sysmon 이 P 를 회수해 worker goroutine 에게 주므로, 남은 90ms 동안 worker 가 `atomic.AddInt64` 로 count 를 계속 올린다. 따라서 출력의 good 값은 수천 이상, 시스템 속도에 따라 수십만까지 커질 수 있다.

`badBlockingSyscall` 의 경우 main 이 `RawSyscall6` 로 들어가도 runtime 은 여전히 P 가 main goroutine 을 실행 중이라고 본다. P 상태는 `_Prunning` 이고 sysmon 은 회수하지 않는다. 100ms 동안 worker 는 단 한 번도 실행되지 않으므로 count 는 정확히 0 으rob이 남는다. main 이 syscall 에서 돌아온 뒤 `close(stop)` 를 하고 `<-done` 으로 기다리면서 worker 가 실행될 수 있지만, worker 는 그때 이미 닫힌 stop 을 보고 바로 return 하므로 count 를 올리지 않는다.

`main_test.go` 는 시간 단정 없이 두 syscall 경로가 정상 반환하는지, `spinCounter` 가 실제로 count 를 올리고 stop 을 받고 종료하는지를 검증한다. 성능 비교를 원하면 `go test -bench . -benchmem` 을 실행해 `BenchmarkGoodBlockingSyscall` 과 `BenchmarkBadBlockingSyscall` 의 syscall 비용 차이를 직접 볼 수 있다.

## 모르면 겪는 일

GOMAXPROCS=1 이거나 컨테이너에서 CPU limit 이 걸려 P 가 하나뿐인데, goroutine 하나가 `RawSyscall` 로 blocking syscall 을 실수로 호출하면 프로세스 전체가 그 syscall 시간 동안 멈춘다. health check timeout 이나 readiness probe 실패가 뜨는데, CPU 사용률은 거의 0 에 가까워서 "왜 여유가 있는데 요청이 안 되지?" 같은 상황이 된다.

P 가 여러 개인 환경에서도 P 하나가 `_Prunning` 인 채로 막히면 전체 CPU 중 하나가 그 시간 동안만 사라진다. 이 증상은 지속적인 성능 저하가 아니라 특정 syscall 경로가 호출될 때만 p99 latency 가 튀는 패턴으로 나타난다. CPU profile 에는 해당 시간이 커널 쪽으로 빠지거나 아예 보이지 않을 수 있어 원인을 착각하기 쉽다.

이 문제는 직접 `syscall.RawSyscall` 을 쓰지 않아도, 내부적으로 raw syscall 을 쓰는 C library 를 cgo 로 호출하거나, syscall 번호를 직접 다루는 저수준 라이브러리를 사용할 때도 발생할 수 있다. 증상은 "goroutine 은 많은데 OS thread 가 실제로는 놀고 있고, 그런데 scheduler 는 굶은 goroutine 이 없다고 말하는" 모습으로 보인다.

특히 `syscall.Select`, `syscall.Nanosleep`, `syscall.Wait4` 같은 syscall package wrapper 들은 entersyscall 을 호출하기 때문에 안전하다. 하지만 `RawSyscall` 은 "이 syscall 은 절대 블로킹되지 않는다"는 가정이 필요한데, 그 가정을 모르고 select 나 sleep 류 syscall 을 넣으면 곧바로 P 를 잃는 버그를 만든다.

## 언제 신경 쓰고 언제 무시하나

일반 애플리케이션 코드가 `os.File.Read`, `net.Conn.Read`, `time.Sleep`, `syscall.Select` 같은 Go 표준 함수만 사용한다면 내부 handoff 동작을 몰라도 된다. runtime 이 이미 올바른 경로로 처리해 주고, 여기서 더 최적화하려고 `RawSyscall` 을 손으로 만지는 것이 오히려 사고를 낸다.

이 지식이 실전에서 중요해지는 순간은 직접 `syscall.RawSyscall` 혹은 `RawSyscall6` 를 쓰거나, cgo 로 만든 blocking call 을 Go 런타임에 통합할 때다. 아니면 P stuck 이 의심되는 장애를 조사할 때 `GODEBUG=schedtrace` 나 runtime trace 에서 특정 P 가 오래 `_Prunning` 으로 남아 있는지 확인해야 한다.

작은 CLI 도구나 배치 작업처럼 짧게 한 번 실행하고 마는 프로그램은 P 하나가 100ms 막혀도 큰 문제가 되지 않는다. 하지만 수천 RPS 이상을 받는 서비스, p99 를 밀리초 단위로 지켜야 하는 서비스, GOMAXPROCS 를 의도적으로 작게 잡은 서비스라면 이 동작을 알아야 한다.

기준을 정하자면, "내 코드가 raw syscall 진입점에 blocking 동작을 넣었는가?"가 아니면 대부분 무시해도 된다. 그리고 그 반대인 과최적화 사례로, handoff 오버헤드를 줄이겠다고 모든 syscall 을 `RawSyscall` 로 바꾸는 것은 P 를 잃는 리스크를 감수할 가치가 없다.

## 더 파보기

- Go 런타임 스케줄러의 실제 구현: https://go.dev/src/runtime/proc.go
- P, M, G 구조체와 상태 상수 정의: https://go.dev/src/runtime/runtime2.go
- syscall wrapper 와 raw syscall 구현: https://go.dev/src/syscall/syscall_linux.go
- syscall 의 어셈블리 진입점 예시: https://go.dev/src/syscall/asm_linux_amd64.s
- macOS syscall 시그니처 참고: https://go.dev/src/syscall/syscall_darwin.go