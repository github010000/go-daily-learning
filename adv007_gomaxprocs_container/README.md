## 한 줄 요약

Go 런타임이 컨테이너 cgroup limit을 보지 않아 GOMAXPROCS를 호스트 CPU 수로 설정하면 스레드 과다 생성과 스로틀링으로 p99 지연이 튄다. automaxprocs 같은 방식으로 limit을 읽어 GOMAXPROCS를 맞추면 이를 피할 수 있다.

## 왜 이런 설계인가

Go 런타임은 원래 베어메탈 호스트나 VM을 기준으로 설계되었다. 초기 버전부터 GOMAXPROCS의 기본값은 `runtime.NumCPU()`가 반환하는 호스트의 논리 CPU 개수였다. 이 값은 `/proc/cpuinfo`나 `sched_getaffinity` 같은 시스템 콜을 통해 얻으며, 컨테이너처럼 cgroup으로 CPU 사용량을 제한하는 환경은 고려하지 않았다. 그래서 컨테이너 안에서 `runtime.NumCPU()`는 호스트 머신 전체의 코어 수를 그대로 반환한다.

컨테이너가 등장하고 Kubernetes 같은 오케스트레이션이 보편화되면서 이 문제가 드러났다. 예를 들어 호스트가 64코어인데 컨테이너에는 `cpu: "2"` limit이 걸려 있다면, Go 런타임은 GOMAXPROCS를 64로 설정한다. 런타임은 그만큼의 P(프로세서)를 만들고, CPU를 사용하는 goroutine이 많으면 OS 스레드도 그만큼 늘린다. 하지만 커널은 cgroup quota에 따라 이 컨테이너에 2코어 분량의 CPU 시간만 허용한다. 결국 64개의 스레드가 2코어를 나눠 쓰는 심각한 오버서브스크립션이 발생한다.

이 문제를 해결하는 대안은 크게 두 가지였다. 첫째는 Go 런타임 자체가 cgroup을 읽도록 패치하는 것이다. 실제로 이 방향의 논의가 있었지만, 모든 플랫폼과 cgroup 버전(v1, v2)을 지원해야 하고 사용자가 명시적으로 GOMAXPROCS를 설정한 경우를 구분해야 하는 등 복잡도가 높았다. 둘째는 외부 라이브러리나 초기화 코드가 cgroup limit을 읽어 `runtime.GOMAXPROCS`를 직접 설정하는 것이다. Uber의 automaxprocs가 대표적이며, 이 방식은 런타임 수정 없이 사용자가 선택적으로 적용할 수 있어 널리 쓰인다.

Go 공식 런타임은 한동안 이 기능을 포함하지 않았다. Go 1.25부터는 일부 환경에서 cgroup 인식이 개선되고 있다는 소식이 있지만, 여전히 명시적인 GOMAXPROCS 설정이 우선시되고, 컨테이너 환경에서도 호스트 CPU 수를 그대로 쓰는 것이 기본 동작이다. 따라서 이 주제는 Go 개발자가 컨테이너 배포를 할 때 반드시 알아야 할 실전 지식이다.

## 어떻게 동작하는가

Go 런타임의 스케줄러는 GOMAXPROCS 개수만큼 P(Processor)라는 논리 프로세서를 유지한다. 각 P는 한 번에 하나의 goroutine을 실행할 수 있으며, 실제 OS 스레드(M)에 바인딩되어 동작한다. GOMAXPROCS가 크면 P가 많아지고, CPU를 점유하는 goroutine이 많으면 그만큼 많은 OS 스레드가 생성된다. 이는 `runtime.GOMAXPROCS`를 높게 설정하면 더 많은 병렬 처리가 가능하지만, 실제 물리 코어보다 많으면 컨텍스트 스위칭 비용이 급증한다.

컨테이너에서 CPU limit은 cgroup 파일시스템으로 표현된다. cgroup v2에서는 `/sys/fs/cgroup/cpu.max` 파일에 `$MAX $PERIOD` 형식으로 적혀 있다. `MAX`가 `max`이면 무제한이고, 숫자면 마이크로초 단위의 quota를 의미한다. `PERIOD`는 보통 100000(100ms)이다. 이 두 값을 나누면 CPU 개수가 된다. 예를 들어 `200000 100000`이면 2.0 CPU다. cgroup v1에서는 `cpu.cfs_quota_us`와 `cpu.cfs_period_us` 파일이 같은 정보를 담는다. quota가 -1이면 무제한이다.

automaxprocs는 이 cgroup 파일을 읽어 오는 간단한 로직을 수행한다. 라이브러리는 초기화 시점에 `runtime.GOMAXPROCS`를 계산된 값으로 설정한다. `recommendedGOMAXPROCS` 함수가 그 로직을 그대로 구현한 것으로, limit이 0(무제한)이면 호스트 CPU 수를 반환하고, 그 외에는 `math.Ceil(limit)`로 올림한다. 소수점 이하를 올림하는 이유는 1.2 CPU limit이면 1개보다는 2개를 주는 것이 스로틀링을 덜 유발하기 때문이다.

이 코드의 `measureMaxThreads` 함수는 GOMAXPROCS 설정에 따라 실제로 생성되는 OS 스레드 수가 어떻게 달라지는지 관찰한다. `busyLoop` goroutine 100개를 실행하면서 `/proc/self/status`의 `Threads:` 필드를 주기적으로 읽는다. 기본 GOMAXPROCS(호스트 CPU 수)로 실행하면 스레드 수가 크게 증가하는 반면, 권장 값으로 설정하면 현저히 줄어든다. 이 간단한 실험만으로도 컨테이너 limit을 무시했을 때의 오버서브스크립션을 체감할 수 있다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령을 순서대로 실행하세요.

```bash
go vet ./...                 # 정적 검사. 코드에 잘못된 사용이 없는지 확인
go build -o /dev/null ./...  # 컴파일 확인. 실행 파일을 만들 필요 없이 문법 오류만 검사
go run .                     # 시연 실행. cgroup limit 감지 여부와 스레드 수 차이를 출력
go test -v ./...             # 테스트. 파서와 GOMAXPROCS 계산 로직을 검증
go test -race ./...          # 동시성 주제이므로 -race로 데이터 레이스가 없는지 확인
```

`go run .`을 실행하면 다음을 관찰할 수 있습니다.
- 호스트 CPU 수와 현재 GOMAXPROCS가 출력된다.
- cgroup limit이 감지되면 그 값과 권장 GOMAXPROCS가 나온다. 감지되지 않으면 시연용으로 낮은 limit(2 이하)을 사용한다고 안내한다.
- busy-loop goroutine 100개를 1초 동안 실행하면서 최대 스레드 수를 측정한다. 기본 설정과 권장 설정에서의 스레드 수가 크게 차이나는 것을 본다.

만약 실제 컨테이너 안에서 실행한다면 cgroup limit이 자동 감지되어 더 정확한 비교가 된다. 로컬에서는 시연용 limit이 적용되므로 개념을 이해하는 데 충분하다.

`go test -race ./...`는 테스트 코드에 동시성 버그가 없음을 확인한다. main.go의 시연 부분은 의도적으로 goroutine을 많이 띄우지만 race가 없도록 설계되어 있다.

## 코드로 확인하기

`main.go`는 크게 세 부분으로 나뉜다. 첫째, `detectContainerCPULimit`와 관련 파서들이 cgroup v2/v1에서 CPU limit을 읽는다. 둘째, `recommendedGOMAXPROCS`가 limit을 GOMAXPROCS 값으로 변환한다. 셋째, `measureMaxThreads`가 실제로 GOMAXPROCS를 바꿔가며 스레드 수를 측정한다.

출력에서 가장 중요한 부분은 "스레드 수 비교" 섹션이다. 예를 들어 호스트 CPU가 8개이고 시연용 limit이 2라면, 기본 GOMAXPROCS=8에서는 최대 스레드 수가 10 근처까지 올라간다. 반면 권장 GOMAXPROCS=2에서는 4~5개 수준에 머문다. 이 차이는 GOMAXPROCS가 P 개수를 결정하고, 그 P들이 동시에 실행될 OS 스레드를 만들기 때문에 발생한다. 컨테이너 limit이 낮은데 GOMAXPROCS가 높으면 이렇게 불필요한 스레드가 늘어난다.

`main_test.go`는 두 가지를 검증한다. `TestRecommendedGOMAXPROCS`는 limit=0일 때 호스트 CPU 수를 반환하고, 소수점 이하는 올림하는지 확인한다. `TestParseCPUmaxV2`는 cgroup v2의 `cpu.max` 파일 형식을 올바르게 해석하는지 검사한다. 무제한(`max 100000`)과 정상 quota(`200000 100000`), 잘못된 입력에 대한 에러 처리를 모두 테스트한다. 이 테스트는 외부 환경에 의존하지 않고 순수 함수만 검증하므로 CI에서도 안정적으로 통과한다.

## 모르면 겪는 일

컨테이너에서 GOMAXPROCS를 그대로 두면 가장 흔하게 나타나는 증상은 p99 지연이 주기적으로 튀는 것이다. cgroup 스로틀링은 quota를 초과한 CPU 사용을 강제로 멈추는데, 이 멈춤이 100ms period마다 반복된다. 이 때문에 특정 요청의 응답 시간이 갑자기 수백 ms로 늘어나는 패턴이 관찰된다. CPU 사용률 그래프는 컨테이너 안에서 100%를 찍지만, 실제로는 스로틀링으로 인해 처리가 지연되는 것이다.

또 하나의 증상은 `runtime.NumCPU()`가 호스트 전체 코어 수를 반환해서, 워커 풀 크기나 커넥션 풀 크기를 잘못 설정하는 것이다. 예를 들어 64코어 호스트에서 CPU limit이 2인 컨테이너로 배포하면, 개발자는 `runtime.NumCPU()`를 기준으로 워커 goroutine을 64개 띄운다. 하지만 실제 사용 가능한 CPU는 2개뿐이므로 워커들이 서로 CPU를 빼앗으며 컨텍스트 스위칭만 늘어난다. 처리량은 오히려 떨어지고 지연은 늘어난다.

GC에도 악영향이 있다. Go GC는 GOMAXPROCS에 비례해 병렬 마킹 워커를 생성한다. GOMAXPROCS가 과다하면 GC도 실제 CPU보다 많은 스레드를 사용해 mark phase를 실행한다. 이는 GC pause 시간에는 큰 영향이 없을 수 있지만, CPU를 더 많이 점유해 애플리케이션의 지연을 증가시킨다. CPU 프로파일을 떠보면 GC 관련 함수가 생각보다 많은 시간을 차지하는 것처럼 보일 수 있다.

이 문제를 모르고 컨테이너를 운영하면, "컨테이너 limit을 2로 주었는데 왜 이렇게 느리지?"라는 질문에 답하기 어렵다. 모니터링 대시보드에는 CPU 사용률이 높게 나오고, 애플리케이션 로그에는 느린 요청만 찍힌다. 원인은 런타임이 잘못된 가정을 하고 있다는 점인데, 겉으로는 드러나지 않는다.

## 언제 신경 쓰고 언제 무시하나

이 지식은 컨테이너 오케스트레이션 환경에서 CPU limit이 호스트 CPU 수보다 현저히 작을 때 중요해진다. 예를 들어 호스트가 32코어인데 컨테이너 limit이 4 이하라면 반드시 GOMAXPROCS를 조정해야 한다. 반대로 limit이 호스트 코어 수와 같거나 거의 차이나지 않는다면(예: 32코어 호스트에 limit 30) 문제가 미미하다. 그런 경우에는 기본 설정을 그대로 써도 무방하다.

워크로드가 I/O bound라면 이 문제가 덜 두드러진다. I/O 대기 중인 goroutine은 CPU를 거의 쓰지 않으므로 OS 스레드가 많아도 CPU contention이 심하지 않다. 하지만 I/O bound라도 커넥션 풀이나 요청량이 많아지면 결국 CPU를 쓰는 구간이 생기므로 안심할 수 없다. 특히 latency가 중요한 API 서버라면 CPU bound 작업이 조금만 있어도 문제가 될 수 있다.

작은 규모의 애플리케이션이나 개발 환경에서는 이 설정을 무시해도 큰 문제가 없다. 로컬 머신에서는 cgroup limit이 없으므로 기본 동작이 최적이다. 또한 GOMAXPROCS를 너무 낮추면 오히려 병렬성이 떨어져 성능이 나빠질 수 있다. 따라서 컨테이너에 배포할 때만, 그것도 CPU limit이 호스트보다 확실히 작을 때만 신경 쓰면 된다.

automaxprocs 같은 라이브러리는 적용이 매우 간단하다. 보통 `import _ "go.uber.org/automaxprocs"` 한 줄이면 된다. 하지만 라이브러리를 추가하고 싶지 않다면, 위 `main.go`의 `detectContainerCPULimit`과 `recommendedGOMAXPROCS`를 복사해 초기화 함수에서 `runtime.GOMAXPROCS`를 호출하는 것도 방법이다. 다만 직접 구현할 경우 cgroup v1/v2 모두 지원해야 하고, 에러 처리를 꼼꼼히 해야 한다.

## 더 파보기

- [Uber automaxprocs GitHub](https://github.com/uber-go/automaxprocs) - 실제 구현과 사용법. cgroup v1/v2 파싱 코드를 직접 볼 수 있다.
- [Go 런타임의 GOMAXPROCS 문서](https://pkg.go.dev/runtime#GOMAXPROCS) - 공식 문서에서 GOMAXPROCS의 의미와 동작을 확인한다.
- [cgroup v2 공식 문서](https://docs.kernel.org/admin-guide/cgroup-v2.html) - cpu.max 파일의 정확한 의미와 스로틀링 동작을 설명한다.
- [Go 이슈 #33803: runtime: detect and respect cgroup CPU limits](https://github.com/golang/go/issues/33803) - Go 런타임에 cgroup 인식을 넣자는 논의가 진행된 이슈. 설계 결정의 배경을 볼 수 있다.