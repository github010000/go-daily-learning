## 한 줄 요약

Go 1.14에서 도입된 비동기 선점(asynchronous preemption)은 함수 호출이 하나도 없는 tight loop가 P를 독점하여 GC의 stop-the-world(STW)나 다른 goroutine의 스케줄링을 무한정 막던 문제를 해결하기 위해 만들어졌다. 이제 goroutine은 대부분의 지점에서 시그널을 통해 안전하게 선점될 수 있지만, runtime 내부의 일부 구간(예: runtime lock을 쥐고 있는 동안)은 여전히 선점되지 않는다. 이 문서는 협조적 선점의 한계, 시그널 기반 비동기 선점의 동작 원리, 그리고 남아 있는 비선점 구간을 코드로 확인하고 설명한다.

## 왜 이런 설계인가

Go 초기부터 1.13까지 사용된 협조적 선점(cooperative preemption)은 goroutine이 함수 호출을 할 때만 선점될 수 있었다. runtime은 함수 진입부에 preemption check를 삽입했고, goroutine이 함수를 호출하면 스택 검사와 함께 선점 요청이 있는지 확인했다. 이 방식은 구현이 단순하고 성능 오버헤드가 거의 없다는 장점이 있었지만, 함수 호출이 없는 순수 연산 루프(예: `for { i++ }`)는 preemption point가 전혀 없어서 해당 goroutine이 P(프로세서)를 무한정 점유할 수 있었다. 특히 `GOMAXPROCS=1` 환경에서는 다른 goroutine이 아예 실행되지 못해 프로그램 전체가 멈추는 것처럼 보였고, GC가 STW를 시작하려 해도 그 goroutine이 멈추지 않아 전체 pause가 무한정 길어질 수 있었다.

이 문제를 해결하기 위한 대안은 크게 두 가지였다. 첫째는 컴파일러가 모든 루프의 back edge에 preemption check를 삽입하는 것이다. 이는 loop가 많은 코드에 오버헤드를 추가하고, 컴파일러 변경이 필요하며, 모든 경우를 커버하지 못할 수 있다(예: 인라인 어셈블리). 둘째는 OS 시그널을 이용해 실행 중인 thread를 강제로 멈추고 scheduler로 복귀시키는 비동기 선점이다. Go 팀은 후자를 선택했다. 시그널 기반 선점은 컴파일러가 삽입한 지점에 의존하지 않고, 운영체제가 thread에 시그널(리눅스에서는 SIGURG)을 보내 실행 흐름을 잠시 가로챈 뒤 goroutine의 상태를 안전하게 저장하고 scheduler를 호출할 수 있게 한다.

비동기 선점이 가능해지면서 goroutine은 거의 모든 사용자 코드에서 선점될 수 있게 되었다. 그러나 runtime 내부에는 절대 선점되면 안 되는 구간이 있다. 예를 들어 runtime이 내부 lock(예: `allpLock`, `mheap_.lock`)을 쥐고 있는 동안 선점되면 다른 스레드가 그 lock을 기다리며 deadlock이 발생할 수 있다. 또한 GC 자체가 실행 중이거나, 스택 성장·축소, write barrier 같은 저수준 작업 중에는 레지스터와 스택 상태가 불안정하여 선점하면 메모리 손상이 일어날 수 있다. 따라서 runtime은 “preemptible” 상태 플래그(`g.preemptStop`, `g.stackguard0` 등)를 검사해 선점 가능 여부를 판별하고, 불가능한 구간에서는 시그널이 와도 선점을 미룬다. 이런 비선점 구간은 극히 짧게 유지되어야 하며, 사용자 코드에서 직접 만들 수는 없지만 runtime 소스를 읽을 때는 반드시 알아야 한다.

## 어떻게 동작하는가

비동기 선점의 핵심은 runtime이 특정 조건에서 goroutine을 선점하기 위해 OS 시그널을 사용한다는 점이다. Go runtime은 goroutine이 실행 중인 thread(OS thread)에 `SIGURG`(리눅스 기준)를 보낸다. signal handler는 `runtime.sighandler`에서 시작하여 `runtime.doSigPreempt`까지 이어진다. 이 함수는 현재 실행 중인 goroutine(`g`)이 선점 가능한 상태인지 `canPreemptM` 함수로 확인한다. 만약 선점 가능하면 `asyncPreempt`라는 어셈블리 함수로 점프하여 현재 레지스터와 PC를 goroutine의 스택에 저장하고, `runtime.schedule`을 호출해 다른 goroutine을 고른다. 선점 불가능하면 시그널을 무시하고 원래 실행 흐름으로 복귀한다.

선점 가능 여부를 판단하는 조건은 까다롭다. goroutine이 시스템 스택(system stack)에서 실행 중이거나, runtime 내부 lock을 소유하고 있거나, `m`(스레드)이 curg와 g0 사이에 있거나, `//go:nosplit` 함수에 있는 경우 등이 모두 비선점으로 간주된다. 이런 상태는 `g.preempt` 플래그와 `g.stackguard0` 값으로 표현되며, `runtime.preemptone`, `runtime.preemptall` 같은 함수에서 설정한다. 예를 들어 GC가 STW를 시작하려 하면 `runtime.stopTheWorldWithSema`가 `preemptall`을 호출해 모든 P에 선점 요청을 보낸다. 각 P는 자신의 로컬 run queue에서 다음 goroutine을 고를 때 요청을 확인하지만, 이미 실행 중인 goroutine은 시그널을 통해서만 선점될 수 있다.

시그널 기반 선점이 실제로 동작하는 과정은 `runtime/signal_unix.go`에 잘 나타나 있다. `sigtramp`에서 시작된 핸들러는 `sigctxt`를 통해 레지스터 상태를 읽고, `doSigPreempt`가 호출되면 `asyncPreempt`로 진입한다. `asyncPreempt`는 `runtime/asm_amd64.s` 같은 아키텍처별 어셈블리 파일에 정의되어 있으며, 모든 범용 레지스터와 플래그를 `g`의 스택에 push한 뒤 `runtime.schedule`을 호출한다. 이때 `g`의 상태는 `_Grunning`에서 `_Grunnable`로 바뀌고, 다시 실행될 때는 저장된 레지스터를 복원해 중단된 지점부터 재개한다. 이 과정은 모두 신중하게 작성되어 있어 선점이 안전하다.

하지만 여전히 선점되지 않는 구간은 존재한다. 대표적으로 `runtime.gcstopm` 함수가 있는데, 이 함수는 GC STW 동안 모든 P를 멈추기 위해 사용되며, 이 안에서 실행되는 코드는 선점 요청을 무시한다. 또한 `runtime.mstart`와 같은 스레드 초기화 루틴이나 `runtime.newstack`(스택 성장/축소)도 비선점으로 처리된다. 소스 코드를 보면 `runtime/proc.go`의 `canPreemptM` 함수에 비선점 조건이 주석과 함께 나열되어 있다. 사용자 코드에서 이런 구간을 직접 만들 수는 없지만, cgo나 syscall 등으로 runtime 밖에서 오래 블록되는 경우에는 전혀 다른 비선점 상태가 될 수 있으므로 주의해야 한다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령을 순서대로 제시한다. 각 명령에서 무엇을 봐야 하는지 덧붙인다.

```bash
go vet ./...                 # 정적 검사: 코드에 명백한 문제가 없는지 확인
go build -o /dev/null ./...  # 컴파일 확인: import 오류나 문법 오류가 없는지 확인
go run . noyield             # 함수 호출 없는 tight loop + async preemption 시연
go run . yield               # runtime.Gosched()를 넣은 tight loop 로 협조적 선점 시연
GODEBUG=asyncpreemptoff=1 go run . noyield   # 1.13 이전처럼 비동기 선점을 끄고 실패(시간 초과)를 관찰
go test -v ./...             # 테스트 실행: yield/no-yield 모드 모두 통과해야 함
go test -race ./...          # data race 여부 확인: -race 환경에서도 통과해야 함
```

`go run . noyield`의 출력에서 `progress=true`와 `elapsed` 값이 약 10ms 내외로 나오는 것을 확인한다. 이 10ms는 Go runtime의 비동기 선점 주기(schedtick 기반) 때문이다. `go run . yield`에서는 `elapsed`가 수십 마이크로초 수준으로 훨씬 짧다. `GODEBUG=asyncpreemptoff=1 go run . noyield`는 1초 타임아웃 후 `progress=false`와 함께 `os.Exit(1)`로 종료되는데, 이것이 1.13 이전의 협조적 선점 한계를 재현한 것이다.

## 코드로 확인하기

`main.go`의 핵심은 두 개의 tight loop 함수와 `runPreemptionDemo`이다. `tightLoopNoYield`는 `sink`에 값을 대입하는 것 외에는 아무 함수 호출도 하지 않는다. 그래서 컴파일러가 루프를 제거하지 않으면서도 preemption point가 전혀 없다. `tightLoopWithYield`는 매 반복마다 `runtime.Gosched()`를 호출해 협조적 선점 지점을 만든다.

`runPreemptionDemo`는 `runtime.GOMAXPROCS(1)`을 호출해 오직 하나의 P만 사용하게 만든다. 그 다음 `tightLoopNoYield`(또는 `tightLoopWithYield`)를 goroutine으로 시작하고, `ticker` goroutine을 시작해 0부터 9까지 채널로 보내게 한다. `GOMAXPROCS=1`이므로 run queue에서 tight loop가 먼저 실행되고(시작 순서가 앞서므로), ticker는 대기한다. 만약 tight loop가 preemption point를 만들지 않는다면 1.13 이전에는 ticker가 영원히 실행되지 못했을 것이다. 하지만 1.14 이후에는 비동기 선점이 10ms 주기로 tight loop를 중단시키고 ticker에게 실행 기회를 준다. `runPreemptionDemo`는 ticker가 보낸 10개의 값을 모두 받으면 `true`를 반환한다. 만약 1초 내에 값을 받지 못하면 `false`를 반환하고, 이는 비동기 선점이 꺼져 있음을 의미한다.

`main` 함수는 명령줄 인자 `noyield` 또는 `yield`를 받아 해당 모드의 데모를 실행하고 결과를 출력한다. 출력에서 `progress`가 `true`인지, `elapsed`가 어느 정도인지 확인한다. `noyield` 모드에서는 `elapsed`가 10ms 이상 걸리는 반면, `yield` 모드에서는 거의 즉시 완료된다. 이 차이가 비동기 선점의 존재를 보여준다.

`main_test.go`는 두 개의 테스트를 정의한다. `TestCooperativePreemptionWithYield`는 `useYield=true`로 `runPreemptionDemo`를 호출해 항상 `ok=true`임을 검증한다. `TestAsyncPreemptionNoYield`는 `useYield=false`로 호출해 함수 호출 없는 루프에서도 `ok=true`임을 검증한다. 두 테스트 모두 `-race` 플래그와 함께 실행해도 data race가 없어야 한다. 이 테스트는 `GODEBUG=asyncpreemptoff=1` 환경에서 실행하면 `TestAsyncPreemptionNoYield`가 실패하도록 의도되었다. 이렇게 함으로써 협조적 선점만으로는 이 시나리오가 통과할 수 없음을 보여준다.

## 모르면 겪는 일

이 개념을 모르고 1.14 이전의 Go로 서버를 운영했다면, CPU-bound 작업을 하는 goroutine 하나가 다른 모든 요청을 멈추게 하는 사고를 경험했을 수 있다. 특히 `GOMAXPROCS=1`인 컨테이너 환경에서 `for { i++ }` 같은 실수로 인해 GC가 STW를 시작하지 못하고, p99 latency가 수십 초까지 치솟는 현상이 발생했다. CPU 프로파일을 보면 해당 goroutine이 CPU를 100% 점유하고 있지만, 왜 다른 goroutine이 실행되지 않는지 이해하기 어려웠을 것이다.

1.14 이후에도 `GODEBUG=asyncpreemptoff=1`을 실수로 설정하거나, 아직 비동기 선점이 구현되지 않은 플랫폼(일부 아키텍처)에서 실행하면 동일한 증상이 나타난다. 또한 runtime 내부의 비선점 구간이 예상보다 길어지면 STW 시간이 튈 수 있는데, 이는 사용자 코드가 직접 유발하는 경우보다 runtime 버그나 특정 syscall 조합에서 드물게 발생한다. 예를 들어 `runtime.GC()`를 주기적으로 호출하면서 동시에 네트워크 I/O가 몰리면 STW pause가 갑자기 늘어나는 것처럼 보일 수 있는데, 이는 비선점 구간에서 시간이 걸렸기 때문일 수 있다.

협조적 선점 시대의 습관으로 `runtime.Gosched()`를 루프에 남발하는 경우도 문제다. 1.14부터는 필요 없지만, 이전 코드에서 남아 있는 `Gosched()` 호출은 성능을 저하시킬 수 있다. 매 반복마다 스케줄러에게 돌아가므로 context switch 오버헤드가 커지고, 같은 P에서 실행되는 다른 goroutine에게 불필요하게 자주 양보하게 된다. 따라서 “아무 생각 없이 넣었던 협조적 양보”가 이제는 최적화의 적이 될 수 있음을 알아야 한다.

## 언제 신경 쓰고 언제 무시하나

대부분의 애플리케이션 개발자는 비동기 선점의 내부 동작을 몰라도 된다. Go 1.14 이상을 사용한다면 tight loop가 다른 goroutine을 굶기는 문제는 자동으로 해결되므로 신경 쓸 필요가 없다. 오히려 과거의 협조적 선점을 의식해 `runtime.Gosched()`를 넣는 것은 성능만 떨어뜨릴 수 있으므로 제거하는 편이 낫다.

이 지식이 중요해지는 경우는 크게 세 가지다. 첫째, `GOMAXPROCS=1`인 환경에서 극도로 낮은 latency를 요구하는 경우. 비동기 선점은 10ms 주기로 일어나므로, 아주 짧은 시간에 정확한 스케줄링이 필요하면 이 주기를 고려해야 한다. 둘째, runtime 내부를 디버깅하거나, cgo/syscall을 많이 사용해 goroutine이 비선점 상태에 빠질 가능성이 있는 경우. 셋째, Go 컴파일러나 runtime을 직접 수정하는 경우. 이때는 `canPreemptM`의 조건을 반드시 이해해야 한다.

반대로, 일반적인 웹 서버나 CLI 도구에서는 비동기 선점의 존재 여부가 사용자에게 직접 보이지 않는다. GC pause가 급증하는 문제가 발생해도 원인이 비선점 구간인 경우는 드물다. 따라서 `GODEBUG=asyncpreemptoff=1`을 쓰지 않는 한 이 주제는 성능 튜닝의 우선순위에서 밀린다. 과최적화를 피하려면 “tight loop가 다른 goroutine을 굶긴다”는 증상이 실제로 관찰될 때만 이 지식을 떠올리는 것이 좋다.

## 더 파보기

1. Go 1.14 Release Notes (runtime 섹션): [https://go.dev/doc/go1.14#runtime](https://go.dev/doc/go1.14#runtime)  
   비동기 선점의 공식 발표와 간단한 설명이 있다.

2. `runtime/preempt.go` 소스: [https://github.com/golang/go/blob/master/src/runtime/preempt.go](https://github.com/golang/go/blob/master/src/runtime/preempt.go)  
   선점 요청과 처리 로직, `preemptone`, `preemptall` 함수가 정의되어 있다.

3. `runtime/signal_unix.go` 소스: [https://github.com/golang/go/blob/master/src/runtime/signal_unix.go](https://github.com/golang/go/blob/master/src/runtime/signal_unix.go)  
   SIGURG를 받아 `doSigPreempt`로 연결하는 실제 시그널 핸들러가 있다.

4. 이슈 #10958: [https://github.com/golang/go/issues/10958](https://github.com/golang/go/issues/10958)  
   비동기 선점 도입을 논의한 초기 이슈로, 협조적 선점의 문제 사례가 다수 수록되어 있다.

5. Go scheduler design doc: [https://go.dev/s/go11sched](https://go.dev/s/go11sched)  
   goroutine 스케줄러의 전반적인 동작 원리를 이해하는 데 도움이 된다.