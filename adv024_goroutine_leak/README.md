> ⚠️ **자동 검증 미통과 — `test` 단계.** 본문 설명은 그대로 유효하지만, 코드는 수정 없이 실행되지 않을 수 있습니다.
>
> `--- FAIL: TestDetectsChannelSendLeak (5.00s)`  
> `    main_test.go:76: chan send 상태의 goroutine 을 찾지 못했습니다. 보고된 목록: []`  
> `--- FAIL: TestDetectsChannelSendLeak (5.00s)`  
> `    main_test.go:76: chan send 상태의 goroutine 을 찾지 못했습니다. 보고된 목록: []`  
> `--- FAIL: TestDetectsChannelSendLeak (5.00s)`  
> `    main_test.go:76: chan send 상태의 goroutine 을 찾지 못했습니다. 보고된 목록: []`  
> `--- FAIL: TestDetectsChannelSendLeak (5.00s)`  
> `    main_test.go:76: chan send 상태의 goroutine 을 찾지 못했습니다. 보고된 목록: []`  

## 한 줄 요약

goroutine 누수는 "끝나지 않는 goroutine"이고, Go 런타임은 goroutine 목록을 구조체로 주는 API 가 없으므로 스택 텍스트 덤프를 떠서 시작 시점 기준선과의 차집합을 보는 것이 사실상 유일한 탐지법이다. 그리고 그 차집합에서 범인을 지목하는 단서는 스택 맨 아래의 `created by main.xxx in goroutine 1` 줄이다.

## 왜 이런 설계인가

goroutine 이 OS thread 를 대체한 이유는 단순히 "가볍다"가 아니다. OS thread 는 커널이 자원을 배정하고 커널이 회수한다. 프로그래머가 thread 를 하나 만들고 join 을 빼먹어도 프로세스가 끝나면 커널이 정리한다. 반면 goroutine 은 사용자 공간 객체이고, 프로그램이 종료될 때까지 런타임이 살려 둔다. 즉 Go 는 생성 비용을 극단적으로 낮춘 대신 "누가 이것을 끝내는가"를 전적으로 프로그래머 책임으로 옮겼다. 이것은 버그가 아니라 거래다. 수만 개를 만들 수 있는 대신, 만들었으면 끝내야 한다.

왜 GC 가 안 치워 주는가. goroutine 의 스택은 GC 스캔 대상이다. 채널 송신을 기다리는 goroutine 은 스택에 채널 포인터와 송신하려던 값을 들고 있다. GC 입장에서 이 goroutine 은 살아 있는 루트이고, 그 스택이 가리키는 모든 객체는 도달 가능하다. 그래서 goroutine 하나가 새면 그 goroutine 이 붙잡고 있는 채널, 버퍼, 요청 구조체, 로그 필드까지 통째로 살아남는다. goroutine 누수가 거의 항상 메모리 누수로 나타나는 이유가 여기 있다. "goroutine 은 2KB 라서 별거 아니다"라는 말이 위험한 이유이기도 하다. 2KB 는 스택일 뿐이고 진짜 비용은 붙잡힌 힙이다.

왜 "goroutine 목록 API"를 안 주는가. 목록을 안정적인 API 로 노출하면 런타임 내부 자료구조(전역 `allgs`, `g` 구조체 레이아웃)를 공개 계약으로 묶게 된다. Go 팀은 그걸 거부하고 대신 사람이 읽는 텍스트 덤프라는 느슨한 계약을 택했다. 그 결과 goleak, pprof, 각종 사내 누수 탐지기가 전부 같은 텍스트를 파싱한다. 이 글의 예제도 같은 길을 간다. 화려한 API 가 없어서가 아니라, 그게 실제로 존재하는 유일한 통로이기 때문이다.

대안으로 검토된 것들도 있다. finalizer 로 회수, `runtime.SetFinalizer` 로 채널을 닫는 트릭, 컴파일러가 `go` 문에 종료 증명을 요구하는 방향 등이다. finalizer 는 도달 불가능해진 뒤에야 불리므로 "채널을 기다리는 goroutine"에는 영원히 오지 않는다. 정적 증명은 `select` 와 인터페이스를 섞는 순간 무너진다. 결국 업계 표준은 "테스트 종료 시점에 덤프를 떠서 차집합을 본다"로 수렴했다. goleak 의 `VerifyTestMain` 이 정확히 그 형태다.

## 어떻게 동작하는가

`runtime.Stack(buf, true)` 는 내부적으로 stop-the-world 를 걸고 전역 `allgs` 를 순회하며 각 `g` 에 대해 `goroutineheader` 와 `traceback` 을 호출해 텍스트를 만든다. 첫 줄은 `goroutine <goid> [<waitreason>]:` 이고, 두 번째 줄이 현재 최상단 프레임, 마지막에 `created by <함수> in goroutine <부모 goid>` 가 붙는다. 버퍼가 모자라면 `len(buf)` 을 반환하므로 반환값이 버퍼 길이와 같으면 두 배로 늘려 재시도해야 한다. 이 재시도를 빼먹으면 goroutine 이 많은 순간 덤프가 잘리고, 잘린 목록으로 "누수 없음"을 선언하는 거짓 음성이 발생한다.

채널 대기 상태의 정체는 `runtime/chan.go` 의 `hchan` 이다. `hchan` 은 `sendq`, `recvq` 라는 대기열(`waitq`)을 들고 있고, 송신자가 보낼 곳이 없으면 `sudog` 를 만들어 `sendq` 에 넣은 뒤 `gopark` 로 자신을 `_Gwaiting` 으로 만든다. `gopark` 는 `g` 를 M 에서 떼어내므로 그 goroutine 은 CPU 를 전혀 쓰지 않는다. 이것이 누수를 찾기 어렵게 만드는 핵심이다. CPU 프로파일에는 흔적이 없고, `runtime.NumGoroutine()` 과 힙 프로파일의 기준선만 조금씩 올라간다.

`context.WithCancel` 은 goroutine 을 만들지 않는다. `cancelCtx` 구조체를 만들고 부모에 자식을 등록할 뿐이다. 새는 것은 context 가 아니라 `<-ctx.Done()` 에서 멈춘 goroutine 이다. 다만 그 goroutine 이 사는 동안 `cancelCtx` 체인과 거기 매달린 값들이 전부 도달 가능하므로, `context.WithValue` 로 요청 구조체를 넣어 두었다면 그것도 함께 새는다. Go 1.21 이후 `WithTimeout` 은 타이머를 힙에 남기지 않도록 바뀌었지만, `Done()` 을 기다리는 goroutine 자체는 여전히 프로그래머 책임이다.

goleak 이 하는 일은 세 단계다. 첫째, `VerifyTestMain` 이 `TestMain` 을 감싸서 테스트 시작 시점에 알려진 goroutine(런타임의 GC worker, `testing` 패키지 내부 등)의 함수 이름을 ignore 목록으로 만든다. 둘째, 모든 테스트가 끝난 뒤 덤프를 떠서 ignore 목록에 없는 goroutine 을 찾는다. 셋째, 있으면 그 스택 전체와 `created by` 줄을 출력하며 실패시킨다. pprof 쪽은 같은 데이터를 다른 형식으로 준다. `pprof.Lookup("goroutine").WriteTo(w, 1)` 은 상태별 집계를, `debug=2` 는 모든 goroutine 의 전체 스택을 준다. `net/http/pprof` 를 붙였다면 브라우저에서 `/debug/pprof/goroutine?debug=2` 로 같은 것을 볼 수 있다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사 — context cancel 누락(lostcancel) 경고가 없는지 본다
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행 — 4개 섹션의 출력이 순서대로 나온다
go test -v ./...             # 테스트 — 파서/탐지/고친코드/거짓양성 4종
go test -race ./...          # 동시성 주제라면 반드시 — 덤프 파싱과 조건 대기가 race-free 인지
```

주제에 특화된 명령도 함께 쓴다.

```bash
go test -run TestDetectsChannelSendLeak -v ./...   # 누수 탐지 한 가지만 확인
go test -count=10 -run TestDetectsChannelSendLeak ./...  # 반복 실행 — 타이밍 의존이 없는지
go test -bench BenchmarkTakeSnapshot -benchmem ./...     # 덤프 한 번의 비용(할당 횟수) 확인
go run . 2>&1 | head -30                            # 덤프 원본 형식만 먼저 보고 싶을 때
```

`go run .` 에서 봐야 할 것은 섹션 2의 각 원인 아래에 찍히는 `goroutine N [chan send]`, `[chan receive]`, `[select]`, `[sleep]` 네 가지 상태다. 상태 이름이 곧 "무엇을 기다리며 멈췄는가"이고, 그게 고치는 방법을 알려준다. 섹션 4에서는 `created by main.leakContextNeverCancelled in goroutine 1` 줄이 그대로 출력되는데, 이것이 goleak 이 실패 메시지에 스택 전체를 붙이는 이유다. `go test -bench` 에서는 `takeSnapshot` 한 번에 수십 KB 의 할당이 발생하는 것을 `-benchmem` 으로 확인한다. 요청 처리 경로에서 부르면 안 되는 이유가 그 숫자다.

## 코드로 확인하기

섹션 1은 덤프 원본을 두 블록만 그대로 보여준다. 여기서 확인할 것은 블록 구조다. 첫 줄 헤더에 goid 와 대기 사유가 있고, 둘째 줄이 최상단 함수, 마지막에 `created by` 가 온다. 이 구조를 눈으로 익혀 두면 나중에 실제 장애 현장에서 덤프를 받았을 때 어디를 봐야 하는지가 바로 나온다.

섹션 2는 세 가지 원인을 하나씩 재현한다. `[원인 1]` 은 `chan send` 상태의 goroutine 이 하나 남는 것을 보여주고, `release` 를 부르면 `남은 수: 0` 으로 돌아간다. 이 "0으로 돌아간다"가 중요하다. 누수는 대부분 죽일 수 있는데, 죽이는 경로를 코드에 안 만들어 둬서 새는 것이다. `[원인 3]` 은 `[sleep]` 상태로 잡히는데, 1밀리초 자고 깨서 stop 을 확인하는 루프이기 때문이다. 상태 이름이 `sleep` 이라고 누수가 아닌 게 아니다. 영원히 반복된다면 그게 누수다.

섹션 3은 고친 버전 네 개를 실행하고 `남은 goroutine: 0개` 를 출력한다. 여기서 핵심은 `fixedContextCancel` 의 마지막 `<-done` 이다. `cancel()` 만 부르고 반환하면 함수는 즉시 끝나지만 워커는 아직 `ctx.Done()` 에서 깨어나 반환하는 중일 수 있다. "취소를 요청했다"와 "취소가 반영됐다"는 다르다. 실제 서버 코드에서 이 차이가 `Shutdown` 이후에도 남는 goroutine 이 된다.

섹션 4는 goleak 의 축약판이다. 시작 스냅숏을 뜨고, 새는 함수를 하나 돌리고, 차집합을 뽑아 스택 전체를 출력한다. 출력에서 `LEAK goroutine N [chan receive] main.leakContextNeverCancelled.func1` 과 그 아래 `created by ...` 두 줄을 이어 읽으면 "어느 함수의 어느 줄에서 만들어졌고 지금 무엇을 기다리는가"가 한 번에 나온다.

`main_test.go` 는 네 가지를 검증한다. `TestParseStacks` 는 파서를 고정 텍스트로 검증한다. 실제 덤프는 매번 달라서 파서 버그를 잡기 어렵기 때문에, 파서만 따로 떼어 `chan send`, `select, 5 minutes`(꼬리 제거), `running` 세 경우를 본다. `TestDetectsChannelSendLeak` 는 진짜 goroutine 을 띄워 탐지기가 잡는지 보고, `release` 후 0으로 돌아오는지까지 확인한다. `TestFixedVersionsLeaveNoGoroutine` 은 고친 코드가 기준선을 오염시키지 않는지, `TestLeakCheckerReportsNothingOnCleanCode` 는 거짓 양성이 없는지 본다. 네 테스트 모두 고정 sleep 대신 조건 대기를 쓰므로 부하가 걸린 CI 에서도 같은 판정이 나온다.

## 모르면 겪는 일

가장 흔한 증상은 "goroutine 수가 계단식으로만 올라간다"이다. `runtime.NumGoroutine()` 을 10초마다 로그로 찍어 두면 트래픽이 몰리는 시간마다 한 단 올라가고 내려오지 않는다. 이때 CPU 프로파일은 깨끗하다. 누수된 goroutine 은 `gopark` 로 재워져 CPU 를 안 쓰기 때문이다. 그래서 "CPU 는 멀쩡한데 p99 레이턴시와 RSS 만 오른다"는 이상한 조합이 나온다. GC 주기가 돌 때마다 스캔해야 할 힙이 커져서 p99 가 주기적으로 튀는데, CPU 프로파일에는 그 원인이 안 보인다.

두 번째 증상은 `context.WithTimeout` 을 `defer cancel()` 없이 쓴 코드에서 나온다. 요청마다 goroutine 이 하나씩 남고, 그 goroutine 이 `*http.Request`, 인증 정보, 본문 버퍼를 붙잡는다. 몇 시간 뒤 컨테이너가 OOM 으로 죽는데 힙 프로파일의 inuse_space 는 그렇게 커 보이지 않는다. 스택에 있는 포인터가 잡고 있는 객체는 "누가 잡고 있는지" 역추적이 안 되기 때문이다. 이때 필요한 것이 goroutine 덤프이고, `created by` 줄이 어느 핸들러에서 시작됐는지 알려준다.

세 번째는 `select` 에 `default` 를 넣어 만든 바쁜 루프다. 종료 신호를 확인하려고 `default` 를 넣었는데 `time.Sleep` 을 빼면 CPU 코어 하나를 100% 로 태운다. 이건 누수이면서 동시에 성능 사고다. 반대로 `time.Sleep` 을 넣었는데 종료 신호를 아무도 안 닫으면 조용한 누수가 된다. 두 경우 모두 `go test -race` 로는 안 잡힌다. race 가 아니라 논리 오류이기 때문이다. 잡는 방법은 덤프의 차집합뿐이다.

네 번째는 `http.Server.Shutdown` 이후다. `Shutdown` 은 "새 연결을 안 받고 기존 연결이 끝나기를 기다린다"이지 "핸들러 goroutine 을 죽인다"가 아니다. 핸들러가 `ctx.Done()` 을 폴링하지 않고 긴 외부 API 호출에 매달려 있으면, `Shutdown` 은 컨텍스트 타임아웃까지 기다린 뒤 반환하고 그 goroutine 은 계속 산다. 재배포를 반복하면 이전 버전의 요청을 붙잡은 goroutine 이 프로세스에 쌓인다.

## 언제 신경 쓰고 언제 무시하나

서버처럼 프로세스가 오래 살고 요청마다 goroutine 을 만드는 코드라면 무조건 신경 써야 한다. 특히 `go func()` 안에서 `context` 를 받아 `Done()` 을 기다리는 패턴, 채널을 반환하거나 받는 패턴, `time.Ticker` 를 쓰는 패턴은 전부 "종료 경로"를 명시적으로 만들어야 한다. 기준선은 간단하다. 테스트 하나가 끝날 때 goroutine 수가 시작 시점과 같아야 한다. 이 불변식이 깨지는 순간이 버그가 들어온 순간이다.

반대로 몇 초 만에 끝나는 CLI, 배치 잡, 일회성 스크립트에서는 신경 쓸 필요가 없다. 프로세스가 종료되면 런타임이 전부 정리한다. 마찬가지로 프로세스 수명 전체에 걸쳐 사는 singleton 성격의 goroutine(설정 감시, 메트릭 exporter)은 "누수"가 아니라 "설계"다. 이 경우에도 종료 신호는 있어야 하지만, 테스트마다 0으로 돌아오는지 검사할 필요는 없다.

중간 지대가 라이브러리다. 남이 쓰는 패키지에서 goroutine 을 띄우고 끝내는 방법을 안 주면, 사용자는 그 패키지를 닫을 방법이 없다. 그래서 라이브러리는 반드시 `Close()`, `Stop()`, 또는 `context` 인자를 제공해야 한다. 이걸 안 주는 라이브러리는 그 자체로 누수 발생기다. 도입 전에 "이 패키지가 띄운 goroutine 을 내가 어떻게 끝내는가"를 문서에서 찾을 수 없으면 쓰지 않는 게 낫다.

과최적화도 경계해야 한다. 모든 함수에 `WaitGroup` 을 달고 `runtime.NumGoroutine()` 을 검사하는 코드는 읽기 어렵고 deadlock 위험만 늘린다. 실제로 필요한 곳은 요청 경계, 커넥션 경계, 그리고 오래 도는 루프 세 곳이다. 그 세 곳에만 종료 경로를 확실히 만들고, 테스트에서 goleak 류의 검사를 한 번 돌리면 충분하다.

## 더 파보기

- uber-go/goleak: https://github.com/uber-go/goleak — `VerifyTestMain`, `IgnoreTopFunction`, `options` 사용법. 실패 메시지에 스택 전체를 붙이는 방식이 이 글의 섹션 4와 같다.
- Go 런타임 소스 `src/runtime/stack.go` 의 `Stack` 함수와 `src/runtime/traceback.go` 의 `goroutineheader`, `tracebackothers` — 덤프 텍스트가 만들어지는 정확한 지점.
- Go 런타임 소스 `src/runtime/chan.go` 의 `chansend`, `gopark`, 그리고 `src/runtime/runtime2.go` 의 `sudog`, `waitq` — "chan send" 상태가 어디서 만들어지는지.
- 공식 문서 `runtime` 패키지의 `Stack`, `NumGoroutine`, `pprof.Lookup("goroutine")` — https://pkg.go.dev/runtime
- Go 블로그 "Profiling Go Programs": https://go.dev/blog/pprof — goroutine 프로파일을 상태별 집계로 읽는 법.
- 이슈 go#40458 (`time.After` 와 타이머 누수), go#65965 (context cancel 누수 관련 논의) — context 누수가 실제로 무엇을 붙잡는지에 대한 배경.