## 한 줄 요약

Go는 `sync/atomic` 패키지에 relaxed, acquire-release 같은 메모리 순서 선택지를 노출하지 않고 **순차 일관성(sequential consistency)** 만 제공한다. Go 1.19부터는 `atomic.Int64`, `atomic.Bool`, `atomic.Pointer` 같은 타입 안전 래퍼가 추가되어 잘못된 주소나 비트 폭 실수를 컴파일 타임에 막는다. `atomic.CompareAndSwap`으로 직접 뮤텍스를 흉내 내면 잠금 대기 goroutine이 잠들지 못하고 CPU를 태우며, runtime의 파킹·공정성·스케줄러 통합까지 잃게 된다.

## 왜 이런 설계인가

Go가 메모리 순서 선택지를 노출하지 않는 이유는 언어의 가장 큰 설계 원칙 중 하나인 **"잘못 쓰기 어렵게 만든다"** 를 지키기 위해서다. C++11의 `std::memory_order_relaxed` 같은 도구는 하드웨어별로 다른 약한 메모리 모델에서 최대 성능을 뽑을 수 있게 해주지만, 그 대가로 개발자가 반드시 acquire/release 짝을 맞춰야 한다. 예를 들어 어떤 goroutine이 `data = 1` 다음에 `flag.Store(true, relaxed)`를 하고, 다른 goroutine이 `flag.Load(relaxed)`가 true를 반환한 뒤 `data`를 읽는 코드를 짰다면, ARM이나 POWER 같은 CPU에서는 `data`가 아직 1이 아닐 수 있다. Go가 이런 선택지를 아예 막았기 때문에 코드 리뷰에서 수상한 memory order 인자를 찾을 일이 없다.

두 번째 이유는 **Go의 태생적 철학**에 있다. Go는 C 계열 언어의 성능과 Python 수준의 단순함 사이에서 타협을 의도적으로 선택했다. 채널, mutex, atomic 모두 "제대로 동작하는 기본값"을 제공하고, 최적화가 필요한 극소수 개발자에게 더 많은 통제권을 주기보다 많은 개발자가 안전하게 동시성 코드를 짓도록 유도한다. 이 선택은 성능 최전선에서는 손해일 수 있지만, 대부분의 서버 프로그램에서는 오히려 유지보수 비용을 크게 줄인다.

세 번째 이유는 **타입 안전 래퍼의 등장 배경**이다. Go 1.19 이전에는 `atomic.AddInt64(&x, 1)`처럼 포인터를 직접 넘기는 함수형 API뿐이었다. 이 API는 `int64` 변수인지, 정말로 8바이트 정렬이 보장되는지, 실수로 `int32` 포인터를 넘기는지 컴파일러가 알 수 없었다. 특히 32비트 아키텍처에서는 `int64` 정렬 문제가 플랫폼별로 달라서 미묘한 panic이나 느린 연산이 생길 수 있었다. 1.19의 `atomic.Int64` 같은 타입 래퍼는 내부에 `v int64` 필드를 두고 메서드를 붙여서 잘못된 타입을 전달할 가능성을 원천적으로 차단한다. 이는 Go 1.18 제네릭 도입과 별개로 표준 라이브러리가 "사용자 실수를 줄이는" 방향으로 진화한 사례다.

네 번째 이유는 **atomic으로 mutex를 만들면 안 되는 이유**와 연결된다. `CompareAndSwap` 루프는 lock-free 알고리즘처럼 보이지만, 잠금을 기다리는 goroutine이 실제로는 잠들지 못하고 계속 CAS를 반복한다. CPU를 점유한 채 `runtime.Gosched`를 호출해도 이는 완전한 차단(blocking)이 아니라 busy-wait에 가깝다. Go의 `sync.Mutex`는 경합이 없을 때 atomic fast path를 타지만, 경합이 생기면 runtime semaphore(`runtime.sema.go`)에 goroutine을 잠재우고, lock을 반납한 쪽이 대기자 하나를 깨우는 구조다. 이렇게 해야 스케줄러가 대기 중인 goroutine을 실행 큐에서 제거해 CPU 낭비를 막고, 공정성과 handoff 정책도 적용할 수 있다.

## 어떻게 동작하는가

Go의 `sync/atomic` 패키지는 `src/sync/atomic/type.go`에 `Int64`, `Uint64`, `Bool`, `Pointer` 등의 타입 래퍼를 정의한다. 예를 들어 `atomic.Int64`는 `v int64`라는 필드를 가진 구조체이며, `Add`, `Load`, `Store`, `CompareAndSwap`, `Swap` 메서드를 제공한다. 이 메서드들은 내부적으로 컴파일러가 인식하는 intrinsic이나 `runtime/internal/atomic` 패키지의 하드웨어별 구현으로 이어진다. amd64에서는 `LOCK XADD`, `LOCK CMPXCHG` 같은 명령어로 번역되어 캐시 라인 단위로 원자성이 보장된다.

메모리 순서 관점에서 Go의 load/store는 모두 **순차 일관성**이다. 순차 일관성은 모든 atomic 연산이 하나의 전역 타임라인에서 어떤 정렬로든 일어난 것처럼 보이는 성질이다. 여기서 핵심은 한 goroutine의 `Store`가 다른 goroutine의 `Load`에 의해 관찰되면 그 `Store`와 `Load` 사이에 happens-before 관계가 생긴다는 점이다. 따라서 `publishState.publish`에서 `data = v` 다음에 `ready.Store(true)`를 하고, `read`에서 `ready.Load()`가 true를 기다린 뒤 `data`를 읽으면 `data` 쓰기는 반드시 보인다. 이 보장은 relaxed memory order를 제공하는 언어들과 달리 개발자가 추가 제어 없이 얻을 수 있다.

`sync.Mutex`도 내부적으로 atomic 연산을 쓰지만 사용 방식이 완전히 다르다. Go의 `Mutex`는 대략 세 가지 상태를 가진다. 잠기지 않음, 잠김, 잠김 + 대기자 있음. `Lock()`은 먼저 `atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked)`로 빠르게 잠근다. 이 fast path가 실패하면 goroutine은 스스로를 대기열에 넣고 `runtime_SemacquireMutex`를 호출해 잠든다. `Unlock()`은 state를 원자적으로 바꾸고, 대기자가 있으면 `runtime_Semrelease`로 하나를 깨운다. 이 과정에서 Go scheduler는 해당 goroutine을 실행 가능 상태로 바꾸고 CPU를 다른 goroutine에게 넘긴다.

위 코드의 `spinMutex`는 이와 다르다. `lock()`은 `locked.CompareAndSwap(false, true)`가 성공할 때까지 무한히 재시도한다. 잠금을 기다리는 동안에도 CPU를 계속 사용하므로 GOMAXPROCS가 코어 수와 같거나 그보다 크다면, 스핀하는 goroutine들이 실제로 일해야 할 goroutine을 밀어내는 스케줄러 스레싱을 유발할 수 있다. 또한 누가 먼저 잠글지에 대한 공정성도 보장되지 않고, 우선순위가 높은 goroutine이 낮은 goroutine의 잠금 해제를 기다리면서 계속 실행되는 우선순위 역전 문제도 생길 수 있다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사. atomic API 오용이나 복사로 인한 문제를 확인한다
go build -o /dev/null ./...  # 컴파일 확인. 타입 래퍼가 실제 표준 API와 맞는지 검증한다
go run .                     # 시연 실행. 4개 시나리오의 출력을 관찰한다
go test -v ./...             # 테스트. 발행 패턴과 카운터 합계를 검증한다
go test -race ./...          # 동시성 안전 검사. 테스트 경로에는 의도적 race가 없다
go test -bench . -benchmem ./... # atomic vs mutex 단일 goroutine 처리량 비교
go run -race .               # 시연 코드 전체를 race detector로 실행하면 unsafePublish 구간에서 경쟁이 보고된다
GODEBUG=schedtrace=1000 go run .  # spinlock 구간에서 스케줄러 동작이 밀리는지 trace를 보려면 추가
```

`go run .`에서는 네 가지 출력을 봐야 한다. 1번 `safe publish`는 `success=10000/10000`이 나와야 한다. 2번 `unsafe publish`는 실행할 때마다 `missing`이 0이 아닐 수 있고, `-race`로 돌리면 data race 경고가 찍힌다. 3번은 atomic과 mutex의 합계가 `40000`으로 같아야 한다. 4번은 spinlock의 CAS retries가 수십만에서 수백만 단위로 크게 나오는 것이 정상이다. `go test -race`는 테스트에 의도적 race가 없으므로 반드시 통과해야 하며, `go run -race .`은 데모용 race를 보여주기 위한 별도 명령이다.

## 코드로 확인하기

`main.go`의 `publishState`는 Go가 atomic에 순차 일관성만 제공한다는 사실을 가장 직접적으로 보여준다. `publish`는 일반 int64 변수에 값을 쓴 뒤 `atomic.Bool`의 `Store(true)`를 호출한다. `read`는 `ready.Load()`가 true가 될 때까지 `runtime.Gosched()`로 양보하면서 기다린 뒤 `data`를 반환한다. 이 코드에는 data 자체를 atomic으로 읽고 쓰지 않지만, `ready`의 Store/Load가 happens-before를 만들기 때문에 data race가 없다. `runPublishDemo`는 이 패턴을 1만 번 반복하며 매번 reader가 정확한 값을 봤는지 atomic 카운터로 집계한다. 출력의 `success=10000/10000`은 메모리 순서 문제가 없다는 뜻이다.

`unsafePublish`는 이와 대비되는 잘못된 패턴이다. `ready`가 plain bool이므로 두 goroutine이 동시에 bool을 읽고 쓰는 data race가 발생한다. Go 메모리 모델상 data race가 있는 프로그램은 어떤 결과가 나와도 보장되지 않는다. 실제 출력에서도 `missing`이 0이 아닐 수 있고, `go run -race .`을 실행하면 `unsafePublish.publish`와 `unsafePublish.read` 사이에서 경쟁이 보고된다. 이 경고는 "원자성 없는 플래그로 발행하지 말라"는 경고다.

`runAtomicCounterDemo`와 `runMutexCounterDemo`는 둘 다 8개 goroutine이 5000번씩 증가시켜 합계 40000을 만든다. atomic은 잠금 없이 원자적 증가를 수행하고, mutex는 잠금을 통해 임계 구역을 보호한다. 출력에서 두 값이 같다는 것은 정합성 측면에서는 둘 다 올바르다는 뜻이다. 그러나 `runSpinMutexDemo`는 CAS spinlock으로 atomic counter를 보호하면서 retries를 센다. 출력의 `CAS retries` 숫자가 크다는 것은 잠금을 기다리는 goroutine이 잠들지 않고 계속 CAS를 시도했다는 뜻이며, 이는 sync.Mutex가 runtime semaphore로 대기자를 재워주는 것과 정반대의 작동 방식이다.

`main_test.go`는 세 가지를 검증한다. `TestPublishState`는 발행 패턴 1000회를 반복하며 reader가 정확한 값을 읽는지 확인한다. 이 테스트는 시간 제한을 두지 않고 `WaitGroup`과 atomic 플래그로만 완료를 기다리므로 CI에서도 결정적이다. `TestAtomicCounterSum`과 `TestMutexCounterSum`은 동시 증가 후 합계가 정확히 남는지 확인한다. `go test -race`가 통과하는 이유는 테스트가 호출하는 함수들에 의도적 data race가 없기 때문이다. 벤치마크는 `atomic.Int64.Add`가 단일 goroutine에서 `sync.Mutex.Lock/Unlock`보다 빠르다는 것을 보여준다. mutex는 fast path에서도 상태 검사와 반환이 있으므로 atomic 증가보다 오버헤드가 크다.

## 모르면 겪는 일

`ready bool`을 가진 구조체를 여러 goroutine에서 읽고 쓰면서 "bool 정도는 원자적으로 보이겠지"라고 생각하면 `-race`에서 data race가 터진다. 더 심한 경우 race detector 없이 운영 배포를 하면, ARM 서버에서 플래그만 true로 보이고 데이터는 0으로 읽히는 버그가 간헐적으로 발생한다. 이 버그는 부하가 커질 때만 재현되고, CPU 프로파일에는 보이지 않는다.

atomic으로 mutex를 흉내 내는 팀은 잠금 경합이 심한 구간에서 p99가 갑자기 치솟는 경험을 한다. CPU 프로파일을 보면 `runtime.Gosched`와 `CompareAndSwap`만 잔뜩 보이고 정작 실제 업무 로직은 거의 실행되지 않는다. GOMAXPROCS가 코어 수와 같으면 스핀하는 goroutine이 실제로 일할 goroutine을 밀어내서 전체 처리량이 급감한다. sync.Mutex를 썼다면 대기 goroutine이 잠들어 CPU를 안 쓰므로 같은 부하에서도 지연이 훨씬 작게 유지된다.

atomic 카운터 대신 일반 `int64`에 `++`를 쓰는 것은 가장 흔한 사고다. 100개 goroutine이 1000번씩 더하면 이론상 100000이 나와야 하지만 실제로는 90000대가 나온다. `go test -race`에서 읽기-수정-쓰기가 쪼개진 것을 확인할 수 있고, 운영에서는 로스트 업데이트로 집계가 매번 어긋난다. 이 문제는 락을 걸지 않았다는 것 자체가 원인이며, atomic.Int64.Add로 바꾸면 사라진다.

## 언제 신경 쓰고 언제 무시하나

atomic의 성능과 메모리 순서 보장은 **동시에 접근하는 공유 변수**에서만 의미가 있다. goroutine 하나만 접근하는 지역 변수나 요청별로 독립적인 구조체에는 atomic도 mutex도 필요 없다. 단일 goroutine에서 연산 속도만 보면 atomic.Int64도 일반 int64보다 느릴 수 있으므로, 공유되지 않는 값을 atomic으로 바꾸는 것은 오히려 과최적화이자 가독성 저하다.

atomic이 중요한 규모는 `sync.Mutex`로 보호하기에는 경합이 너무 잦아 잠금 대기 비용이 커지는 경우다. 예를 들어 초당 수십만 건의 요청을 받는 rate limiter나 계수기, lock-free 큐의 스택 포인터 같은 짧은 임계 구역이 그렇다. 반면 초당 수천 건 정도의 트래픽에서는 `sync.Mutex`가 훨씬 분명하고 디버깅하기 쉬우며, 성능 차이도 서비스 전체에서 무시할 만하다. 꼭 필요한 경우가 아니라면 "lock-free로 더 빠르게"라는 유혹을 먼저 의심하자.

메모리 순서를 고민해야 하는 상황은 한정적이다. 발행 플래그, 상태 전이, 지연 초기화처럼 "이 데이터를 다 쓴 뒤에 신호를 준다"는 패턴은 `atomic.Bool` 하나로 충분하다. 여러 atomic 변수 사이에 복잡한 순서가 얽힌다면 잘못 설계했을 가능성이 높다. 그때는 atomic을 여러 개 조합하기보다 `sync.Mutex`로 묶거나 채널로 상태 전이를 표현하는 편이 낫다. Go가 relaxed/acquire-release를 막은 덕분에, 신호 하나에 데이터 하나를 매달아 쓰는 수준에서는 거의 모든 코드가 안전하다.

## 더 파보기

- Go 메모리 모델 공식 문서: https://go.dev/ref/mem
- sync/atomic 패키지 문서: https://pkg.go.dev/sync/atomic
- 타입 래퍼 소스 파일: https://github.com/golang/go/blob/master/src/sync/atomic/type.go
- sync.Mutex 소스 파일: https://github.com/golang/go/blob/master/src/sync/mutex.go
- runtime semaphore 소스 파일: https://github.com/golang/go/blob/master/src/runtime/sema.go
- Go race detector 블로그: https://go.dev/blog/race-detector