## 한 줄 요약

GOGC 는 "지금 살아 있는 힙의 몇 배까지 쓰레기를 쌓아 둘지"를 정하는 **비율** 손잡이고, GOMEMLIMIT 은 "runtime 이 추적하는 총메모리가 이 선을 넘지 않게 GC 를 재촉하라"는 **절대값** 손잡이다. 비율은 처리량을 책임지고, 절대값은 생존을 책임진다. 컨테이너 안에서 OOMKilled 를 피하려면 후자가 필요하고, CPU 를 아끼려면 전자를 조정한다. 둘은 대체 관계가 아니라 서로 다른 축에 있는 계층 관계다.

이 글은 "GOGC 를 낮추면 메모리가 줄어든다" 같은 교과서 문장을 다시 쓰지 않는다. 대신 왜 Go 가 처음에 비율만 두었는지, 그 선택이 컨테이너 시대에 어디서 깨지는지, 1.19 의 GOMEMLIMIT 이 왜 hard limit 이 아니라 soft limit 인지, 그리고 그 soft 함 때문에 여러분이 어떤 새로운 실패 모드(GC 폭주, assist 로 인한 지연)를 만나게 되는지를 다룬다.

첨부한 프로그램은 한 process 안에서 `debug.SetGCPercent` 와 `debug.SetMemoryLimit` 을 바꿔 가며 같은 workload 를 네 번 돌리고, `runtime.MemStats` 와 `runtime/metrics` 로 최대 힙, 목표 힙, GC 횟수, GC CPU 비율을 찍는다. 숫자가 왜 그렇게 나오는지 본문에서 하나씩 해석한다.

## 왜 이런 설계인가

1990년대에서 2000년대까지 JVM 계열 런타임의 표준은 "사용자가 최대 힙을 직접 정한다"였다. `-Xmx2g` 처럼 절대값을 준다. 이 방식의 장점은 명확하다. 상한이 확정되므로 용량 계획이 쉽다. 단점은 더 명확하다. 프로그램이 지금 실제로 얼마를 살려 두는지와 무관하게 한 번 정한 값이 계속 적용된다는 것이다. 살아 있는 데이터가 50MB 인 서비스에 4GB 를 주면 GC 는 거의 안 돌지만 놀고 있는 메모리가 낭비되고, 살아 있는 데이터가 2GB 로 자라면 `OutOfMemoryError` 가 난다. 사용자는 live 힙의 시간에 따른 변화를 예측해서 상수를 골라야 하는데, 이건 사실상 불가능한 튜닝이다.

Go 는 1.5 의 concurrent mark-sweep GC 를 설계하면서 반대 방향을 택했다. "힙 크기는 사용자가 정하지 않는다. 지금 살아 있는 것의 일정 배수를 목표로 삼고, 그 목표를 넘으면 GC 를 돌린다." 이것이 `GOGC=100` 이고, 식은 `목표 = live + live*GOGC/100` 이다. 이 선택의 미덕은 자기 조정이다. live 힙이 10배로 늘면 목표도 자동으로 10배가 되므로, 프로그램이 커져도 사용자가 아무것도 안 해도 된다. GC 비용은 live 힙에 비례하고, 할당 사이의 여유 공간도 live 힙에 비례하므로, 처리량 대비 GC 오버헤드가 규모와 무관하게 일정해진다. 이게 비례 제어의 진짜 값어치다. 튜닝 파라미터를 지운 게 아니라, 파라미터를 시스템이 스스로 맞추게 만든 것이다.

그 대가는 "상한이 없다"는 것이다. 비율만 있으면 절대량을 말할 수 없다. live 힙이 2GB 인 순간 목표는 4GB 가 되고, 그 사이에 4GB 를 실제로 쓸 수 있어야 한다. 컨테이너에 2GB 를 배정한 운영 환경에서는 이 지점에서 프로세스가 죽는다. 그리고 여기서 사람들이 흔히 하는 대응이 `GOGC=20` 같은 값으로 비율을 낮추는 것인데, 이건 상한을 만드는 게 아니라 그냥 GC 를 자주 돌리는 것이다. live 힙이 2GB 이면 `GOGC=20` 의 목표는 2.4GB 이므로 여전히 2GB 컨테이너에서는 죽는다. 비율 손잡이로는 "절대 이 선을 넘지 마라"를 표현할 수 없다는 것이 구조적 한계다.

Go 1.19 가 추가한 GOMEMLIMIT 이 이 자리를 채운다. runtime 이 추적하는 총메모리에 절대 상한을 걸고, 그 한도에서 힙이 아닌 몫을 뺀 값을 힙 목표로 삼는다. 중요한 설계 결정은 이것이 hard limit 이 아니라는 점이다. hard 로 만들면 두 가지 중 하나가 된다. 살아 있는 힙이 한도를 넘는 순간 프로그램을 죽이거나(그건 GC 가 아니라 감시자다), 한도를 지키려고 GC 를 무한히 돌린다. 후자가 이른바 GC death spiral 이고, 실제로 1.18 이전에 메모리 압박 상황에서 관측된 현상이다. Go 팀은 세 번째 길을 택했다. "한도를 지키기 위해 최선을 다하되, GC 가 전체 CPU 의 절반을 넘게 쓰지 않는다." 상한을 넘기면 넘긴 채로 두고 CPU 를 돌려준다. 그래서 이름이 soft memory limit 이다.

그렇다면 GOGC 를 없앴어야 하지 않나. 그렇지 않다. GOMEMLIMIT 이 "천장"을 정한다면 GOGC 는 "게으름"을 정한다. 천장이 아주 멀리 있는 개발 노트북에서는 GOGC 만 있으면 충분하고, 비례 제어는 처리량에 유리하다. 컨테이너에서는 천장을 GOMEMLIMIT 으로 걸고, 그 아래에서 GOGC 를 올려 GC 빈도를 낮춘다. 즉 GOGC 는 "한도까지 얼마나 여유를 두고 쓰레기를 쌓을지"를 정하는 하위 손잡이로 남는다. 이 계층 구조를 이해하지 못하면 "GOMEMLIMIT 을 걸었으니 GOGC 도 낮춰야 한다"는 잘못된 조합을 만들게 된다.

## 어떻게 동작하는가

먼저 회계부터. runtime 은 힙 사용량을 `runtime/mstats.go` 의 `heapStats` 구조체에서 관리한다. 필드 이름이 곧 지표 이름이다. `heapAlloc` 은 할당되어 살아 있는 바이트(우리가 흔히 live 라고 부르는 것에 가깝다), `heapInuse` 는 mspan 단위로 확보된 바이트, `heapReleased` 는 OS 에 돌려준 바이트, `heapSys` 는 OS 에서 받아 온 총량이다. `runtime.MemStats` 는 이 값들을 스냅샷으로 복사한 것이고, `runtime/metrics` 는 같은 값을 이름으로 읽는 더 가벼운 경로다. 이 프로그램이 쓰는 `/gc/heap/goal:bytes`, `/cpu/classes/gc/total:cpu-seconds` 가 후자다.

목표 힙은 `runtime/mgcpacer.go` 의 `gcControllerState` 가 계산한다. 이 구조체에 `gcPercent`, `memoryLimit`(atomic int64), `heapLive`, `heapMarked`, `heapGoal` 같은 필드가 있고, 매 GC 사이클이 끝날 때 `commit()` 이 새 목표를 확정한다. 목표는 두 값의 최솟값이다. 하나는 `gcPercentHeapGoal()` 로, 마킹이 끝난 시점의 살아 있는 힙(`heapMarked`)에 `GOGC/100` 을 곱해 더한 값이다. 다른 하나는 `memoryLimitHeapGoal()` 로, `memoryLimit` 에서 힙이 아닌 몫(goroutine 스택, 전역 변수, runtime 자료구조, GC 메타데이터, 단편화 여유분)을 뺀 값이다. 여기에 아주 작은 힙에서 GC 가 폭주하지 않도록 최소 힙 목표가 하한으로 걸린다.

그다음 트리거다. `runtime/mheap.go` 의 `gcTrigger` 에는 `gcTriggerHeap`, `gcTriggerTime`, `gcTriggerCycle` 같은 종류가 있고, 할당 경로에서 `heapLive` 가 `heapGoal` 에 닿으면 `gcStart` 가 불린다. 즉 GC 는 타이머가 아니라 **할당 압력**에 의해 시작된다. `GODEBUG=gctrace=1` 을 켜면 사이클마다 `gc 12 @3.456s 5%: ... 128->130->64 MB, 130 MB goal, 8 P` 같은 줄이 찍히는데, 여기서 `128->130->64 MB` 는 "GC 시작 시점 힙 크기 -> GC 종료 시점 힙 크기 -> live 힙", 뒤의 `130 MB goal` 이 그때의 목표 힙이다. 이 숫자 세 개를 보면 지금 무엇이 문제인지 즉시 갈린다. live 힙이 goal 에 거의 붙어 있으면 한도가 너무 빡빡한 것이고, goal 이 live 의 몇 배로 벌어져 있으면 GOGC 가 큰 것이다.

할당하는 goroutine 은 GC 가 진행 중이면 `gcAssistAlloc` 으로 마킹 작업을 돕는다. 이 assist 양은 "내가 할당한 만큼 마킹도 분담한다"는 규칙으로 정해지므로, GC 가 뒤처질수록 할당이 느려진다. 메모리 한도가 빡빡할수록 assist 비율이 올라가고, 애플리케이션의 할당 지연이 올라간다. 여기서 GOMEMLIMIT 의 두 번째 비용이 나온다. 메모리를 아끼는 대신 CPU 를 낸다.

1.19 는 여기에 `gcCPULimiterState` 라는 안전장치를 붙였다. GC 가 쓸 수 있는 CPU 를 전체 용량의 절반으로 제한한다. 한도가 살아 있는 힙보다 작아 목표를 도저히 만족할 수 없는 상황에서, runtime 은 한도를 포기하고 CPU 를 반납한다. 그래서 soft 다. 이 프로그램의 네 번째 단계(`GOMEMLIMIT=8MiB`, live 32MiB)가 정확히 이 상황이고, 표에서 peak HeapAlloc 이 8MiB 를 훌쩍 넘는 값으로 나온다.

마지막으로 OS 반환이다. GC 가 메모리를 회수해도 RSS 는 즉시 줄지 않는다. `runtime/mgcscavenge.go` 의 scavenger 가 유휴 페이지를 배경에서 조금씩 `madvise` 로 돌려주고, `heapReleased` 로 집계된다. 그래서 컨테이너의 RSS 를 보면 HeapAlloc peak 보다 크고, HeapSys 와 비슷한 수준에 머무는 것을 볼 수 있다. 이 프로그램이 표에 `peak HeapSys` 를 같이 찍는 이유다. GC 목표는 힙의 목표일 뿐 프로세스의 상한이 아니다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사: 미사용 변수, 잘못된 포맷 문자열 확인
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행: 네 단계 표와 목표 힙 공식표
go test -v ./...             # 테스트
go test -race ./...          # 동시성 주제라면 반드시
GODEBUG=gctrace=1 go run .   # 사이클마다 시작 힙 / 종료 힙 / live 힙 / goal 확인
GOGC=400 GOMEMLIMIT=64MiB go run .   # 환경변수로도 같은 설정이 들어가는지 확인
go test -bench=. -benchmem -run=^$ ./...   # GOGC 별 할당 처리량과 GC 빈도 비교
go test -v -run TestMemoryLimit -count=3 ./...  # 한도 관련 테스트 반복 안정성 확인
```

`go run .` 에서 봐야 할 것은 표의 세 줄이다. 첫 줄과 둘째 줄의 `peak HeapAlloc` 차이가 GOGC 가 만드는 차이다. 셋째 줄에서 `GOGC` 가 `off` 인데도 `GC 횟수` 가 0 이 아닌 것이 GOMEMLIMIT 이 일하고 있다는 증거다. 넷째 줄에서 `peak HeapAlloc` 이 `GOMEMLIMIT` 값보다 큰 것을 확인하라. 그게 soft 의 의미다.

`GODEBUG=gctrace=1` 로 돌리면 넷째 단계에서 `gc N @...s 40%: ...` 처럼 괄호 안 퍼센트가 급격히 올라가는 것을 볼 수 있다. 프로그램 시작 이후 누적 GC 시간 비율이므로, 네 번째 단계에 들어서며 치솟으면 GC 폭주다. 동시에 `goal` 값이 `GOMEMLIMIT` 보다도, live 힙보다도 작게 찍히는 사이클이 나온다. 목표를 만족할 수 없다는 표시다.

`go test -bench=. -run=^$ ./...` 는 벤치마크만 돌린다. `GOGC=400` 이 `GOGC=100` 보다 ns/op 이 낮게(빠르게) 나오는 대신 할당당 GC 비용이 뒤로 밀린 것을 볼 수 있고, `GOGC=off+LIMIT=32MiB` 는 assist 때문에 처리량이 오히려 기본값보다 낮게 나올 수 있다. 메모리를 아끼면 CPU 를 낸다는 맞교환이 그대로 수치로 드러난다.

## 코드로 확인하기

`runPhase()` 는 단계마다 세 가지를 한다. 이전 단계의 쓰레기를 `runtime.GC()` 로 정리하고, `allocLiveBlocks()` 로 살아 있는 힙을 만든 뒤, `churn()` 으로 4MiB 덩어리를 계속 만들어 버린다. 매 덩어리마다 `runtime.ReadMemStats` 로 `HeapAlloc` 을 읽어 최댓값을 갱신하고, 그 순간의 `/gc/heap/goal:bytes` 도 함께 기록한다. `live` 슬라이스는 `runtime.KeepAlive(live)` 로 함수 끝까지 살려 둔다. 이 한 줄이 없으면 컴파일러가 liveness 분석으로 live 블록을 앞당겨 회수할 수 있고, 그러면 "살아 있는 힙이 일정하다"는 전제가 무너져 단계 간 비교가 무의미해진다.

출력에서 읽어야 할 숫자는 이렇게 해석한다. 첫 줄 `GOGC=100` 은 live 32MiB 에 대해 목표가 64MiB 이므로 peak HeapAlloc 이 대략 64MiB 근처(60~70MiB)로 나온다. 256MiB 를 32MiB 씩 나눠 쓰므로 GC 횟수는 8회 안팎이다. 둘째 줄 `GOGC=400` 은 목표가 160MiB 로 커지므로 peak 가 160MiB 근처로 뛰고 GC 횟수는 2회 안팎으로 떨어진다. 스루풋은 좋아지고 메모리는 2.5배가 된 것이다. 셋째 줄`GOGC=off + GOMEMLIMIT=48MiB` 는 비율 목표가 무한대지만 한도 목표가 약 43MiB(48MiB 에서 힙 아닌 몫을 뺀 값)이므로 peak 가 45MiB 근처로 눌리고, GC 횟수는 40회 안팎으로 늘어난다. 넷째 줄은 한도 8MiB 인데 live 가 32MiB 이므로 GC 가 아무리 돌아도 목표를 만족할 수 없다. peak 가 8MiB 를 넘는 값으로 나오고, GC 횟수는 많으며, GC CPU% 는 50% 근처에서 멈춘다.

표의 `peak HeapSys` 는 `peak HeapAlloc` 보다 항상 크다. 그 차이가 아직 OS 에 반환되지 않은 페이지와 runtime 내부 구조다. `peak 목표힙` 열은 피크를 관측한 순간 GC 가 잡고 있던 목표로, 첫 줄에서는 live 의 약 2배, 셋째 줄에서는 한도 근처, 넷째 줄에서는 live 근처의 값이 찍힌다.

`main_test.go` 는 다섯 가지를 검증한다. `TestGcPercentHeapGoalIsProportional` 은 비례식 자체(`2배`, `3배`, `5배`, `1.5배`)와 GOGC off, 최소 힙 목표를 고정한다. `TestEffectiveHeapGoalPicksSmallerSide` 는 두 목표의 최솟값 규칙과, 한도가 live 보다 작을 때 목표가 live 로 올라가는 soft 성질을 검증한다. `TestMemoryLimitCapsHeapWhenGcPercentOff` 는 실제로 두 단계를 돌려 GOGC 를 껐을 때 GOMEMLIMIT 만으로 힙이 절반 이하로 눌리는지 본다. `TestMemoryLimitIsSoftWhenBelowLiveHeap` 은 한도를 넘긴다는 사실과 GC 가 실제로 돌았다는 사실만 본다(CPU 비율은 장비 의존이라 단정하지 않는다). `TestGcPercent100KeepsHeapNearTwiceLive` 는 기본값에서 peak 가 live 의 5배를 넘지 않는지 보는 느슨한 상한이다. `BenchmarkAllocThroughput` 은 세 설정의 할당 처리량을 비교한다.

## 모르면 겪는 일

가장 흔한 증상은 **아무 로그 없이 사라지는 프로세스**다. 컨테이너 메모리 한도 2GB, live 힙 1.2GB 인 서비스를 `GOGC=100` 기본값으로 돌리면 목표 힙이 2.4GB 가 되고, 트래픽 피크에 할당이 몰리는 순간 cgroup 이 프로세스를 죽인다. 종료 코드 137, 애플리케이션 로그 없음, `dmesg` 에 OOM killer 흔적. Go 런타임 입장에서는 아무 잘못이 없다. 자신의 목표를 정상적으로 달성했을 뿐이다. 이때 CPU 프로파일을 봐도 GC 는 한가하고, 힙 프로파일을 봐도 live 는 1.2GB 로 멀쩡하다. **"프로파일에 안 보이는데 죽는다"** 가 이 문제의 전형적인 얼굴이다. 봐야 할 것은 `/gc/heap/goal:bytes` 와 컨테이너 RSS 의 최댓값이다.

두 번째는 **GOGC 를 낮춰서 해결하려는 시도**다. `GOGC=100` 에서 `GOGC=20` 으로 내리면 목표가 live 의 1.2배가 되니 peak 메모리는 줄어 보인다. 하지만 CPU 사용률이 오르고 p99 지연이 나빠진다. live 힙 1.2GB 에 목표 1.44GB 이므로 여전히 여유가 240MB 뿐이라, 조금만 할당이 튀어도 GC 가 연속으로 돈다. 처리량이 20~30% 떨어지고, 그런데도 OOM 은 계속 난다. 비율을 조여서 절대 상한을 흉내 내려는 시도가 실패하는 이유다.

세 번째는 **GOMEMLIMIT 을 live 힙보다 낮게 거는 실수**다. "메모리를 아끼려고" 컨테이너 한도의 절반쯤으로 넉넉히 잡는 게 아니라, 현재 live 힙보다 작은 값을 넣으면 GC 가 쉬지 않고 돈다. 이 프로그램의 넷째 단계가 그 상황이다. `gctrace` 에 사이클이 초당 수십 번 찍히고, assist 가 붙어 애플리케이션 지연이 튀고, 그런데 메모리는 한도 아래로 내려가지 않는다. 최악의 조합은 "CPU 는 타는데 메모리는 그대로"다. 1.19 의 CPU 상한 덕분에 death spiral 로 완전히 멈추지는 않지만, 처리량은 조용히 반토막 난다.

네 번째는 **RSS 가 안 줄어든다고 leak 이라고 오해하는 것**이다. GC 가 회수한 메모리는 `heapReleased` 로 넘어가지만 scavenger 가 배경에서 천천히 돌려주므로 RSS 는 계단식으로 줄어든다. `HeapAlloc` 은 안정적인데 RSS 만 높다고 leak 이 아니다. `debug.FreeOSMemory()` 를 호출하면 즉시 반환되지만, 그 호출 자체가 STW 를 유발하므로 요청 경로에 넣으면 p99 가 튄다.

다섯 번째는 **GOMEMLIMIT 이 모든 메모리를 세는 줄 아는 오해**다. cgo 로 malloc 한 버퍼, 라이브러리가 mmap 한 파일, sidecar 가 쓰는 몫은 집계에 들어가지 않는다. 컨테이너 한도의 100% 를 GOMEMLIMIT 으로 잡으면 그 몫만큼 초과해서 결국 OOM 이다. 이 프로그램의 `recommendMemoryLimit` 이 여유분을 먼저 떼는 이유다.

## 언제 신경 쓰고 언제 무시하나

live 힙이 수십 MB 수준이고 배포 환경이 단일 머신이거나 메모리가 넉넉하면 **아무것도 하지 마라**. GOGC 기본값 100 은 대부분의 서비스에서 합리적이고, 손대면 대개 나빠진다. 목표가 live 의 2배라는 것은 live 가 작을 때 절대량도 작다는 뜻이므로 위험 자체가 작다.

신경 써야 하는 조건은 세 가지다. 첫째, 메모리 한도가 있는 컨테이너에서 돌고 있다. 둘째, steady state 의 live 힙이 그 한도의 1/3 을 넘는다. 셋째, 관측된 peak RSS 가 한도의 80% 를 넘은 적이 있다. 이 셋 중 둘 이상이면 GOMEMLIMIT 을 걸어야 한다. 절차는 간단하다. live 힙을 재고, 컨테이너 한도에서 Go 가 못 보는 몫(대개 100~300MB 또는 한도의 10~15%)을 빼서 GOMEMLIMIT 을 정하고, GOGC 는 100 으로 두거나 CPU 여유가 있으면 200~400 으로 올린다. GOGC 를 올리는 것은 이제 안전하다. 상한은 GOMEMLIMIT 이 잡고 있기 때문이다.

반대로 **GOMEMLIMIT 을 무시해도 되는 경우**도 분명히 있다. 배치 작업처럼 처리량이 전부이고 메모리가 노드 전체에 열려 있는 경우, 또는 live 힙이 한도의 10% 도 안 되는 경우다. 후자에서 GOMEMLIMIT 을 걸면 아무 일도 일어나지 않는다. 한도가 비율 목표보다 훨씬 멀리 있으면 min 연산에서 항상 GOGC 쪽이 선택되기 때문이다. 즉 손해는 없지만 이득도 없다. 설정 항목을 하나 더 관리하는 비용만 남는다.

지연에 민감한 서비스라면 GOGC 를 **올리는** 방향을 먼저 검토하라. GC 빈도를 줄이면 p99 꼬리가 짧아지는 경우가 많다. 메모리를 내주고 지연을 사는 거래인데, GOMEMLIMIT 이 천장을 잡아 주므로 메모리 쪽 위험이 상한된다. 이 조합이 1.19 이후 Go 서비스 튜닝의 기본형이다.

## 더 파보기

- A Guide to the Go Garbage Collector — https://go.dev/doc/gc-guide (GOGC 와 GOMEMLIMIT 의 상호작용을 공식 문서로 정리한 1차 자료. "Memory limit" 절부터 읽어라)
- `runtime/debug.SetMemoryLimit` 문서 — https://pkg.go.dev/runtime/debug#SetMemoryLimit (soft limit 의 정의와 math.MaxInt64 의 의미)
- 제안 이슈 "proposal: runtime: soft memory limit" — https://github.com/golang/go/issues/48409 (왜 hard limit 을 버렸는지, death spiral 논의가 그대로 들어 있다)
- Rick Hudson, "Evolving the Go Memory Manager's RAM and CPU Efficiency" — https://go.dev/blog/ismmkeynote (GOGC 라는 비례 제어가 어떤 목표로 설계됐는지的历史)
- `runtime/metrics` 지표 목록 — https://pkg.go.dev/runtime/metrics#pkg-constants (`/gc/heap/goal:bytes`, `/gc/heap/live:bytes`, `/cpu/classes/gc/total:cpu-seconds`)
- 소스: `runtime/mgcpacer.go` (`gcControllerState.commit`, `memoryLimitHeapGoal`), `runtime/mheap.go` (`gcTrigger`), `runtime/mgcscavenge.go` (scavenger)