## 한 줄 요약

`go f()` 는 함수를 다른 고루틴(goroutine)에서 실행하고 곧바로 돌아옵니다. 그런데 main 이 끝나면 아직 일하던 고루틴도 전부 죽으므로, 끝날 때까지 기다려야 한다면 `sync.WaitGroup` 을 쓰고, `Add` 는 반드시 `go` 문 앞에서 부릅니다.

## 왜 필요한가

여러 일을 동시에 진행하는 것은 서버와 CLI 도구 어디서나 필요합니다. 파일 100개 읽기, API 여러 곳 호출, 요청마다 따로 처리하기 같은 경우입니다. Java 에서는 `Thread` 를 만들거나 `ExecutorService` 에 작업을 제출하고 `Future.get()` 이나 `awaitTermination` 으로 기다립니다. JavaScript 는 단일 스레드 이벤트 루프 위에서 `async/await` 와 `Promise.all` 로 기다립니다. Python 은 `threading.Thread.join()` 이나 `asyncio.gather` 를 씁니다. 모두 "작업을 시작한다"와 "끝나길 기다린다"가 별개의 API 라는 점은 같습니다.

Go 는 이 둘을 언어 수준에서 아주 가볍게 만들었습니다. 시작은 키워드 `go` 하나입니다. 스레드 풀을 만들 필요가 없고, 고루틴은 OS 스레드보다 훨씬 가벼워서 수천 개를 띄우는 것도 흔합니다. 반면 "기다림"은 언어가 대신해 주지 않습니다. Java 의 `Future` 처럼 반환값을 들고 있는 객체도 없습니다. 기다리는 도구는 표준 라이브러리의 `sync.WaitGroup`(카운터 기반)이나 채널입니다.

그래서 Go 초심자는 두 가지에서 넘어집니다. 하나는 main 이 고루틴을 기다리지 않고 끝나 버리는 것입니다. 프로그램은 main 함수가 반환되는 순간 종료되고, 실행 중이던 고루틴은 정리 없이 사라집니다. 다른 하나는 WaitGroup 카운터를 올리는 `Add` 의 위치입니다. 이 자료는 이 두 가지를 직접 돌려 보며 확인하는 것이 목표입니다.

## 기본 사용

핵심 패턴은 세 줄입니다. 고루틴을 만들기 전에 `Add(1)`, 고루틴 안에서 `defer wg.Done()`, 다 띄운 뒤 `wg.Wait()`.

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	results := make([]string, 3)
	for i := range 3 {
		wg.Add(1) // go 문 앞에서
		go func() {
			defer wg.Done() // 어떻게 끝나든 반드시 호출
			results[i] = fmt.Sprintf("작업 %d 완료", i)
		}()
	}
	wg.Wait() // 카운터가 0 이 될 때까지 블록
	fmt.Println(results)
}
```

알아 둘 것은 다음과 같습니다.

- `Add(n)` 은 카운터를 올리고 `Done()` 은 1 내립니다. `Wait()` 는 0 이 될 때까지 막습니다.
- 고루틴의 실행 순서는 보장되지 않습니다. 순서가 필요하면 위처럼 인덱스에 결과를 쓰거나 채널을 씁니다.
- 고루틴마다 서로 다른 인덱스에 쓰는 것은 안전하지만, 같은 변수를 여럿이 고치면 `sync.Mutex` 가 필요합니다. main.go 의 `sumWithMutex` 를 보세요.
- Go 1.25 이상이라면 `wg.Go(func() {...})` 로 `Add` 와 `Done` 을 한 번에 처리할 수 있습니다. 어느 방식이든 원리는 같으니 먼저 직접 쓰는 형태를 익히세요.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

- `go vet ./...`: 아무 출력이 없어야 정상입니다. WaitGroup 을 값으로 복사하는 실수 같은 것을 여기서 잡아 줍니다.
- `go run .`: 마지막 세 줄이 핵심입니다. 두 「실수 버전」의 끝난 고루틴 수와 「고친 버전」의 수를 비교해 보세요. 앞의 두 줄은 기본 사용 결과입니다.
- `go test -v ./...`: `TestRunWorkers`, `TestSumWithMutex`, `TestFixedAddBefore` 가 모두 PASS 인지 확인하세요. `t.Run` 으로 나뉜 하위 테스트도 함께 보입니다.
- `go test -race ./...`: 경고 없이 PASS 여야 합니다. 이 줄이 통과한다는 것은 공유 데이터 접근이 안전하게 짜였다는 뜻입니다.

(`go.mod` 가 없다면 먼저 `go mod init goroutine-demo` 를 한 번 실행하세요. `for range 5` 같은 문법은 Go 1.22 이상이 필요합니다.)

## 실전에서는 이렇게 쓴다

**1. 같은 일을 여러 대상에게 병렬로.** 여러 URL 을 점검하거나 여러 파일을 처리하는 CLI 의 전형입니다. 대상마다 고루틴을 하나 띄우고, 결과는 미리 만든 슬라이스의 자기 인덱스에 씁니다. main.go 의 `runWorkers` 가 바로 이 모양입니다. 결과 순서가 입력 순서와 같다는 장점도 있습니다.

**2. 서버 종료 시 진행 중인 작업 기다리기.** 서버가 백그라운드 작업 고루틴을 `wg` 로 추적해 두고, 종료 신호를 받으면 새 작업 접수를 멈춘 뒤 `wg.Wait()` 로 진행 중인 작업이 끝나기를 기다립니다. 표준 라이브러리의 `http.Server.Shutdown` 도 같은 발상으로 진행 중인 연결을 기다립니다.

**3. 에러가 필요하면 errgroup.** 실무에서는 `golang.org/x/sync/errgroup` 을 자주 씁니다. WaitGroup 에 "첫 번째 에러 반환"과 취소를 더한 것입니다. 이것도 `Wait` 로 기다린다는 점은 같으니 WaitGroup 을 먼저 이해하면 됩니다. (이 자료의 코드는 표준 라이브러리만 씁니다.)

## 흔한 실수

### 1. main 이 기다리지 않고 끝난다

```go
// 틀린 코드
package main

import "fmt"

func main() {
	for i := range 3 {
		go fmt.Println("고루틴", i)
	}
	fmt.Println("main 끝")
}
```

고루틴이 시작되기도 전에 main 이 끝나서, 실행하면 "고루틴 ..." 줄이 하나도 안 나오거나 일부만 나옵니다. 몇 번 돌려도 결과가 달라질 수 있습니다. 이유는 프로그램의 수명이 main 고루틴의 수명과 같기 때문입니다. `go` 문은 "나중에 실행해 달라"고 스케줄러에 맡길 뿐 완료를 보장하지 않습니다. 이것을 `time.Sleep` 으로 때우는 것도 안 됩니다. 얼마나 기다려야 하는지 알 수 없고, 기계가 느리면 다시 깨집니다.

main.go 의 `mistakeNoWait` 는 이 상황을 항상 같은 결과로 재현합니다. 고루틴을 막아 놓고 바로 세므로 끝난 수는 0 입니다.

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fmt.Println("고루틴", i)
		}()
	}
	wg.Wait() // 여기서 세 고루틴이 끝나길 기다린다
	fmt.Println("main 끝")
}
```

### 2. Add 를 고루틴 안에서 부른다

```go
// 틀린 코드
var wg sync.WaitGroup
for range 10 {
	go func() {
		wg.Add(1) // 고루틴이 늦게 시작하면 이미 늦었다
		defer wg.Done()
		work()
	}()
}
wg.Wait()
```

`Wait` 가 일찍 반환되어 일이 끝나기 전에 다음 코드로 넘어갑니다. 에러 메시지도 없이 조용히 틀립니다. 가끔은 잘 돌아서 더 위험합니다. 왜냐하면 main 이 `Wait` 에 도달한 시점에 아직 어떤 고루틴도 `Add` 를 실행하지 못했다면 카운터는 0 이고, 0 이면 `Wait` 는 기다릴 것이 없다고 판단하기 때문입니다. 카운터를 올리는 일은 "고루틴을 띄우는 쪽"이 먼저 끝내 둬야 합니다. main.go 의 `mistakeAddInside` 는 고루틴이 늦게 시작하는 상황을 만들어 이 실수를 항상 재현하고, 출력에서 끝난 수가 0 임을 보여 줍니다.

```go
package main

import "sync"

func work() {}

func main() {
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1) // 고루틴을 만들기 전에, 이 흐름에서
		go func() {
			defer wg.Done()
			work()
		}()
	}
	wg.Wait()
}
```

### 3. WaitGroup 을 값으로 넘긴다

```go
// 틀린 코드
func worker(id int, wg sync.WaitGroup) { // 복사본을 받는다
	defer wg.Done()
	fmt.Println("worker", id)
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go worker(1, wg)
	wg.Wait()
}
```

실행하면 `fatal error: all goroutines are asleep - deadlock!` 로 죽습니다. `worker` 가 받은 것은 복사본이라 거기서 `Done` 을 해도 main 의 카운터는 그대로 1 이기 때문입니다. `go vet` 도 "passes lock by value" 라고 경고해 줍니다. 그래서 vet 를 습관적으로 돌리라고 하는 것입니다. 고칠 때는 포인터로 넘깁니다.

```go
package main

import (
	"fmt"
	"sync"
)

func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Println("worker", id)
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go worker(1, &wg)
	wg.Wait()
}
```

### 4. Add 와 Done 의 횟수가 맞지 않는다

```go
// 틀린 코드
var wg sync.WaitGroup
wg.Add(1)
go func() {
	defer wg.Done()
	wg.Done() // 실수로 한 번 더
}()
wg.Wait()
```

운이 나쁘면 `panic: sync: negative WaitGroup counter` 가 납니다. 반대로 `Add` 가 `Done` 보다 많으면 `Wait` 가 영원히 끝나지 않아 deadlock 이 됩니다. `Done` 은 항상 `defer wg.Done()` 한 줄로 고루틴 맨 위에서 걸어 두세요. 그러면 `return` 이 여러 군데 있어도, 중간에 에러가 나도 정확히 한 번만 호출됩니다.

### 5. 결과를 공유 슬라이스에 append 한다

```go
// 틀린 코드
var results []int
var wg sync.WaitGroup
for i := range 100 {
	wg.Add(1)
	go func() {
		defer wg.Done()
		results = append(results, i) // 여러 고루틴이 동시에 results 를 고친다
	}()
}
wg.Wait()
fmt.Println(len(results)) // 100 이 아닐 수 있다
```

`append` 는 슬라이스 헤더(길이, 용량, 배열 포인터)를 읽고 다시 쓰는 여러 단계의 작업이라 동시에 부르면 일부 값이 사라집니다. 에러 메시지는 없고 길이가 100 보다 작게 나올 수 있습니다. `go run -race .` 로 돌리면 `WARNING: DATA RACE` 가 나옵니다. 해결은 두 가지입니다. 미리 `make([]int, 100)` 으로 크기를 잡고 `results[i]` 에 쓰거나(main.go 의 `runWorkers`), `Mutex` 로 `append` 를 감싸세요.

### 6. time.Sleep 으로 기다린다

```go
// 틀린 코드
go doWork()
time.Sleep(time.Second) // 1초면 되겠지
```

작업이 1초보다 오래 걸리면 중간에 잘리고, 훨씬 빨리 끝나도 1초를 낭비합니다. 테스트에서는 CI 가 느려지는 날에만 실패하는 플레이키(flaky) 테스트가 됩니다. 기다림은 시간이 아니라 "완료 신호"(WaitGroup, 채널)로 해결하세요.

## 주의할 점

- **Go 1.22 의 루프 변수 변경.** 1.22 이전에는 `for i := 0; i < n; i++` 의 `i` 가 루프 전체에서 하나였습니다. 그래서 고루틴 안에서 `i` 를 캡처하면 모두 마지막 값을 보는 일이 흔했고, `go func(i int){...}(i)` 처럼 인자로 넘기는 것이 관용구였습니다. 1.22 부터는 반복마다 새 변수라 이 문제가 사라졌습니다. 다만 `go.mod` 의 `go` 버전이 1.22 미만이면 옛 동작이 유지되니 오래된 프로젝트에서는 주의하세요.
- **실행 순서는 약속이 없다.** 출력 순서가 우연히 0, 1, 2 로 나와도 믿으면 안 됩니다. 순서에 의존하는 코드는 환경이 바뀌면 깨집니다. 테스트에서도 순서가 아니라 결과의 집합이나 인덱스별 값을 검증하세요.
- **고루틴 안의 panic 은 프로그램 전체를 죽인다.** main 에서 `recover` 를 걸어 두어도 다른 고루틴의 panic 은 잡히지 않습니다. 각 고루틴 안에서 직접 `recover` 해야 합니다. 다른 언어의 스레드 예외와 달리 "그 고루틴만 죽는" 일은 없습니다.
- **WaitGroup 재사용 규칙.** 카운터가 0 으로 돌아온 뒤에 다시 쓰는 것은 괜찮지만, 이전 `Wait` 가 완전히 끝나기 전에 새 `Add` 를 하면 문제가 생길 수 있습니다. 단순하게 일 하나에 WaitGroup 하나를 쓰는 편이 안전합니다.
- **`Wait` 는 에러를 모른다.** 어떤 고루틴이 실패했는지 알려 주지 않습니다. 에러가 필요하면 에러 슬라이스(인덱스별)나 채널로 모으거나 errgroup 을 쓰세요.
- **끝나지 않는 고루틴은 누수다.** 채널에서 영원히 기다리는 고루틴은 `Wait` 도 영원히 막고, 막히지 않더라도 메모리를 계속 차지합니다. 고루틴을 띄울 때는 "이게 언제 끝나지?"를 항상 같이 생각하세요. main.go 의 시연 함수들이 `gate` 를 닫고 `cleanup` 으로 정리하는 이유입니다.

## 코드로 확인하기

`go run .` 의 앞 두 줄은 기본 사용입니다. 제곱 결과는 `[0 1 4 9 16]` 처럼 인덱스 순서대로 나오고, Mutex 합계는 고루틴 수 곱하기 1000 과 정확히 같습니다. 고루틴 실행 순서는 제각각이어도 결과가 같은 이유는 서로 다른 칸에만 쓰거나 락으로 보호했기 때문입니다.

뒤의 세 줄이 이 주제의 핵심입니다.

- 「실수 버전 (Wait 없음)」 은 기다리지 않고 센 값이 0 입니다. 고루틴을 시작하는 것과 끝나는 것은 별개의 사건이기 때문입니다.
- 「실수 버전 (Add 를 고루틴 안에서)」 도 `Wait` 직후 끝난 수가 0 입니다. `Wait` 가 기다릴 것이 있는지 판단하는 시점에 카운터가 아직 0 이었기 때문입니다. 실제 프로그램에서는 이 타이밍이 운에 달려 있어 가끔 맞고 가끔 틀리는 버그가 됩니다. 여기서는 `gate` 로 그 운을 고정해 매번 틀리게 만들었습니다.
- 「고친 버전」 은 `Wait` 직후 10 개가 모두 끝나 있습니다.

`main_test.go` 는 「고친 버전」과 기본 동작만 검증합니다. `TestRunWorkers` 는 테이블 주도 형식으로 n 이 0, 1, 5 일 때 결과 슬라이스를 비교하고, `TestSumWithMutex` 는 합계를, `TestFixedAddBefore` 는 `Wait` 직후 모두 끝났음을 확인합니다. 시간에 의존하는 단정은 쓰지 않았습니다.

## 연습 문제

1. (쉬움) 슬라이스 `[]string{"a","b","c"}` 의 각 원소를 고루틴에서 대문자로 바꿔 같은 인덱스에 저장하고, WaitGroup 으로 기다린 뒤 출력해 보세요. 힌트: `strings.ToUpper` 와 인덱스 쓰기를 쓰면 락이 필요 없습니다.
2. (보통) `wg.Add(1)` 을 고루틴 안으로 옮기고 `go run -race .` 와 `go vet ./...` 를 돌려 어떤 일이 벌어지는지 관찰해 보세요. 힌트: 고루틴 시작 전에 `time.Sleep` 을 넣은 뒤와 안 넣은 뒤의 결과를 비교해 보세요.
3. (어려움) 고루틴 100 개를 띄우되 동시에 실행되는 것은 최대 5 개로 제한해 보세요. 힌트: 버퍼 크기가 5 인 채널을 세마포어처럼 쓰고, 고루틴 시작 전에 칸을 채우고 끝나면 비웁니다.

## 더 읽기

- Go Tour, 동시성 시작: https://go.dev/tour/concurrency/1
- `sync.WaitGroup` 문서: https://pkg.go.dev/sync#WaitGroup
- Go 1.22 루프 변수 변경 설명: https://go.dev/blog/loopvar-preview
- Go 동시성 패턴(파이프라인과 취소): https://go.dev/blog/pipelines