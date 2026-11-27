# 채널 패턴: 파이프라인과 fan-out/fan-in

## 한 줄 요약
채널로 단계를 이어 붙이면 파이프라인, 한 단계를 여러 고루틴이 나눠 읽으면 fan-out, 그 결과를 한 채널로 합치면 fan-in 이고, 받는 쪽이 떠날 때를 위해 done 채널을 꼭 함께 둡니다.

## 왜 필요한가
데이터를 "읽기, 변환, 저장" 같은 단계로 처리하는 일은 어디에나 있습니다. Python 에서는 제너레이터를 겹쳐 쓰거나 `multiprocessing.Pool` 을 쓰고, JavaScript 에서는 스트림이나 `Promise.all`, Java 에서는 `Stream` 과 `ExecutorService` 를 씁니다. 대부분 프레임워크가 스레드 풀과 큐를 감춰 줍니다. 대신 취소와 에러 전파는 각 라이브러리의 규칙을 따로 배워야 합니다.

Go 는 이런 프레임워크를 언어에 넣지 않고, 고루틴과 채널이라는 작은 부품만 줬습니다. 단계 하나는 "입력 채널을 읽어 출력 채널에 쓰는 함수" 일 뿐이라 함수 합성처럼 이어 붙입니다. 대가는 책임이 개발자에게 있다는 것입니다. 누가 채널을 닫는지, 소비자가 중간에 떠나면 생산자는 어떻게 멈추는지를 직접 설계해야 합니다. 이 설계가 틀리면 에러도 없이 고루틴만 쌓이는 누수가 생깁니다.

## 기본 사용
단계의 기본 모양입니다. 출력 채널은 단계 안에서 만들고, 보내는 쪽이 `defer close` 로 닫습니다.

```go
package main

import "fmt"

func double(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in {
			out <- n * 2
		}
	}()
	return out
}

func main() {
	src := make(chan int)
	go func() {
		defer close(src)
		for i := 1; i <= 3; i++ {
			src <- i
		}
	}()
	for v := range double(src) {
		fmt.Println(v)
	}
}
```

fan-out 은 같은 입력 채널을 여러 고루틴이 `range` 로 읽기만 하면 됩니다. 값 하나는 정확히 한 작업자만 받습니다. fan-in 은 채널마다 고루틴을 두고 하나의 출력으로 보낸 뒤 `sync.WaitGroup` 으로 모두 끝나면 닫는 것입니다. 멈추는 방법은 `done` 채널입니다. 모든 send 를 `select` 로 감싸 `case <-done` 을 같이 기다리면, `close(done)` 한 번으로 모든 단계가 깨어납니다. 닫힌 채널은 읽을 때마다 즉시 반환되므로 "방송" 효과가 납니다. 실무에서는 같은 역할을 `context.Context` 의 `Done()` 이 합니다.

## 돌려보기
```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
go test -race ./...  # 데이터 레이스 검사
```
- `go vet ./...`: 아무 출력이 없으면 정상입니다.
- `go run .`: 파이프라인 결과가 입력 순서대로 나오는지, fan-out 1 명과 4 명의 결과가 같은지, 마지막 두 줄에서 실수 버전은 살아 있는 고루틴이 0 이 아니고 고친 버전은 0 인지 보세요.
- `go test -v ./...`: 각 `t.Run` 하위 테스트가 PASS 로 나오는지 보세요.
- `go test -race ./...`: `DATA RACE` 경고가 없어야 합니다.

## 실전에서는 이렇게 쓴다
첫째, 서버의 요청 처리입니다. `ctx.Done()` 을 done 채널 대신 쓰는 것이 표준입니다. 단계 함수 시그니처는 `func stage(ctx context.Context, in <-chan T) <-chan U` 형태가 되고, 모든 send 는 `select { case out <- v: case <-ctx.Done(): return }` 로 씁니다. 클라이언트가 연결을 끊으면 요청 컨텍스트가 취소되어 파이프라인 전체가 풀립니다.

둘째, 디렉터리를 훑어 파일 해시를 구하는 CLI 입니다. 한 고루틴이 경로를 내보내고, CPU 개수만큼의 작업자가 해시를 계산하고(fan-out), 하나의 수집 루프가 결과를 모읍니다(fan-in). 공식 블로그 글의 MD5 예제가 이 구조입니다.

셋째, 작업자 수를 제한하는 용도로도 쓰입니다. 작업자 N 개가 하나의 작업 채널을 읽는 구조라, 입력이 아무리 많아도 동시 실행은 N 개입니다. 외부 API 호출 횟수를 제한할 때 흔히 씁니다. 이미 만들어진 도구로는 `golang.org/x/sync/errgroup` 이 있습니다. 이 글의 예제는 표준 라이브러리만 씁니다.

## 흔한 실수

### 1. done 없이 일부만 읽고 떠나기 (고루틴 누수)
```go
// 틀린 코드
package main

import "fmt"

func gen(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, n := range nums {
			out <- n // 받는 사람이 없으면 여기서 영원히 멈춤
		}
	}()
	return out
}

func main() {
	fmt.Println(<-gen(1, 2, 3)) // 첫 값만 읽고 끝
}
```
에러도 panic 도 없이 잘 끝난 것처럼 보입니다. 하지만 `gen` 고루틴은 두 번째 send 에서 영원히 막혀 있습니다. 짧은 프로그램에서는 티가 안 나지만 서버에서는 요청마다 고루틴과 메모리가 쌓입니다. `main.go` 의 `LeakyFirst` 가 이것을 재현하고, 출력에서 살아 있는 고루틴이 0 이 아닌 것으로 확인됩니다. 채널의 send 는 받는 쪽이 있어야 끝나는데, 받는 쪽이 사라졌기 때문입니다. 고친 코드는 done 을 두고 send 를 `select` 로 감쌉니다.

```go
package main

import "fmt"

func gen(done <-chan struct{}, nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, n := range nums {
			select {
			case out <- n:
			case <-done:
				return
			}
		}
	}()
	return out
}

func main() {
	done := make(chan struct{})
	defer close(done) // 떠날 때 반드시 닫는다
	fmt.Println(<-gen(done, 1, 2, 3))
}
```

### 2. 받는 쪽에서 닫거나 여러 곳에서 닫기
```go
// 틀린 코드
package main

func main() {
	c := make(chan int)
	close(c)
	c <- 1 // panic: send on closed channel
}
```
닫힌 채널에 send 하면 `panic: send on closed channel`, 이미 닫은 채널을 또 닫으면 `panic: close of closed channel` 입니다. 채널은 "더 보낼 것이 없다" 를 알리는 신호이므로, 보내는 쪽만 닫을 수 있고 보내는 쪽이 여럿이면 누가 마지막인지 알아야 합니다. fan-in 에서 작업자 고루틴이 각자 `close(out)` 하면 이 panic 이 납니다. 고친 방법은 `WaitGroup` 으로 모두 끝나기를 기다린 하나의 고루틴만 닫는 것입니다.

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	out := make(chan int)
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out <- i
		}()
	}
	go func() {
		wg.Wait()
		close(out) // 닫는 곳은 여기 한 군데
	}()
	sum := 0
	for v := range out {
		sum += v
	}
	fmt.Println(sum)
}
```

### 3. merge 에서 wg.Wait 을 바로 호출하기 (교착)
```go
// 틀린 코드
package main

import "sync"

func main() {
	out := make(chan int)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		out <- 1 // 읽는 사람이 아직 없음
	}()
	wg.Wait() // 읽기 전에 기다리면 서로 기다린다
	close(out)
}
```
결과는 `fatal error: all goroutines are asleep - deadlock!` 입니다. 작업자는 out 을 읽어 줄 사람을 기다리고, main 은 작업자가 끝나기를 기다립니다. 그래서 `Wait` 와 `close` 는 별도 고루틴에서 부르고, 호출한 쪽은 곧바로 out 을 읽어야 합니다. 이 글의 `merge` 가 그렇게 합니다.

### 4. fan-out 결과가 입력 순서일 것이라 믿기
```go
// 틀린 코드
got := FanOut([]int{1, 2, 3, 4}, 4)
// got 이 항상 [1 4 9 16] 일 거라고 가정하고 비교
```
작업자가 여럿이면 먼저 끝난 것이 먼저 나옵니다. 어쩌다 순서가 맞는 날이 있어서 테스트가 가끔만 실패하는 불안정한 상태가 됩니다. 순서가 필요하면 값에 인덱스를 붙여 보내고 수집 쪽에서 자리를 맞추거나, 이 글처럼 정렬해서 비교하세요.

### 5. 취소를 반복문 맨 위에서만 확인하기
```go
// 틀린 코드
package main

func stage(done <-chan struct{}, in <-chan int, out chan<- int) {
	for n := range in {
		select {
		case <-done:
			return
		default:
		}
		out <- n // 여기서 막히면 done 이 닫혀도 모름
	}
}

func main() {}
```
검사한 직후에 `out <- n` 에서 막히면 done 을 볼 기회가 없습니다. send 와 done 을 같은 `select` 에 넣어야 둘 중 먼저 되는 쪽으로 풀립니다.

## 주의할 점
- Go 1.22 부터 `for` 의 루프 변수는 반복마다 새로 만들어집니다. 그래서 작업자를 띄우는 반복문에서 `go func() { use(i) }()` 가 안전합니다. 1.21 이하(또는 `go.mod` 의 go 버전이 낮은 경우)에서는 `i := i` 복사가 필요합니다.
- 채널은 비용이 없지 않습니다. 값 하나를 옮길 때마다 동기화가 일어나므로 작업이 아주 가벼우면 채널을 쓰는 쪽이 더 느릴 수 있습니다. 작업이 충분히 무거울 때(I/O, 해시, 파싱) 나눠 처리하세요. 성능은 직접 측정하세요.
- 버퍼 채널은 교착과 누수를 감춥니다. 버퍼가 차기 전까지는 send 가 막히지 않아서, 부하가 커진 뒤에야 문제가 나타납니다. 먼저 버퍼 없는 채널로 올바르게 만들고, 필요할 때만 버퍼를 줍니다.
- nil 채널은 영원히 막힙니다. `select` 에서 nil 채널의 case 는 선택되지 않으므로 의도적으로 case 를 끌 때 유용하지만, 초기화를 잊은 채널과 구분이 어렵습니다. 이 글의 시연은 일부러 done 에 nil 을 넘겨 "취소 없는 버전" 을 만듭니다.
- 에러 전파는 직접 설계해야 합니다. 값과 에러를 함께 담는 구조체를 채널로 보내거나 `errgroup` 을 쓰세요.

## 코드로 확인하기
`go run .` 의 앞 세 줄은 정상 동작입니다. 파이프라인은 단계마다 고루틴이 하나라 입력 순서가 유지되고, fan-out 은 작업자가 1 명이든 4 명이든 정렬 후 결과가 같습니다. 마지막 두 줄이 핵심입니다. 「실수 버전」은 done 이 없어 첫 값만 받고 떠난 뒤에도 `gen` 고루틴이 살아 있어 그 수가 0 이 아닙니다. 「고친 버전」은 `close(done)` 뒤에 모든 단계가 반환해 0 으로 내려갑니다.

`main_test.go` 는 고친 버전과 기본 동작만 검증합니다. 파이프라인 순서와 빈 입력, 여러 작업자 수에서의 fan-out 결과, `SafeFirst` 후 고루틴이 남지 않는지, 입력이 없을 때 merge 가 교착 없이 끝나는지를 봅니다.

## 연습 문제
1. 쉬움: `gen` 과 `sq` 사이에 "짝수만 통과시키는" 단계를 하나 더 끼워 보세요. (힌트: 입력을 `range` 로 읽고 조건이 맞을 때만 `select` 로 send 하세요.)
2. 보통: `done` 채널을 `context.Context` 로 바꿔 보세요. (힌트: `ctx.Done()` 을 쓰고, 테스트에서는 `context.WithCancel` 로 취소하세요.)
3. 어려움: 결과를 입력 순서대로 돌려주는 fan-out 을 만드세요. (힌트: 값과 인덱스를 구조체로 묶어 보내고, 수집 쪽에서 인덱스 자리에 채우세요.)

## 더 읽기
- Go Blog, Go Concurrency Patterns: Pipelines and cancellation: https://go.dev/blog/pipelines
- Go Blog, Go Concurrency Patterns: Context: https://go.dev/blog/context
- Effective Go, Concurrency: https://go.dev/doc/effective_go#concurrency