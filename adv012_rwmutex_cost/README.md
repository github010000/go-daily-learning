## 한 줄 요약

`sync.RWMutex`의 읽기 잠금은 공짜가 아니다. `RWMutex.RLock`은 `readerCount`라는 단일 원자 카운터를 증가시키고, 모든 reader goroutine이 같은 캐시 라인을 무효화시키면서 경합한다. 실제 읽기 계산이 수십 나노초 이하로 짧다면 `sync.Mutex`의 단순한 fast path가 `RWMutex`의 병렬 읽기보다 빠를 수 있다.

## 왜 이런 설계인가

읽기 전용 작업이 많을 때 `sync.Mutex`는 모든 reader를 직렬화하므로 멀티코어 CPU를 충분히 활용하지 못한다. readers-writers 문제는 여러 reader는 동시에 읽어도 안전하지만 writer는 배타적이어야 한다는 가정에서 출발한다. `sync.RWMutex`는 이 문제를 풀기 위해 reader 잠금 `RLock`과 writer 잠금 `Lock`을 분리했다. 하지만 이 분리에는 조정 비용이 들어간다.

Go 런타임은 OS가 제공하는 rwlock을 직접 사용하지 않는다. OS rwlock은 블로킹되면 OS 스레드가 멈추기 때문에, 그 스레드에 붙은 다른 goroutine까지 멈출 수 있다. Go는 goroutine-aware한 런타임 세마포어를 쓸 수 있도록 `sync.RWMutex`를 사용자 공간에서 구현했다. 그래서 `src/sync/rwmutex.go`에는 `w sync.Mutex`, `writerSem uint32`, `readerSem uint32`, `readerCount atomic.Int32`, `readerWait atomic.Int32` 같은 필드가 있다.

`readerCount`는 현재 활성 reader 수를 나타내면서, 동시에 writer가 대기 중이거나 실행 중임을 표현하기 위해 음수 구간을 사용한다. 모든 reader가 `RLock`과 `RUnlock`에서 이 하나의 카운터를 원자적으로 증가·감소시키기 때문에, 여러 CPU 코어가 같은 메모리 워드를 공유하게 된다. 이는 캐시 라인 무효화 트래픽을 유발하고, 계산 시간이 아주 짧은 경우에는 실제 작업보다 잠금 조정 비용이 더 커진다.

`sync.Mutex`는 `src/sync/mutex.go`에 `state`와 `sema` 정도만 있다. fast path는 하나의 atomic compare-and-swap이고, readerCount 같은 별도 필드가 없다. 물론 Mutex도 잠금 경합 시 캐시 라인을 튕기지만, RWMutex처럼 reader를 추적하는 추가 상태와 분기, 런타임 세마포어 경로가 없기 때문에 짧은 임계 구간에서는 오히려 Mutex가 유리하다. RWMutex의 병렬 읽기 이득은 계산이 충분히 길어 잠금 비용을 상쇄할 때만 나타난다.

## 어떻게 동작하는가

`src/sync/rwmutex.go`의 `RLock`은 대략 다음과 같은 형태다. 최신 Go에서는 `readerCount`가 `atomic.Int32` 타입이지만, 이전 구현에서는 `int32` 필드에 `atomic.AddInt32`를 직접 호출했다. 어느 쪽이든 핵심은 `readerCount` 증가다.

```go
if atomic.AddInt32(&rw.readerCount, 1) < 0 {
    runtime_SemacquireRWMutexR(&rw.readerSem, false, 0)
}
```

`readerCount`가 음수가 되면 writer가 활성 상태이거나 대기 중이라는 뜻이다. 이때 reader는 `readerSem`에서 대기한다. `RUnlock`은 반대로 `readerCount`를 하나 줄이고, 결과가 음수면 writer를 깨우는 slow path로 들어간다. 즉, 읽기 잠금을 한 번 걸고 풀 때마다 모든 코어가 같은 `readerCount` 캐시 라인에 접근한다.

writer 쪽 `Lock`은 먼저 내부 `w sync.Mutex`로 writer 간 상호 배제를 확보한다. 그다음 `rwmutexMaxReaders`만큼 `readerCount`를 빼서 이후 들어오는 reader를 차단한다. 이미 실행 중인 reader가 있으면 `readerWait`에 그 수를 기록하고 `writerSem`에서 잠든다. `Unlock`은 `readerCount`를 복원하고 대기 중인 reader들을 `readerSem`으로 깨운다. 이처럼 하나의 `readerCount`가 reader와 writer의 모든 조정을 담당한다.

`sync.Mutex`는 `state` 하나를 CAS로 바꾸는 방식이다. writer와 reader를 구분하지 않으므로 `readerCount` 같은 필드를 관리하지 않는다. 물론 여러 goroutine이 Mutex를 잡으면 `state` 캐시 라인이 오가지만, RWMutex의 RLock 경로는 원자 연산 뒤에 writer 대기 여부를 확인하는 분기까지 추가로 수행한다. 게다가 RWMutex의 `readerCount`는 `writerSem`, `readerSem` 등과 가까운 필드에 있어서 `RLock`이 그 주변 캐시 라인까지 건드릴 가능성이 크다.

결국 짧은 임계 구간에서는 Mutex가 reader를 직렬화하지만, 하나의 CAS로 임계 구간을 빠르게 통과시키는 편이 RWMutex가 모든 reader를 조정하면서 병렬성을 준비하는 것보다 낫다. 임계 구간이 충분히 길어지면 읽기 작업이 여러 코어에서 겹쳐 실행되므로 RWMutex가 역전한다.

## 돌려보기

이 디렉토리에서 다음 명령을 실행하면 시연, 정적 검사, 안전성 검사, 벤치마크를 모두 확인할 수 있다.

```bash
go vet ./...                 # 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 짧은 구간과 긴 구간에서 Mutex vs RWMutex 처리량 비교
GOMAXPROCS=4 go run .        # 코어 수를 줄여서 경계가 어떻게 변하는지 확인
go test -v ./...             # 동시 읽기 정합성과 -race 대비 검증
go test -race ./...          # race 검출기로 안전성 확인
go test -bench 'BenchmarkParallel' -benchmem ./... # 실제 병렬 읽기 벤치마크
```

`go run .`을 실행하면 GOMAXPROCS 값과 함께 `dataSize=4`, `128`, `4096`에 대한 Mutex와 RWMutex의 ops/sec가 출력된다. `dataSize=4`에서는 Mutex가 RWMutex보다 같거나 빠르고, `dataSize=4096`에서는 RWMutex가 여러 배 빠른 경향을 확인할 수 있다.

`GOMAXPROCS=4 go run .`으로 코어를 줄이면 reader 병렬성의 이득이 줄어들고 캐시 라인 경합도 줄어든다. 따라서 짧은 구간의 차이는 작아지고, 긴 구간에서도 RWMutex의 이점이 `GOMAXPROCS`가 클 때보다 줄어든다. 이는 이 최적화가 코어 수에 민감하다는 뜻이다.

`go test -bench 'BenchmarkParallel' -benchmem ./...`에서는 짧은 구간과 긴 구간의 ns/op를 나란히 볼 수 있다. `BenchmarkParallelMutexShortRead`와 `BenchmarkParallelRWMutexShortRead`를 비교하면 짧은 구간에서 RWMutex가 더 느릴 수 있음을 실측할 수 있고, `LongRead` 벤치마크에서는 반대 경향이 나타난다.

## 코드로 확인하기

`main.go`에는 `mutexState`와 `rwMutexState`가 있다. 둘 다 같은 `sharedState.vals`를 읽어 합을 계산한다. `readSumMutex`는 `Lock`과 `Unlock`을 쓰고, `readSumRWMutex`는 `RLock`과 `RUnlock`을 쓴다. 계산 자체는 동일하기 때문에 출력의 처리량 차이는 잠금 방식의 차이에서 온다.

`runReadBenchmark`는 정해진 시간 동안 reader goroutine을 돌리면서 완료한 연산 수를 atomic하게 센다. `findBoundary`는 `dataSize`를 `4`, `128`, `4096`으로 바꿔가며 두 방식을 실행한다. `dataSize=4`는 정수 4개를 더하는 매우 짧은 임계 구간이다. 이때는 `RLock`의 `readerCount` 원자 연산과 캐시 라인 경합이 실제 덧셈보다 비싸다. `dataSize=4096`은 합 계산이 충분히 커서 reader들이 여러 코어에서 동시에 작업하는 이득이 잠금 오버헤드를 넘어선다.

`main_test.go`의 `TestReadSumCorrectness`는 두 잠금 구현이 같은 값을 반환하는지 확인한다. `TestConcurrentReadSumsNoRace`는 여러 goroutine이 동시에 읽어도 결과가 틀리지 않고 `-race`에서도 걸리지 않는지 검증한다. 벤치마크 함수들은 `RunParallel`을 사용해 실제 동시 reader 상황을 재현하고, `b.ResetTimer`를 상태 생성 이후에 두어 잠금 성능만 측정하도록 한다.

## 모르면 겪는 일

읽기 비율이 높은 코드라고 해서 `sync.Mutex`를 무조건 `sync.RWMutex`로 바꾸는 팀이 많다. 그런데 임계 구간이 `map`에서 값 하나 꺼내기, 슬라이스 길이 확인, 작은 구조체 필드 읽기 정도로 짧다면 p99 레이턴시가 오히려 늘어날 수 있다. CPU 프로파일을 보면 사용자 로직은 거의 안 보이고 `atomic.AddInt32`나 `(*RWMutex).RLock` 근처에서 시간이 소모된다. 프로파일에는 잠금 경합으로 보이지 않아 원인을 찾기 어렵다.

코어 수가 늘어나도 처리량이 기대만큼 늘지 않는 증상도 나타난다. 모든 reader가 같은 `readerCount` 캐시 라인을 무효화시키기 때문에 8코어, 16코어로 확장해도 캐시 코히어런시 트래픽 때문에 속도가 정체된다. `perf c2c` 같은 도구로 보면 `readerCount`가 있는 캐시 라인에서 HITM이 높게 나온다. 이는 코드상으로는 완전히 독립된 읽기 작업인데도 CPU 간 메모리 트래픽이 병목이 되는 전형적인 모습이다.

가끔 쓰는 writer가 있을 때도 문제가 달라진다. Go의 `RWMutex`는 writer starvation을 막기 위해 writer가 대기 중이면 새 reader를 차단하는 정책을 쓴다. 따라서 짧은 읽기가 대부분인 서비스에서 희귀한 writer가 한 번 들어오면, 그 순간부터 새 reader들이 줄줄이 블로킹되며 레이턴시 스파이크가 생길 수 있다. "읽기가 많으니 RWMutex"라고 생각했던 선택이 예상과 반대로 동작하는 것이다.

## 언제 신경 쓰고 언제 무시하나

임계 구간이 정말 짧다면, 예를 들어 슬라이스의 길이를 읽거나 작은 변수를 읽는 정도라면 `sync.Mutex`를 그대로 쓰는 것이 더 낫다. 코드가 단순해지고 실수할 여지도 줄어든다. 수백 나노초 미만의 읽기 구간에서는 RWMutex의 병렬성 이득이 조정 비용을 넘기 어렵다.

읽기 구간이 크고 동시 reader가 실제로 여러 코어에서 실행될 수 있는 상황이라면 `RWMutex`가 좋은 선택이다. 예를 들어 커다란 불변 슬라이스를 순회하며 합계를 구하거나, 공유 읽기 전용 설정을 오래 탐색하는 경우다. 그래도 추측하지 말고 반드시 벤치마크를 돌려야 한다. 같은 논리도 코어 수, CPU 캐시 크기, GC 부하에 따라 경계가 달라진다.

초당 몇천 건 정도의 낮은 부하에서는 Mutex와 RWMutex의 차이가 거의 나지 않는다. 이 경우 가독성과 유지보수를 우선해 더 단순한 Mutex를 고르는 편이 좋다. 잠금 경합이 실제 프로파일에 보이기 전까지 RWMutex, sharded lock, copy-on-write 같은 고급 기법을 미리 넣는 것은 과최적화다. 먼저 `pprof`의 mutex 프로파일로 경합을 확인하고 그때 바꿔도 늦지 않다.

## 더 파보기

- Go 소스 코드 `src/sync/rwmutex.go`: https://go.dev/src/sync/rwmutex.go
- Go 소스 코드 `src/sync/mutex.go`: https://go.dev/src/sync/mutex.go
- `sync.RWMutex` 패키지 문서: https://pkg.go.dev/sync#RWMutex
- Go 메모리 모델 문서: https://go.dev/ref/mem