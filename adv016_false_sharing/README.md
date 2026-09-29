## 한 줄 요약

false sharing 은 서로 다른 변수가 같은 캐시 라인에 들어가서, 여러 CPU 코어가 그 라인을 두고 쓰기 경쟁을 할 때 생기는 성능 저하다. 논리적으로는 완전히 독립적인 연산이지만 캐시 일관성 프로토콜이 라인 단위로 움직이기 때문에 한쪽 쓰기가 다른 쪽 쓰기를 계속 무효화한다.

이 문제는 race detector 에 잡히지 않는 대표적인 성능 함정이다. 데이터 경쟁은 없는데도 성능이 수직으로 떨어지기 때문에 프로파일에서 원인을 찾기 어렵다.

가장 간단한 해결책은 캐시 라인 경계를 넘기도록 필드 사이에 패딩을 넣는 것이다. 이 저장소는 잘못된 배치와 패딩 배치를 벤치마크로 비교해서 그 차이를 관찰할 수 있게 만든다.

## 왜 이런 설계인가

CPU 는 메모리를 1바이트 단위로 읽지 않는다. x86-64 기준으로 보통 64바이트 단위인 캐시 라인을 통째로 가져온다. 이렇게 하는 이유는 공간 지역성 때문이다. 어떤 주소를 읽었다면 그 근처 주소도 곧 읽을 가능성이 높으므로, 한 번에 64바이트를 가져오면 메모리 트랜잭션 횟수를 크게 줄일 수 있다. 하지만 이 선택은 소프트웨어에게 "서로 다른 변수라도 가까이 놓으면 같은 캐시 라인에 들어갈 수 있다"는 부담을 남긴다.

여러 코어가 같은 캐시 라인을 읽는 것 자체는 문제가 되지 않는다. 문제는 한 코어가 그 라인에 쓰기를 할 때 시작된다. CPU 캐시 일관성 프로토콜, 대표적으로 MESI 프로토콜은 라인이 Modified, Exclusive, Shared, Invalid 중 어떤 상태인지 추적한다. 한 코어가 Shared 상태인 라인에 쓰려면 Read For Ownership 요청을 보내 다른 코어의 라인을 Invalid 로 만든 뒤 자기 것으로 소유해야 한다. 이 과정은 라인 전체에 적용된다. 따라서 라인 안의 서로 다른 바이트를 쓰더라도 라인 소유권이 계속 코어 사이를 오간다.

Go 를 포함한 대부분의 언어 컴파일러는 자동으로 false sharing 을 막아주지 않는다. Go 는 특히 구조체 필드 순서를 소스 코드에 적힌 그대로 유지하고, 개발자가 명시적으로 패딩을 넣지 않는 한 필드를 분리하지 않는다. 이는 cgo, unsafe, 리플렉션과 같은 기능이 필드 오프셋을 예측 가능하게 유지해야 하기 때문이다. 구조체 필드 순서를 바꾸면 메모리 레이아웃이 바뀌면서 이미 컴파일된 패키지와의 호환이 깨질 수 있다.

Go 런타임도 이 문제를 안다. 런타임의 스케줄러 구조체는 자주 쓰이는 필드와 그렇지 않은 필드가 같은 캐시 라인에서 경쟁하지 않도록 `cpu.CacheLinePad` 를 사용한다. 예를 들어 `runtime2.go` 에 정의된 `p` 구조체는 `_ cpu.CacheLinePad` 를 끼워 넣어 스케줄러가 여러 CPU 에서 쓰는 필드를 분리한다. 사용자 코드에서도 같은 문제가 생길 수 있다는 뜻이다.

## 어떻게 동작하는가

false sharing 을 이해하려면 캐시 라인의 상태 변화를 봐야 한다. 두 코어가 같은 라인을 읽으면 그 라인은 두 코어 모두 Shared 상태다. 코어 A 가 그 라인의 변수 a 를 수정하려면 코어 B 의 라인을 Invalid 로 만들고 자기 라인을 Modified 로 바꾼다. 코어 B 가 이어서 변수 b 를 수정하려면 이번에는 코어 A 의 라인을 무효화해야 한다. 라인은 A와 B 사이를 계속 오가며, 실제로는 서로 다른 변수를 쓰고 있음에도 불구하고 마치 같은 변수를 보호하는 스핀락처럼 동작한다.

이 저장소의 `FalseSharingCounters` 는 `a int64` 와 `b int64` 를 순서대로 배치한다. int64 가 8바이트이므로 a 는 offset 0, b 는 offset 8 이다. 64바이트 라인 안에 a 와 b 가 모두 들어가므로 두 goroutine 이 각각 a 와 b 만 갱신해도 라인 소유권을 두고 경쟁한다. `runFalseSharing` 은 두 goroutine 이 `atomic.AddInt64` 로 각각 n 번 증가시키면서 이 경쟁을 재현한다.

`PaddedCounters` 는 a 다음에 `_ [cacheLineSize - 8]byte` 패딩을 넣어 b 의 offset 을 64로 만든다. 이제 a 는 첫 번째 캐시 라인, b 는 두 번째 캐시 라인에 속한다. `runPadded` 는 같은 goroutine 두 개가 같은 횟수를 증가시키지만, a 와 b 가 다른 라인이므로 각 코어는 자기 라인만 Modified 상태로 유지한다. 라인 무효화가 없으므로 보통 더 빠르다.

Go 런타임에서도 이 원리를 그대로 쓴다. `runtime2.go` 의 `p` 구조체, `m` 구조체, `mcache` 구조체 등은 CPU 캐시 라인 크기만큼의 빈 패딩을 포함한다. 예를 들어 `p` 구조체 안에는 `_ cpu.CacheLinePad` 가 여러 군데 있어, 스케줄러가 자주 쓰는 run queue 헤드와 타이머 관련 필드가 한 라인에서 부딪히지 않게 한다. 개발자가 직접 만드는 구조체도 동일한 접근이 필요할 수 있다.

`unsafe.Offsetof` 를 쓰면 이 구조가 실제로 그렇게 배치됐는지 쉽게 검증할 수 있다. `FalseSharingCounters` 에서 b 는 offset 8, `PaddedCounters` 에서 b 는 offset 64 로 출력된다. 이 오프셋이 64바이트 경계를 넘는지가 false sharing 을 막았는지 판단하는 기준이다.

## 돌려보기

이 디렉토리에서 아래 명령을 순서대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행. 오프셋과 시간 차이를 확인
go test -v ./...             # 테스트. 오프셋 불변식과 카운터 정확성 검증
go test -race ./...          # 동시성 코드 race 검사. 통과해야 한다
go test -bench=. -benchmem -run '^$' ./... # BenchmarkFalseSharing vs BenchmarkPadded
```

`go run .` 에서는 구조체 크기와 b 의 오프셋이 출력된다. 이어서 `false sharing` 라벨과 `padded` 라벨의 실행 시간이 출력된다. 멀티코어 머신이라면 보통 false sharing 쪽이 더 오래 걸린다. 단일 코어이거나 GOMAXPROCS=1 이면 그 차이가 거의 없을 수 있다.

`go test -v ./...` 는 캐시 라인 분리가 실제로 이뤄졌는지, 그리고 false sharing 이 있어도 카운터 수는 정확한지 검증한다. `go test -race ./...` 는 이 시연 코드가 데이터 경쟁을 일으키지 않는다는 것을 보여준다. false sharing 은 성능 문제이지 race 문제가 아니다.

`go test -bench=. -run '^$' ./...` 는 두 배치를 직접 비교한다. 결과에서 `BenchmarkPadded` 의 `ns/op` 가 `BenchmarkFalseSharing` 보다 낮아야 한다. 낮아지지 않는다면 실행 환경의 CPU 개수나 전원 관리 설정, 혹은 캐시 라인 크기가 64바이트가 아닌 경우를 의심해볼 수 있다.

## 코드로 확인하기

`main.go` 는 먼저 `cacheLineSize`, `GOMAXPROCS`, `NumCPU` 를 출력한다. 그 아래 두 구조체의 크기와 b 오프셋을 출력하는데, 잘못된 배치는 `size=16 offset(b)=8` 이고 패딩 배치는 `size=72 offset(b)=64` 다. 여기서 핵심은 패딩 배치의 b 오프셋이 정확히 캐시 라인 경계인 64에 있다는 점이다. a 는 0부터 7까지, b 는 64부터 71까지를 차지하므로 두 필드는 절대 같은 64바이트 라인에 들어가지 않는다.

실행 시간 부분은 `runFalseSharing(5_000_000)` 과 `runPadded(5_000_000)` 을 각각 호출해 `elapsed` 를 출력한다. `n` 은 각 goroutine 이 수행하는 증가 횟수다. 두 goroutine 이 총 2n 번 증가하므로 `a` 와 `b` 는 항상 n 으로 같아야 한다. 타이밍은 CPU 캐시 크기, 코어 수, CPU 클럭 스케일링, 백그라운드 부하에 따라 달라질 수 있다. 그래서 main 의 타이밍은 관찰용일 뿐이고, 안정적인 비교는 benchmark 로 한다.

`main_test.go` 의 `TestPaddedCountersSeparateLines` 는 `falseSharingOffsetB` 가 64 미만이고 `paddedOffsetB` 가 64 이상인지 검증한다. 이는 "같은 라인인가, 다른 라인인가"를 구조적으로 보장하는 테스트다. `TestRunCounts` 는 두 실행 함수가 동시성 상황에서도 카운터를 정확히 n 으로 만드는지 확인한다. false sharing 은 정확성에는 영향을 주지 않는다는 사실을 못박는 테스트다.

벤치마크 함수는 `b.ResetTimer` 를 goroutine 생성 이후, 시작 신호를 보내기 직전에 호출한다. 이렇게 하면 goroutine 생성 비용은 측정에서 제외되고 실제 증가 루프만 측정된다. 채널은 시작 전에 이미 만들어져 있고, 두 goroutine 은 start 채널을 기다리고 있으므로 close 하면 동시에 루프를 시작한다.

## 모르면 겪는 일

이 문제를 모르면 "CPU 를 두 배 늘렸는데 처리량이 거의 안 늘어난다"는 증상을 흔히 겪는다. 예를 들어 요청마다 독립적인 카운터를 여러 개 증가시키는 구조체가 있다고 하자. 요청을 처리하는 goroutine 이 늘어나면 코어도 늘어나는데, 구조체 필드들이 한 캐시 라인에 몰려 있으면 atomic increment 가 사실상 직렬화된다. CPU 프로파일에는 `atomic.AddInt64` 나 `runtime.procyield` 비슷한 부분만 보이고, 왜 느린지 명확한 원인은 안 보인다.

Go 에서는 특히 슬라이스를 사용할 때 이 문제가 자주 생긴다. `[]int64` 같은 슬라이스를 worker 수만큼 나눠서 각 goroutine 이 자기 인덱스만 갱신해도, 인접한 두 원소는 거의 반드시 같은 캐시 라인에 있다. 데이터 경쟁은 없지만 캐시 라인 경쟁은 생긴다. p99 레이턴시가 GC 주기와는 무관하게 특정 부하 구간에서 튀는데, 프로파일에는 GC 나 lock contention 이 아닌 캐시 미스만 잔뜩 잡힌다.

또 다른 증상은 race detector 를 켜도 아무것도 안 잡힌다는 것이다. 개발자는 서로 다른 변수를 각자 쓰고 있으니 안전하다고 판단한다. 실제로도 안전하다. 하지만 성능은 안전한 예상보다 몇 배 느려진다. 이 때문에 "잘못된 배치로 느려지는 것"과 "동기화를 잘못해서 느려지는 것"을 구분하지 못해 엉뚱한 mutex 를 더 잠그는 방향으로 코드를 바꾸기도 한다.

Go 런타임 내부 구조체처럼 자주 쓰는 카운터를 가진 커스텀 구조체도 마찬가지다. 예를 들어 `type Metrics struct { req int64; err int64 }` 를 모든 요청이 공유한다면, req 와 err 는 거의 항상 같은 캐시 라인에 있다. 한 요청 goroutine 이 req 를 올리고 다른 goroutine 이 err 를 올리면 라인 소유권 경쟁이 발생한다. 이건 `sync/atomic` 으로도 해결되지 않는다.

## 언제 신경 쓰고 언제 무시하나

신경 써야 하는 조건은 분명하다. 여러 goroutine 이 서로 다른 메모리 위치를 매우 자주 쓰고, 그 위치들이 물리적으로 인접해 있어야 하며, 갱신이 전체 처리량의 병목이 될 만큼 잦아야 한다. 다시 말해 hot path 에서만 의미가 있다. 로그 한 줄에 카운터 하나 올리는 정도라면 false sharing 은 체감되지 않는다.

갱신이 이미 mutex 로 보호되고 있다면 패딩은 별 효과가 없다. 뮤텍스 자체가 라인 소유권 경쟁보다 더 큰 직렬화 지점이기 때문이다. 락 없이 atomic 으로 다수의 독립 필드를 갱신하는 경우에만 캐시 라인 분리가 직접적인 이득을 준다.

구조체가 배열이나 슬라이스에 많이 들어가 있다면 패딩을 넣기 전에 반드시 메모리 사용량을 따져야 한다. 예를 들어 `FalseSharingCounters` 는 16바이트인데 `PaddedCounters` 는 72바이트다. 원소 백만 개면 메모리가 16MB 에서 72MB 로 늘어난다. 캐시에 들어가는 원소 수가 줄어들면서 오히려 전체 성능이 나빠질 수도 있다. 이럴 때는 패딩 대신 슬라이스 원소를 코어별로 분리하거나, 아예 배열의 stride 를 바꾸는 방법이 나을 수 있다.

결론적으로 이 지식이 중요해지는 규모는 "코어 수가 최소 4개 이상이고, atomic write 가 초당 수백만 회 이상 일어나는 구간"이다. 그 이하에서는 패딩보다 가독성과 메모리 밀도가 더 중요하다. pprof 와 `perf stat -e cache-misses` 로 실제 캐시 미스가 뜨는지 확인한 뒤에 패딩을 넣어도 늦지 않다.

## 더 파보기

- false sharing 기본 개념: https://en.wikipedia.org/wiki/False_sharing
- Go 런타임이 패딩을 쓰는 예: https://github.com/golang/go/blob/master/src/runtime/runtime2.go
- Go 런타임이 CPU 캐시 라인 크기를 정하는 코드: https://github.com/golang/go/blob/master/src/internal/cpu/cpu.go
- Mechanical Sympathy 블로그의 false sharing 설명: https://mechanical-sympathy.blogspot.com/2011/07/false-sharing.html
- Intel 의 false sharing 식별 및 회피 가이드: https://www.intel.com/content/www/us/en/developer/articles/technical/avoiding-and-identifying-false-sharing-among-threads.html