## 한 줄 요약

Go의 race detector는 ThreadSanitizer(TSan)의 벡터 클록과 shadow memory 기법을 사용해, 실제 실행 중에 일어난 메모리 접근 사이의 happens-before 관계를 추적하고 동기화 없이 충돌하는 읽기/쓰기를 보고한다. 실행된 경로에서만 탐지하므로 false positive는 거의 없지만 false negative는 존재하며, 모든 메모리 접근에 계측 코드가 끼어들어 CPU와 메모리 오버헤드가 크다.

## 왜 이런 설계인가

멀티스레드 프로그램에서 data race는 재현이 어렵고, 로그를 남기는 행위 자체가 타이밍을 바꿔 버그를 숨긴다. 단순히 테스트를 많이 돌리는 것으로는 잡히지 않는 경우가 많다. 그래서 실행 중에 메모리 접근을 감시하는 동적 분석 도구가 필요했다.

정적 분석은 전체 경로를 다 볼 수 있지만 Go처럼 동적 고루틴 생성과 채널 통신이 많은 언어에서는 거짓 경보가 너무 많아진다. 실제 프로그램 경로 중 상당수는 실행되지 않거나, 실행되더라도 어떤 고루틴이 어떤 주소에 언제 접근할지 정적으로 결정하기 어렵다. 반대로 동적 분석은 실제 실행 중에 접근한 주소와 스레드의 클록만 보고 판정하므로 정확도가 높다.

ThreadSanitizer가 선택한 방식은 벡터 클록이다. Lamport 시계 같은 단일 카운터는 동시성 여부를 판정할 수 없기 때문이다. 벡터 클록은 각 스레드의 논리 시계를 배열로 들고 있어서, 두 이벤트가 인과적으로 연결되어 있는지 아니면 동시적인지 비교할 수 있다. 동시적이면서 최소 하나가 쓰기면 data race다.

다른 대안으로는 하드웨어 감시나 오프라인 재실행이 있다. 하드웨어 감시는 CPU 기능에 의존하고 모든 충돌을 잡지 못한다. 오프라인 재실행은 기록한 순서를 강제로 재현해야 해서 오버헤드가 더 크고 Go 런타임과 통합하기 어렵다. TSan은 컴파일러가 삽입한 계측 호출만으로 동작하므로 언어 런타임과 밀접하게 붙일 수 있었고, 벡터 클록 비교로 메모리 접근 충돌을 정확히 판정한다.

## 어떻게 동작하는가

Go 빌드에 `-race` 플래그를 주면 컴파일러는 거의 모든 메모리 읽기/쓰기 앞에 race runtime 호출을 삽입한다. 예를 들어 `x = 1` 같은 코드는 실제 메모리 연산 전에 `runtime.racewrite` 류의 함수를 호출한다. 이 함수들은 주소를 받아서 shadow memory에 저장된 접근 기록과 현재 goroutine의 벡터 클록을 비교한다.

shadow memory는 애플리케이션 주소 공간의 일부 영역을 분석용 그림자 영역으로 대응시킨다. 원본 주소를 비트 시프트하거나 마스킹해서 shadow cell을 찾고, 그 cell에는 최근 읽기/쓰기 정보와 벡터 클록이 저장된다. 만약 현재 접근이 어떤 이전 접근과 동시적이고 둘 중 하나가 쓰기라면 race detector가 보고를 출력한다.

벡터 클록은 goroutine마다 유지된다. 고루틴이 생성되면 부모의 클록을 복사하고 자신의 인덱스를 증가시킨다. 채널 send는 release 의미를 가져 송신 고루틴의 클록을 올리고, 채널 receive는 acquire 의미를 가져 송신자의 클록을 병합한 뒤 자신의 클록을 올린다. sync.Mutex, sync.WaitGroup, atomic 연산도 각각 release/acquire 쌍으로 처리되어 happens-before 간선을 만든다.

따라서 두 접근의 벡터 클록을 비교했을 때 어느 한쪽이 다른 쪽보다 작거나 같으면 happens-before가 성립하므로 race가 아니다. 두 클록이 서로 비교 불가능하면 동시적이며, 그때 쓰기가 하나라도 있으면 race로 판정한다. 이 판정은 실제 시간 선후가 아니라 메모리 모델상 동기화 여부에 의존하므로, 실행 인터리빙이 조금 달라져도 동일한 불변식을 위반하면 탐지된다.

`race_detector` 문서와 `runtime/race` 소스를 보면, 실제 계측은 `runtime.racewriterange`, `runtime.raceread`, `runtime.raceacquire` 같은 함수로 내려간다. 이 함수들은 Go 런타임과 C로 작성된 TSan 런타임을 연결하는 다리 역할을 한다. `go/src/runtime/race/race.go`, `race_amd64.s`를 보면 주소 기반으로 shadow cell을 조회하는 저수준 루틴이 있다.

## 돌려보기

이 디렉토리에서 아래 명령을 순서대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행 - unsafe 결과가 5000보다 작은지 본다
go run -race .               # race detector 실행 - unsafe race 보고가 나오는지 본다
go test -v ./...             # 테스트 - safe 합계와 vector clock 출력을 검증
go test -race ./...          # 동시성 주제라면 반드시 - 테스트는 race 없이 통과해야 한다
go test -bench=. -benchmem            # race detector 없는 벤치마크
go test -bench=. -benchmem -race      # race detector 켠 벤치마크 - 오버헤드 비교
```

`go run -race .`는 의도적으로 만든 `sumConcurrentlyUnsafe`에서 race 보고를 출력한다. 여러 보고가 나오고 계속 실행될 수 있으므로 첫 번째 race에서 멈추고 싶으면 `GORACE=halt_on_error=1 go run -race .`를 사용하면 된다. `go test -bench`는 `BenchmarkMemoryAccess`가 단순 메모리 접근을 반복하므로 `-race` 여부에 따라 실행 시간이 크게 차이 나는 것을 보여준다.

## 코드로 확인하기

`vectorClockDemo`는 벡터 클록 비교를 2개 goroutine의 축약된 클록으로 보여준다. 첫 번째 시나리오에서는 g1 clock `[1 0]`과 g2 clock `[0 1]`이 서로 작거나 같지 않기 때문에 concurrent로 판정되어 `race? true`가 출력된다. 두 번째 시나리오는 g1 write `[1 0]`이 g1 send `[2 0]`과 g2 recv `[2 2]`를 거쳐 g2 write `[2 3]`보다 작거나 같으므로 `race? false`가 출력된다.

`sumConcurrentlyUnsafe`는 `counter++`을 잠금 없이 실행한다. `++`은 읽고, 더하고, 다시 쓰는 세 단계라서 두 goroutine이 같은 값을 읽으면 한 번의 증가가 사라진다. `-race` 없이 실행하면 결과가 5000보다 작게 나오는 경우가 많다. `-race`로 실행하면 race detector가 이 코드의 읽기/쓰기 충돌을 보고한다.

`sumConcurrentlySafe`는 `atomic.AddInt64`를 사용해 각 증가를 원자적으로 수행한다. 결과는 항상 정확히 `goroutines * 1000`이 된다. `conditionalRaceDemo`는 `trigger=false`로 실행하면 쓰기 경로가 실행되지 않아 `-race`에서도 이 부분은 보고가 없다. `trigger=true`로 바꾸면 쓰기와 읽기가 동기화 없이 동시에 실행되어 race 보고가 만들어진다.

`main_test.go`의 `TestSumConcurrentlySafeKeepsTotal`은 safe 함수가 정확한 합계를 유지하는지 검증한다. `TestVectorClockDemoShowsRaceAndSync`는 벡터 클록 출력에 `race? true`와 `race? false`가 모두 들어 있는지 확인한다. `BenchmarkMemoryAccess`는 setup 뒤에 `b.ResetTimer`를 호출해 순수 메모리 접근 루프만 측정하므로 `-race` 유무에 따른 오버헤드를 직접 비교할 수 있다.

## 모르면 겪는 일

race detector를 테스트에만 켜고 프로덕션에는 끄는 이유를 모르면, 개발 단계에서는 잡히지 않던 data race가 특정 트래픽 패턴에서만 나타난다. 증상은 로그에 이상한 값이 찍히거나, 가끔 nil 포인터 역참조로 패닉이 나거나, 응답 값이 간헐적으로 오염되는 방식으로 드러난다. 재현은 되지 않고 CPU 프로파일에는 사용자 코드가 아닌 런타임 지점만 보여서 원인 추적이 어렵다.

race detector가 "실행된 경로만" 잡는다는 사실을 모르면, 한 번 `-race` 테스트를 통과한 뒤 모든 동시성 버그가 없다고 믿는 실수를 한다. 예를 들어 특정 기능 플래그가 false일 때만 실행되는 배치 경로에 race가 있다면, 평소 테스트에서는 그 경로가 실행되지 않아 발견되지 않는다. 카나리 배포에서 해당 플래그를 켜는 순간 race가 살아나고 장애로 이어진다.

오버헤드가 5배에서 10배에 이른다는 것을 모르면, 프로덕션 바이너리에 `-race`를 넣고 배포하는 시도를 할 수 있다. CPU 사용률이 급증하고 메모리 사용량도 shadow memory 때문에 크게 늘어난다. p99 응답 시간이 평소보다 몇 배로 튀는데 애플리케이션 프로파일에는 비즈니스 로직이 아니라 race runtime 함수가 상위를 차지한다. 오토스케일링이 이를 트래픽 증가로 오인해 불필요한 인스턴스를 띄우는 2차 비용도 생긴다.

race detector가 적발하는 조건을 정확히 알지 못하면, `atomic`이나 `mutex`를 썼는데도 race 보고가 뜨는 이유를 오해한다. 예를 들어 `sync.WaitGroup.Wait` 이후에는 반드시 happens-before가 생긴다고 생각하지만, 고루틴 내부에서 Wait 전에 수행된 쓰기와 Wait 후 메인 고루틴의 읽기가 올바르게 동기화되지 않는 경우가 있을 수 있다. Go 메모리 모델을 벡터 클록 관점에서 이해하지 못하면 race 보고를 보고도 어느 쪽 동기화가 잘못됐는지 파악하기 어렵다.

## 언제 신경 쓰고 언제 무시하나

race detector는 개발·테스트·CI에서 상시 켜는 것이 좋다. 특히 동시성 패턴이 많은 패키지에서는 `go test -race ./...`가 몇 초만 더 걸리지만 data race를 초기에 잡아준다. 단위 테스트, 통합 테스트, 짧은 부하 테스트에서 `-race`를 켜면 실행 경로가 늘어나 탐지 확률이 올라간다.

프로덕션에는 `-race`를 켜지 않는다. CPU 5~10배, 메모리 수 배 오버헤드가 붙고, shadow memory가 모든 접근을 감시하므로 가비지 컬렉션과 스케줄러 타이밍이 바뀐다. 이는 오히려 성능 문제를 일으키고, race detector 자체가 관찰 대상 시스템의 동작을 바꾸는 observer effect를 만든다. 프로덕션에서 race를 잡으려면 카나리 일부에만 `-race`를 켜거나, race가 의심되는 코드만 별도 테스트로 빼서 반복 실행하는 방식이 낫다.

작은 CLI 도구나 스크립트처럼 고루틴을 거의 사용하지 않고 공유 메모리 접근이 없는 프로그램이라면 `-race` 오버헤드가 아깝지 않을 만큼 작을 수 있다. 하지만 그래도 CI에서만 켜는 습관이 안전하다. 특히 `go run`으로 데모를 돌릴 때는 의도적으로 race를 만들었으므로 프로덕션 빌드에 `-race`를 넣으면 안 된다.

"실행된 경로만 잡는다"는 한계를 인지하고, race detector가 통과했다고 해서 절대적으로 race-free를 보장하지는 않는다고 생각해야 한다. 동시성 코드 리뷰, 메모리 모델 문서, 가능하면 정적 분석 도구를 함께 사용해야 한다. race detector는 실행 중에 실제로 지나간 경로에서 충돌하는 접근을 잡는 도구일 뿐, 실행되지 않은 경로의 race까지 미리 찾아주지는 않는다.

## 더 파보기

- Go 공식 문서: https://go.dev/doc/articles/race_detector
- ThreadSanitizer 알고리즘 개요: https://github.com/google/sanitizers/wiki/ThreadSanitizerAlgorithm
- Go 런타임 race 통합 소스: https://go.dev/src/runtime/race/race.go
- race detector 관련 이슈: https://github.com/golang/go/issues?q=race+detector
- Go 메모리 모델 문서: https://go.dev/ref/mem