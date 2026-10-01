## 한 줄 요약

채널은 하나의 `hchan` 구조체에 고정 크기 환형 버퍼와 두 개의 대기 큐 `sendq`/`recvq`를 가진다. buffered와 unbuffered는 `dataqsiz`가 0인지 아닌지만 다르고, 실제 차이는 송신·수신이 상대방 goroutine을 만나지 못했을 때 버퍼에 머무는가 아니면 즉시 대기 큐로 들어가는가에 있다. 여기서 말하는 동기화는 추상적인 약속이 아니라, goroutine이 `sudog`로 `sendq`나 `recvq`에 들어가 스스로 멈추고 상대방이 값을 복사한 뒤 `goready`로 다시 깨우는 구체적인 실행 과정이다.

## 왜 이런 설계인가

Go는 OS 스레드를 수천 개 만드는 대신 goroutine을 만들고, goroutine 사이 통신을 채널이라는 첫 번째 시민으로 만들었다. 그런데 채널은 그냥 "동시성 안전한 큐"가 아니다. `select`에서 여러 채널을 동시에 기다릴 수 있어야 하고, `close` 이후에도 값이 남아 있는 상태를 정확히 구분해야 한다. 일반 `sync.Mutex`와 slice로 큐를 만들면 이런 선택적 대기와 닫힘 상태 전파를 흉내 내기 어렵고, goroutine을 세밀하게 멈추고 깨우는 것도 불가능하다. 그래서 채널은 runtime 패키지 안에 builtin으로 들어갔다.

`hchan`은 고정 크기 환형 버퍼를 가진다. 동적으로 자라는 slice를 쓰지 않은 이유는 채널이 고속 send/receive 경로에서 매번 할당이나 복사를 일으키지 않게 하기 위해서다. `buf`는 생성 시점에 한 번 할당되고, `sendx`와 `recvx`라는 두 인덱스만 바뀐다. 버퍼가 가득 차면 송신자가 블록되므로 채널 용량은 그대로 backpressure 역할을 한다. 대신 용량을 키우려면 새 채널을 만들어야 하고, 콘솔 명령 한 줄로 capacity를 바꿀 수 없다.

unbuffered 채널에서는 `dataqsiz == 0`이다. 이때 버퍼를 거치지 않고, 수신자가 이미 `recvq`에서 기다리고 있다면 송신자는 `sendDirect`로 값을 receiver의 스택 슬롯에 바로 복사한다. 버퍼로 한 번 넣고 receiver가 다시 꺼내는 두 번의 `memmove`를 피하는 최적화다. buffered 채널이더라도 버퍼가 비어 있고 수신자가 기다리는 경우에는 같은 직접 복사 경로를 탄다. 버퍼에 값이 남아 있는데 수신자가 기다릴 수는 없기 때문에 이 경로는 FIFO 순서를 깨지 않는다.

송신자나 수신자가 블록될 때는 goroutine을 통째로 재우지 않고 `sudog`라는 대기 구조체로 큐에 넣는다. `sudog`에는 자기 goroutine을 가리키는 `g`와 값을 받거나 줄 스택 주소 `elem`이 들어 있다. 이렇게 하면 select에서 여러 채널을 기다릴 때도 goroutine 하나를 여러 대기 큐에 넣거나 뺄 수 있고, 하나의 채널이 성공하면 다른 채널에서 뺄 수 있다. lock-free 큐보다 느릴 수 있지만 select, close, 메모리 안전성까지 고려한 실용적 선택이다.

## 어떻게 동작하는가

채널의 실제 구조체는 `runtime/chan.go`에 정의되어 있다. 핵심 필드는 다음과 같다.

```go
type hchan struct {
    qcount   uint           // 현재 버퍼에 들어 있는 원소 수 = len(ch)
    dataqsiz uint           // 버퍼 전체 크기 = cap(ch)
    buf      unsafe.Pointer // 버퍼 배열의 시작 주소
    elemsize uint16
    closed   uint32
    elemtype *_type
    sendx    uint           // 다음에 보낼 버퍼 인덱스
    recvx    uint           // 다음에 받을 버퍼 인덱스
    recvq    waitq          // 수신 대기 중인 sudog 연결 리스트
    sendq    waitq          // 송신 대기 중인 sudog 연결 리스트
    lock     mutex
}
```

`waitq`는 `first *sudog`와 `last *sudog`를 가진 단순 연결 리스트다. `make(chan int)`는 `dataqsiz == 0`이므로 버퍼가 없다. `make(chan int, 3)`은 `elemtype` 크기와 `dataqsiz`를 곱한 만큼 `buf` 배열을 hchan과 함께 할당한다.

송신 경로 `chansend`는 크게 세 갈래로 나뉜다. 첫째, `c.recvq.dequeue()`로 대기 중인 수신자가 있으면 그 `sudog`를 꺼내 `send` 함수를 호출한다. `send`는 `sg.elem != nil`이면 `sendDirect`를 호출해 송신자의 스택 값에서 receiver의 스택 주소로 `memmove` 한 번을 수행한다. 그 후 `goready(gp)`로 receiver goroutine을 run queue에 올리고 송신자는 반환한다. 이 과정은 unbuffered 채널에서 매우 흔하게 일어난다.

둘째, 대기 수신자가 없고 `qcount < dataqsiz`이면 환형 버퍼에 값을 넣는다. `chanbuf(c, sendx)` 위치에 값을 복사하고 `sendx`를 `(sendx + 1) % dataqsiz`로 바꾸고 `qcount`를 증가시킨다. 셋째, 버퍼가 가득 차면 송신자 자신을 `sudog`로 만들어 `sendq`에 넣고 `gopark`로 스스로 멈춘다. 깨어나면 다시 락을 잡고 보내거나, 채널이 이미 닫혔으면 panic한다.

수신 경로 `chanrecv`도 대칭적이다. `qcount > 0`이면 `recvx` 위치에서 값을 receiver로 복사하고 그 자리를 정리한 뒤 `recvx`를 전진시킨다. 만약 버퍼가 가득 차서 이미 기다리고 있던 송신자가 있다면, 그 송신자의 값을 비워진 버퍼 슬롯에 넣고 송신자를 깨운다. `qcount == 0`이고 `sendq`에 송신자가 있다면 unbuffered처럼 송신자의 스택에서 receiver 스택으로 직접 가져온다. 그조차 없으면 receiver는 `recvq`에 `sudog`로 들어가 `gopark`한다.

여기서 "동기화"는 단순히 "잠깐 멈춘다"가 아니다. `gopark`는 현재 goroutine을 run queue에서 빼서 채널 대기 큐에만 남겨 두고, 스케줄러가 다른 goroutine을 실행하게 한다. `goready`는 깨어난 상대방을 run queue에 넣는다. 즉시 실행되는 것이 아니라 스케줄링될 뿐이다. 값 복사와 락, `gopark`/`goready` 사이의 순서는 Go 메모리 모델이 정한 채널 동기화 관계를 만든다. unbuffered 송신-수신 한 쌍은 happens-before 관계를 만들고, buffered 채널에서는 k번째 send가 k번째 receive와 동기화된다.

## 돌려보기

이 디렉토리에서 아래 명령을 그대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사: 포맷 문제, 의심스러운 동시성 패턴 확인
go build -o /dev/null ./...  # package main 포함 전체 컴파일 성공 확인
go run .                     # 이벤트 순서, len/cap, schedtrace 안내 출력
go test -v ./...             # 이벤트 순서와 FIFO 불변식 테스트
go test -race ./...          # race detector 로 goroutine 사용부 검증
go test -bench . -benchmem -run '^$' ./...  # unbuffered vs buffered cap1 벤치마크
GODEBUG=schedtrace=1000 go run .            # 1초마다 goroutine 상태/runqueue 관찰
```

`go run .`에서는 세 가지 시연이 출력된다. unbuffered 시연에서는 receiver가 먼저 blocked 상태를 기록하고, sender가 send한 뒤 receiver got, sender returned 순서가 된다. buffered 시연에서는 sender returned가 receiver got보다 먼저 나온다. FIFO 시연에서는 1과 2를 넣은 뒤 꺼낸 순서가 1, 2로 유지된다.

`go test -race ./...`는 이 디렉토리의 테스트가 goroutine 경합 없이 통과하는지 보여 준다. 벤치마크 명령에서는 `BenchmarkUnbufferedChan`과 `BenchmarkBufferedChanCap1`의 ns/op와 allocs/op을 비교할 수 있다. `GODEBUG=schedtrace=1000`을 붙이면 Go runtime이 1초마다 P 개수, runqueue 길이, idle/runnable/syscall 상태를 출력한다. 이 데모는 짧게 끝나므로 trace 출력이 많지 않을 수 있고, 채널에 수천 개 goroutine이 붙는 부하 테스트를 돌릴 때 더 유용하다.

## 코드로 확인하기

`main.go`의 `unbufferedHandoffSequence`는 unbuffered 채널 `make(chan int)`를 만든다. receiver goroutine이 `rec.add("receiver goroutine: blocked before receive")`를 기록하고 `ready`를 닫으면 main은 그때부터 sender 역할을 한다. `ch <- 42`를 실행하면 runtime은 receiver를 `recvq`에서 꺼내 값 42를 receiver 스택으로 바로 옮긴다. main은 `got` 채널을 기다려 receiver가 값을 확인한 뒤에야 "send returned"를 기록하므로 출력 순서가 receiver got → sender returned로 고정된다. 이는 "unbuffered 송신은 receiver가 실제로 값을 가져갈 때까지 반환하지 않는다"는 사실을 순서로 보여 준다.

`bufferedSendSequence`는 `make(chan int, 1)`로 버퍼 한 칸을 확보한다. main goroutine이 `ch <- 42`를 실행하면 `qcount`가 0에서 1로 올라가고 바로 다음 줄이 실행된다. 아직 receiver goroutine은 시작하지도 않았다. 그래서 "sender goroutine: send returned even though no receiver yet"가 먼저 나온다. 그 후 receiver가 시작되어 버퍼에서 값을 꺼낸다. `bufferedCircularFIFOSequence`는 `make(chan int, 2)`에 1과 2를 넣고 `recvx` 순서대로 1, 2를 꺼낸다. 버퍼가 환형이어도 FIFO가 유지됨을 확인한다.

`channelCapacityAndLength`는 `len`과 `cap`이 각각 `qcount`와 `dataqsiz`에 대응한다는 것을 보여 준다. unbuffered 채널은 `len=0 cap=0`이고, buffered 채널에 값을 하나 넣으면 `len=1 cap=2`가 된다. 테스트는 `TestUnbufferedHandoffSequence`가 sender 반환 전에 receiver got이 반드시 있어야 한다는 순서를 검증하고, `TestBufferedSendSequence`는 sender returned가 receiver got보다 먼저 나올 수 있다는 사실을 검증한다. `TestBufferedCircularFIFOSequence`는 1 다음 2가 나온다는 FIFO 불변식을 확인한다.

벤치마크는 sender와 receiver가 한 쌍이 되어 채널을 왕복하는 비용을 측정한다. `b.ResetTimer`는 receiver goroutine을 미리 만들어 놓고 호출하므로 goroutine 시작 비용이 측정에 섞이지 않는다. buffered cap1은 버퍼를 한 칸 두어 일부 블록을 흡수하지만, 이 벤치마크에서는 sender와 receiver가 거의 동시에 움직이므로 unbuffered와 큰 차이가 없을 수도 있다. 실제로는 producer와 consumer의 속도가 다를 때 buffered가 블록 빈도를 줄이는 효과가 훨씬 크다.

## 모르면 겪는 일

buffered 채널이 "완전히 비동기 큐"라고 생각하면 deadlock을 만난다. `ch := make(chan int, 1); ch <- 1; ch <- 2`를 main goroutine에서 실행하면 첫 send만 버퍼에 들어가고 두 번째 send는 `sendq`에서 영원히 잔다. main goroutine이 유일한 goroutine이면 runtime이 `fatal error: all goroutines are asleep - deadlock!`을 출력하고 프로그램이 죽는다. 버퍼가 있어도 그 용량을 넘는 순간 buffered 채널도 synchronous rendezvous처럼 동작한다는 사실을 모르면 자주 겪는 사고다.

unbuffered 채널을 빠른 producer와 느린 consumer 사이에 넣으면 producer가 매 send마다 consumer를 기다리므로 producer goroutine들이 `sendq`에 쌓인다. CPU 사용률은 낮은데 처리량이 오르지 않고, goroutine 프로파일에는 `chan send`에서 잠든 상태로 보인다. 이 경우 CPU 프로파일에는 뜨겁게 나오는 함수가 없어서 "왜 느린지" 찾기 어렵다. p99 latency가 스파이크처럼 튀고, 큐 길이는 쌓이지 않으니 메트릭만 봐서는 병목을 놓친다.

`len(ch)`를 동기화 판단에 쓰는 것도 위험하다. unbuffered 채널은 receiver가 기다리고 있어도 `len`이 0이다. buffered 채널은 `len`이 버퍼에 남은 개수만 의미할 뿐, 지금 receiver가 기다리는지 sender가 기다리는지는 전혀 알려 주지 않는다. 게다가 채널은 동시에 변하므로 `if len(ch) > 0 { x := <-ch }` 같은 코드는 race는 아니어도 논리 오류를 만들기 쉽다. 채널 상태를 보고 분기하기보다 `select`로 기다리는 것이 Go의 관례다.

채널이 값을 직접 복사한다는 사실도 잊으면 안 된다. unbuffered 송신이 receiver 스택에 직접 쓴다는 것은 곧 채널이 값을 보관하는 상자가 아니라 전달 지점이라는 뜻이다. 큰 구조체를 매번 값으로 주고받으면 그 크기만큼 `memmove`가 반복된다. 반면 포인터를 보내면 포인터 복사는 값싸지만 힙 객체의 소유권이 모호해지고 race가 날 수 있다. buffered 채널에 포인터를 넣으면 버퍼에 포인터가 남아 참조한 객체의 GC 시점이 늦춰진다.

## 언제 신경 쓰고 언제 무시하나

일반 애플리케이션 코드에서는 hchan 내부 구조를 몰라도 된다. "unbuffered는 만남, buffered는 짧은 대기열"이라는 의미만 알아도 대부분의 코드는 안전하게 돌아간다. 채널 연산 자체가 충분히 빠르기 때문에 초당 수십만 회 미만이라면 send/receive 한 번의 `memmove`나 `gopark` 비용은 프로파일에 거의 잡히지 않는다. 이 수준에서는 버퍼 용량을 미세 조정하기보다 producer-consumer 구조를 단순하게 유지하는 것이 더 중요하다.

이 지식이 중요해지는 때는 채널이 처리량의 중심에 있는 파이프라인을 만들 때다. producer n개, consumer m개를 두고 초당 수백만 메시지를 보내야 하거나, 어떤 burst를 얼마나 버텨야 하는지 결정해야 한다면 buffered/unbuffered의 블록 특성을 알아야 한다. buffered 채널은 짧은 burst를 흡수해 producer가 잠깐 멈추지 않게 해 주지만, consumer가 계속 느리면 결국 버퍼가 차고 같은 블록 현상이 일어난다. backpressure를 의도적으로 만들고 싶다면 unbuffered가 더 분명한 rendezvous 지점이 된다.

runtime trace나 goroutine dump에서 `chan send`/`chan receive`에 수천 개 goroutine이 잠들어 있는 것을 봤다면 hchan의 `sendq`/`recvq` 구조를 떠올려야 한다. goroutine들이 CPU를 먹지 않고 큐에만 붙어 있다는 것은 채널 통신이 오래 막히고 있다는 뜻이다. 반대로 CPU 사용률은 높은데 채널 연산이 자주 보이지 않으면 채널 문제가 아니다. 이 구분을 모르면 엉뚱한 곳에서 최적화하게 된다.

대규모 분산 처리나 초저지연 네트워크 서버가 아니라면 buffered 채널의 capacity 몇 개 차이로 성능이 크게 바뀌는 경우는 드물다. capacity 1과 10은 동시성 의미가 다를 수 있지만, 대개는 코드 의도에 맞게 "burst를 얼마나 허용할 것인가"로 정하면 된다. benchmark에서 차이가 나도 실제 부하에서는 조금 다르게 나오므로, 추측으로 capacity를 고르기보다 `go test -bench`와 실제 워크로드 프로파일을 함께 보는 편이 낫다.

## 더 파보기

- runtime/chan.go 공식 소스: https://go.dev/src/runtime/chan.go
- runtime/select.go 공식 소스: https://go.dev/src/runtime/select.go
- Go 메모리 모델: https://go.dev/ref/mem
- Share Memory by Communicating 블로그: https://go.dev/blog/codelab-share
- Go 스케줄러가 gopark/goready를 다루는 방식: https://go.dev/src/runtime/proc.go