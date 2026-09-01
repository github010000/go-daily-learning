## 한 줄 요약

Go 런타임의 netpoller는 epoll/kqueue를 감춰서 `net.Conn.Read`가 블로킹 API처럼 보이게 하지만, 실제로는 goroutine만 park시키고 OS 스레드 M은 다른 goroutine에게 넘긴다. 반면 regular file I/O는 epoll 대상이 아니어서 blocking syscall로 흘러 M을 붙잡을 수 있다. 이 차이를 알면 network I/O에는 goroutine을 그냥 사용해도 되고, file I/O에는 동시성을 따로 관리해야 하는 이유가 보인다.

## 왜 이런 설계인가

전통적인 thread-per-connection 모델은 연결 하나에 OS 스레드 하나를 할당한다. 클라이언트가 요청을 보내지 않아 `read`가 블로킹되면 해당 스레드는 데이터가 도착할 때까지 아무 일도 못 하고 대기한다. 연결이 1만 개면 1만 개의 스레드가 쌓이고, 리눅스에서 스레드 한 개의 기본 스택만 해도 8MB에 가깝다. 스택 메모리와 컨텍스트 스위칭 비용 때문에 이 방식은 C10K를 넘어가는 순간 급격히 무너진다.

반대로 epoll/kqueue를 직접 사용하는 이벤트 루프 모델은 파일 디스크립터를 커널에 등록해 두고, 준비된 이벤트만 콜백으로 처리한다. 스레드 수는 적게 유지할 수 있지만 코드가 콜백 지옥으로 바뀐다. `read` 요청을 보내고 나면 함수가 끝나 버리기 때문에, 그 뒤에 실행할 로직을 콜백이나 state machine으로 쪼개야 한다. Node.js의 `async/await`도 결국 이 콜백 문제를 문법으로 감춘 것일 뿐, 근본적으로는 같은 구조다.

Go는 goroutine과 scheduler라는 이미 갖고 있는 자원을 I/O에 연결하는 길을 선택했다. 네트워크 파일 디스크립터를 논블로킹으로 열고, `Read`가 `EAGAIN`을 만나면 현재 goroutine만 `gopark`로 재운다. M은 그 goroutine을 계속 기다리지 않고 run queue에서 다른 goroutine을 꺼내 실행한다. 그러면 사용자 코드는 블로킹 스타일로 작성하면서도 내부에서는 이벤트 루프의 확장성을 얻을 수 있다. 이것이 `net.Conn.Read`를 그냥 사용해도 되는 이유다.

그런데 regular file I/O는 이 구조를 타지 않는다. 디스크 파일은 epoll/kqueue에 등록할 수 없거나, 등록해도 "데이터가 준비되었다"라는 알림을 주지 않는 경우가 많다. 그래서 `os.File.Read`는 blocking syscall을 직접 호출하고, 그 syscall이 오래 걸리면 M 자체가 커널 안에서 멈춘다. Go runtime은 이 경우 P를 다른 M에게 넘겨서 실행은 계속되게 하지만, 블로킹된 M은 여전히 OS 스레드 하나를 점유한다. netpoller가 G만 재우는 것과 분명히 다르다.

## 어떻게 동작하는가

먼저 `net.Listen`으로 만들어진 `net.TCPListener`는 내부에 `netFD`를 갖는다. `netFD`는 `net/fd_posix.go`에 정의되어 있고, `poll.FD`를 임베딩한다. `poll.FD`는 `internal/poll/fd_unix.go`에 있으며 `Sysfd`, `pd`, `isBlocking`, `IsStream` 같은 필드를 가진다. 네트워크 파일 디스크립터는 생성 시 논블로킹으로 설정되고 `pd`에 runtime poller를 위한 상태가 연결된다.

`net.Conn.Read`는 결국 `internal/poll.FD.Read`에 도달한다. 그 함수는 먼저 `syscall.Read`를 시도하고, `EAGAIN`이 돌아오면 `fd.pd.waitRead`를 호출한다. 이 `waitRead`는 runtime으로 들어가 `poll_runtime_pollWait`를 부르고, 실제 구현은 `runtime/netpoll.go`의 `netpollblock`이다. `netpollblock`은 현재 goroutine을 `gopark`에 넣고 `waitReasonIOWait`로 표시한다. 이때 M은 block되지 않고 scheduler에게 반환된다.

runtime의 `pollDesc`에는 `fd`, `link`, `rg`, `wg`, `rt`, `wt`, `rd`, `wd` 같은 필드가 있다. `rg`와 `wg`는 read/write 대기 중인 goroutine을 담는 포인터이고, `rt`, `wt`는 deadline 타이머다. 이벤트가 없으면 goroutine은 이 `pollDesc`에 묶인 채 park된다. 반대로 epoll/kqueue가 이벤트를 알려주면 `netpollready`가 해당 goroutine을 run queue에 올려서 다시 실행되게 한다.

이벤트 수집은 `runtime/netpoll_epoll.go`의 `epollwait`, `epollctl`이나 `runtime/netpoll_kqueue.go`의 kqueue 계열 함수가 담당한다. `findrunnable`은 실행할 goroutine이 없을 때 `netpoll(0)`을 호출해 준비된 network goroutine을 찾는다. 또 `sysmon`도 주기적으로 `netpoll`을 호출해 park된 goroutine을 깨운다. 여기서 중요한 점은 `epollwait` 자체는 커널에서 잠시 블로킹될 수 있지만, 그것은 전용 M이나 sysmon에서 실행되고 사용자 goroutine을 맡은 M을 점유하지 않는다는 것이다.

regular file의 경우 `os/file_unix.go`의 `newFile`에서 파일 종류가 `kindOpenFile`이면 pollable을 false로 둔다. 그러면 `internal/poll.FD.Read`는 `pd.waitRead`를 호출하지 않고 그냥 `syscall.Read`만 실행한다. syscall이 블로킹되면 `entersyscall`/`exitsyscall` 경로를 타고 P는 다른 M에게 넘어갈 수 있지만, 원래 M은 블로킹 상태로 남는다. 따라서 파일 I/O가 많아지면 OS 스레드 수가 늘어난다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령이다.

```bash
go vet ./...                 # net.Conn 사용 코드의 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # netpoller가 G만 재우는지 CPU worker 진행으로 확인
GODEBUG=schedtrace=1000 go run .  # 스케줄러가 M을 어떻게 돌리는지 로그로 확인
go test -v ./...             # 스택 기반 검증과 CPU progress 검증
go test -race ./...          # 동시성 코드의 data race 검증
```

`go run .`을 실행하면 GOMAXPROCS가 1이어도 5개의 `net.Conn.Read`가 park된 상태에서 CPU work target이 1,000,000까지 채워지는 것을 볼 수 있다. `GODEBUG=schedtrace=1000 go run .`은 1초마다 스케줄러 상태를 출력하는데, network read에 park된 goroutine은 `IO wait`로 보이고 M은 다른 goroutine을 계속 실행한다는 것을 관찰할 수 있다.

## 코드로 확인하기

`main.go`의 `parkNetworkReads(5)`는 TCP listener를 하나 만들고, 클라이언트 5개를 연결한 뒤 각 서버 goroutine이 `c.Read`에서 멈추도록 만든다. 그런 다음 `runtime.Stack(buf, true)`로 모든 goroutine의 스택을 읽어서 `internal/poll.(*FD).Read` 문자열이 5번 나타날 때까지 기다린다. 이는 단순히 goroutine을 생성한 것이 아니라 실제로 netpoller에 park된 상태임을 관찰 가능하게 보여준다.

`runCPUWorkUntilTarget(1_000_000)`은 CPU만 소모하는 goroutine을 띄우고 그 수가 1,000,000에 도달할 때까지 기다린다. `GOMAXPROCS(1)`로 설정했기 때문에 만약 network read가 M을 블로킹했다면 CPU goroutine은 전혀 실행되지 못했을 것이다. 하지만 netpoller가 G만 재우고 M을 scheduler에게 돌려주므로 CPU work가 정상적으로 목표를 채운다. 출력에서 `runtime.NumThread`도 함께 찍어 M이 크게 늘지 않는다는 점을 보여준다.

`main_test.go`에는 시간에 의존하지 않는 두 개의 테스트가 있다. 첫 번째 테스트는 `parkNetworkReads(3)`을 호출한 뒤 `countStackNeedle`로 실제 park된 goroutine 수를 검증한다. 두 번째 테스트는 동일하게 3개 read를 park시키고 `runCPUWorkUntilTarget(1000)`이 1000 이상을 반환하는지 확인한다. 둘 다 sleep 없이 `runtime.Gosched`와 조건 충족까지의 반복만 사용한다.

## 모르면 겪는 일

정기 파일을 네트워크처럼 잘못 다루는 경우가 많다. 예를 들어 slow NFS에 있는 파일을 읽는 goroutine을 수백 개 생성하면, 각 `Read`가 blocking syscall로 M을 점유한다. runtime이 P를 다른 M에 넘기더라도 원래 M은 커널에서 대기하므로 OS 스레드가 계속 늘어난다. 운영체제의 스레드 제한에 걸리면 `runtime: failed to create new OS thread` 같은 에러가 나고, CPU profile에는 거의 아무것도 안 잡히는데 p99 latency는 GC 주기나 페이지 캐시 미스 때마다 튀는 증상이 나타난다.

network I/O에서도 deadline을 걸지 않으면 문제가 생긴다. 상대방이 데이터를 보내지 않으면 goroutine은 `IO wait` 상태로 영원히 park된다. M은 막히지 않으므로 프로세스 전체는 죽지 않지만, goroutine과 연결 fd는 계속 남는다. 이런 goroutine이 쌓이면 힙 메모리와 fd 테이블이 조용히 증가하고, `goroutine` 수가 수십만 개에 도달한 뒤에야 발견되기도 한다.

raw syscall을 직접 사용해서 netpoller를 우회하는 것도 위험하다. `net.Conn`을 두고 굳이 `syscall.Select` 같은 블로킹 syscall을 호출하면, runtime이 그 syscall을 처리하는 방식에 따라 M이 오래 묶이거나 필요 없는 스레드가 생길 수 있다. netpoller는 fd 단위로 G를 재우는 구조이므로, 같은 fd에 대해 netpoller와 raw syscall을 섞으면 어떤 goroutine이 그 fd를 깨울지 예측하기 어려워진다.

## 언제 신경 쓰고 언제 무시하나

netpoller의 동작은 동시에 수백 개 이상의 연결을 다룰 때 중요해진다. 연결이 몇 개뿐이라면 OS 스레드가 블로킹되든 말든 성능 차이가 거의 없다. 오히려 이 내용을 모른 채 과도하게 `runtime.GOMAXPROCS`를 조정하거나, network read를 미리 논블로킹으로 바꾸려고 하면 코드만 복잡해지고 이득은 없다. 그냥 `net.Conn`을 goroutine에서 블로킹으로 읽는 것이 Go의 의도된 사용법이다.

file I/O는 정기 파일과 pipe, tty를 구분해서 봐야 한다. pipe와 tty는 netpoller를 탈 수 있으므로 큰 문제가 되지 않는다. 반면 정기 파일 read/write는 본질적으로 blocking syscall이고, 로컬 SSD에서 몇 개의 goroutine이 도는 수준이라면 전혀 신경 쓸 필요가 없다. 다만 NFS나 아주 느린 디스크에서 병렬 file I/O가 많아질 때는 worker pool이나 `runtime.NumThread` 모니터링을 고려하는 것이 좋다.

정리하면, 네트워크 서버를 작성할 때는 netpoller를 믿고 blocking API를 그대로 쓰는 것이 맞다. 파일 I/O가 병목이 되는 배치 작업이나 스트리밍 작업에서는 netpoller가 아닌 파일 I/O 특성과 OS 스레드 점유를 의식해야 한다. 과최적화를 막는 기준은 "지금 병목이 network socket인가, 정기 파일 syscall인가"를 먼저 확인하는 것이다.

## 더 파보기

- [runtime/netpoll.go](https://go.dev/src/runtime/netpoll.go)
- [runtime/netpoll_epoll.go](https://go.dev/src/runtime/netpoll_epoll.go)
- [runtime/netpoll_kqueue.go](https://go.dev/src/runtime/netpoll_kqueue.go)
- [internal/poll/fd_unix.go](https://go.dev/src/internal/poll/fd_unix.go)
- [os/file_unix.go](https://go.dev/src/os/file_unix.go)
- [net/fd_posix.go](https://go.dev/src/net/fd_posix.go)