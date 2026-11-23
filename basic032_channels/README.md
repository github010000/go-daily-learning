## 한 줄 요약

채널은 고루틴끼리 값을 주고받는 통로이고, 닫는 일은 보내는 쪽이 맡으며, 받는 쪽이 없는 채널에 보내면 그 고루틴은 영원히 멈춘다.

## 왜 필요한가

여러 실행 흐름이 데이터를 주고받으려면 보통 공유 변수와 락을 씁니다. Java 는 `synchronized` 와 `BlockingQueue`, Python 은 `threading.Lock` 과 `queue.Queue`, JavaScript 는 단일 스레드라 락 대신 `Promise` 와 `async/await` 로 순서를 맞춥니다. 공통점은 "값을 넘기는 일"과 "기다리는 일"이 따로 놀아서, 락을 잡는 순서나 콜백 중첩 때문에 코드가 복잡해진다는 점입니다.

Go 는 이 둘을 한 연산으로 묶었습니다. `ch <- v` 는 값을 넘기면서 동시에 기다림을 표현합니다. 그래서 "메모리를 공유해서 통신하지 말고, 통신으로 메모리를 공유하라"는 말이 나옵니다. 큐 라이브러리가 아니라 언어 문법(`<-`, `close`, `range`, `select`)이기 때문에, 닫힘이나 타임아웃도 같은 문법 안에서 다룹니다.

대신 Go 는 안전장치를 많이 주지 않습니다. 보내는 쪽이 사라진 채널, 받는 쪽이 사라진 채널은 컴파일러가 잡아 주지 않고, 실행 중에 멈춤(deadlock)이나 누수로 나타납니다. Python 의 `Queue` 는 타임아웃을 안 주면 마찬가지로 막히지만, Go 에는 `close` 라는 "더 보낼 것이 없다"는 신호가 내장되어 있습니다. 이 신호를 누가, 언제 보내는지가 이 주제의 핵심 규칙입니다.

## 기본 사용

버퍼 없는 채널(`make(chan T)`)은 보내는 쪽과 받는 쪽이 만나야 값이 넘어갑니다. 버퍼 있는 채널(`make(chan T, n)`)은 칸이 남아 있는 동안 보내는 쪽이 기다리지 않습니다.

```go
package main

import "fmt"

func main() {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	fmt.Println(len(ch), cap(ch)) // 들어 있는 개수, 칸 수
	close(ch)                     // 보내는 쪽이 닫는다
	for v := range ch {           // 닫히고 비면 루프가 끝난다
		fmt.Println(v)
	}
	v, ok := <-ch // 닫힌 채널: 제로값과 ok=false
	fmt.Println(v, ok)
}
```

규칙은 세 가지입니다.

- `close` 는 보내는 쪽이 합니다. 받는 쪽은 채널이 닫힐 예정인지 알 수 없기 때문입니다.
- 닫힌 채널에서 받으면 남은 값을 먼저 주고, 그 뒤에는 제로값과 `ok == false` 를 즉시 줍니다.
- 닫는 일은 "더 보낼 것이 없다"는 알림일 뿐, 필수 정리 작업이 아닙니다. `range` 로 끝을 기다리는 쪽이 있을 때만 필요합니다.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
go test -race ./...  # 데이터 레이스 검사
```

- `go vet ./...`: 아무것도 출력되지 않으면 정상입니다.
- `go run .`: 「실수 버전(ok 무시, 4번 받기)」 줄에 제로값이 섞여 나오는지, 「닫힌 채널에 보내기」 줄에 panic 메시지가 찍히는지, 「버퍼 0」 일꾼과 「버퍼 1」 일꾼의 `끝났나` 값이 서로 다른지 보세요.
- `go test -v ./...`: `TestProducerSum` 아래에 `t.Run` 으로 나뉜 하위 테스트가 줄지어 PASS 로 나옵니다.
- `go test -race ./...`: 경고 없이 `ok` 로 끝나야 합니다.

## 실전에서는 이렇게 쓴다

**1. 생산자가 채널을 만들어 읽기 전용으로 돌려준다.** 반환 타입을 `<-chan int` 로 두면 호출자는 보내거나 닫을 수 없으므로 "닫는 건 보내는 쪽"이라는 규칙이 타입으로 강제됩니다. main.go 의 `producer` 가 이 모양입니다.

```go
package main

import "fmt"

func gen(n int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := range n {
			out <- i
		}
	}()
	return out
}

func main() {
	for v := range gen(3) {
		fmt.Println(v)
	}
}
```

**2. 결과 하나를 기다리되 타임아웃을 두는 패턴.** 일꾼 고루틴의 결과 채널을 버퍼 1 로 만들면, 호출자가 타임아웃으로 포기해도 일꾼은 보내고 끝납니다. 표준 라이브러리와 많은 서버 코드가 이 모양을 씁니다.

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	res := make(chan string, 1) // 버퍼 1: 아무도 안 받아도 일꾼이 끝난다
	go func() { res <- "완료" }()
	select {
	case v := <-res:
		fmt.Println(v)
	case <-time.After(time.Second):
		fmt.Println("시간 초과")
	}
}
```

**3. `chan struct{}` 를 신호용으로 쓴다.** `context.Context` 의 `Done()` 도 닫히는 채널입니다. 닫힌 채널은 받는 쪽 전부에게 동시에 즉시 알려 주므로, "모두 멈춰라" 방송에 좋습니다. `sync.WaitGroup` 과 함께 쓰는 워커 풀도 같은 원리입니다.

## 흔한 실수

### 1. 받는 쪽에서 close 하고, 보내는 쪽이 계속 보낸다

```go
// 틀린 코드
package main

func main() {
	ch := make(chan int, 1)
	close(ch)
	ch <- 1
}
```

실행하면 `panic: send on closed channel` 이 납니다. 닫힌 채널에 보내는 일은 항상 panic 이고, 받는 쪽은 보내는 쪽이 아직 보낼 것이 남았는지 알 수 없습니다. 같은 이유로 이미 닫은 채널을 또 닫으면 `panic: close of closed channel` 입니다. 고치는 방법은 닫는 책임을 보내는 쪽에 두는 것입니다. 보내는 고루틴이 여럿이면 `sync.WaitGroup` 으로 전부 끝난 뒤 한 곳에서 닫으세요.

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	ch := make(chan int)
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- i
		}()
	}
	go func() {
		wg.Wait() // 보내는 쪽이 전부 끝난 뒤에만 닫는다
		close(ch)
	}()
	total := 0
	for v := range ch {
		total += v
	}
	fmt.Println(total)
}
```

### 2. 닫힌 채널에서 받은 제로값을 진짜 데이터로 착각한다

```go
// 틀린 코드
package main

import "fmt"

func main() {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	close(ch)
	for range 4 {
		fmt.Println(<-ch) // 1, 2 다음에 0 이 계속 나온다
	}
}
```

에러도 panic 도 없이 0 이 나옵니다. 그래서 더 위험합니다. 실제 값이 0 인지 닫혀서 나온 0 인지 구분할 수 없기 때문입니다. `main.go` 의 「실수 버전」 줄에서 확인할 수 있습니다. `v, ok := <-ch` 로 `ok` 를 보거나, 값을 전부 받을 때까지는 `for v := range ch` 를 쓰세요.

```go
package main

import "fmt"

func main() {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	close(ch)
	for v := range ch {
		fmt.Println(v)
	}
}
```

### 3. 아무도 안 받는 채널에 보내서 영원히 멈춘다

```go
// 틀린 코드
package main

func main() {
	ch := make(chan int)
	ch <- 1 // 받는 쪽이 없다
}
```

main 고루틴이 막히고 다른 고루틴도 모두 잠들면 런타임이 `fatal error: all goroutines are asleep - deadlock!` 로 프로그램을 끝냅니다. 더 나쁜 경우는 다른 고루틴이 살아 있어서 deadlock 감지가 안 되는 서버입니다. 이때는 오류 없이 고루틴만 쌓이는 누수가 됩니다. 요청마다 일꾼을 띄우고, 호출자가 타임아웃으로 먼저 떠나 버리면, 버퍼 없는 채널에 보내려던 일꾼은 영원히 남습니다. 고치는 방법은 결과 채널에 버퍼를 한 칸 주는 것(일꾼이 보내고 바로 끝남), 또는 `select` 에서 `ctx.Done()` 도 함께 기다리는 것입니다.

```go
package main

import "fmt"

func main() {
	ch := make(chan int, 1) // 받는 쪽이 없어도 한 번은 보낼 수 있다
	ch <- 1
	fmt.Println(<-ch)
}
```

### 4. 값을 다 보내고도 close 를 안 해서 range 가 끝나지 않는다

```go
// 틀린 코드
package main

import "fmt"

func main() {
	ch := make(chan int)
	go func() {
		for i := range 3 {
			ch <- i
		}
		// close(ch) 를 빠뜨렸다
	}()
	for v := range ch {
		fmt.Println(v)
	}
}
```

세 값을 출력한 뒤 `fatal error: all goroutines are asleep - deadlock!` 이 납니다. `range` 는 채널이 닫혀야 끝나는데, 아무도 닫지 않았기 때문입니다. 보내는 고루틴 맨 위에 `defer close(ch)` 를 두는 습관이 가장 안전합니다.

```go
package main

import "fmt"

func main() {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := range 3 {
			ch <- i
		}
	}()
	for v := range ch {
		fmt.Println(v)
	}
}
```

### 5. nil 채널을 닫거나, 초기화 안 된 채널에 보낸다

```go
// 틀린 코드
package main

func main() {
	var ch chan int // make 를 안 했다. 값은 nil
	close(ch)
}
```

`close` 는 `panic: close of nil channel` 입니다. 반면 nil 채널에 보내거나 받으면 panic 이 아니라 영원히 막힙니다. 선언만 하고 `make` 를 잊는 것이 원인이므로, `ch := make(chan int)` 로 선언과 생성을 한 번에 하세요.

## 주의할 점

- **`len` 과 `cap` 으로 흐름 제어를 하지 마세요.** `len(ch)` 를 읽는 순간 다른 고루틴이 값을 넣거나 뺄 수 있어서, 확인 뒤의 행동이 이미 낡은 정보에 기댑니다. 가득 찼는지 알고 싶으면 `select` 에 `default` 를 둔 비차단 송신을 쓰세요(`bufferedIsFull` 참고).
- **버퍼는 문제를 숨길 수 있습니다.** 버퍼를 크게 키워 deadlock 이 사라진 것처럼 보여도, 받는 쪽이 느리면 결국 같은 곳에서 막힙니다. 버퍼 크기는 "왜 이 크기인지" 설명할 수 있는 값(예: 일꾼 수, 1)으로 두세요.
- **Go 1.22 이후 루프 변수는 반복마다 새로 만들어집니다.** 위 `WaitGroup` 예제처럼 `for i := range 3` 안의 고루틴이 `i` 를 그대로 써도 각자 자기 값을 봅니다. 그 이전 버전 코드에서는 `i := i` 복사가 필요했으니 옛 코드를 읽을 때 놀라지 마세요.
- **`time.After` 는 루프 안에서 매번 만들면 부담이 됩니다.** Go 1.23 부터는 참조가 끊긴 타이머를 GC 가 회수하지만, 오래 도는 루프에서는 `time.NewTimer` 를 재사용하거나 `context.WithTimeout` 을 쓰는 것이 더 명확합니다.
- **모든 채널을 닫을 필요는 없습니다.** 채널은 GC 가 수거합니다. 닫기는 받는 쪽에게 끝을 알리는 용도이고, 파일의 `Close` 처럼 자원 해제가 아닙니다.
- **`select` 에서 여러 case 가 준비되면 무작위로 하나를 고릅니다.** Python 의 `if/elif` 처럼 위에서부터 우선순위가 매겨지지 않습니다.

## 코드로 확인하기

`go run .` 출력에서 먼저 볼 줄은 「실수 버전(ok 무시, 4번 받기)」 과 「고친 버전(range)」 입니다. 앞은 값 2개 뒤에 제로값이 붙어 4개가 되고, 뒤는 실제로 보낸 2개에서 멈춥니다. 같은 닫힌 채널인데 `ok` 와 `range` 가 닫힘을 알아채는지가 차이를 만듭니다.

「닫힌 채널에 보내기」 줄은 recover 로 잡은 panic 메시지를 보여 줍니다. 마지막 두 줄은 같은 일꾼 코드에서 채널 버퍼만 0 과 1 로 바꾼 결과입니다. 버퍼 0 일꾼은 받는 쪽이 없어서 `끝났나` 가 `false` 이고(누수 상태), 버퍼 1 일꾼은 `true` 입니다. 시연이 끝날 때는 `<-ch` 로 누수된 고루틴을 풀어 주도록 해 두었습니다.

`main_test.go` 는 고친 버전과 기본 동작만 검증합니다. 버퍼 없는 채널 전달, `len`/`cap` 과 FIFO 순서, 닫힌 채널의 제로값과 `ok`, 생산자가 닫은 채널의 합계(`t.Run` 으로 나눈 테이블 주도 테스트), 버퍼 1 일꾼이 끝나는지를 확인합니다.

## 연습 문제

1. `make(chan int, 3)` 에 값을 4개 보내는 코드를 `select` 와 `default` 로 바꿔, 넘치는 값을 버리고 몇 개를 버렸는지 세어 보세요. (힌트: `select` 의 `default` 가 있으면 송신이 막히지 않습니다.)
2. 고루틴 3개가 각각 숫자를 보내고, 모두 끝나면 채널이 닫혀 `range` 가 끝나게 만들어 보세요. (힌트: `sync.WaitGroup` 과 닫기 전용 고루틴 하나.)
3. `startWorker` 를 `context.Context` 를 받도록 고쳐서, 버퍼 0 채널로도 취소되면 일꾼이 종료되게 하세요. (힌트: 보내기를 `select` 로 감싸고 `ctx.Done()` 를 함께 기다리기.)

## 더 읽기

- Go Tour, 채널: https://go.dev/tour/concurrency/2
- Go Tour, 버퍼 채널과 range, close: https://go.dev/tour/concurrency/3
- Effective Go, Channels: https://go.dev/doc/effective_go#channels
- Go 블로그, Go Concurrency Patterns: Pipelines and cancellation: https://go.dev/blog/pipelines