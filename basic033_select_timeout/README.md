# select 와 타임아웃

## 한 줄 요약

`select` 는 여러 채널 연산 중 준비된 것 하나를 골라 실행하고, `default` 를 붙이면 기다리지 않으며, `time.After` 나 `time.NewTimer` 를 case 로 넣으면 타임아웃이 됩니다. 단, 반복문 안의 `time.After` 는 "전체 마감" 이 아니라 "매번 새 마감" 이 된다는 점을 조심해야 합니다.

## 왜 필요한가

여러 곳에서 오는 신호를 동시에 기다려야 할 때가 많습니다. 요청 결과, 취소 신호, 시간 초과 중 무엇이 먼저 올지 모르는 상황입니다. Python 의 `asyncio.wait(..., return_when=FIRST_COMPLETED)`, JavaScript 의 `Promise.race`, Java 의 `CompletableFuture.anyOf` 나 `future.get(timeout, unit)` 이 비슷한 일을 합니다. 이들은 대부분 "라이브러리 함수" 이고, 타임아웃은 예외나 거부된 Promise 로 표현됩니다.

Go 는 이것을 언어 문법(`select`)으로 넣었습니다. 채널 수신, 채널 송신, 타임아웃, 취소가 모두 "채널 연산" 이라는 같은 모양이기 때문에 한 문장 안에서 섞어 쓸 수 있습니다. 타임아웃도 특별한 API 가 아니라 "일정 시간 뒤에 값이 오는 채널" 일 뿐입니다. 덕분에 예외 처리 없이 평범한 제어 흐름으로 타임아웃을 다룹니다.

대신 책임도 개발자에게 옵니다. 타임아웃이 났을 때 상대 고루틴을 자동으로 멈춰 주지 않고, 타이머의 수명도 직접 생각해야 합니다. 이 자료의 코드는 그 부분을 직접 돌려 볼 수 있게 짰습니다.

## 기본 사용

여러 채널 기다리기: 준비된 case 가 없으면 블록되고, 여럿이면 무작위로 하나를 고릅니다.

```go
package main

import "fmt"

func main() {
	a := make(chan string, 1)
	b := make(chan string, 1)
	b <- "b 가 먼저"
	select {
	case s := <-a:
		fmt.Println(s)
	case s := <-b:
		fmt.Println(s)
	}
}
```

`default` 로 막히지 않게 하기:

```go
package main

import "fmt"

func main() {
	ch := make(chan int, 1)
	select {
	case v := <-ch:
		fmt.Println("받음", v)
	default:
		fmt.Println("지금은 없음")
	}
}
```

타임아웃:

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	ch := make(chan int)
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	select {
	case v := <-ch:
		fmt.Println(v)
	case <-timer.C:
		fmt.Println("시간 초과")
	}
}
```

한 번만 쓰는 짧은 코드에서는 `case <-time.After(d):` 로 줄여 써도 됩니다. 반복문 안에서는 아래 「흔한 실수」 를 먼저 읽으세요.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
go test -race ./... # 데이터 레이스 검사
```

- `go vet ./...`: 아무 출력이 없으면 정상입니다.
- `go run .`: 마지막 두 줄을 보세요. 실수 버전은 10개를 다 받고 마감이 발동하지 않았다고 나오고, 고친 버전은 10개보다 적게 받고 마감이 발동했다고 나옵니다.
- `go test -v ./...`: 각 `t.Run` 이름 옆에 PASS 가 찍히는지 보세요.
- `go test -race ./...`: 경고 없이 ok 로 끝나야 합니다.

## 실전에서는 이렇게 쓴다

**1. 취소와 타임아웃을 함께 기다리는 워커.** 서버 코드에서 가장 흔한 모양입니다. `context` 가 내부적으로 같은 원리입니다.

```go
package main

import (
	"context"
	"fmt"
	"time"
)

func worker(ctx context.Context, jobs <-chan int) {
	for {
		select {
		case <-ctx.Done():
			fmt.Println("종료:", ctx.Err())
			return
		case j := <-jobs:
			fmt.Println("작업", j)
		}
	}
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	jobs := make(chan int, 1)
	jobs <- 1
	worker(ctx, jobs)
}
```

**2. 버리기 허용 전송.** 느린 구독자 때문에 발행자가 멈추면 안 될 때 `default` 로 건너뜁니다. 이벤트 브로드캐스트나 로그 수집에서 자주 보입니다. 이 자료의 `trySend` 가 그 모양입니다.

**3. 주기 작업과 종료 신호.** `time.NewTicker` 와 `done` 채널을 한 `select` 에 넣고, 종료 시 `ticker.Stop()` 을 부릅니다. 표준 라이브러리의 `net/http` 서버 내부도 타이머와 취소를 이런 식으로 조합합니다.

**4. 채널을 nil 로 만들어 case 끄기.** 이 자료의 `drainBoth` 처럼 닫힌 채널을 nil 로 바꾸면 그 case 가 영원히 선택되지 않습니다. 여러 입력을 합치는 코드의 관용구입니다.

## 흔한 실수

### 1. 반복문 안의 time.After 를 "전체 마감" 으로 착각하기

```go
// 틀린 코드
package main

import "time"

func consume(ch <-chan int) int {
	count := 0
	for {
		select {
		case <-ch:
			count++
		case <-time.After(time.Second): // 매 반복마다 새 1초
			return count
		}
	}
}

func main() { consume(nil) }
```

값이 0.9초 간격으로 계속 오면 이 함수는 영원히 끝나지 않습니다. 반복할 때마다 타이머가 새로 만들어져 시계가 0 으로 돌아가기 때문입니다. 에러도 panic 도 없이 "마감이 안 지켜지는" 조용한 버그입니다. main.go 의 실수 버전 출력(10개를 모두 받고 마감이 발동하지 않음)이 그 증거입니다. 또한 반복마다 타이머가 하나씩 생성됩니다. Go 1.23 부터는 참조되지 않는 타이머를 가비지 컬렉터가 회수하지만, 그 이전 버전에서는 만료될 때까지 메모리에 남았고, 어느 버전이든 매번 할당하는 비용은 있습니다.

고친 코드는 타이머를 반복문 밖에서 한 번 만듭니다. "마지막 수신 후 일정 시간" 의미(유휴 타임아웃)가 정말 필요하다면 `Timer.Reset` 을 쓰는 방법도 있습니다.

```go
package main

import "time"

func consume(ch <-chan int, total time.Duration) int {
	timer := time.NewTimer(total)
	defer timer.Stop()
	count := 0
	for {
		select {
		case <-ch:
			count++
		case <-timer.C:
			return count
		}
	}
}

func main() { consume(nil, time.Millisecond) }
```

### 2. default 를 넣은 반복문이 CPU 를 다 먹는 경우

```go
// 틀린 코드
package main

func wait(ch <-chan int) int {
	for {
		select {
		case v := <-ch:
			return v
		default:
			// 값이 올 때까지 쉬지 않고 계속 돈다
		}
	}
}

func main() { wait(nil) }
```

`default` 는 "준비된 게 없으면 즉시 다음으로" 라는 뜻이라서 이 반복문은 쉬지 않고 돕니다. 프로그램은 정상으로 보이지만 코어 하나가 100% 가 됩니다. 기다리는 게 목적이라면 `default` 를 빼서 `select` 가 블록되게 하세요. 기다리는 동안 다른 일을 해야 한다면 `default` 대신 타이머나 티커 case 를 쓰세요.

```go
package main

import "fmt"

func wait(ch <-chan int) int {
	return <-ch // 블록되는 동안 CPU 를 쓰지 않는다
}

func main() {
	ch := make(chan int, 1)
	ch <- 3
	fmt.Println(wait(ch))
}
```

### 3. 타임아웃 후 결과 채널에 보내려다 고루틴이 영원히 남기

```go
// 틀린 코드
package main

import (
	"fmt"
	"time"
)

func slow() int {
	time.Sleep(100 * time.Millisecond)
	return 1
}

func call() (int, error) {
	res := make(chan int) // 버퍼 없음
	go func() { res <- slow() }()
	select {
	case v := <-res:
		return v, nil
	case <-time.After(10 * time.Millisecond):
		return 0, fmt.Errorf("시간 초과")
	}
}

func main() { fmt.Println(call()) }
```

호출자는 타임아웃으로 돌아갔는데, 안쪽 고루틴은 받아 줄 사람이 없는 채널에 보내려고 영원히 블록됩니다. 에러 메시지 없이 호출할 때마다 고루틴과 메모리가 쌓이는 누수입니다. 결과 채널에 버퍼 1 을 주면 보내는 쪽이 기다리지 않고 끝납니다.

```go
package main

import (
	"fmt"
	"time"
)

func slow() int {
	time.Sleep(20 * time.Millisecond)
	return 1
}

func call() (int, error) {
	res := make(chan int, 1) // 버퍼 1: 아무도 안 받아도 보내기는 끝난다
	go func() { res <- slow() }()
	select {
	case v := <-res:
		return v, nil
	case <-time.After(5 * time.Millisecond):
		return 0, fmt.Errorf("시간 초과")
	}
}

func main() { fmt.Println(call()) }
```

작업 자체를 멈추려면 `context` 를 넘겨 작업이 취소를 확인하게 해야 합니다.

### 4. case 에서 받은 변수를 쓰지 않아 컴파일 오류

```go
// 틀린 코드
package main

func main() {
	ch := make(chan int, 1)
	ch <- 1
	select {
	case v := <-ch:
	}
}
```

컴파일러는 `declared and not used: v` 라고 합니다(버전에 따라 문구는 조금 다를 수 있습니다). 경고가 아니라 오류입니다. 값이 필요 없으면 `case <-ch:` 로 쓰거나 `_ = v` 대신 변수를 아예 선언하지 마세요.

```go
package main

func main() {
	ch := make(chan int, 1)
	ch <- 1
	select {
	case <-ch:
	}
}
```

### 5. select 안의 break 가 반복문을 끝낸다고 생각하기

```go
// 틀린 코드
package main

import "fmt"

func main() {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	close(ch)
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				break // select 만 빠져나온다
			}
			fmt.Println(v)
		}
	}
}
```

`break` 는 가장 안쪽의 `select`, `switch`, `for` 를 끝냅니다. 여기서는 `select` 만 끝나고 `for` 는 계속 돌아서, 닫힌 채널에서 즉시 반환되는 수신을 반복하며 CPU 를 쓰는 무한 루프가 됩니다. `return` 을 쓰거나 반복문에 레이블을 붙이세요.

```go
package main

import "fmt"

func main() {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	close(ch)
loop:
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				break loop
			}
			fmt.Println(v)
		}
	}
}
```

## 주의할 점

- **여러 case 가 동시에 준비되면 무작위 선택입니다.** 위에서 아래 순서로 우선순위가 주어지지 않습니다. 취소 신호를 우선하고 싶다면 먼저 취소만 `select` + `default` 로 확인하세요.
- **닫힌 채널의 수신은 항상 준비된 상태입니다.** `tryReceive` 가 닫힌 채널에서는 제로값과 true 를 돌려줄 수 있습니다. 구분하려면 `v, ok := <-ch` 의 `ok` 를 확인하세요.
- **nil 채널은 영원히 블록됩니다.** 실수로 초기화하지 않은 채널을 case 에 넣으면 그 case 는 절대 선택되지 않습니다. 모든 case 가 nil 이고 `default` 도 없으면 영원히 멈춥니다.
- **타이머 동작은 Go 버전에 따라 다릅니다.** Go 1.23 부터 `time.After` 의 타이머는 참조가 사라지면 회수되고, `Stop` 과 `Reset` 뒤 채널에 오래된 값이 남지 않습니다. 하지만 오래된 버전도 지원해야 하는 라이브러리라면 `NewTimer` 와 `Stop` 을 습관으로 두는 편이 안전합니다. `go.mod` 의 `go` 줄 버전에 따라 동작이 달라질 수 있다는 점도 기억하세요.
- **타임아웃은 상대 작업을 멈추지 않습니다.** 기다리기를 포기할 뿐입니다. 진짜 중단은 `context` 취소와 작업 쪽의 확인이 필요합니다.
- **시간 기반 테스트는 불안정합니다.** 이 자료의 테스트처럼 "값이 있으면 값, 없으면 ErrTimeout" 같은 결과만 단정하고, 걸린 시간은 단정하지 마세요.

## 코드로 확인하기

`go run .` 의 앞부분은 기본 동작입니다. `firstOf` 는 값이 준비된 b 채널을 고르고, `tryReceive` 는 빈 채널에서 false, `trySend` 는 버퍼가 차면 false 를 보여 줍니다.

핵심은 마지막 두 줄입니다. 생산자는 20ms 마다 값을 하나 보내 10개에 약 200ms 가 걸립니다. 실수 버전은 반복마다 50ms 짜리 새 타이머를 만들기 때문에 20ms 간격의 값이 올 때마다 시계가 초기화되어 10개를 다 받습니다. 고친 버전은 100ms 짜리 타이머를 한 번만 만들었으므로 도중에 마감이 발동해 10개보다 적게 받습니다. 정확한 개수는 실행 환경마다 다르니 직접 돌려 확인하세요.

`main_test.go` 는 고친 버전과 기본 동작만 검증합니다. 테이블 주도 테스트로 `tryReceive`, `trySend`, `consumeWithDeadline` 을 확인하고, `recvWithTimeout` 은 `errors.Is` 로 에러 종류를, `drainBoth` 는 nil 채널 기법으로 두 채널을 끝까지 비우는지를 검증합니다.

## 연습 문제

1. `tryReceive` 를 변형해 값이 없으면 "비었음" 을 출력하는 함수를 만들어 보세요. (힌트: `default` 에서 출력)
2. `recvWithTimeout` 에 `done <-chan struct{}` 인자를 추가해 취소도 기다리게 하세요. (힌트: case 를 하나 더, 취소 시 에러 값은 새로 정의)
3. 유휴 타임아웃(마지막 수신 후 일정 시간)을 반복문 밖의 타이머 하나로 구현해 보세요. (힌트: 값을 받을 때 `Reset`, Go 1.23 이전 규칙은 `Stop` 의 반환값 확인과 채널 비우기)

## 더 읽기

- Go Tour, select: https://go.dev/tour/concurrency/5
- Go Tour, default selection: https://go.dev/tour/concurrency/6
- time 패키지 문서: https://pkg.go.dev/time
- Go 블로그, Pipelines and cancellation: https://go.dev/blog/pipelines