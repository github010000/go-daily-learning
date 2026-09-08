## 한 줄 요약

Go 메모리 모델은 "동시에 접근하면 안 된다"가 아니라, 어떤 쓰기가 어떤 읽기에 보이는지를 happens-before라는 부분 순서로 정의한다. 채널 close/send, mutex Unlock/Lock, sync.Once 완료, sync/atomic 연산만이 명시적인 순서 보장을 주며, 그 외에는 컴파일러와 CPU가 재배치해도 된다는 전제로 동작한다.

## 왜 이런 설계인가

Go는 수많은 goroutine이 한 프로세스에서 동시에 도는 모델을 선택했다. 모든 메모리 접근을 순차 일관성(sequential consistency)으로 강제하면 매 읽기와 쓰기에 메모리 배리어가 붙고, 대부분의 단일 goroutine 코드까지 느려진다. 그래서 Go는 C/C++이나 Java처럼 완화된 메모리 모델을 도입했다. 프로그래머가 동기화 지점을 명시하면 그 지점 사이에서만 강한 순서를 보장하고, 나머지는 컴파일러 최적화와 CPU 재배치를 허용하는 것이다.

대안은 모든 공유 변수를 atomic으로 다루거나, 모든 접근을 잠그는 것이었다. atomic은 잘못 쓰면 논리는 맞지만 race detector가 잡지 못하는 미묘한 버그를 만들고, 모든 접근을 잠그면 성능이 급락한다. 그래서 Go는 고수준 동기화 프리미티브인 channel, mutex, once에 happens-before 규칙을 부여했다. 이들은 코드의 의도를 드러내면서도 런타임이 필요한 배리어만 효율적으로 넣을 수 있게 한다.

또 하나의 이유는 data race를 undefined behavior로 정의하기 위해서다. 만약 race가 있어도 "조금 이상한 값"만 나온다고 보장하면 컴파일러는 항상 그 가능성을 염두에 둬야 한다. Go처럼 race를 undefined로 두면 컴파일러는 "이 변수는 동시에 접근되지 않는다"고 가정하고 레지스터 캐싱, 로드 호이스팅, 상수 폴딩 같은 최적화를 마음껏 수행할 수 있다. 그 결과 race가 있는 코드는 x86에서 멀쩡해 보이다가 ARM이나 amd64에서 인라이닝이 바뀐 순간 터질 수 있다.

테스트가 통과하는 이유도 여기에 있다. happends-before가 없는 코드는 대부분 단일 CPU, 낮은 부하, x86의 강한 메모리 순서(total store order) 때문에 우연히 올바르게 동작한다. goroutine이 즉시 스케줄링되고 캐시가 우연히 일치하며 CPU가 쓰기 순서를 지켜주는 것처럼 보일 뿐이다. 이는 보장이 아니라 실행 환경의 우연이며, GOMAXPROCS를 올리거나 ARM 서버에 배포하면 같은 코드가 다른 결과를 낸다.

## 어떻게 동작하는가

Go 메모리 모델은 세 가지 관계로 이루어진다. 첫째는 sequenced before로, 한 goroutine 안에서 문장이 작성된 순서다. 둘째는 synchronized before로, 동기화 연산이 만드는 순서다. 예를 들어 채널 send는 대응하는 receive보다 앞서고, mutex Unlock은 이후 Lock보다 앞선다. 셋째는 happens-before로, sequenced before와 synchronized before의 전이적 폐쇄다. 어떤 쓰기 W가 어떤 읽기 R보다 happens-before라면 R은 W의 값을 보게 되고, 그렇지 않다면 관찰 가능한 순서가 정의되지 않는다.

구체적인 synchronized before 규칙은 go.dev/ref/mem에 정확히 나와 있다. 채널은 "send는 해당 receive 완료보다 앞선다", "close는 그 채널에서 zero value를 받는 receive보다 앞선다", 그리고 unbuffered 채널에서는 "receive가 send 완료보다 앞선다"는 규칙이 있다. mutex는 "n번째 Unlock이 m번째 Lock 반환보다 앞선다(n < m)"는 규칙을 가진다. sync.Once는 "once.Do(f)의 완료가 모든 once.Do(f) 호출의 반환보다 앞선다"고 정의한다. sync/atomic 연산은 순차 일관성을 제공하며, atomic.Store와 atomic.Load 사이에도 각각 순서가 생긴다.

런타임 구현을 보면 이 규칙들이 실제 메모리 배리어로 어떻게 연결되는지 알 수 있다. 채널은 runtime/chan.go의 hchan 구조체와 closechan, chanrecv 함수에서 sendq, recvq 대기열을 조작하며, 채널 연산의 완료는 atomic 연산과 배리어로 다른 goroutine에게 퍼진다. mutex는 sync/mutex.go에서 state 필드를 atomic compare-and-swap으로 바꾸고, 대기 중인 goroutine은 sema 필드의 세마포어로 깨어난다. once는 sync/once.go의 done atomic 플래그를 먼저 확인하고, 최초 실행이 끝난 뒤 done을 원자적으로 기록해 이후 호출이 그 쓰기를 보게 만든다.

이 규칙들을 어기면 data race가 된다. 같은 메모리 위치에 대해 happens-before 순서가 없는 두 접근 중 적어도 하나가 쓰기라면 그 프로그램은 잘못된 것이다. 컴파일러는 data race가 없다고 가정하고 코드를 재배치하므로, 겉으로 보기에는 "그냥 값이 조금 늦게 보이겠지"라고 생각해도 실제로는 nil 포인터, 절반만 초기화된 struct, 잘못된 map 상태를 만날 수 있다. 이 때문에 "동시 접근하면 안 된다"가 아니라 "동시 접근 간에 happens-before가 있어야 한다"고 말하는 것이 정확하다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령이다. safe demo는 항상 결정적인 값을 내고, unsafe demo는 실행 환경에 따라 lost update가 보이거나 보이지 않을 수 있다.

```bash
go vet ./...                 # 정적 검사: race를 만들 수 있는 구조가 없는지 확인
go build -o /dev/null ./...  # 컴파일 확인: unsafe 함수도 main에서만 호출되므로 빌드는 통과
go run .                     # safe demo와 unsafe demo 실행, lost update 발생 여부 관찰
GOMAXPROCS=4 go run .        # 멀티코어 경합을 높여 unsafe demo에서 lost update를 더 잘 보여줌
go test -v ./...             # safe 함수들이 결정적으로 기대값을 반환하는지 테스트
go test -race ./...          # 테스트 경로는 race 없이 통과해야 함
```

`go run -race .`은 unsafe demo 때문에 race detector가 즉시 멈춘다. 이것은 의도된 동작이며, main.go의 unsafeIncrement가 실제로 data race를 일으킨다는 증거다. `go test -race ./...`는 main_test.go가 unsafe 함수를 호출하지 않기 때문에 통과한다.

## 코드로 확인하기

main.go는 크게 세 부분으로 나뉜다. `runSafeDemos`는 publishWithChannel, publishWithMutex, concurrentOnce를 차례로 호출해 항상 n*2라는 정확한 값을 출력한다. 이들이 결정적인 이유는 각 함수가 happens-before edge를 만들기 때문이다. publishWithChannel은 close(done)과 <-done이 순서를 만들고, publishWithMutex는 Unlock/Lock이, concurrentOnce는 once.Do 완료가 모든 반환보다 앞선다.

`runUnsafeDemo`는 unsafeIncrement를 5회 호출해 매 라운드 actual과 expected를 출력한다. expected는 workers * loops = 8000이지만, actual은 동기화 없는 counter++ 때문에 8000보다 작은 값이 자주 나온다. 이 숫자가 줄어드는 이유는 두 goroutine이 같은 메모리 위치를 읽고 1을 더해 다시 쓰는 과정이 원자적이지 않아 서로의 갱신을 덮어쓰기 때문이다. 만약 이 실행에서 lost update가 0이라도, race detector는 같은 코드에서 data race를 검출한다.

main_test.go는 위의 안전한 함수들이 실제로 happens-before 보장을 지키는지 검증한다. TestPublishWithChannel, TestPublishWithMutex, TestConcurrentOnce는 각각 100회, 100회, 50회 반복하면서 항상 기대값을 반환하는지 확인한다. 이것들은 `go test -race`에서도 통과해야 하며, 이 테스트들이 통과하는 것이 곧 해당 동기화 수단이 메모리 모델 규칙을 실제로 지킨다는 경험적 증거가 된다.

## 모르면 겪는 일

가장 흔한 사고는 flag로 완료를 알리는 패턴이다. writer goroutine이 `data = x; done = true`처럼 쓰고 reader가 `if done { use(data) }`를 읽는 코드는 done이 true인데 data가 예전 값인 경우를 만들 수 있다. 컴파일러는 done과 data의 쓰기 순서를 재배치할 수 있고, x86에서는 재배치가 잘 안 일어나지만 ARM에서는 자주 목격된다. 증상은 테스트에서는 잘 돌다가 실제 배포 후 간헐적으로 zero value를 읽거나 nil 포인터를 역참조하는 것이다.

또 하나는 double-checked locking이다. sync.Once 대신 `if p == nil { mu.Lock(); if p == nil { p = newConfig() }; mu.Unlock() } return p`를 쓰면, p에 할당하는 쓰기가 내부 필드 초기화보다 먼저 보일 수 있다. 다른 goroutine은 p가 nil이 아니라고 판단해 아직 초기화되지 않은 내부 필드를 읽고 panic을 낼 수 있다. Go에서 올바른 해법은 sync.Once를 쓰거나, p를 atomic.Value 또는 sync.Mutex로 보호하는 것이다.

lost update도 실제로 자주 겪는다. 여러 goroutine이 공유 카운터를 증가시키는 코드를 테스트하면 대부분 통과한다. 하지만 프로덕션에서 GOMAXPROCS가 높은 서버나 ARM 인스턴스에 올라가면 일부 증가분이 사라져 통계 수치가 실제보다 작게 집계된다. 이 문제는 CPU 프로파일에도 잡히지 않고, 로그도 정상처럼 보이기 때문에 오래 방치되기 쉽다. race detector를 CI에 넣으면 이런 부분을 즉시 찾을 수 있다.

## 언제 신경 쓰고 언제 무시하나

멀티 goroutine이 같은 변수를 읽고 쓰는 순간부터는 반드시 happens-before를 만들어야 한다. 값이 단순 int 하나라도, 읽기 전용이라면 공유가 안전하지만 한쪽이라도 쓰면 그렇지 않다. 특히 ARM, MIPS, RISC-V처럼 약한 메모리 모델을 쓰는 서버에 배포할 계획이라면 x86에서 통과하는 테스트만으로는 안심할 수 없다. 이런 환경에서는 채널, mutex, once, atomic 중 하나라도 명시적으로 쓰는 습관이 필요하다.

반대로 단일 goroutine만 접근하는 변수는 동기화가 전혀 필요 없다. 처음에 잠깐 초기화하고 이후에는 읽기만 하는 설정도 main goroutine이 시작 전에 모두 완료했다면 안전하다. 작은 CLI 도구나 일회성 스크립트에서 goroutine이 몇 개 안 돌 때는 과도한 mutex나 atomic 도배가 코드를 읽기 어렵게 만들고 성능에도 도움이 되지 않는다. 성급한 최적화보다는 먼저 race detector를 돌려 실제 data race가 있는지 확인하는 것이 낫다.

테스트가 통과한다고 안심하면 안 되지만, 반대로 "동기화가 조금이라도 있으면 무조건 좋다"는 태도도 문제가 된다. 쓸데없는 잠금은 goroutine을 블록시켜 처리량을 떨어뜨리고 데드락 가능성을 높인다. 실무에서는 공유 상태가 생기는 경계를 먼저 파악하고, 그 경계에 channel 또는 mutex 하나를 두는 식으로 단순하게 설계하는 것이 낫다. 이 패턴을 지키면 나중에 ARM 서버로 옮겨도 happens-before 규칙이 동일하게 보장해준다.

## 더 파보기

- [The Go Memory Model](https://go.dev/ref/mem) — happens-before와 synchronizes-before의 공식 정의
- [Russ Cox: Memory Models](https://research.swtch.com/mm) — Go 메모리 모델이 만들어진 배경과 역사
- [runtime/chan.go](https://go.dev/src/runtime/chan.go) — 채널 send/close/receive가 hchan과 sendq, recvq를 다루는 구현
- [sync/mutex.go](https://go.dev/src/sync/mutex.go) — Mutex의 state atomic 조작과 sema 사용
- [sync/once.go](https://go.dev/src/sync/once.go) — Once가 done 플래그로 happens-before를 만드는 구현
- [Data Race Detector](https://go.dev/blog/race-detector) — race detector가 실제로 잡는 버그 패턴