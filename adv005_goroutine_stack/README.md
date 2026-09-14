## 한 줄 요약

goroutine stack 은 2KB 에서 시작하고, 필요할 때마다 더 큰 연속된 메모리로 통째로 복사되며, 함수가 반환되고 GC 가 돌면 다시 줄어든다. 이 방식은 한때 쓰던 segment stack 의 hot split 문제를 없애고 수십만 goroutine 을 작은 메모리로 돌릴 수 있게 해준다.

## 왜 이런 설계인가

운영체제가 thread 하나를 만들 때 기본으로 주는 stack 은 보통 1MB 정도다. 스레드 수백 개만 만들어도 수백 MB 를 stack 으로 점유한다. Go 는 동시성 흐름을 수만 개 이상 만드는 것을 주 사용처로 삼았기 때문에, 각 흐름에 1MB 씩 주는 방식은 처음부터 불가능했다. goroutine 당 시작 메모리를 극도로 작게 잡고, 실제로 깊은 호출이 필요한 goroutine 에만 큰 stack 을 나중에 할당하는 전략이 거의 유일한 답이었다.

초기 Go 는 stack 을 연결 리스트처럼 여러 segment 로 쪼개서 쓰는 방식이었다. 함수가 호출되어 현재 segment 가 부족하면 새 segment 를 할당해 이어 붙였다. 이 방식은 메모리를 아낄 수 있었지만, 특정 함수 호출 경계가 반복해서 segment 경계와 겹치면 매번 segment 할당과 해제가 일어나는 문제가 있었다. 이 문제는 흔히 hot split 이라고 불렸고, 실제 코드에서 갑자기 수십 배 느려지는 원인이 됐다.

또한 segment 방식은 컴파일러가 만드는 코드가 segment 리스트를 따라가야 했고, C 언어의 alloca 류처럼 가변 크기 stack frame 을 다루기 어려웠다. 결국 Go 1.4 즈음에 segment stack 을 버리고 연속 stack(contiguous stack)으로 전환했다. 연속 stack 은 실제로 부족할 때 더 큰 메모리 블록을 새로 할당해 기존 stack 전체를 복사한다. 복사 비용은 들지만, 일반적인 코드에서는 한 goroutine 이 stack 을 커지는 횟수가 제한적이기 때문에 이 비용이 더 싸다.

연속 stack 의 핵심 가정은 "대부분의 goroutine 은 얕은 stack 을 쓴다"이다. 2KB 시작 크기는 이 가정을 극단적으로 반영한다. 이 덕분에 100,000 개의 goroutine 을 만들어도 stack 초기 점유는 약 200MB 정도에 그친다. OS thread 100,000 개였다면 stack 만 100GB 가 필요하다. 실제로는 Go 런타임이 8KB 또는 16KB 의 배수로 할당하고, 시작 후 한 번이라도 커지면 2KB 낭비를 줄이기 위해 2KB 그대로 유지하지 않을 수 있지만, 규모의 차이는 여전하다.

## 어떻게 동작하는가

goroutine 의 stack 상태는 runtime/stack.go 와 runtime/runtime2.go 에 정의된 `g` 구조체에 기록된다. `g.stack` 은 `stack` 구조체로 `lo` 와 `hi` 필드가 현재 stack 메모리 범위를 가리킨다. `g.stackguard0` 은 함수 프롤로그가 stack 부족을 검사할 때 비교하는 한계값이다. Go 컴파일러는 거의 모든 함수 시작 부분에 현재 stack pointer 와 `stackguard0` 을 비교하는 코드를 넣는다. 이 검사에서 stack 이 부족하다고 판정되면 `runtime.morestack` 이라는 런타임 함수로 점프한다.

`morestack` 은 곧바로 새 stack 을 만들지 않고, 현재 goroutine 상태를 저장한 뒤 `runtime.newstack` 을 호출한다. `runtime/stack.go` 의 `newstack` 은 말 그대로 더 큰 stack 을 만들고 기존 stack 내용을 복사한다. 새 stack 크기는 `stackpool` 이나 `mcache` 같은 런타임 할당자에서 가져온다. 크기는 현재 사용량보다 넉넉하게 잡아 반복적인 복사를 줄인다. 큰 stack 이 필요할 때마다 약 2배씩 커지는 방식이 일반적이다.

복사가 일어날 때 가장 중요한 작업은 포인터 조정이다. 기존 stack 안에는 지역 변수, 함수 인자, 반환 주소, 또 다른 포인터들이 들어 있다. 이 포인터 중 heap 이나 다른 stack 을 가리키는 것은 새 stack 주소로 옮겨진 뒤에도 같은 대상 주소를 가리켜야 한다. 런타임은 pclntab(프로그램 카운터 라인 테이블)과 스택 맵(stack map)을 이용해 각 프레임에서 포인터가 들어 있는 위치를 찾고, `runtime.adjustpointers` 류 함수가 그 값을 보정한다. 이 과정이 틀리면 재귀 깊은 곳에서 heap 객체에 값을 썼는데 엉뚱한 메모리를 바꾸는 버그가 생긴다.

stack 축소는 함수가 반환해서 남는 공간이 많아져도 바로 일어나지 않는다. stack shrink 는 GC 사이클 중에 goroutine 을 스캔할 때 실행된다. 런타임은 stack 사용량을 검사해 사용하지 않는 꼬리 부분이 충분히 크면 stack 을 더 작은 메모리로 복사하고 `g.stack` 정보를 갱신한다. 그래서 "재귀가 끝났는데 메모리가 즉시 안 줄어든다"는 현상은 버그가 아니라 설계 동작이다. GC 를 강제로 돌리면 줄어드는 것을 확인할 수 있다.

`runtime.Stack` 함수는 현재 goroutine 의 stack trace 텍스트를 돌려준다. 이 함수가 돌려주는 바이트 수는 stack 메모리 크기가 아니라 trace 문자열 길이다. 이 예제의 `traceLenAtDepth` 는 그 차이를 보여주기 위해 일부러 사용한다. 실제 stack 메모리 크기는 `runtime.MemStats.StackInuse` 로 전역 합계를 관찰할 수 있다. 특정 goroutine 하나의 stack 크기를 공개 API 로 직접 읽는 방법은 없고, 런타임 내부에서는 `g.stack.hi - g.stack.lo` 로 계산한다.

## 돌려보기

```bash
go vet ./...                 # 정적 검사. noinline 지시어와 포인터 사용을 확인합니다.
go build -o /dev/null ./...  # 표준 라이브러리만 사용해 macOS/리눅스에서 컴파일되는지 확인합니다.
go run .                     # stack 성장, 축소, hot split 시뮬레이션, 메모리 이점을 순서대로 출력합니다.
go test -v ./...             # 포인터 무결성, trace 길이, 많은 goroutine 메모리 상한을 검증합니다.
go test -race ./...          # 동시성 코드가 race 를 일으키지 않는지 확인합니다.
```

`go run .` 출력에서 `StackInuse` 수치가 깊은 재귀 goroutine 대기 중 증가하고, `재귀 반환 + GC 후` 줄어드는지 봐야 한다. `go test -v` 는 시간 단정이 아니라 포인터 값, trace 길이, 메모리 상한 같은 불변식을 검증한다. `go test -race` 는 채널로 대기하는 깊은 재귀 goroutine 과 main goroutine 사이의 동기화가 올바른지 확인한다.

## 코드로 확인하기

`showStackGrowth` 는 두 가지를 출력한다. 첫째는 `traceLenAtDepth(0)` 과 `traceLenAtDepth(200)` 의 비교다. 깊이 0 에서는 런타임이 main 함수 몇 개만 나열하지만, 깊이 200 에서는 200 개의 `traceLenAtDepth` 프레임이 trace 에 들어가므로 텍스트 길이가 크게 늘어난다. 이것은 stack 메모리 크기가 아니라 프레임 수가 늘었다는 증거다. 둘째는 `deepRecursionThenBlock` 으로 2,048 프레임을 쌓은 goroutine 을 블록 상태로 둔 뒤 `runtime.MemStats.StackInuse` 를 읽는 부분이다. 깊이 2,048 에 각 512바이트 padding 이 있으므로 약 1MB 의 stack 사용이 생긴다. 시작 stack 이 2KB 였다면 이 지점에서 몇 번의 morestack 복사가 이미 일어난 상태다.

`showStackCopy` 는 `fillStack` 을 여러 깊이로 호출하고 heap 객체 `target.value` 가 42 로 남아 있는지 확인한다. 각 재귀 프레임은 `ctx *pointerTarget` 을 인자로 들고 있다. stack 이 자라면서 복사될 때 런타임이 이 인자 포인터를 새 stack 주소로 조정해야 한다. 조정이 실패하면 depth 0 에서 `ctx.value = 42` 가 과거 stack 을 가리켜 잘못된 메모리 접근이 되거나 test 가 실패한다. 이 코드는 실제로 `go test -race` 에서도 안전하게 통과한다.

`simulateHotSplit` 은 `//go:noinline` 이 붙은 아주 작은 함수를 백만 번 호출해 시간을 잰다. 연속 stack 에서는 함수 호출이 단순히 stack pointer 를 옮기고 반환 주소를 저장하는 정도다. segment stack 시절에는 이 경계에서 segment 할당과 해제가 반복될 수 있었다. 시뮬레이션 자체는 지금 Go 에서 segment 오버헤드를 재현하지 않지만, 아주 작은 함수 호출이 충분히 빠르다는 것을 수치로 보여준다.

`manyGoroutines` 는 100,000 개 goroutine 을 만들고 `StackInuse` 를 출력한다. 각 goroutine 은 거의 stack 을 쓰지 않으므로 시작 크기 2KB 수준을 유지한다. 출력에서 `StackInuse` 는 대략 200~300MB 근처가 된다. 같은 개수를 OS thread 로 만들면 1MB x 100,000 = 약 100GB 가 필요하다는 비교를 출력에 넣었다. 이는 goroutine 의 가벼움을 막연한 비유가 아니라 stack 정책에서 오는 구체적 수치로 보여준다.

`main_test.go` 의 `TestDeepRecursionPointerIntegrity` 는 깊이를 점점 늘리며 `fillStack` 이 `target.value` 를 항상 42 로 만드는지 확인한다. `TestTraceLenGrowsWithDepth` 는 재귀 깊이가 늘면 `runtime.Stack` trace 길이가 늘어나는지 확인한다. `TestManyGoroutinesMemoryUsage` 는 5,000 개 goroutine 의 stack 총합이 1GB 를 넘지 않는지 확인한다. 만약 goroutine 하나가 1MB 씩 stack 을 썼다면 5GB 가 되어 이 테스트는 반드시 실패한다. `BenchmarkFillStack` 과 `BenchmarkSimulateBoundaryCrossing` 은 stack 성장 복사 비용과 작은 함수 호출 비용을 측정한다.

## 모르면 겪는 일

첫 번째 사고는 재귀가 조금만 깊어도 stack overflow 로 보이는 panic 을 보고 "goroutine 은 스택이 2KB 밖에 없다"고 오해하는 것이다. 실제로는 morestack 이 자동으로 stack 을 키우므로 일반적인 재귀에서는 stack overflow 가 잘 나지 않는다. 다만 정말 무한 재귀를 하면 결국 1GB 근처까지 자라다가 `runtime: goroutine stack exceeds 1000000000-byte limit` 같은 fatal error 가 난다. 이때 stack 이 자랐다가 줄어드는 과정을 모르면 원인을 추적하기 어렵다.

두 번째는 segment stack 시절에 나온 옛 자료를 보고 "Go stack 은 연결 리스트"라고 기억하는 경우다. 지금은 연속 stack 이므로 stack 주소가 연속된 범위에 있다. 이걸 모르고 포인터 연산으로 stack 주소를 비교하거나, stack 주소가 조각나 있다고 가정하는 로우레벨 코드를 짜면 잘못된 판단을 한다. 예를 들어 stack 변수 두 개의 주소 차이가 크다고 해서 그 사이가 전부 유효한 stack 이라고 가정하면 안 된다.

세 번째는 stack 복사가 포인터를 조정한다는 사실을 모른 채 unsafe 를 쓰는 경우다. 예를 들어 stack 에 있는 변수의 주소를 `uintptr` 로 변환해 heap 객체 어딘가에 저장해 두고, 그 뒤 깊은 재귀를 호출하면 런타입은 그 `uintptr` 값이 포인터인지 알 수 없어 조정하지 못한다. 나중에 그 주소를 참조하면 변경된 stack 주소가 아니라 옛 주소를 보게 되어, 값이 사라지거나 프로그램이 죽는다. 이런 버그는 보통 재귀 깊이가 특정 임계를 넘을 때만 나타나기 때문에 재현이 매우 어렵다.

네 번째는 stack shrink 시점을 모르고 메모리 프로파일을 오해하는 경우다. 깊은 재귀를 빠져나왔는데도 `StackInuse` 나 pprof 의 stack 메모리가 줄어들지 않아 보인다. GC 를 돌리기 전까지는 stack 이 유지되기 때문이다. 이걸 모르고 코어 덤프를 뜯으면 "메모리 누수"로 오진하게 된다. 특히 p99 지연 시간이 GC 주기마다 튀는데 CPU 프로파일에는 stack 복사 비용이 잘 안 잡히는 경우가 있다. stack 복사는 짧은 순간에 일어나므로 CPU 프로파일보다 실행 시간 히스토그램에서 더 잘 보인다.

## 언제 신경 쓰고 언제 무시하나

goroutine stack 은 대부분의 애플리케이션 코드에서 신경 쓸 필요가 없다. 함수 호출이 깊어도 런타임이 알아서 키우고, GC 가 알아서 줄인다. 2KB 시작이 부담되는 곳은 stack 을 실제로 많이 쓰는 워크로드, 예를 들어 XML 이나 JSON 을 깊은 재귀로 파싱하는 코드, 복잡한 AST 를 다루는 컴파일러류 코드다. 이런 곳에서는 stack 복사가 여러 번 일어날 수 있고, 더 큰 초기 stack 을 갖는 OS thread 가 아니라 goroutine 을 쓰면서 추가로 얻는 메모리 절약이 제한적일 수 있다.

이 지식이 중요한 규모는 goroutine 수가 수만 개를 넘거나, 재귀 깊이가 수천을 넘을 때다. 그 전에는 "goroutine 은 가벼우니까 많이 만들어도 된다" 정도의 직관으로 충분하다. 일부러 `GOMAXPROCS` 를 바꾸거나 stack 크기를 조정하려고 하지 않는 편이 낫다. Go 런타임은 stack 초기 크기를 사용자가 설정하는 공식적인 플래그를 제공하지 않으며, 억지로 바꾸면 오히려 메모리 사용량만 늘어난다.

unsafe 로 stack 주소를 다룰 때는 이 지식이 필수다. `runtime.KeepAlive`, `unsafe.Pointer` 와 `uintptr` 의 차이, stack 복사에서 포인터 조정이 되지 않는 경우를 정확히 알아야 한다. 이런 코드는 주로 syscall 이나 CGO 경계에서 나오는데, 학습용이 아니라면 작성하지 않는 것이 최선이다. 만약 작성해야 한다면 `go vet` 의 unsafe 경고와 `-race` 를 반드시 통과시켜야 한다.

## 더 파보기

- `runtime/stack.go` — `newstack`, `copystack`, `shrinkstack`, `adjustpointers` 의 실제 구현
- `runtime/runtime2.go` — `g` 구조체의 `stack`, `stackguard0` 필드 정의
- Go 언어 블로그 "The Go Memory Model" 과 "The Go scheduler" — goroutine stack 과 스케줄러의 관계
- Go 1.4 release notes 중 "Continuing the cleanup of the runtime" — segment stack 폐기와 연속 stack 전환 배경
- `runtime/stack_test.go` — stack 성장, 복사, shrink 를 검증하는 런타임 테스트
- `go test -gcflags="-m"` — 어떤 함수가 인라인되는지, stack escape analysis 결과를 보는 방법