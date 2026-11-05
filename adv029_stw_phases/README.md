> ⚠️ **자동 검증 미통과 — `vet` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `vet: ./main.go:302:9: invalid operation: n > len(after.Pause) (mismatched types int64 and int)`  

## 한 줄 요약

Go GC 의 stop-the-world 구간은 사이클당 두 번, 대개 수십에서 수백 마이크로초이며, 여러분이 프로덕션에서 보는 p99 지연 스파이크의 대부분은 그 정지 구간이 아니라 mark assist 와 스케줄링 지연에서 온다. 이 자료는 그 두 가지를 분리해서 수치로 보여준다.

## 왜 이런 설계인가

2015년 이전의 Go GC 는 전면 stop-the-world 방식의 mark-sweep 이었다. 힙 전체를 멈춰 놓고 살아있는 객체를 표시한 뒤 쓰레기를 쓸어 담았다. 정지 시간이 힙 크기에 거의 비례했기 때문에, 힙이 수 GB 로 커지면 한 번에 수백 밀리초에서 길게는 1초 가까이 애플리케이션이 얼어붙었다. 요청을 처리하는 서버 입장에서는 이것이 곧 p99 폭발이었고, "Go 는 GC 때문에 프로덕션에서 못 쓴다"는 평판의 실체가 바로 이것이었다.

대안은 크게 셋이었다. 첫째, 세대별 GC. Java 의 CMS 나 G1 처럼 약한 세대 가설(대부분의 객체는 금방 죽는다)을 이용해 매번 전체 힙을 스캔하지 않는 방식이다. 이론적으로 매력적이지만 객체를 다른 세대로 옮기거나 압축(compaction)하는 순간 모든 포인터를 갱신해야 하고, Go 는 인터페이스 값과 슬라이스 헤더가 런타임 내부에 흩어져 있어 객체 이동 비용이 매우 비싸다. 둘째, 참조 카운팅. Swift 나 CPython 처럼 참조가 끊기는 즉시 회수하므로 긴 정지가 없다. 그러나 순환 참조를 따로 처리해야 하고, 모든 포인터 대입마다 카운터를 원자적으로 갱신해야 해서 멀티코어에서 캐시 라인 경합이 심각해진다. Go 는 채널과 인터페이스로 인한 포인터 대입이 매우 잦아 이 비용을 감당할 수 없다고 판단했다. 셋째, 세대별도 참조 카운팅도 아닌, 동시 삼색 마킹(concurrent tri-color mark-sweep).

Go 가 택한 것은 셋째다. 핵심 아이디어는 "마킹을 애플리케이션과 나란히 돌리면 정지 시간이 힙 크기와 무관해진다"는 것이다. 이를 위해 힙을 흰색(아직 안 봄), 회색(봤지만 자식은 안 봄), 검은색(자식까지 다 봄)으로 나누고, 애플리케이션이 검은 객체에서 흰 객체로 가는 포인터를 만들지 못하게 write barrier 로 막는다. Go 는 여기서 Yuasa 삭제 배리어와 Dijkstra 삽입 배리어를 섞은 하이브리드 배리어를 쓴다. 사이클 초반에는 삭제 배리어로 스택을 나중에 다시 스캔하는 비용을 줄이고, 후반에는 삽입 배리어로 정확성을 보장하는 식이다.

그 대가는 정직하게 지불되었다. 첫째, 마킹이 도는 동안에도 애플리케이션이 계속 메모리를 할당하므로 "할당 속도가 마킹 속도보다 빠르면 힙이 무한히 커진다"는 위험이 생겼다. 이걸 막는 장치가 mark assist 다. 할당을 트리거한 goroutine 을 징발해서, 자기가 할당한 만큼의 마킹 작업을 대신 시킨다. 둘째, 정지 시간 자체는 사라지지 않았다. 루트 집합(전역 변수, 각 goroutine 의 스택)의 스냅샷을 떠야 하고, 사이클이 끝날 때 회색 큐가 비었는지 확인해야 한다. 이 두 지점이 남은 STW 이고, 힙 크기가 아니라 스택 개수와 P 개수에 비례한다. 셋째, write barrier 는 모든 포인터 대입에 비용을 붙인다. 그래서 Go 는 이 비용을 상쇄하려고 할당 자체를 빠르게 만드는 데(mcache, tiny allocator) enormous 한 노력을 기울였다.

결과적으로 Go 의 GC 는 "지연을 없앤" 것이 아니라 "지연을 재분배한" 것이다. 큰 덩어리의 정지를 없애는 대신, 작은 조각들을 할당 지점과 스케줄링 경계에 흩뿌렸다. 이 자료의 주제는 바로 그 재분배된 조각들이 어디에 있는지 아는 것이다. 그것을 모르면 "GC 를 껐더니 빨라졌다" 같은 잘못된 결론에 도달하게 된다.

## 어떻게 동작하는가

사이클 하나는 `runtime/mgc.go` 의 `gcStart` 에서 시작한다. 가장 먼저 하는 일이 sweep termination 이다. 이전 사이클이 남긴 스위핑이 아직 끝나지 않았으면 `mheap_.sweepdone` 이 1 이 될 때까지 기다려야 한다. 이 대기가 첫 번째 STW 이고, gctrace 에서 `a` 로 찍히는 값이다. 이어서 `gcBgMarkStartWorkers` 가 P 개수만큼 백그라운드 mark worker goroutine 을 깨우고, 루트 스캔을 예약한 뒤 월드를 재개한다. 여기서부터가 동시 구간이다.

동시 구간에서는 각 P 가 자기 몫의 회색 객체를 꺼내 자식을 회색으로 칠한다. 작업 큐는 `gcWork` 라는 per-P 구조체에 있고, 가득 차면 전역 `work.full` 리스트로 넘긴다. 다른 P 의 큐가 비면 `workSteal` 로 훔쳐 온다. 루트 스캔도 이 시기에 한다. 각 goroutine 은 `gcDrain` 에 들어가기 전에 자기 스택을 스캔하는데, 이때 스택이 실행 중에 바뀌면 안 되므로 `suspendG` 로 잠깐 멈추거나 `asyncPreempt` 로 안전 지점을 만든다. 이 부분이 사람들이 흔히 "STW" 로 오해하는 구간인데, 정확히는 개별 goroutine 이 아주 짧게 멈추는 것이고 월드 전체가 멈추는 것은 아니다.

마킹이 거의 끝나면 `gcMarkTermination` 이 두 번째 STW 를 건다. 여기서는 모든 P 가 마킹을 완전히 비웠는지 확인하고, 남은 `gcWork` 를 플러시하고, 통계를 갱신하고, `gcController.endCycle` 로 다음 사이클의 목표 힙 크기를 계산한다. gctrace 에서 `c` 로 찍히는 값이 이것이다. 이 시점 이후로 흰 객체는 없어야 하므로, 남아 있으면 즉시 throw 된다.

mark assist 는 `runtime/malloc.go` 의 `gcAssistAlloc` 에 있다. 할당 경로에서 `gcBlackenEnabled` 가 켜져 있으면 호출된다. 이 함수는 `gcController.assistWorkPerByte` 를 읽어 "지금까지의 할당량 대비 내가 해야 할 마킹 작업량"을 계산하고, 부족하면 `gcDrainN` 으로 실제 스캔을 수행한다. `assistWorkPerByte` 는 사이클 중 `gcController.revise` 가 주기적으로 갱신하는데, 남은 마킹 작업을 남은 힙 여유로 나눈 값이다. 즉 힙 목표에 가까워질수록, 그리고 백그라운드 marker 가 느릴수록 이 값이 커지고, 애플리케이션 goroutine 이 더 많이 징발된다. `GOMAXPROCS=1` 이면 백그라운드 marker 가 쓸 P 가 없어서 이 값이 폭발한다. 시연 2 의 표에서 GOMAXPROCS 를 1 과 4 로 바꿨을 때 assist 비중이 달라지는 것이 이 메커니즘의 직접적인 관찰이다.

## 돌려보기

```bash
go vet ./...                              # 정적 검사. 지표 이름 오타나 타입 불일치를 여기서 잡는다
go build -o /dev/null ./...               # 컴파일 확인
go run .                                  # 세 가지 시연을 순서대로 출력
GODEBUG=gctrace=1 go run .                # 사이클마다 gcN(a+...c ms) 형태로 STW 두 조각이 찍힌다
GODEBUG=gctrace=1,schedtrace=1000 go run . # 위에 더해 1초마다 P 별 실행 큐 상태가 찍힌다
go test -v ./...                          # 테스트 5개 + 벤치마크 비활성 실행
go test -race ./...                       # 동시성 주제이므로 반드시. 전역 sink 접근에 race 가 없는지 확인
go test -run=XXX -bench=. -benchmem ./... # summarizePauses 집계 비용 측정
```

`go run .` 에서는 먼저 시연 1 의 "사이클당 실제 GC 사이클 개수"와 "누적 STW 시간 증가"를 본다. 강제 GC 5회에 STW 합계가 1밀리초를 넘지 않는다면 정상이다. 시연 2 에서는 GOMAXPROCS=1 과 4 의 assist 백분율 차이를 본다. 시연 3 에서는 p99 열과 STW 합계 열의 자릿수가 몇 개나 다른지 본다.

`GODEBUG=gctrace=1` 로 돌리면 각 사이클마다 `gc 5 @0.123s 4%: 0.012+0.5+0.008 ms clock` 같은 줄이 나온다. 여기서 `+` 로 구분된 세 숫자가 각각 sweep termination STW, 동시 마킹 구간, mark termination STW 다. 첫 번째와 세 번째만이 월드가 멈춘 구간이고, 가운데 숫자가 아무리 커도 그것은 벽시계 시간이지 정지 시간이 아니다.

## 코드로 확인하기

시연 1 은 `runtime.GC()` 를 5번 부르고 그 사이의 `debug.GCStats.PauseTotal` 차이를 찍는다. `PauseTotal` 은 누적 나노초이므로 차이가 곧 이번 실행에서 발생한 STW 합계다. `Pause` 슬라이스는 최신이 앞쪽에 오도록 채워지며, 크기는 런타임 내부 링 버퍼와 같은 256이다. 출력에서 `#0`, `#1` 이 대개 수십 마이크로초 수준인 것을 볼 수 있다. 이어서 `/sched/pauses/stopping/gc:seconds` 히스토그램을 찍는데, 이것은 프로세스 시작 이후 누적이므로 모든 값이 `0` 근처의 첫 버킷에 몰려 있으면 정상이다. 만약 중간 버킷에 카운트가 있다면 그만큼 긴 정지가 있었다는 뜻이다.

시연 2 는 같은 양의 쓰레기(1500회 × 32KB)를 GOMAXPROCS=1 과 4 에서 각각 치우고, `/cpu/classes/gc/mark/assist:cpu-seconds`, `/cpu/classes/gc/mark/dedicated:cpu-seconds`, `/cpu/classes/gc/pause:cpu-seconds` 의 증가분을 `/cpu/classes/gc/total:cpu-seconds` 로 나눠 백분율로 보여준다. 여기서 `pause` 백분율이 소수점 아래로 떨어지는데, 이것이 "STW 는 전체 GC 비용의 극히 일부"라는 주장의 근거다. 반면 `assist` 는 GOMAXPROCS=1 에서 크게 올라간다. 이 숫자는 "애플리케이션 goroutine 이 마킹에 끌려간 CPU 시간"이므로, 곧 여러분의 요청 처리 시간에서 빠져나간 몫이다.

시연 3 은 가장 중요한 부분이다. 세 가지 설정에서 32KB 할당 2000회의 호출 구간 지연을 재고 p50, p99, max 를 낸다. GC 를 끈 기준선과 비교해 `(p99 - 기준 p99) × 2000` 을 계산하면 "체감 지연 총량"이 나오고, 같은 구간의 STW 합계는 `PauseTotal` 차이로 나온다. 두 값의 자릿수가 두세 개 차이 나는 것을 볼 수 있다. 예를 들어 꼬리 지연 총량이 40밀리초인데 STW 합계는 300마이크로초라면, 정지 구간은 꼬리의 1퍼센트도 설명하지 못한다. 나머지는 mark assist 와 그것이 유발한 스케줄링 지연이다.

`main_test.go` 는 다섯 가지를 검증한다. 집계 함수가 입력을 변형하지 않는지, 강제 GC 가 STW 기록을 남기고 그 합이 누적치를 넘지 않는지, 지연 도구가 요청한 개수만큼 표본을 모으고 분위수 순서가 맞는지, 부하 생성기가 실제로 바이트를 할당하는지, 그리고 `/cpu/classes/gc/*` 지표에서 pause 가 total 을 넘지 않는지. 어느 것도 절대적인 시간 값을 단정하지 않으므로 CI 부하와 런타임 버전 변화에 견딘다.

## 모르면 겪는 일

첫째, p99 가 주기적으로 튀는데 CPU 프로파일에는 아무것도 안 보이는 상황. 원인이 mark assist 라면 프로파일에서 `runtime.gcAssistAlloc` 과 `runtime.scanobject` 가 상위에 나타나야 정상인데, `go tool pprof` 의 기본 샘플링은 짧은 스파이크를 놓치기 쉽다. 증상은 "특정 시간대에만 p99 가 30밀리초씩 튀고, 그 시간대에 트래픽은 평소와 같다"는 형태다. GOGC 를 기본값에서 200이나 400으로 올리면 사이클 빈도가 줄어 스파이크도 줄어드는데, 이때 "메모리를 늘렸더니 빨라졌다"는 잘못된 결론을 내리기 쉽다. 실제로는 assist 빈도가 줄어든 것이다.

둘째, 컨테이너에 CPU limit 을 걸었더니 갑자기 전부 느려지는 상황. Go 1.24 이전에는 GOMAXPROCS 가 cgroup limit 을 무시하고 호스트 코어 수를 따랐다. limit 이 2코어인데 GOMAXPROCS 가 32면, 32개의 P 가 있고 그중 30개는 스로틀링으로 언제 멈출지 모른다. mark termination STW 는 모든 P 가 정지 지점에 도달할 때까지 기다리므로, 스로틀링에 걸린 P 하나가 STW 를 수십 밀리초로 늘린다. 증상은 "STW 히스토그램에 유독 긴 버킷이 있고, 그것이 스로틀 기간과 일치한다"는 형태다. 해결은 `GOMAXPROCS` 를 limit 에 맞추거나 `automaxprocs` 를 쓰는 것이다.

셋째, 힙이 아주 작은 서비스에서 오히려 지연이 심한 상황. 상주 집합이 4MB 인데 초당 100MB 를 할당하면 GOGC=100 기준으로 초당 수십 번 사이클이 돈다. 사이클마다 assist 가 붙고, 백그라운드 marker 를 깨우는 비용도 매번 든다. 증상은 "메모리는 10MB 도 안 쓰는데 GC 사이클 수가 초당 수백"이고, `gctrace` 출력이 화면을 뒤덮는다. 이때는 GOGC 를 올리는 것보다 `debug.SetMemoryLimit` 로 상한을 정하고 GOGC 를 아주 크게 두는 편이 낫다.

넷째, STW 를 없애려고 `debug.SetGCPercent(-1)` 로 GC 를 끄는 상황. 처음 몇 분은 빨라 보이지만 힙이 선형으로 늘다가 OOM killer 를 만난다. 특히 Kubernetes 에서는 파드가 조용히 재시작되고, 재시작 직후에는 다시 빨라서 원인을 찾기 어렵다. GC 를 끄는 것은 벤치마크에서 기준선을 잡을 때만 쓰는 도구다.

## 언제 신경 쓰고 언제 무시하나

힙이 수십 MB 이하이고 초당 할당량이 수 MB 수준이면 이 모든 논의는 과최적화다. 이 규모에서는 STW 가 수십 마이크로초, assist 는 거의 발생하지 않으며, 네트워크 왕복 한 번이 그보다 훨씬 크다. p99 를 논할 때 GC 를 의심하기 전에 먼저 봐야 할 것은 DB 커넥션 풀 대기, 락 경합, DNS 조회다.

신경 써야 하는 조건은 세 가지다. 첫째, 상주 집합이 수백 MB 이상이면서 초당 할당량이 상주 집합의 10퍼센트를 넘는 경우. 이때 사이클이 초당 여러 번 돌고 assist 가 상시화된다. 둘째, p99 목표가 한 자릿수 밀리초인 저지연 서비스. 이 경우 수백 마이크로초 STW 도 관찰 가능하다. 셋째, 컨테이너 CPU limit 이 코어 수보다 훨씬 작은 환경. 이 경우는 GC 튜닝보다 GOMAXPROCS 설정이 먼저다.

튜닝 순서도 정해져 있다. 먼저 `GODEBUG=gctrace=1` 로 사이클 빈도와 STW 크기를 확인한다. STW 자체가 문제라면 (드물다) 할 수 있는 것이 거의 없다. 사이클 빈도가 문제라면 GOGC 를 올리거나 `SetMemoryLimit` 을 쓴다. assist 가 문제라면 할당 자체를 줄이는 것이 유일한 근본 해법이다. `sync.Pool` 로 요청별 버퍼를 재사용하고, `bytes.Buffer` 를 미리 키우고, 작은 객체를 구조체로 묶는 것이 GOGC 조정보다 훨씬 효과가 크다. 마지막 수단으로 `GOGC=off` + `SetMemoryLimit` 조합이 있지만, 이는 힙 사용량을 직접 관리하겠다는 선언이므로 상주 집합 프로파일링이 선행되어야 한다.

## 더 파보기

- Go 공식 GC 가이드: https://go.dev/doc/gc-guide — GOGC 와 SetMemoryLimit 의 상호작용, assist 가 언제 발생하는지에 대한 가장 정확한 설명
- `runtime/mgc.go` 소스: https://github.com/golang/go/blob/master/src/runtime/mgc.go — `gcStart`, `gcMarkTermination`, `gcBgMarkWorker` 의 실제 순서
- `runtime/malloc.go` 의 `gcAssistAlloc`: https://github.com/golang/go/blob/master/src/runtime/malloc.go — assist 부채 계산과 상환 로직
- Go 1.5 동시 GC 설계 문서: https://go.dev/s/go14gc — 하이브리드 write barrier 를 왜 그렇게 조합했는지
- Rick Hudson 의 ISMM 기조연설: https://go.dev/blog/ismmkeynote — 2015년 이전에서 지금까지의 설계 변경 역사
- `/sched/pauses/*` 지표 추가 이슈: https://github.com/golang/go/issues/54641 — STW 히스토그램이 런타임 지표로 들어온 배경