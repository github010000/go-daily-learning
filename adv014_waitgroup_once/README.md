## 한 줄 요약

WaitGroup 은 counter 와 waiter 수를 하나의 64비트 atomic word 에 packed 해서
Add/Wait 전환을 atomically 처리하고, Once 는 atomic done flag 로 빠른 길을 연 뒤
Mutex 와 double-checked locking 으로 최초 1회 실행만 보장한다. 이 구현을 모르면
Add 위치를 한 줄 잘못 두었을 뿐인데 부하가 커졌을 때 Wait 가 일찍 반환하거나
간헐적으로 panic 이 난다.

## 왜 이런 설계인가

goroutine 은 OS 스레드처럼 명시적인 join API 가 없다. 어떤 작업 집합 전체를
기다리려면 "아직 끝나지 않은 작업 수"를 세고, 그 수가 0 이 되는 순간 기다리는
쪽을 깨워야 한다. 이 일을 단순한 `done` 채널로도 할 수 있지만, 채널은 보통
1회성 신호에 가깝고 동적으로 작업 수가 변하거나 waiter 가 여럿인 상황에는
Count 와 Wake 를 직접 관리하는 전용 구조가 필요하다. WaitGroup 은 이 Count 와
Wake 관리를 sync 패키지 안에서 runtime semaphore 로 연결한 것이다.

가장 직접적인 대안은 `sync.Mutex` 로 상태를 보호하는 것이다. 예를 들어
`mu.Lock()` 으로 `counter`, `waiters`, `sema` 를 바꾸는 구조를 생각할 수 있다.
그런데 그렇게 하면 worker 가 Done 을 부를 때마다, 그리고 waiter 가 Wait 에
들어갈 때마다 Mutex 경합이 생긴다. 수천 개 goroutine 이 동시에 Done 을 부르는
부하에서는 이 Mutex 가 p99 latency 를 밀어올린다. WaitGroup 은 공통 경로에서
Mutex 를 쓰지 않고, 단 하나의 64비트 atomic add 로 counter 와 waiter 를 함께
바꾸도록 만들어 이 경합을 없앴다.

왜 두 값을 하나의 64비트에 넣었는지가 핵심이다. counter 와 waiter 는 서로
독립적인 값처럼 보이지만, 실제로는 "counter 가 0 이 되었고 waiter 가 있다"는
상태 전이가 atomic 해야만 한다. 만약 counter 와 waiter 를 별도 atomic 변수로
두면, Done 이 counter 를 0 으로 만든 뒤 waiter 를 읽기 전에 Wait 가 waiter 를
증가시킬 수 있다. 이 순서가 꼬이면 waiter 가 sema 에서 잠들었는데 아무도 깨우지
않는 lost wakeup 이 생길 수 있다. 한 word 에 packed 하면 두 값을 하나의 atomic
load/add 로 읽거나 바꿀 수 있으므로 이 경쟁 창 자체가 사라진다.

배치도 중요하다. 상위 32비트는 signed counter 로 쓴다. Go 구현은 `Add(delta)` 에서
`uint64(delta) << 32` 를 더하고, 결과를 `int32(state >> 32)` 로 읽는다. 이렇게
하면 counter 가 음수인지 확인해 Done 없는 Add(-1) 를 panic 으로 잡을 수 있다.
하위 32비트는 waiter count 로 쓴다. waiter 가 0 이면 sema release 없이 반환하고,
0 이 아니면 counter 가 0 이 됐을 때 waiter 수만큼 runtime semaphore 를 풀어준다.

정렬 요구는 32비트 플랫폼에서 특히 중요하다. 64비트 atomic 연산은 주소가 8바이트
정렬돼 있어야 하는 아키텍처가 있다. 옛날 WaitGroup 구현은 `state1 [3]uint32` 를
두고 주소가 8의 배수인지 확인한 뒤 그렇지 않으면 한 칸 밀린 위치를 state 로
사용하는 방식으로 정렬을 보장했다. 현재 Go 소스는 `atomic.Uint64` 타입을 쓴다.
이 타입은 Go 컴파일러가 64비트 atomic 연산에 필요한 정렬을 보장하도록 되어 있다.
그래도 sync 패키지 밖에서 직접 atomic.Uint64 필드를 만들 때는 필드 순서를 함부로
바꾸면 안 된다. 표준 sync.WaitGroup 은 이 문제를 이미 해결해 두었지만, 직접
비슷한 구조를 만들 때는 정렬 규칙을 알아야 한다.

Once 도 비슷한 성능 고민에서 나왔다. "한 번만 실행"을 그냥 Mutex 로 구현하면 모든
호출이 Mutex 를 잡아야 해서, 이미 초기화된 뒤에도 느리다. 그렇다고 `if !done {
f(); done = true }` 처럼 atomic 확인만 하면 여러 goroutine 이 동시에 `f` 를 실행할
수 있다. Once 는 이 둘을 섞은 double-checked locking 을 쓴다. 이미 done 이면
atomic load 한 번만으로 돌아가고, 아직 done 이 아니면 Mutex 로 직렬화한 뒤 다시
확인해 최초 1회만 실행한다.

## 어떻게 동작하는가

최근 Go 의 `src/sync/waitgroup.go` 를 보면 WaitGroup 구조는 대략 다음과 같다.

```go
type WaitGroup struct {
    noCopy noCopy
    state  atomic.Uint64
    sema   uint32
}
```

`noCopy` 는 `sync` 패키지에서 정의한 zero-size 타입이다. WaitGroup 을 실수로
복사하면 `go vet` 이 잡아주도록 돕는다. `state` 의 상위 32비트는 counter, 하위
32비트는 waiter 이다. `sema` 는 runtime semaphore 를 나타내는 값으로, 실제로는
runtime semaphore 테이블의 키로 쓰인다.

`Add(delta int)` 는 `delta` 를 상위 32비트에 반영한다. 구현의 핵심 부분만
의사적으로 보면 다음과 같다.

```go
state := wg.state.Add(uint64(delta) << 32)
v := int32(state >> 32) // counter
w := uint32(state)      // waiters

if v < 0 {
    panic("sync: negative WaitGroup counter")
}

if w != 0 && delta > 0 && v == int32(delta) {
    panic("sync: WaitGroup misuse: Add called concurrently with Wait")
}

if v > 0 || w == 0 {
    return
}

// counter 가 0 이고 waiter 가 있을 때만 아래로 내려온다.
wg.state.Store(0)
for ; w != 0; w-- {
    runtime_Semrelease(&wg.sema, false, 0)
}
```

여기서 `v == int32(delta)` 조건이 특히 중요하다. 이 조건은 positive Add 가
들어왔을 때 이전 counter 가 0 이었음을 뜻한다. waiter 가 있고 counter 가 0 인
상태에서 새 작업을 추가하면 이미 Wait 가 반환 경로에 있는데 새 task 가 끼어드는
것이므로 misuse panic 을 낸다. 이 검사는 모든 오남용을 잡지 못한다. 하지만
가장 위험한 재사용 경쟁의 일부를 확실히 알려준다.

`Wait()` 는 `wg.state.Add(1)` 로 waiter 수를 1 늘린다. 그 뒤 counter 가 0 이면
즉시 반환한다. counter 가 0 이 아니면 `runtime_Semacquire(&wg.sema)` 로 잠든다.
WaitGroup 은 waiter count 를 나중에 다시 감소시키지 않는다. counter 가 0 이 된
`Add` 가 state 전체를 0 으로 밀어버리고, 잠든 waiter 수만큼 sema 를 release 한다.
그래서 state 의 상위와 하위가 반드시 한 word 안에 있어야 하며, 그렇지 않으면
이 관계가 깨질 수 있다.

`src/sync/once.go` 의 구조는 더 단순하다.

```go
type Once struct {
    done atomic.Uint32
    m    Mutex
}

func (o *Once) Do(f func()) {
    if o.done.Load() == 0 {
        o.doSlow(f)
    }
}

func (o *Once) doSlow(f func()) {
    o.m.Lock()
    defer o.m.Unlock()
    if o.done.Load() == 0 {
        defer o.done.Store(1)
        f()
    }
}
```

첫 `if o.done.Load() == 0` 이 fast path 다. 이미 done == 1 이면 Mutex 를 아예
잡지 않는다. 이런 호출이 수백만 번 일어나도 atomic load 하나만 쓴다. 반면 아직
done == 0 이면 slow path 에 들어가 Mutex 를 잡고 다시 done 을 확인한다. 이 재확인이
없으면 Mutex 를 잡기 직전에 여러 goroutine 이 f 를 실행할 수 있다. `defer
o.done.Store(1)` 를 f 보다 먼저 등록해 두기 때문에 f 가 정상 반환하든 panic 으로
풀리든 done 은 1 이 된다. 물론 f 가 panic 하면 작업이 완료되지 않았어도 Once 는
이미 사용된 것으로 간주한다.

이 구현에서 done 은 atomic 이므로 fast path 와 slow path 사이에 data race 가 없다.
done 을 plain bool 로 바꾸면 race detector 에 걸리고, 메모리 모델에 따라 f 실행
여부가 잘못 보일 수 있다. Mutex 는 최초 실행 직전의 경쟁만 직렬화한다. 한 번
done 이 1 로 바뀐 뒤에는 대부분의 호출이 lock 없이 끝난다.

## 돌려보기

이 디렉토리에서 아래 명령을 순서대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사: sync.Once 복사나 lock 복사가 없음을 확인한다
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행: packed state, 오용, Once 동작 출력을 본다
GOMAXPROCS=2 go run .        # goroutine 스케줄링을 강제한 시연
go test -v ./...             # 테스트: 반환 값 불변식을 검증한다
go test -race ./...          # 동시성 주제이므로 data race 가 없음을 확인한다
```

`go vet` 은 혹시라도 sync.Once 나 WaitGroup 같은 lock-containing 값을 복사하면
copylock 경고를 낸다. `go run .` 에서는 특히 3번 섹션과 6번 섹션의 숫자 차이를
봐야 한다. `go test -race` 는 의도적으로 race 를 만든 코드를 테스트 함수에서는
부르지 않기 때문에 반드시 통과해야 한다.

## 코드로 확인하기

`main.go` 의 첫 번째 출력은 WaitGroup 이 내부적으로 쓰는 packed state 의 모양을
보여준다. `Add(2) 후` 는 `raw=0x0000000200000000` 으로 시작한다. 상위 32비트가
`0x00000002`, 하위 32비트가 `0x00000000` 이다. `Wait 등록 후` 는
`raw=0x0000000200000001` 이 되고, 이는 waiter 가 1 늘었다는 뜻이다. `Done 후` 는
`raw=0x0000000100000001` 이 되어 counter 는 2 에서 1 로 줄고 waiter 는 그대로다.
이 16진수 출력이 WaitGroup 의 단일 word 상태 전환을 눈으로 보여준다.

두 번째와 세 번째 출력은 올바른 사용과 잘못된 사용을 나란히 놓는다. 올바른
`runCorrectWaitGroup` 은 `Add(n)` 를 goroutine 을 만들기 전에 호출하므로
`Wait 반환 시 completed = 8/8` 이 나온다. 반면 `runWrongAddInGoroutine` 은
worker goroutine 안에서 `wg.Add(1)` 을 부른다. 이 함수는 start 채널로 worker 를
막아 두어 Wait 가 counter 0 인 시점에 호출되도록 만든다. 따라서
`Wait 반환 시 completed=0/8, 실제 최종 완료=8/8` 이 출력된다. Wait 가 반환한
시점에는 아무 worker 도 등록되지 않았지만, 나중에는 8개가 완료된다. 이는 Wait 가
작업을 전혀 기다리지 않았음을 보여준다.

네 번째 출력은 `wg.Done()` 을 Add 없이 호출해 counter 를 음수로 만든다. 그 결과
`sync: negative WaitGroup counter` panic 이 recover 로 잡혀 출력된다. 이 panic 은
상위 32비트를 signed int32 로 읽기 때문에 발생한다.

다섯 번째 출력은 진짜 sync.Once 를 여러 goroutine 에서 동시에 호출한다.
`runOnce(8)` 는 f 가 몇 번 실행됐는지를 atomic counter 로 반환하는데, 출력은
`실행된 f 횟수 = 1 (기대 1)` 이다. 이 숫자가 2 이상이라면 Once 의 재검사가 없는
잘못된 구현이라는 뜻이다.

여섯 번째 출력은 `countingDoubleOnce` 라는 교습용 Once 로 double-checked locking
경로를 센다. 처음 한 번 f 를 실행해 done 을 1 로 만든 뒤 8개 goroutine 이 다시
Do 를 호출한다. 출력은 `fast path=8, slow path=1` 이다. slow path 1 은 warm-up
때 한 번만 진입한 것이고, 이후 8회는 모두 atomic load 만으로 빠져나갔다. 이는
sync.Once 가 성능을 위해 fast path 를 두는 이유를 그대로 보여준다.

일곱 번째 출력은 `once.Do(func(){ once.Do(func(){}) })` 의 deadlock 을 확인한다.
바깥 Do 가 Mutex 를 쥔 채 안쪽 Do 를 호출하므로 500ms 타임아웃 뒤
`deadlock: inner Do waits for outer Do` 가 출력된다. 이 시나리오는 main 전용이라
테스트에서는 돌지 않는다.

`main_test.go` 는 이 관찰 가능한 동작을 불변식으로 잡아 둔다.
`TestRunCorrectWaitGroup` 은 `completed == n` 을 확인한다.
`TestRunWrongAddInGoroutine` 은 채널 fence 를 사용했기 때문에 시간에 기대지 않고
`returnedBefore == 0`, `finalCompleted == n` 을 확인한다.
`TestRunNegativeAddPanic` 은 panic 문자열에 `negative WaitGroup counter` 가
들어 있는지 확인한다.
`TestRunOnceExactlyOnce` 는 동시 호출에도 f 가 정확히 1회만 실행되는지 본다.
`TestRunCountingOnceAfterFirst` 는 warm-up 후 fast path 가 n 회, slow path 가
1 회임을 확인해 double-checked locking 구조가 깨지지 않았는지 검증한다.

## 모르면 겪는 일

가장 자주 겪는 버그는 WaitGroup 의 Add 를 goroutine 안에서 부르는 것이다. 개발
환경에서는 goroutine 이 바로 스케줄링되어 Wait 전에 Add 가 실행될 수 있어서
멀쩡해 보인다. 그런데 운영 환경에서 GOMAXPROCS 가 높아지거나 CPU 경합이 생기면
Wait 가 counter 0 을 보고 먼저 반환한다. 증상은 main 이 자식 goroutine 을 기다리지
않고 종료해 로그 마지막 부분이 잘리거나, 요청을 처리하기 전에 HTTP 서버가 셧다운
되는 식으로 나타난다. 테스트에서는 가끔 잘 통과하기 때문에 원인을 찾기 어렵다.

또 다른 증상은 `sync: WaitGroup misuse: Add called concurrently with Wait`
panic 이다. 이 panic 은 항상 나지 않는다. WaitGroup 을 재사용할 때 Waiter 가 있고
counter 가 0 이 되는 아주 좁은 창에서 positive Add 가 들어와야 발생한다. 부하가
늦게 걸리는 서비스에서 배포 직후에는 없다가 트래픽이 몰리면 간헐적으로 나온다.
이 panic 을 로컬에서 재현하려고 해도 스케줄링이 다르면 재현되지 않아서
"가끔 죽는 버그"로 남는다.

Once 를 잘못 쓰면 역시 간헐적으로 문제가 생긴다. Once 를 복사하면 done flag 와
Mutex 상태가 함께 복사된다. 복사본을 만들기 전에 이미 done 이면 복사본의 Do 는
f 를 실행하지 않을 수 있다. 반대로 복사 전에 사용하지 않았다면 원본과 복사본이
각각 f 를 실행할 수도 있다. `go vet` 이 잡아주는 경우도 있지만, Once 를 포함한
구조체를 value 로 복사하거나 interface 를 통해 넘기면 눈에 잘 띄지 않는다.

Once 의 f 가 같은 Once 를 재귀적으로 호출하면 프로세스가 아무 로그도 남기지 않고
그 자리에서 멈춘다. 예를 들어 초기화 함수가 내부에서 동일한 초기화 경로를 한 번
더 태우는 실수를 하면, outer Do 가 Mutex 를 쥔 채 inner Do 가 같은 Mutex 를
기다리는 deadlock 이 된다. 이런 문제는 설정이 많거나 패키지 init 순서가 꼬인
서비스에서 실제로 나타난다.

구조를 모르고 직접 Once 비슷한 것을 plain bool 로 만들면 `go test -race` 에서
data race 가 잡히고, 경쟁이 심한 운영 환경에서는 f 가 두 번 실행되거나 다른
goroutine 이 done 을 보지 못해 영영 초기화되지 않은 것처럼 보일 수 있다.
WaitGroup 과 Once 는 둘 다 atomic 과 runtime semaphore, Mutex 를 정교하게 써서
이 경쟁을 피한다. 겉보기에는 단순해 보여도 이 내부 규칙을 무시한 재구현은
쉽게 무너진다.

## 언제 신경 쓰고 언제 무시하나

일반 애플리케이션 코드에서는 WaitGroup 의 64비트 packing 을 직접 알 필요가 없다.
수십 개 goroutine 을 기다리는 정도라면 `Add` 를 goroutine 만들기 전에 호출하고
`Done` 을 defer 로 호출하는 규칙만 지키면 된다. 이 수준에서는 WaitGroup 대신
channel 이나 errgroup 을 써도 아무 문제 없다. WaitGroup 내부의 atomic 최적화는
sync 패키지 자체나 대규모 동시성 라이브러리에서 의미가 크다.

Once 도 프로세스당 한 번만 불리는 초기화라면 double-checked locking 을 전혀
의식할 필요 없다. 하지만 logger, connection pool, reflection cache 처럼 최초
1회 뒤에도 Do 가 반복해서 호출되는 경로에서는 fast path 의 atomic load 가
p99 에 영향을 준다. 그렇다고 직접 Once 를 재구현할 이유는 없다. 표준 sync.Once 로
충분하고, 관찰이 필요할 때만 이 예제처럼 계수용 구조체로 실험해 보면 된다.

깊게 신경 써야 하는 때는 동시성 primitive 자체를 만들거나, 재사용 가능한 라이브러리
코드를 작성하거나, 실전에서 간헐적 misuse panic 을 디버깅할 때다. 특히 WaitGroup 을
재사용할 때는 다음 세대의 Add 가 반드시 이전 Wait 반환 이후에 일어나야 한다는
규칙을 문서로 남겨야 한다. 이 경계를 무시하면 사용자가 순서를 조금만 바꿔도
패닉이나 조기 반환이 발생한다.

부하가 낮고 goroutine 수가 적은 테스트에서는 잘못된 패턴이 잘 드러나지 않는다.
따라서 "내 코드는 잘 돌았다"는 증거가 되지 않는다. 반대로 부하 테스트에서
간헐적으로 panic 이 보이면 무작정 재시도로 넘기지 말고 WaitGroup 의 Add 위치와
Once 복사 여부부터 점검하는 것이 빠르다.

## 더 파보기

- https://go.dev/src/sync/waitgroup.go
- https://go.dev/src/sync/once.go
- https://go.dev/src/runtime/sema.go
- https://pkg.go.dev/sync#WaitGroup
- https://pkg.go.dev/sync#Once
- https://go.dev/doc/articles/race_detector