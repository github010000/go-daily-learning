## 한 줄 요약

Go 런타임의 GMP 모델은 goroutine(G), OS 스레드(M), logical processor(P)를 분리해 스케줄링 상태와 실행 수단을 떼어놓는다. P는 로컬 런큐와 메모리 캐시를 보유한 자원 자격증 역할을 하며, 같은 개수의 G라도 P가 몇 개인지에 따라 실제 병렬성이 결정된다.

## 왜 이런 설계인가

Go 1.1 이전 스케줄러는 G와 M만 있었고 모든 실행 대기 goroutine을 하나의 글로벌 런큐에 넣었다. 스케줄링할 때마다 전역 뮤텍스를 잡아야 했기 때문에 goroutine 수가 늘어나면 락 경합이 처리량을 갉아먹었다. CPU 코어가 몇 개이든 관계없이 글로벌 큐 하나를 두고 싸우는 구조는 멀티코어 확장성이 거의 없었다.

G와 M을 직접 묶는 대안도 문제가 있었다. goroutine 하나에 OS 스레드 하나를 할당하면 스택과 스레드 생성 비용 때문에 Go의 장점이 사라진다. 반대로 여러 G를 하나의 M에 멀티플렉싱하려면 대기 중인 G를 어딘가에 줄 세워야 하는데, 그 줄을 글로벌 런큐 하나로 만들면 다시 락 경합이 생긴다. 차라리 M마다 로컬 큐를 주고 필요할 때만 글로벌 큐를 쓰게 하는 것이 낫다.

P는 그 로컬 큐를 소유하는 존재로 등장했다. P는 M이 Go 코드를 실행하려면 반드시 붙잡고 있어야 하는 자격이다. goroutine이 blocking syscall에 들어가면 해당 M은 커널에서 멈추지만, P는 그 M으로부터 떼어져 다른 M에 붙는다. 이렇게 하면 blocking syscall 때문에 CPU가 쉬지 않고, 동시에 Go 코드를 실행하는 M 개수는 GOMAXPROCS로 제한된다. OS 스레드는 필요할 때 더 생길 수 있지만, 실제로 Go 코드를 돌리는 스레드 수는 P 개수를 넘지 않는다.

P 하나에는 크기가 256인 로컬 런큐와 runnext 슬롯이 있다. 로컬 큐가 가득 차면 절반을 글로벌 런큐로 넘기고, 로컬 큐가 비면 다른 P의 로컬 큐에서 절반을 훔쳐 온다. 이렇게 하면 전역 락을 자주 잡지 않고도 작업 분배가 된다. runnext는 채널에서 깨어난 goroutine처럼 방금 대기가 풀린 G를 즉시 다음 실행 후보로 넣어 캐시 지역성을 높이는 장치다.

## 어떻게 동작하는가

`runtime/runtime2.go`를 보면 G, M, P가 각각 struct로 정의되어 있다. G는 `stack`, `sched`, `goid`, `atomicstatus`를 가진다. `sched`는 gobuf로 sp, pc, ret 같은 실행 재개 정보를 담는다. G의 상태는 `_Gidle`, `_Grunnable`, `_Grunning`, `_Gsyscall`, `_Gwaiting`, `_Gdead` 등으로 나뉜다. runnable 상태의 G는 어떤 P의 로컬 런큐나 글로벌 런큐에서 실행 차례를 기다린다.

M은 OS 스레드를 감싼다. struct m에는 `g0`, `curg`, `p`, `mOS` 필드가 있다. `g0`는 사용자 코드가 아니라 스케줄러 자체가 쓰는 특수 goroutine이다. `curg`는 이 M이 현재 실행 중인 사용자 G를 가리킨다. M은 `p` 필드로 P를 참조하며, P 없이는 Go 코드를 실행하지 못한다. goroutine이 syscall에 들어가면 `runtime/proc.go`의 `entersyscall`이 P를 `_Psyscall` 상태로 만들고, sysmon이 오래 멈춰 있다고 판단하면 P를 빼앗아 다른 M에게 준다.

P는 struct p에 `runq [256]guintptr`, `runqhead`, `runqtail`, `runnext`, `mcache`, `status`를 가진다. runq는 원형 배열처럼 head와 tail로 관리한다. 크기가 256인 이유는 캐시 라인 몇 개 안에 들어가면서도 전역 락을 자주 쓰지 않을 만큼은 길게 가져가기 위한 절충이다. `runnext`는 runq보다 먼저 확인하는 단일 슬롯이다. `mcache`는 P 전용 할당 캐시라서 같은 P에서 도는 goroutine들이 메모리를 인접하게 쓰게 만든다.

`runtime/proc.go`의 `schedule` 함수는 스케줄링 한 사이클의 중심이다. 현재 P의 `runnext`를 확인하고, 비어 있으면 `runqget`으로 로컬 런큐에서 꺼낸다. 로컬이 비면 `globrunqget`으로 글로벌 큐를 보고, 계속 비어 있으면 `findrunnable`에서 다른 P의 로컬 런큐를 `runqsteal`로 훔친다. 훔칠 때는 절반 정도를 가져와서 한쪽 P만 바쁘고 다른 P는 빈 채로 도는 불균형을 줄인다.

GOMAXPROCS를 바꾸면 `procresize`가 P 개수를 조정한다. P가 1이면 수백 개의 goroutine이 있어도 Go 코드를 동시에 실행하는 M은 하나뿐이다. P를 늘리면 그만큼 M이 병렬로 Go 코드를 실행할 수 있다. 그러나 P가 늘어날수록 mcache와 로컬 런큐도 늘고 work stealing 시도도 많아진다. 물리 코어 수를 넘어서 P를 늘리면 parallel speedup은 없고 컨텍스트 스위칭 오버헤드만 늘어난다.

이 구조는 `runtime.Gosched`를 쓰는 협조적 양보에서도 드러난다. Gosched를 호출하면 현재 G를 자기 P의 로컬 런큐에 넣고 `mcall(gosched_m)`을 타고 다시 스케줄러로 돌아간다. 이후 같은 P의 runnext나 runq에 있던 다른 G가 실행된다. main.go의 `cooperativeYieldWorkers`는 이 양보가 실제로 카운터를 몇 번 올리는지 보여준다.

## 돌려보기

이 디렉토리에서 아래 명령을 순서대로 실행하면 된다.

```bash
go vet ./...                 # 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행
GODEBUG=schedtrace=1000 GOMAXPROCS=2 go run .  # 스케줄러 내부 상태 로그
GOMAXPROCS=1 go run .        # P 1개로 실행
go test -v ./...             # 테스트
go test -race ./...          # 동시성 race 검사
go test -bench=. -benchmem ./...  # 벤치마크
```

`go vet ./...`는 코드에 복사된 락이나 의심스러운 동시성 패턴이 있는지 확인한다. 통과하면 정적 검사 결과가 없다는 뜻이다. `go build -o /dev/null ./...`는 main.go와 main_test.go가 함께 컴파일되는지 확인한다.

`go run .`은 처음에 `NumCPU`, `GOMAXPROCS`, `NumGoroutine`을 출력한 뒤, 현재 P를 그대로 사용하는 실행 시간과 P를 1, 2, 4, NumCPU로 바꿔가며 잰 실행 시간을 표로 보여준다. P가 1에서 2, 4로 늘어날수록 elapsed가 대략 절반씩 줄어드는지 봐야 한다. CPU-bound 작업이므로 물리 코어 한계까지는 거의 선형에 가깝게 줄어든다.

`GODEBUG=schedtrace=1000 GOMAXPROCS=2 go run .`는 1초마다 `SCHED` 로그를 출력한다. `gomaxprocs=2`, `idleprocs`, `threads`, `runqueue` 값을 봐야 한다. 이 로그에서 P 개수가 1, 2, 4 등으로 바뀌는 구간과 CPU-bound 작업 중 idleprocs가 0이 되는 구간이 겹친다. `GOMAXPROCS=1 go run .`은 외부 환경변수로 P를 1로 고정한 뒤 첫 시연의 elapsed가 크게 나오는 모습을 보여준다.

`go test -v ./...`는 결정적 계산과 모든 goroutine 완료, 그리고 협조적 양보 횟수를 검증한다. `go test -race ./...`는 runParallel이 서로 다른 슬라이스 인덱스에만 쓰고 atomic 카운터를 쓴다는 점을 race detector로 확인한다. `go test -bench=. -benchmem ./...`는 현재 P 개수에서 runParallel의 반복당 시간과 할당량을 보여준다.

## 코드로 확인하기

main.go의 `cpuBound`는 0부터 iterations-1까지 더하는 CPU-bound 함수다. 반환값에 workerID를 더해서 호출부가 값을 쓰게 만들었다. 이 반환값이 없으면 컴파일러가 루프를 통째로 제거할 가능성이 있고, 그러면 P 개수에 따른 시간 차이가 사라진다. firstResult가 0이 아닌 값으로 출력된다면 실제 계산이 수행된 것이다.

`runParallel`은 16개의 goroutine을 만들고 각자 `cpuBound`를 실행한다. 각 goroutine은 results 슬라이스의 고유한 인덱스에만 쓰기 때문에 `go test -race`에서도 경쟁이 없다. 부모 goroutine은 wg.Wait()로 모든 자식이 끝날 때까지 기다린다. 이 함수는 같은 입력에 같은 결과를 내므로 테스트에서 순차 계산과 비교할 수 있다.

`demonstrateCurrentP`는 현재 GOMAXPROCS를 그대로 사용해 한 번 실행한다. `demonstrateScaling`은 P를 1, 2, 4, NumCPU로 잠시 바꿔가며 같은 작업을 반복한다. 출력에서 GOMAXPROCS=1일 때 elapsed가 가장 크고 P를 늘릴수록 짧아지는 것을 보면, G가 많아도 P가 병렬 실행의 문턱이라는 것이 직접 관찰된다. P보다 worker 수가 많으면 일부 G는 로컬 런큐에서 대기하게 된다.

`cooperativeYieldWorkers`는 runtime.Gosched를 1000번씩 호출하면서 atomic 카운터를 올린다. 결과는 workers * 1000이 나와야 한다. 이 숫자가 정확하다는 것은 Gosched가 현재 G를 실행 큐에서 빼내고 다시 스케줄링해도 어떤 횟수도 잃지 않는다는 뜻이다. 실행 순서는 바뀔 수 있지만 총 횟수라는 불변식은 유지된다.

main_test.go의 `TestCPUBoundDeterministic`은 cpuBound의 반환값이 수식과 일치하는지 확인한다. `TestRunParallelCompletesAllWorkers`는 16개 goroutine 전부가 끝났는지, 각 결과가 순차 계산과 같은지 확인한다. 시간 단정은 전혀 없다. `TestCooperativeYieldWorkersCount`는 workers*1000이라는 총 횟수를 검증한다. `BenchmarkRunParallel`은 현재 GOMAXPROCS에서 처리량을 측정해 P를 조정할 때 참고할 기준을 제공한다.

## 모르면 겪는 일

CPU-bound 워크로드에서 GOMAXPROCS를 의식하지 않으면 goroutine을 아무리 많이 만들어도 처리량이 늘지 않는다. 예를 들어 컨테이너 CPU limit이 4인데 GOMAXPROCS가 1로 고정되어 있으면, 서버는 한 코어만 100% 쓰고 나머지 코어는 놀게 된다. goroutine 수는 수백 개인데 CPU 프로파일에는 `runtime.futex`나 syscall이 별로 없고 전부 runnable 상태로 보이면 P 부족을 의심해야 한다.

P가 여러 개인데도 OS 스레드가 수천 개까지 늘어나는 경우가 있다. blocking syscall을 수행하는 goroutine이 많으면 M이 커널에서 멈추는 동안 P를 다른 M에게 넘기기 위해 새 OS 스레드가 계속 생길 수 있다. 이걸 모르면 "GOMAXPROCS=4인데 왜 스레드가 2000개지?" 하는 혼란이 온다. OS 스레드 개수는 P 개수와 별개로 늘어날 수 있다는 사실을 알아야 진단이 가능하다.

GODEBUG=schedtrace를 보면 `gomaxprocs`, `idleprocs`, `threads`, `runqueue`가 함께 찍힌다. 이 중 runqueue가 긴 경우 병목이 로컬 런큐인지 글로벌 런큐인지 구분하지 않으면 잘못된 결론을 내린다. 예를 들어 어떤 goroutine이 오래 CPU를 점유하면 한 P의 로컬 큐만 길어지고 다른 P는 놀 수 있다. 이때 GOMAXPROCS를 올려도 해결되지 않는 경우가 많다. 스케줄러 내부 구조를 모르면 이런 증상을 전부 "Go는 느리다"로 치부하게 된다.

## 언제 신경 쓰고 언제 무시하나

대부분의 I/O bound 서비스에서는 기본 GOMAXPROCS를 건드릴 필요가 없다. 데이터베이스 조회나 HTTP 호출을 기다리는 동안 P는 다른 G로 넘어가고, syscall에서 돌아온 G는 runnext나 로컬 런큐에 다시 적재된다. 런타임이 이미 잘 해주는 일이다. 이 단계에서는 P 개수를 최적화해도 응답 시간의 대부분이 네트워크와 디스크에 묻혀 있으므로 체감 효과가 거의 없다.

GMP 구조가 진짜로 중요해지는 때는 CPU-bound goroutine이 동시에 수백 개 이상 도는 경우다. 이미지 처리, 암호 연산, 시뮬레이션 같은 작업이 그렇다. 또는 cgo 호출이나 파일 I/O가 많아 스레드 수가 비정상적으로 늘어날 때도 P와 M의 분리를 알아야 원인을 찾을 수 있다. 이럴 때는 `GODEBUG=schedtrace`와 CPU profile을 함께 보며 P 개수, 로컬 런큐 길이, thread 수를 추적한다.

로컬 런큐 크기 256이나 runnext의 존재는 성급하게 코드에서 의존할 필요가 없는 구현 세부 사항이다. 일반 애플리케이션 코드는 goroutine을 만들고 채널로 통신하는 수준을 넘어 스케줄러를 직접 조작할 이유가 없다. 성능이 문제가 될 때 측정을 먼저 하고, 그 측정치가 P 부족이나 스레드 증가를 가리킬 때만 GOMAXPROCS나 실행 환경을 손대는 것이 맞다.

컨테이너 환경에서는 한 가지 예외가 있다. Go 1.24 이전에는 GOMAXPROCS가 컨테이너 CPU limit을 인식하지 못하고 호스트 머신의 코어 수를 그대로 쓰는 문제가 있었다. 최신 Go는 CPU quota를 인식하지만, 명시적으로 GOMAXPROCS를 설정하는 배포가 여전히 많다. 이런 환경에서는 GOMAXPROCS를 limit에 맞추는 것이 GMP 모델을 이해하고 적용하는 가장 실용적인 사례다.

## 더 파보기

- runtime/proc.go 스케줄러 코어: https://go.dev/src/runtime/proc.go
- runtime/runtime2.go G/M/P struct 정의: https://go.dev/src/runtime/runtime2.go
- Go 스케줄러 설계 문서: https://go.dev/s/go11sched
- 런타임 환경변수 공식 문서: https://pkg.go.dev/runtime#hdr-Environment_Variables
- Go issue 트래커의 비동기 preemption 논의: https://github.com/golang/go/issues/24543