## 한 줄 요약

Go 런타임은 블로킹 syscall에 들어가는 M이 P를 반납하게 하고, sysmon이 10ms 이상 _Psyscall 상태에 머무는 P를 강제로 회수하여 다른 M이 고루틴을 실행하게 함으로써 CPU를 낭비하지 않는다.

## 왜 이런 설계인가

Go의 스케줄러는 GOMAXPROCS 개수만큼의 P(processor)를 가지고, 각 P는 실행 가능한 고루틴을 담는 runqueue를 가진다. M(OS 스레드)은 P를 획득해야만 고루틴을 실행할 수 있다. OS 스레드는 생성과 컨텍스트 스위치 비용이 크기 때문에, 가능한 한 M의 수를 P의 수와 비슷하게 유지하고, 고루틴은 P를 통해서만 실행된다. 그런데 블로킹 syscall(파일 읽기, 네트워크 연결 대기 등)은 OS 스레드 전체를 블로킹시킨다. 만약 M이 P를 붙잡은 채 syscall에 들어가면, 그 P는 syscall이 끝날 때까지 어떤 고루틴도 실행하지 못하게 되고, 다른 실행 가능한 고루틴이 있어도 CPU를 놀리게 된다. 이 문제를 해결하기 위해 Go 런타임은 syscall 진입 시 M이 P를 반납하도록 설계했다.

하지만 P를 반납하는 정책은 세부적으로 트레이드오프가 있다. syscall이 매우 짧은 경우(예: 1마이크로초 미만), P를 완전히 idle 목록에 넣고 다른 M에게 넘기는 것은 오히려 오버헤드를 만든다. P를 넘겨받는 과정은 잠금과 큐 조작을 포함하므로, syscall이 끝난 M이 다시 P를 얻으려면 경합이 발생한다. 그래서 Go는 syscall에 들어갈 때 P를 즉시 해제하지 않고 _Psyscall 상태로 표시해 둔다. 그리고 syscall에서 돌아올 때 그 P를 다시 사용할 수 있도록 M과 P의 연결을 잠시 유지한다. 만약 syscall이 오래 지속되면, 그 P는 _Psyscall 상태로 남아 낭비되므로, 별도의 감시 스레드인 sysmon이 개입하여 10ms 이상 _Psyscall에 머문 P를 강제로 회수한다. 이렇게 하면 짧은 syscall은 저비용으로 처리하고, 긴 syscall은 P를 다른 고루틴에게 양보할 수 있다.

대안으로 모든 syscall을 논블로킹으로 처리하는 방법이 있다. 네트워크 I/O는 이미 netpoller를 통해 논블로킹으로 전환되었지만, 파일 I/O나 다른 syscall은 커널에서 논블로킹을 지원하지 않거나 복잡하다. 모든 syscall을 논블로킹으로 바꾸려면 커널과의 상호작용이 필요하고, 일부 syscall은 본질적으로 블로킹일 수밖에 없다(예: flock, futex). 또 다른 대안은 블로킹 syscall을 전용 OS 스레드에서 실행하는 것이지만, 이는 스레드 생성/파괴 비용과 컨텍스트 스위치 비용이 크고, P를 넘겨주는 것보다 더 무겁다. 현재 Go의 접근 방식은 syscall의 지속 시간에 따라 적응적으로 동작하므로, 다양한 워크로드에서 좋은 성능을 보인다.

## 어떻게 동작하는가

Go 런타임 소스(runtime/proc.go)에서 핵심 함수는 entersyscall, exitsyscall, sysmon, retake이다. 고루틴이 syscall을 호출하면, syscall 패키지가 런타임의 entersyscall을 호출한다. entersyscall은 현재 M이 가진 P를 떼어내어 그 상태를 _Psyscall로 설정한다. 이때 P는 sched.pidle 목록에 들어가지 않고, M과의 연관을 유지한 채 남는다. M은 P 없이 syscall을 실행한다. 만약 syscall이 오래 걸리면, sysmon이 개입한다.

sysmon은 런타임이 시작될 때 생성되는 특별한 M으로, P 없이 동작한다. sysmon은 주기적으로 깨어나서 (약 10ms 간격, 또는 필요에 따라 더 자주) 다음 작업을 수행한다. netpoll 감시, 타이머 처리, 그리고 retake를 호출한다. retake 함수는 모든 P를 순회하면서 _Psyscall 상태인 P를 찾는다. 각 P에는 syscalltick이라는 카운터가 있는데, 이는 P가 syscall에 들어간 시간을 추적하는 데 사용된다. retake는 P가 _Psyscall 상태로 있는 시간이 일정 기준(대략 10ms)을 넘었는지 검사한다. 만약 넘었다면, P를 강제로 회수하여 상태를 _Pidle로 바꾸고 sched.pidle 목록에 넣는다. 이렇게 하면 다른 M이 그 P를 획득하여 실행 가능한 고루틴을 실행할 수 있다.

syscall에서 돌아온 M은 exitsyscall을 호출한다. exitsyscall은 먼저 자신이 이전에 사용하던 P를 다시 얻을 수 있는지 시도한다. 만약 그 P가 여전히 _Psyscall 상태로 M과 연결되어 있다면, M은 그 P를 되찾아 즉시 고루틴 실행을 재개한다. 하지만 sysmon에 의해 P가 회수되어 다른 M에게 넘어갔다면, M은 sched.pidle에서 새 P를 얻으려 시도한다. 만약 idle P가 없다면, M은 실행 가능한 고루틴을 글로벌 runqueue에서 찾아 실행하거나, 그마저도 없으면 스레드를 sleep 상태로 전환하고, 나중에 깨어날 조건을 기다린다. 이 과정은 proc.go의 exitsyscall 함수에서 확인할 수 있다. 핵심은 고루틴이 syscall 때문에 영원히 블로킹되지 않고, P가 효율적으로 재사용된다는 점이다.

## 돌려보기

이 디렉토리에서 다음과 같이 실행해 보자.

```bash
go vet ./...                 # 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행 (기본 시나리오)
GODEBUG=schedtrace=1000 go run .  # 스케줄러 트레이스 출력
go test -v ./...             # 테스트
go test -race ./...          # race 검사
go test -bench=. -benchmem   # 벤치마크 (선택)
```

`go run .`은 GOMAXPROCS=2로 설정하고, 4개의 syscall 고루틴과 2개의 CPU 바운드 고루틴을 동시에 실행한다. 출력에서 `CPU progress`가 0보다 훨씬 큰 값이어야 한다. 이는 sysmon이 _Psyscall 상태의 P를 회수하여 CPU 고루틴이 실행되었다는 증거다. `GODEBUG=schedtrace=1000 go run .`을 실행하면, 1초마다 스케줄러 상태가 출력된다. P의 상태가 `P`로 시작하는 라인에 표시되는데, syscall 고루틴이 실행되는 동안 `_Psyscall` 상태가 보이고, sysmon이 회수한 후에는 `_Pidle` 또는 `_Prunning`으로 바뀌는 것을 관찰할 수 있다.

## 코드로 확인하기

main.go의 `runScenario`는 먼저 `runtime.GOMAXPROCS(2)`를 호출하여 P 개수를 2개로 제한한다. 그런 다음 4개의 `blockingSyscallWorker` 고루틴을 시작하고, 각각은 `syscall.Select`를 호출하여 100ms 동안 커널에서 블로킹된다. 동시에 2개의 `cpuBoundWorker` 고루틴이 시작되어 CPU를 태운다. `blockingSyscallWorker`는 `syscall.Select(0, nil, nil, nil, &tv)`를 호출하는데, 이는 파일 디스크립터를 감시하지 않고 주어진 시간 동안 현재 스레드를 잠들게 하는 순수한 블로킹 syscall이다. 이 syscall을 호출하는 순간 Go 런타임은 M이 P를 반납하도록 처리한다. `cpuBoundWorker`는 반복문을 돌며 `progress`를 증가시킨다. 만약 sysmon이 P를 회수하지 않았다면, CPU 고루틴은 P를 얻지 못해 거의 실행되지 않을 것이다. 하지만 실행 결과를 보면 `CPU progress`가 크게 나오는데, 이는 sysmon이 P를 회수하여 CPU 고루틴이 실행되었음을 보여준다. `GODEBUG=schedtrace=1000`을 사용하면 더 자세한 내부 상태를 볼 수 있다.

main_test.go의 테스트들은 함수의 동작을 검증한다. `TestBlockingSyscallWorker`는 syscall 고루틴이 정확한 횟수만큼 syscall을 호출하는지 확인한다. `TestCPUWorker`는 CPU 바운드 계산이 올바른 결과를 반환하는지 검증한다. `TestConcurrentWorkers`는 두 종류의 고루틴을 동시에 실행했을 때 모든 작업이 완료되고 race가 발생하지 않는지 확인한다. 이 테스트는 `-race` 플래그로 실행해도 통과해야 한다.

## 모르면 겪는 일

이 메커니즘을 모르면 블로킹 syscall이 많은 서버에서 CPU 사용률이 낮게 나오는 원인을 오진할 수 있다. 예를 들어, 파일 업로드나 다운로드가 많은 서비스에서 CPU 바운드 작업이 갑자기 느려지는 경우, syscall 고루틴이 P를 점유하고 있어서라고 추측할 수 있다. 하지만 실제로는 sysmon이 P를 회수하므로 CPU 사용률은 정상적으로 유지된다. 오히려 짧은 syscall이 매우 빈번한 경우, P를 반납하고 되찾는 과정에서 잠금 경합이 발생하여 오버헤드가 커질 수 있다. 이 오버헤드는 CPU 프로파일에서 `runtime.entersyscall`이나 `runtime.exitsyscall` 근처에서 관찰될 수 있다. 이걸 모르면 syscall을 줄이는 최적화가 필요한데도 불구하고 GOMAXPROCS를 늘리는 잘못된 해결책을 시도할 수 있다.

또한, syscall이 매우 길어서 (예: 몇 초 이상) 고루틴이 오래 블로킹되는 경우, sysmon이 P를 회수하더라도 그 고루틴은 여전히 M에 묶여 있다. 이 M은 블로킹 상태이므로 OS 스레드 자원을 계속 차지한다. 만약 동시에 많은 장기 syscall이 발생하면, 스레드 수가 늘어나고 컨텍스트 스위치 비용이 증가할 수 있다. 이는 GOMAXPROCS와 무관하게 발생하므로, p99 레이턴시가 튀는 원인이 될 수 있다. 이러한 상황에서는 별도의 워커 풀을 사용하거나, syscall을 논블로킹으로 전환하는 것이 필요하다.

## 언제 신경 쓰고 언제 무시하나

이 지식이 중요해지는 조건은 두 가지다. 첫째, 블로킹 syscall이 전체 실행 시간에서 차지하는 비중이 높고, 그 syscall이 10ms 이상 지속되는 경우. 예를 들어 디스크 I/O가 많은 데이터베이스, 파일 처리 서버, 또는 클라우드 스토리지 접근이 많은 서비스다. 이런 워크로드에서는 sysmon의 P 회수가 CPU 활용률을 유지하는 데 핵심 역할을 한다. 둘째, 매우 짧은 syscall이 초당 수십만 번 이상 발생하는 경우. 이 경우 P 반납/회수 오버헤드가 누적되어 성능에 영향을 줄 수 있다. 이런 경우 syscall 자체를 줄이거나, 배치 처리로 전환하는 것이 필요하다.

반대로, CPU 바운드 워크로드나 네트워크 I/O만 사용하는 경우(네트워크는 netpoller가 논블로킹으로 처리)에는 이 메커니즘을 신경 쓸 필요가 거의 없다. GOMAXPROCS가 작고 고루틴 수가 적어 P가 남아도는 경우에도 sysmon의 회수는 큰 의미가 없다. 대부분의 애플리케이션 개발자는 이 내부 동작을 몰라도 문제없이 코드를 작성할 수 있다. 다만, 성능 문제를 진단할 때 CPU 사용률이 낮거나 레이턴시가 튀는 경우, 이 메커니즘을 이해하고 있으면 syscall 프로파일과 스케줄러 트레이스를 해석하는 데 큰 도움이 된다.

## 더 파보기

- Go 런타임 스케줄러 소스: https://go.dev/src/runtime/proc.go (특히 entersyscall, exitsyscall, sysmon, retake 함수)
- Go 스케줄러 디자인 문서: https://golang.org/s/go11sched
- 런타임 튜닝 문서: https://go.dev/doc/diagnostics
- Go 블로그 "The Go scheduler": https://go.dev/blog/waza-talk
- sysmon과 관련된 이슈: https://github.com/golang/go/issues/12345 (예시, 실제로는 다른 이슈를 찾을 것)