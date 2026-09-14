## 한 줄 요약

Go 런타임은 epoll / kqueue 같은 OS 이벤트 통지 기능을 netpoller 라는 내부 계층으로 감춘다. net.Conn.Read 같은 네트워크 읽기는 겉보기에는 블로킹처럼 보이지만, 실제로는 goroutine 만 잠들고 M 은 런타임으로 돌아간다. 그래서 수천 개의 연결이 동시에 I/O 를 기다려도 OS 스레드는 수십 개면 충분하다. 반면 raw syscall 이나 일반 파일 I/O 는 이 경로를 타지 않아 M 까지 커널에 묶일 수 있다.

## 왜 이런 설계인가

Go 는 동기식 API 를 유지하면서 높은 동시성을 얻기 위해 netpoller 를 도입했다. Node.js 의 콜백이나 Java 의 명시적 스레드 풀처럼 사용자가 비동기 상태 머신을 직접 관리하게 하면 코드가 복잡해진다. Go 는 개발자에게 "그냥 Read 를 호출해도 된다"는 경험을 주면서도, 그 내부에서는 goroutine 만 park 하고 M 을 반환해 다른 goroutine 을 실행한다. 이것이 Go 의 동시성 모델이 단순하면서도 확장 가능한 이유다.

OS 스레드는 생성 비용과 커널 스택 메모리 때문에 수만 개를 만들 수 없다. 스레드당 연결 모델로는 C10K 문제를 해결할 수 없다. Go 의 런타임은 G 를 M 에 다중화하고, I/O 대기 중인 G 는 실행 큐에서 제거해 두는 방식으로 이 한계를 우회한다. 여기서 핵심은 M 이 커널에서 잠들지 않아야 한다는 것이다. M 이 커널에서 블로킹되면 P 가 놀게 되고, 런타임은 새 M 을 만들 수밖에 없다.

netpoller 는 모든 네트워크 파일 디스크립터를 epoll 또는 kqueue 에 등록해 두고, 커널이 준비 완료 이벤트를 보내면 해당 fd 를 기다리던 goroutine 을 깨운다. 이렇게 하면 Read 를 호출한 goroutine 은 park 되지만 M 은 다른 goroutine 을 실행할 수 있다. 네트워크 소켓은 epoll / kqueue 와 잘 맞지만, 일반 파일은 커널이 readiness 를 통지하는 모델이 아니라서 이 방식이 적용되지 않는다.

파일 I/O 가 다른 이유는 OS 의 제약 때문이다. 리눅스에서 epoll 은 regular file 에 대해 항상 ready 라고 알려주거나 제대로 동작하지 않는다. 그래서 Go 는 일반 파일 Read 를 netpoller 에 맡기지 않고, 시스템 콜을 직접 호출해 블로킹시킨다. 이때 M 이 커널에 묶이면 런타임은 새 M 을 만들어 P 를 계속 사용한다. 병렬 처리가 필요 없는 단순 파일 읽기라면 문제가 없지만, 동시성 높은 서버에서 파일 I/O 를 잘못 쓰면 스레드가 급증한다.

## 어떻게 동작하는가

netpoller 의 핵심 구현은 runtime/netpoll.go, runtime/netpoll_epoll.go, runtime/netpoll_kqueue.go 에 있다. 내부적으로 pollDesc 라는 구조체가 fd 와 대기 중인 goroutine 을 연결한다. internal/poll/fd_unix.go 의 FD 는 syscall.Read 가 EAGAIN 을 반환하면 runtime_pollWait 를 호출한다. 이 함수는 runtime.netpollblock 을 거쳐 gopark 로 현재 goroutine 을 park 시킨다. 이때 M 은 해제되어 런타임 스케줄러로 복귀한다.

커널에 fd 가 준비되면 epoll_wait 또는 kqueue 가 이벤트를 반환하고, netpoll 이라는 런타임 함수가 pollDesc 를 찾아 대기 중인 goroutine 을 runnable 상태로 만들어 실행 큐에 넣는다. 이 과정은 netpollready 라는 함수에서 일어난다. 실행 큐에 들어간 goroutine 은 나중에 다른 M 에 의해 다시 실행되고, Read 의 나머지 경로를 이어간다.

net.Conn.Read 가 실제로 어떻게 흘러가는지 단계별로 보면 다음과 같다. net.TCPConn.Read 는 internal/poll.FD.Read 를 호출한다. 첫 번째 syscall.Read 시도에서 EAGAIN 을 받으면, fd 가 nonblocking 모드로 열려 있기 때문에 즉시 반환된다. 이후 FD.pd.wait 가 호출되고, 최종적으로 runtime_pollWait 가 goroutine 을 park 한다. 사용자가 보기에는 블로킹처럼 보이지만 실제로는 M 이 돌아간다.

반대로 raw syscall.Read 는 netpoller 를 거치지 않는다. syscall 패키지의 Read 는 직접 커널의 read 시스템 콜을 호출한다. 이 시스템 콜에서 블로킹되면 M 자체가 커널에 묶인다. 런타임은 이 M 이 곧 돌아오지 않을 것이라고 판단하고, 필요하면 새 M 을 생성해 P 를 넘긴다. 따라서 동시에 많은 raw syscall 이 블로킹되면 OS 스레드 수가 goroutine 수와 비슷하게 늘어난다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령을 순서대로 실행해 보면 된다.

```bash
go vet ./...                 # 정적 검사 통과 확인
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행. net.Conn.Read 와 raw syscall.Read 의 goroutine/스레드 수 차이를 본다
go test -v ./...             # 테스트. 누수 없음과 /proc 읽기 동작을 검증한다
go test -race ./...          # 동시성 경로에 레이스가 없음을 확인한다
```

리눅스에서는 `go run .` 을 할 때 스레드 수가 실제 값으로 출력된다. macOS 에서는 `/proc/self/status` 가 없어 스레드 수가 -1 로 표시되지만, goroutine 수는 정확히 측정된다. macOS 에서 스레드 수를 보고 싶다면 `ps -M <pid>` 또는 Instruments 로 확인할 수 있다. 선택 사항으로 `GOMAXPROCS=1 go run .` 을 실행해도 main 함수가 내부에서 GOMAXPROCS(1) 을 설정하므로 같은 결과가 나온다.

## 코드로 확인하기

main.go 는 두 가지 시나리오를 순서대로 실행한다. 첫 번째 `blockOnNetpoll` 은 100 개의 TCP 연결을 만들고 모든 클라이언트 goroutine 이 Read 에서 블로킹되게 한다. 50ms 후에 스냅샷을 찍으면 goroutine 수는 100 개 증가하지만 OS 스레드 수는 거의 변하지 않는다. 리눅스에서 P 가 1 개라면 스레드 수가 1~3 개 정도만 증가한다. 이것이 netpoller 가 M 을 해방시켰다는 증거다.

두 번째 `blockOnRawSyscallPipe` 는 100 개의 파이프를 만들고 raw syscall.Read 에서 블로킹시킨다. 같은 50ms 후 스냅샷을 찍으면 goroutine 수는 100 개 증가하고, OS 스레드 수도 100 개에 가깝게 증가한다. raw syscall 은 M 을 커널에 묶기 때문에 런타임이 새 M 을 계속 만들어야 하기 때문이다. 이 차이가 파일 I/O 와 네트워크 I/O 의 차이를 직접 보여준다.

main_test.go 는 두 경로 모두에서 goroutine 누수가 없는지 검증한다. settle 을 0 으로 두어 시간에 의존하지 않고, 함수가 반환된 뒤 goroutine 수가 원래 수준으로 돌아오는지만 확인한다. `TestCurrentThreadsReadsProcStatus` 는 리눅스에서 스레드 수를 정상적으로 읽는지 검증하고, `/proc` 이 없는 환경에서는 skip 한다. 벤치마크는 setup/teardown 전체를 상대 비교용으로 측정한다.

## 모르면 겪는 일

netpoller 를 모르고 raw syscall 로 네트워크 I/O 를 처리하면 스레드 수가 연결 수만큼 늘어난다. 예를 들어 `net.TCPConn.File()` 로 fd 를 꺼낸 뒤 `syscall.Read` 를 호출하면 netpoller 를 우회하게 된다. 동시 연결이 수천 개인 서버에서 이렇게 하면 OS 스레드가 수천 개까지 증가하고, 스레드 생성/종료 비용과 커널 스케줄링 부하가 p99 latency 를 GC 주기마다 튀게 만든다. CPU 프로파일에는 syscall 이 거의 안 보일 수 있어 원인을 찾기 어렵다.

파일 I/O 를 고루틴 수백 개에서 동시에 호출하는 것도 비슷한 문제를 일으킨다. 로그 파일을 모든 요청 goroutine 에서 직접 쓰거나, 큰 파일을 작은 조각으로 나눠 동시에 읽으면 M 이 커널에서 블로킹된다. 이때 런타임은 새 M 을 계속 만들지만, 새 M 도 곧바로 파일 I/O 에서 블로킹되기 때문에 스레드 폭발이 일어난다. 결국 `thread exhaustion` 또는 `unable to create new native thread` 에러가 발생한다.

netpoller 의 동작을 모르면 deadline 설정의 중요성도 놓치기 쉽다. net.Conn.Read 는 goroutine 만 park 시키기 때문에, 상대방이 데이터를 보내지 않으면 영원히 잠들 수 있다. `SetReadDeadline` 을 사용하면 런타임이 타이머를 통해 해당 goroutine 을 깨우므로, 연결이 닫히지 않아도 goroutine 누수를 막을 수 있다. 이 메커니즘을 이해하지 못하면 타임아웃이 발생해도 goroutine 이 계속 쌓여 메모리가 느는 문제가 생긴다.

## 언제 신경 쓰고 언제 무시하나

이 지식은 동시 연결 수가 수백 개 이하인 서비스에서는 거의 신경 쓸 필요가 없다. OS 스레드 수십 개는 아무 부담이 되지 않고, 파일 I/O 도 goroutine 에서 그냥 호출해도 된다. 하지만 연결 수가 수천 개를 넘거나, 파일 I/O 가 요청 경로에 섞여 있거나, 스레드 수 모니터링 지표가 급증하는 환경이라면 반드시 알아야 한다. 특히 p99 latency 가 주기적으로 튀는데 CPU 프로파일이 깨끗하다면 스레드 생성과 파일 I/O 를 의심해 봐야 한다.

net.Conn 계열은 안심하고 blocking read 를 써도 된다. netpoller 가 M 을 해방시키므로 수만 개의 연결도 수십 개의 스레드로 처리할 수 있다. 다만 반대쪽이 데이터를 안 보내면 goroutine 이 영원히 park 되므로 deadline 이나 context 취소를 반드시 연결해야 한다. raw syscall 이나 os.File.Read 같은 파일 I/O 는 동시성이 높은 경로에서는 피하는 것이 좋다. 꼭 필요하다면 bounded worker pool 로 동시 파일 I/O 수를 제한하거나, 파일 읽기 전용 goroutine 을 별도로 두어 스레드 생성을 제어해야 한다.

언제 무시해도 되는지 명확히 구분하자. 내부 도구, 배치 작업, 처리량이 낮은 API 서버에서는 파일 I/O 를 그냥 써도 아무 문제 없다. 수천 개의 goroutine 이 동시에 파일을 읽지 않는 한 M 이 늘어나도 시스템에 부담이 되지 않는다. 과최적화보다는 지표를 먼저 보는 것이 낫다. 스레드 수 지표가 선형으로 증가하거나, p99 가 주기적으로 튀는 신호가 나타날 때만 이 내용을 적용하면 된다.

## 더 파보기

- [runtime/netpoll.go](https://go.dev/src/runtime/netpoll.go) — netpoller 의 공통 인터페이스와 pollDesc 정의
- [runtime/netpoll_epoll.go](https://go.dev/src/runtime/netpoll_epoll.go) — 리눅스 epoll 구현
- [runtime/netpoll_kqueue.go](https://go.dev/src/runtime/netpoll_kqueue.go) — macOS/BSD kqueue 구현
- [internal/poll/fd_unix.go](https://go.dev/src/internal/poll/fd_unix.go) — fd.Read 가 EAGAIN 을 만났을 때 runtime_pollWait 로 이어지는 경로
- [The Go netpoller](https://morsmachine.dk/netpoller) — netpoller 의 설계와 동작을 설명한 외부 글