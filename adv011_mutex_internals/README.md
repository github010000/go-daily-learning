## 한 줄 요약

Go 의 `sync.Mutex` 는 무조건 OS 스레드를 블록시키지 않고, 짧은 경합에서는 spin 으로 재시도하고, 어떤 goroutine 이 1ms 보다 오래 기다리면 starvation mode 로 전환하여 도착 순서대로 락을 건네준다. 이 설계는 낮은 지연과 공정성 사이에서 적응적으로 균형을 잡기 위한 것이다.

## 왜 이런 설계인가

OS 스레드를 블록하고 깨우는 것은 커널로 제어권이 넘어가는 시스템 콜이 필요하다. 한 번 블록하고 다시 실행되는 비용은 마이크로초 수준으로 보이지만, 짧은 임계 구역을 보호하는 락이라면 블록 비용이 락을 잡고 있는 시간보다 더 클 수 있다. 락이 아주 짧게만 잡혀 있다면, 블록하지 않고 CPU 에서 잠깐 돌면서 락이 풀리기를 기다리는 편이 전체 처리량에 유리하다. 그래서 Go mutex 는 잠들기 전에 spin 을 시도한다.

하지만 spin 은 CPU 시간을 태우는 것이다. 코어가 하나뿐이거나 spin 하는 goroutine 보다 실행 가능한 goroutine 이 충분하지 않으면, spin 은 락을 소유한 goroutine 의 실행을 오히려 방해할 수 있다. 그래서 `runtime_canSpin` 은 `GOMAXPROCS > 1`, 현재 P 의 run queue 가 비어 있는지, 최대 spin 횟수인지 등을 검사한다. Go 소스에는 spin 을 몇 번까지 할지 제한하는 상수들이 있다. `active_spin = 4`, `active_spin_cnt = 30` 같은 값이 그 예다.

spin 만으로는 불공정 문제가 생긴다. 락이 해제될 때, 대기 중인 goroutine 을 깨우는 대신 이미 CPU 에서 돌고 있던 새 goroutine 이 락을 먼저 가져갈 수 있다. 이를 barging 이라고 한다. barging 을 허용하면 깨어나는 goroutine 을 스케줄링할 필요가 없어 처리량이 높아질 수 있다. 하지만 늦게 도착한 요청이 먼저 도착한 요청을 계속 제치면, 오래 기다린 goroutine 은 계속 밀려 기아 상태에 빠진다.

이 문제를 해결하기 위해 Go mutex 는 1ms 라는 starvation threshold 를 두었다. 대기 중인 goroutine 이 1ms 보다 오래 기다렸다고 판단하면 mutex 내부의 `mutexStarving` 비트를 켠다. starvation mode 에서는 barging 을 막고 언락 시 락을 첫 번째 waiter 에게 직접 넘긴다. 이렇게 하면 오래 기다린 goroutine 이 확실히 락을 얻을 수 있다. 즉, 평상시에는 barging 으로 처리량을 올리고, 기아가 감지되면 FIFO 로 공정성을 확보하는 적응형 락이다.

## 어떻게 동작하는가

`sync.Mutex` 는 `go/src/sync/mutex.go` 에 정의되어 있다. 구조체는 두 필드만 가진다.

```
type Mutex struct {
    state int32
    sema  uint32
}
```

`state` 는 락의 모든 상태를 인코딩한다. 하위 비트부터 `mutexLocked`, `mutexWoken`, `mutexStarving` 이 있고, 그 위부터는 대기 중인 goroutine 수가 저장된다. `sema` 는 락을 기다리는 goroutine 을 잠들게 하고 깨우는 semaphore 이다. Go 런타임의 semaphore 는 OS semaphore 가 아니라 Go 런타임이 관리하는 goroutine 대기열이다.

`Lock()` 은 먼저 fast path 를 시도한다. `state` 가 0 이면 `atomic.CompareAndSwapInt32` 로 0 을 `mutexLocked` 로 바꾸고 바로 반환한다. fast path 가 실패하면 `lockSlow()` 로 들어간다. `lockSlow()` 는 상태를 다시 읽고, 락이 이미 잠겨 있거나 starving 이며, `runtime_canSpin(iter)` 가 true 면 spin 루프를 돈다. spin 루프에서는 `runtime_doSpin()` 을 호출한다. 실제로는 CPU 의 `PAUSE` 명령을 반복 실행하는 것이며, 여기서 `active_spin_cnt` 만큼 돌면 spin 을 포기한다.

spin 을 포기하면 `state` 의 waiter count 를 1 증가시키고 `runtime_SemacquireMutex(&m.sema, queueLifo, 1)` 을 호출해 semaphore 에서 잠든다. 이때 대기 시작 시간(`waitStartTime`)을 기록해 둔다. 이후 깨어나면 대기 시간이 1ms 를 넘었는지 검사한다. 1ms 를 넘었고 여전히 락이 다른 goroutine 에게 빼앗긴 상태라면, 다음 CAS 에서 `mutexStarving` 비트를 켠다. 이렇게 starvation mode 에 진입한다.

`Unlock()` 은 `state` 에서 `mutexLocked` 비트를 원자적으로 지운다. 이후 waiter 가 있으면 `unlockSlow()` 가 호출된다. starvation mode 가 아니라면 `runtime_Semrelease` 로 waiter 한 명을 깨운다. 이때 깨어난 waiter 와 새로 도착한 goroutine 이 락을 두고 경쟁한다. 하지만 starvation mode 라면 `runtime_Semrelease(&m.sema, true, 1)` 처럼 handoff 모드로 첫 번째 waiter 에게 직접 락을 넘긴다. 새로 도착한 goroutine 은 `mutexStarving` 비트를 보고 barging 을 시도하지 않는다.

`runtime_canSpin` 의 실제 구현은 `go/src/runtime/proc.go` 에 있다. 이 함수는 `GOMAXPROCS > 1`, 현재 P 가 비어 있는지, 그리고 너무 많은 goroutine 이 spin 하고 있지 않은지 등을 검사한다. spin 은 정말 짧은 구간에서만 의미가 있기 때문에 runtime 은 spin 조건을 엄격하게 제한한다. `runtime_doSpin` 은 `go/src/runtime/proc.go` 또는 어셈블리 파일에서 구현되며, CPU 수준의 backoff 없이 일정 횟수만 돈다.

starvation mode 는 영구적이지 않다. starvation mode 에서 마지막 waiter 가 락을 얻거나 waiter 수가 0 이 되면, `mutexStarving` 비트가 꺼지고 다시 normal mode 로 돌아간다. 이렇게 해야 다음에 짧은 경합 구간에서 spin 과 barging 의 이점을 다시 살릴 수 있다.

## 돌려보기

이 디렉토리에서 아래 명령을 그대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사 — unsafe 사용이 있지만 문법 문제가 없어야 한다
go build -o /dev/null ./...  # 컴파일 확인 — 교육용 unsafe 레이아웃이 현재 Go 와 맞는지 확인
go run .                     # 시연 실행 — 내부 state 비트와 starvation 전환 시도
GO111MODULE=off go run .     # 필요시 모듈 없이 실행
go test -v ./...             # 테스트 — 락 상태 플래그와 counter 동시성 검증
go test -race ./...          # 동시성 주제이므로 반드시 race 검사
go test -bench=BenchmarkMutexUncontended -benchmem ./...  # contention 없을 때 Lock/Unlock 비용
```

`go run .` 을 실행하면 세 부분이 출력된다. 첫 부분은 `sync.Mutex` 의 초기 상태, Lock 이후, Unlock 이후를 보여준다. 두 번째 부분은 goroutine 여러 개가 락을 기다리며 spin 에서 sleep 으로 넘어가는 동안 `waiterCount` 가 0 에서 4 로 늘어나는 것을 보여준다. 세 번째 부분은 barging 을 통해 starvation mode 를 유도하는 시도를 반복한다. 시도 중 `starving=true` 가 출력되면 starvation mode 진입을 관찰한 것이다.

`go test -race ./...` 는 `unsafe` 로 state 를 읽을 때도 `atomic.LoadInt32` 를 사용했기 때문에 race 로 판정되지 않아야 한다. 테스트가 실패하면 실제로 데이터 레이스가 있거나, 사용 중인 Go 버전에서 `sync.Mutex` 의 메모리 레이아웃이 이 예제와 달라졌을 수 있다.

## 코드로 확인하기

`main.go` 의 `mutexInternal` 은 현재 Go 런타임의 `sync.Mutex` 앞부분을 흉내낸 struct 이다. `unsafe.Pointer` 로 `sync.Mutex` 포인터를 이 struct 로 변환한 뒤 `atomic.LoadInt32` 로 `state` 필드만 읽는다. 이렇게 하면 락을 잠그지 않고도 runtime 이 보는 것과 같은 상태 비트를 관찰할 수 있다. 이 코드는 교육용이며, Go 버전이 바뀌면 깨질 수 있다.

`demoSpinAndStarvation` 은 락을 잡은 채로 4 개의 goroutine 을 `Lock()` 에 진입시킨다. `runtime.GOMAXPROCS(4)` 로 설정해 P 를 여러 개 만든다. 처음에는 goroutine 들이 spin 을 하며 `waiterCount` 가 아직 0 일 수 있다. 50us 간격으로 상태를 샘플링하면, spin 이 실패하고 semaphore 에 잠들면서 `waiterCount` 가 점점 올라가는 모습이 출력된다. 마지막에 `waiterCount=4` 가 되는 것을 보면, 짧은 spin 뒤에 모든 goroutine 이 semaphore 대기열로 넘어갔다는 뜻이다.

`tryInduceStarvation` 은 실제 starvation mode 전환을 목표로 한다. main 이 락을 2ms 넘게 붙잡고 있으면 waiter 는 1ms threshold 를 넘긴다. 그 다음 main 이 `Unlock` 직후 `TryLock` 으로 락을 훔치면, 깨어난 waiter 는 락을 얻지 못하고 다시 내부 상태를 본다. 이 waiter 는 `waitStartTime` 부터 계산한 대기 시간이 1ms 를 넘었으므로 `mutexStarving` 비트를 세운다. 출력에서 해당 시도의 `starving=true` 가 나타나면 barging 이 실제로 기아를 유발했고, 런타임이 starvation mode 로 전환했음을 의미한다.

`main_test.go` 의 `TestReadMutexStateUnlocked` 와 `TestReadMutexStateLocked` 는 우리가 만든 내부 상태 읽기 함수가 기본 상태를 정확히 반환하는지 검증한다. `TestSafeCounterRace` 는 `sync.Mutex` 로 보호된 카운터를 여러 goroutine 이 동시에 증가시켰을 때 최종 값이 정확한지 검사한다. 이 테스트는 `-race` 플래그를 붙여도 통과해야 한다. `BenchmarkMutexUncontended` 는 경합이 없을 때 락 한 쌍의 비용을 측정한다.

## 모르면 겪는 일

이 내부 동작을 모르면 단순히 `sync.Mutex` 를 쓰면서도 tail latency 가 튀는 원인을 오해하기 쉽다. 예를 들어 특정 코드 경로에서 락을 2ms 이상 잡는 일이 흔하지 않게 발생한다고 하자. 요청이 몰리면 어떤 goroutine 은 1ms 이상 대기하게 되고, starvation mode 로 전환되면서 갑자기 barging 이 막힌다. 그 결과 새로 도착한 요청이 더 이상 먼저 락을 가져갈 수 없게 되어 전체 처리량이 떨어질 수 있다. CPU 프로파일에는 락 대기 시간이 보이지 않거나, 시스템 콜이 아닌 Go 런타임 내부 대기로 잡히기 때문에 병목을 GC 나 네트워크로 오인할 수 있다.

또 하나 전형적인 증상은 spin 으로 인한 CPU 소모다. P 가 여러 개이고 짧은 경합이 많다면, goroutine 들이 semaphore 에 잠들기 전에 spin 을 하면서 CPU 를 태운다. 이것은 의도된 동작이지만, `GOMAXPROCS` 가 코어 수보다 훨씬 크거나 짧은 락이 매우 빈번하면 CPU 사용률이 높아지면서 실제 락을 잡은 goroutine 의 실행이 지연될 수 있다. 이 경우 `sync.Mutex` 자체가 느린 것이 아니라, 과도한 spin 과 barging 이 원인이다.

starvation mode 의 1ms threshold 를 모르고 있으면, 일정 시간이 지난 뒤 갑자기 락 획득 순서가 FIFO 로 바뀌는 현상을 버그로 착각할 수 있다. 테스트에서 락 획득 순서에 대한 단정을 잘못 세우면 가끔씩만 깨지는 flaky test 가 생긴다. 특히 `-race` 나 `GOMAXPROCS` 설정에 따라 barging 성공 확률이 달라지므로, 순서를 보장해야 한다면 `sync.Mutex` 에 기대지 말고 채널이나 큐 같은 명시적 FIFO 구조를 써야 한다.

마지막으로 `TryLock` 을 barging 과 starvation 문제를 우회하는 도구로 잘못 쓰면 상황이 더 나빠진다. `TryLock` 은 즉시 실패를 반환하므로 대기 시간이 없어 보이지만, 이미 오래 기다린 waiter 가 있는 상태에서 새 goroutine 이 계속 `TryLock` 을 시도하면 starvation mode 로 빨리 진입할 수 있다. 이것은 오히려 처리량을 떨어뜨리는 패턴이다. `TryLock` 은 락을 기다리지 않아도 되는 아주 제한된 상황에서만 써야 한다.

## 언제 신경 쓰고 언제 무시하나

락 경합이 거의 없는 일반적인 애플리케이션에서는 이 모든 내용을 무시해도 된다. `sync.Mutex` 의 fast path 는 단순한 CAS 한 번이므로 아주 빠르다. 경합이 없으면 spin 도 starvation mode 도 등장하지 않는다. 이 시점에서 락 내부를 최적화하려는 시도는 오히려 코드를 복잡하게 만들 뿐이다.

락 경합이 있더라도 임계 구역이 수십 나노초 또는 수 마이크로초 수준이면 spin 과 barging 이 큰 문제를 일으키지 않는다. 이 구간에서는 짧게 spin 하고 락을 잡는 편이 합리적이다. 다만 `pprof` 의 mutex profile 이나 runtime metrics 를 통해 contention 수준을 정량적으로 확인하기 전에는 최적화하지 않는 것이 좋다.

이 지식이 실제로 중요해지는 때는 동시 goroutine 수가 수백 이상이고, 특정 락의 평균 대기 시간이 1ms 근처에 접근하거나 p99 지연 시간이 요동칠 때다. 그런 경우에는 락을 오래 잡는 구간을 찾아 임계 구역을 줄이거나, 락을 분리(sharding)하거나, 락 대신 atomic 연산이나 channel 기반 구조를 고려해야 한다. starvation mode 전환은 시스템이 이미 상당한 기아를 겪고 있다는 신호다.

단일 P 환경(`GOMAXPROCS=1`)에서는 spin 이 의미가 없으므로 `runtime_canSpin` 이 항상 false 를 반환한다. 따라서 spin 관련 문제는 최소한 멀티코어 환경에서만 고려 대상이 된다. 컨테이너에서 CPU 할당량이 제한된 경우 `GOMAXPROCS` 가 크게 설정되어 있으면 실제로는 P 가 많아 보여도 CPU 시간을 나눠 쓰므로 spin 이 오히려 손해일 수 있다. 이럴 때는 `GOMAXPROCS` 를 실제 사용 가능한 CPU 수에 맞추는 것이 mutex 동작에도 영향을 준다.

## 더 파보기

- [`go/src/sync/mutex.go`](https://go.dev/src/sync/mutex.go) — `Lock`, `Unlock`, `lockSlow`, `unlockSlow`, starvation mode 상수가 실제로 정의된 소스
- [`go/src/runtime/proc.go`](https://go.dev/src/runtime/proc.go) — `runtime_canSpin`, `runtime_doSpin`, `active_spin` 상수 등 spin 조건 구현
- [`go/src/runtime/sema.go`](https://go.dev/src/runtime/sema.go) — goroutine semaphore 구현, `runtime_SemacquireMutex` 와 `runtime_Semrelease` 의 동작 방식
- [`go.dev/blog/race-detector`](https://go.dev/blog/race-detector) — `-race` 플래그와 atomic 접근의 관계
- [`go.dev/doc/faq`](https://go.dev/doc/faq) — mutex 와 goroutine 스케줄링에 대한 런타임 FAQ
- [`github.com/golang/go/issues/13086`](https://github.com/golang/go/issues/13086) — mutex starvation mode 도입과 관련된 논의