## 한 줄 요약

세마포어는 동시에 실행 중인 goroutine 개수(동시성)를 제한하고, rate limiter는 단위 시간당 처리량을 제한한다. 버퍼 채널은 Go 런타임에 이미 존재하는 대기열을 재사용하는 채널 세마포어가 되며, weighted semaphore는 goroutine마다 다른 양의 자원을 요구할 때 필요하다. token bucket은 burst를 허용하면서 평균 처리율을 지키기 때문에 x/time/rate의 기본 알고리즘으로 쓰인다.

## 왜 이런 설계인가

동시성 제한과 처리율 제한은 초보 시점에는 비슷해 보이지만 보호하려는 자원이 다르다. 동시성 제한은 DB connection pool, CPU, 메모리처럼 "지금 몇 개가 물려 있는가"에 민감한 자원을 지킨다. 처리율 제한은 외부 API quota, 초당 요청 수 제한처럼 "지난 1초에 몇 건을 보냈는가"를 지킨다. 둘 중 하나만 쓰면 다른 하나가 무너질 수 있다. goroutine을 3개로 제한해도 각 goroutine이 loop로 요청을 쏘면 초당 1000건이 나갈 수 있고, rate limit만 걸면 동시 실행 수는 여전히 폭증해 메모리를 잡아먹는다.

버퍼 채널이 세마포어 역할을 하는 이유는 Go 런타임의 hchan에 이미 sendq와 recvq라는 대기열이 내장되어 있기 때문이다. `make(chan struct{}, n)`으로 버퍼를 만들고 `s <- struct{}{}`를 P 연산으로 쓰면, 버퍼가 가득 찼을 때 런타임이 goroutine을 G waiting 상태로 바꾸고 sendq에 넣는다. 별도의 mutex와 condition variable을 만들 필요가 없다. 반면 weighted semaphore가 필요한 이유는 채널 세마포어가 항상 "1개 슬롯"만 소비하는 구조라서, goroutine마다 요구량이 다른 자원을 표현하려면 값을 여러 번 보내야 하거나 복잡해지기 때문이다. golang.org/x/sync/semaphore의 Weighted는 mutex와 cond를 조합해 n개의 자원을 한 번에 확보한다.

token bucket이 rate limiter의 기본 알고리즘으로 선택된 이유는 두 가지 상반된 목표를 동시에 만족시키기 때문이다. 평균 처리율은 지키면서도 짧은 순간에는 요청이 몰려도 일부를 바로 허용하고 싶다. leaky bucket은 요청을 일정 간격으로 균일하게 흘려보내기 때문에 burst를 흡수하지 못한다. fixed window counter는 경계 시각에 요청이 두 배로 몰릴 수 있다. token bucket은 burst 크기만큼의 즉시 허용량을 제공하면서, 그 뒤에는 rate에 맞춰 토큰이 채워질 때까지 기다리게 한다. 이때 "기다린다"는 계산이 바로 ReserveN이 반환하는 duration이고, 이 duration을 sleep으로 소비하는 것이 Wait이다.

왜 굳이 직접 만드는가? 표준 라이브러리에는 rate limiter가 없다. x/time/rate는 외부 모듈이지만 Go 팀이 관리하는 de facto 표준이다. 여기서는 외부 모듈 없이 표준 라이브러리만 써야 하므로 tokenBucket을 직접 구현했다. 하지만 실제 운영에서는 검증된 x/time/rate를 쓰는 편이 낫다. context 취소, Inf/0 처리, reservation의 OK 반환, monotonic clock 보정 같은 세부 처리를 직접 구현하면 놓치기 쉽다. 이 학습 자료의 tokenBucket은 ReserveN의 계산 원리를 보여주는 최소 구현이다.

## 어떻게 동작하는가

채널 세마포어는 runtime/chan.go의 hchan 구조체가 중심이다. `make(chan struct{}, n)`은 hchan.buf에 n개의 슬롯을 만든다. `s <- struct{}{}`는 hchan.sendq에 대기열이 없으면 버퍼에 값을 넣고, 버퍼가 가득 차면 현재 goroutine을 sendq에 넣은 뒤 gopark로 스케줄러에게 넘긴다. `<-s`는 recvq에서 기다리는 수신자가 있으면 바로 넘기고, 없으면 버퍼에서 꺼내면서 sendq의 첫 번째 goroutine을 runnable 상태로 만든다. 즉, 채널 세마포어의 대기열은 Go 런타임의 스케줄러가 이미 최적화한 자료구조를 그대로 쓰는 것이다.

weighted semaphore는 이와 달리 사용자 영역에서 상태를 관리한다. newWeightedSemaphore는 sync.Mutex와 sync.Cond를 만든다. acquire(n)은 mu를 잡은 뒤 `cur+n > max`이면 cond.Wait를 호출한다. cond.Wait는 뮤텍스를 풀고 대기열에 들어간다. release(n)는 cur에서 n을 빼고 cond.Broadcast로 대기자들을 깨운다. 깨어난 goroutine은 for loop에서 조건을 다시 검사하는데, 이는 허위 각성(spurious wakeup)과 여러 대기자가 동시에 깨어났을 때 자원 부족을 막기 위한 필수 패턴이다. x/sync/semaphore의 Weighted도 구조는 동일하지만, 깨울 때 Signal을 쓰고 waiter를 정밀하게 관리해 불필요한 각성을 줄인다.

token bucket은 tokens, last, rate, burst 네 필드로 상태를 유지한다. ReserveN(now, n)은 먼저 `elapsed := now.Sub(last)`로 마지막 보충 이후 경과 시간을 구한다. 경과 시간에 rate를 곱해 tokens에 더하고, burst를 넘지 않도록 잘라낸다. 그런 다음 last를 now로 갱신한다. 이후 요청량 n이 tokens보다 작거나 같으면 tokens에서 빼고 0을 반환한다. 부족하면 `deficit := need - tokens`만큼 채우는 데 걸리는 시간 `deficit/rate`를 duration으로 반환한다. tokens는 0으로 만들어 남은 토큰이 소비된 것으로 처리한다. x/time/rate의 rate.go도 동일한 구조이며, 여기에 context와 reservation 객체를 덧붙인 것이다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사: 복사 금지 타입, 포맷 문제 등을 발견한다
go build -o /dev/null ./...  # 컴파일 확인: 불필요한 import나 선언 오류가 없어야 통과한다
go run .                     # 시연 실행: 동시성 최대치 3과 rate limit 대기 패턴을 직접 확인한다
go test -v ./...             # 테스트: 세마포어 불변식과 token bucket 계산이 정확한지 출력한다
go test -race ./...          # 동시성 테스트: atomic 외에 data race가 없다는 것을 검증한다
go test -bench=BenchmarkChanSemaphore -benchmem ./...  # 채널 세마포어 연산 비용 측정
```

`go run .`에서는 동시성 제한 비교와 rate limiting 두 블록이 출력된다. `go test -v`는 세마포어가 limit 이상으로 동시 실행되지 않는다는 것과 token bucket의 대기 시간 계산이 정확하다는 것을 보여준다. `-race`는 테스트가 atomic을 올바르게 사용하는지 검증하므로 반드시 통과해야 한다.

## 코드로 확인하기

main.go의 runConcurrencyDemo는 limit을 3으로 두고 jobs 30개를 실행한다. 제한이 없으면 최대 동시 실행 수가 30에 가깝게 나온다. 채널 세마포어와 weighted semaphore는 모두 3을 출력한다. 이 3은 active를 atomic.AddInt64로 증가시키고 updateMax로 최댓값을 갱신한 결과다. 시간을 1ms만 잤기 때문에 실행 순서에 따라 정확히 3보다 낮을 수 있지만, 3보다 큰 값은 절대로 나오지 않는다. 테스트는 이 "limit 이하"라는 불변식을 검증한다.

runRateLimitDemo는 newTokenBucket(2, 3)으로 초당 2개 버스트 3개를 만든다. 출력을 보면 요청 0, 1, 2는 대기 0으로 즉시 허용된다. 요청 3부터는 약 500ms의 대기가 번갈아 나타난다. 그 이유는 rate가 2/s이므로 토큰 1개가 채워지는 시간이 0.5초이기 때문이다. 버스트가 3이라서 초기에 3개를 쓴 뒤에는 토큰이 0이 되고, 다음 요청은 0.5초를 기다려야 토큰 1개를 받을 수 있다. 이후에는 쓰자마자 다시 0이 되는 패턴이 반복된다. 테스트의 TestTokenBucketReserveN은 이 계산을 결정적으로 검증한다. newTokenBucket(1, 2)에서 2개 즉시, 같은 시각 1개는 1초 대기, 1초 후 1개는 즉시, 같은 시각 추가 1개는 다시 1초 대기를 각각 확인한다.

테스트 파일은 채널 세마포어와 weighted semaphore의 동시성 제한을 각각 검증한다. goroutine 실행 순서나 대기열 진입 순서는 스케줄러가 결정하므로 순서를 검증하지 않고, 오직 "최대 동시 실행 수가 limit 이하"라는 개수 기반 불변식만 본다. 이렇게 하면 CI에서 부하가 걸려도 테스트가 안정적으로 통과한다. BenchmarkChanSemaphore는 경합이 없는 상태에서 acquire/release 1회의 평균 비용을 ns/op 단위로 보여준다.

## 모르면 겪는 일

동시성 제한과 rate limiting을 혼동하면 공유 자원을 지키지 못한다. 예를 들어 외부 결제 API가 초당 10건만 허용한다고 하자. goroutine을 10개로 동시성 제한해도 각 goroutine이 loop를 돌며 즉시 요청하면 1초에 수백 건이 나가 429나 계정 정지를 당한다. 반대로 rate limit만 걸고 동시성 제한을 빼면 순간적으로 goroutine이 수천 개 늘어나 메모리가 급증하고 GC pressure가 올라간다. p99 지연이 GC 주기마다 튀는데 CPU 프로파일에는 잡히지 않는 일이 생긴다.

채널 세마포어를 가변 자원에 잘못 쓰면 deadlock이나 자원 회계 오류가 생긴다. goroutine이 메모리 버퍼를 1KB 쓰는 요청과 1MB 쓰는 요청이 섞여 있는데, 채널 세마포어로 동시 실행 10개를 제한하면 실제 메모리 사용량은 10KB가 아니라 10MB에 가까워질 수 있다. weighted semaphore를 쓰지 않으면 이런 불균형을 표현할 방법이 없다. 반대로 weighted semaphore에서 release할 때 acquire한 양보다 큰 n을 넘기면 cur가 음수가 되어 다음 acquire가 비정상적으로 즉시 허용될 수 있다.

token bucket을 직접 구현할 때 시간을 되돌리면 대기 시간이 비정상적으로 커진다. now가 last보다 과거이면 elapsed가 음수가 되어 tokens가 음수가 되고, deficit가 폭발적으로 커져서 수백 년을 기다리는 결과가 나온다. 실제 운영에서는 NTP 보정이나 컨테이너 migration으로 벽시계가 뒤로 갈 수 있다. x/time/rate는 monotonic clock을 사용하고, 시간이 뒤로 가면 last를 now로 당겨서 이 문제를 막는다. 우리 구현도 elapsed를 0으로 보정했다. 테스트에서 과거 시각을 주입했을 때 이 문제가 실제로 발생했었다.

rate limiter를 context 없이 쓰면 서비스 종료 시 goroutine이 오래 sleep에 남을 수 있다. x/time/rate의 Wait(ctx)는 ctx가 취소되면 대기를 중단하지만, 위의 단순한 wait은 time.Sleep만 하므로 취소가 불가능하다. 트래픽이 몰려 대기 시간이 수십 초로 늘어나면 그동안 goroutine이 블로킹되어 자원을 점유한다. 결국 서비스가 느려지는 원인이 rate limiter 자체가 될 수 있다. 이럴 때는 context deadline이나 요청 우선순위를 함께 고려해야 한다.

## 언제 신경 쓰고 언제 무시하나

동시성 제한은 DB connection pool, 파일 descriptor, 고정 크기 워커 풀처럼 자원 수가 명확하고 초과 시 장애로 이어질 때 신경 써야 한다. goroutine 수가 수십 개 이하이고 공유 자원이 없다면 채널 세마포어조차 과할 수 있다. 단순히 `sync.WaitGroup`만으로 충분한데 세마포어를 넣으면 코드만 복잡해진다. 반대로 한 번에 실행되는 goroutine이 수천 개가 되면 동시성 제한이 메모리와 GC latency에 직접 영향을 준다.

weighted semaphore는 goroutine마다 소비 자원량이 다를 때만 가치가 있다. 모든 요청이 동일한 크기의 자원을 쓴다면 채널 세마포어가 더 단순하고 런타임 최적화도 받는다. x/sync/semaphore의 Weighted는 관리 코드가 더 많고 각 연산에 뮤텍스와 cond 시그널 비용이 들어간다. 그 비용은 초당 수십만 건 이상의 acquire/release가 있을 때부터 의미를 가진다. 그 이하에서는 병목이 되지 않는다.

rate limiter는 외부 API, 결제, 인증, 파일 업로드처럼 시간당 제한이 있는 자원을 호출할 때만 필요하다. 내부 서비스끼리 호출하는데 rate limit을 넣으면 장애 전파를 늦추는 효과가 있지만, 그만큼 지연이 늘고 운영 복잡도가 올라간다. burst를 얼마로 둘지도 신중해야 한다. burst가 너무 크면 순간 부하를 막지 못하고, 너무 작으면 정상적인 burst 트래픽까지 잘게 쪼개어 사용자 경험을 해친다. 지표를 보면서 "평균 초당 요청 수"와 "최대 순간 요청 수"를 구분해 burst를 정해야 한다.

## 더 파보기

- golang.org/x/time/rate 공식 문서: https://pkg.go.dev/golang.org/x/time/rate
- x/time/rate의 rate.go 소스: https://cs.opensource.google/go/x/time/+/master:rate/rate.go
- x/sync/semaphore의 semaphore.go 소스: https://cs.opensource.google/go/x/sync/+/master:semaphore/semaphore.go
- Go 런타임의 hchan과 sendq/recvq: https://cs.opensource.google/go/go/+/master:src/runtime/chan.go
- sync.Cond와 condition variable 패턴: https://pkg.go.dev/sync#Cond