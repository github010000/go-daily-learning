# 주제 35: context 패키지

## 한 줄 요약

context 는 「이 일을 이제 그만두라」는 신호와 「언제까지 끝내라」는 마감 시간을 호출 사슬 전체에 전달하는 표준 방법이며, 함수의 첫 인자 `ctx` 로 넘기고, 만든 쪽이 `cancel` 을 반드시 부릅니다.

## 왜 필요한가

서버가 요청 하나를 처리하면 보통 고루틴 여러 개가 생깁니다. DB 조회, 외부 API 호출, 캐시 확인이 각각 돌아가죠. 그런데 클라이언트가 연결을 끊었거나 5초 마감이 지났다면 이 일들을 전부 멈춰야 합니다. 안 멈추면 아무도 읽지 않는 결과를 만드느라 CPU, 커넥션, 메모리를 계속 씁니다. 이 「멈춰라」를 어떻게 모든 하위 호출에 전달하느냐가 문제입니다.

다른 언어는 저마다 방식이 있습니다. Java 는 `Thread.interrupt()` 나 `Future.cancel()` 을 쓰는데 스레드마다 따로 다뤄야 합니다. JavaScript 는 `AbortController` 와 `signal` 을 `fetch` 에 넘깁니다. Go 의 `ctx` 와 가장 비슷하지만 언어 기능이 아니라 라이브러리별 약속입니다. Python 의 asyncio 는 `task.cancel()` 로 예외(`CancelledError`)를 던지고, 스레드 쪽에는 표준적인 방법이 없습니다. 이 방식들은 대부분 「취소를 외부에서 강제로 일으키는」 쪽입니다.

Go 는 고루틴을 밖에서 강제로 죽이는 기능을 일부러 만들지 않았습니다. 대신 취소를 **값**으로 만들어 `ctx` 에 담고, 일하는 쪽이 `<-ctx.Done()` 을 스스로 확인해 정리하고 돌아오게 했습니다. 그래서 취소는 협력적입니다. 확인하지 않는 코드는 취소되지 않습니다(main.go 의 「실수 버전」이 이를 보여 줍니다). 대신 정리 코드(defer, 연결 반환)가 정상 경로로 실행되어 예측하기 쉽습니다. 또 표준 라이브러리의 `net/http`, `database/sql`, `os/exec` 가 모두 같은 `context.Context` 를 받기 때문에 한 번 만든 마감이 서버 핸들러에서 DB 쿼리까지 이어집니다. 모두가 같은 타입과 같은 위치(첫 인자)를 쓰기로 약속한 것이 이 패키지의 진짜 가치입니다.

## 기본 사용

ctx 는 트리 구조입니다. 뿌리는 `context.Background()` 이고, `WithCancel`, `WithTimeout`, `WithDeadline`, `WithValue` 로 자식을 만듭니다. 부모가 취소되면 모든 자식이 취소되고, 자식을 취소해도 부모는 영향이 없습니다.

```go
func fetch(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err() // context.Canceled 또는 context.DeadlineExceeded
	}
}

func caller() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel() // 일찍 끝나도 자원을 바로 풀어 준다
	return fetch(ctx, time.Second)
}
```

지킬 관례는 네 가지입니다.

- `ctx` 는 함수의 **첫 번째 인자**이고 이름은 `ctx` 입니다. `func Do(ctx context.Context, id int) error` 처럼 씁니다.
- `With...` 가 돌려주는 `cancel` 은 **반드시** 부릅니다. 보통 만들자마자 `defer cancel()` 을 씁니다.
- ctx 를 구조체 필드에 저장하지 않고 호출마다 인자로 넘깁니다.
- 어떤 ctx 를 써야 할지 모르면 `context.TODO()` 를 씁니다. `nil` 은 절대 넘기지 않습니다.

종료 원인은 `ctx.Err()` 로 확인합니다. `errors.Is(err, context.DeadlineExceeded)` 처럼 비교하세요.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

- `go vet ./...`: 아무 출력이 없어야 정상입니다. 만약 `cancel` 을 버리는 코드를 직접 만들어 보면 lostcancel 검사가 경고를 냅니다.
- `go run .`: 「마감 시간」 절에서 느린 작업은 `context deadline exceeded`, 빠른 작업은 `<nil>` 이 나옵니다. 「취소 전파」에서 부모와 자식 오류가 둘 다 `context canceled` 인지 보세요. 「취소를 무시하는 작업」에서 실수 버전은 `false`, 고친 버전은 `true` 여야 합니다. 마지막으로 string 키 실수 버전은 「패키지 B 의 값」이 나와 A 가 B 에게 덮어쓰인 것을 보여 줍니다.
- `go test -v ./...`: `TestRunWithTimeout`, `TestPropagate`, `TestRespectingWorkerStops`, `TestRequestID` 와 하위 테스트가 전부 `PASS` 여야 합니다.
- 동시성을 다루므로 `go test -race ./...` 도 돌려 보세요. 경고 없이 `ok` 가 나와야 합니다.

```bash
go test -race ./...
```

## 실전에서는 이렇게 쓴다

**1. HTTP 서버 핸들러와 클라이언트**

서버는 요청마다 ctx 를 만들어 두고, 클라이언트가 연결을 끊으면 그 ctx 가 취소됩니다. 핸들러는 그것을 아래로 그대로 흘려보내면 됩니다.

```go
func handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = $1", 1)
	var name string
	if err := row.Scan(&name); err != nil {
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
		return
	}
	fmt.Fprintln(w, name)
}
```

바깥으로 나가는 요청은 `http.NewRequestWithContext(ctx, ...)` 로 만들어야 마감이 걸립니다. 같은 이름에 `Context` 가 붙은 메서드(`QueryContext`, `ExecContext`, `exec.CommandContext`)가 표준 라이브러리 곳곳에 있습니다.

**2. CLI 의 Ctrl+C 처리**

`signal.NotifyContext` 는 인터럽트 신호가 오면 취소되는 ctx 를 돌려줍니다. 장기 실행 프로그램의 종료 처리에 전형적으로 쓰입니다.

```go
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			fmt.Println("작업 중")
		}
	}
}
```

**3. 요청 범위 메타데이터**

요청 ID, 트레이스 ID 처럼 요청을 따라 흘러야 하지만 함수 인자로 일일이 넘기기 번거로운 값은 context 값으로 둡니다. main.go 의 `WithRequestID` 와 `RequestID` 처럼 키 타입을 숨기고 접근 함수만 공개하는 방식이 표준입니다. OpenTelemetry 같은 유명 라이브러리도 이 형태입니다.

## 흔한 실수

### 1. cancel 을 부르지 않는다

```go
// 틀린 코드
func work(parent context.Context) error {
	ctx, _ := context.WithTimeout(parent, time.Minute)
	return fetch(ctx, time.Millisecond)
}
```

`go vet` 이 「cancel 함수가 모든 경로에서 쓰이지 않는다(context 누수 가능)」는 취지의 lostcancel 경고를 냅니다. 컴파일은 되기 때문에 vet 을 안 돌리면 그냥 넘어갑니다.

왜 문제일까요? `WithTimeout` 은 부모에 자식을 등록하고 타이머를 만듭니다. 작업이 1밀리초 만에 끝나도, `cancel` 을 부르지 않으면 이 등록과 타이머가 마감(여기서는 1분)이 지나거나 부모가 취소될 때까지 남습니다. 요청마다 이러면 서버 메모리와 타이머가 쌓입니다. 부모가 `Background` 라면 끝내 부모가 취소되지 않으므로 마감까지 버팁니다.

```go
// 고친 코드
func work(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, time.Minute)
	defer cancel() // 일찍 끝나도 즉시 정리
	return fetch(ctx, time.Millisecond)
}
```

`cancel` 은 여러 번 불러도 안전하므로 망설일 이유가 없습니다.

### 2. ctx 를 받고도 Done 을 확인하지 않는다

```go
// 틀린 코드
func slowLoop(ctx context.Context) {
	for range 1000 {
		time.Sleep(100 * time.Millisecond) // ctx 를 전혀 보지 않음
	}
}
```

컴파일도 되고 에러도 없지만 호출자가 취소해도 100초를 다 돕니다. main.go 의 `ignoringWorker` 가 같은 실수이며 실행하면 「30ms 안에 멈췄는가 = false」가 나옵니다. 인자에 ctx 가 있다는 사실이 취소를 보장하지 않습니다. Go 는 취소를 강제하지 않고 일하는 쪽이 확인하게 합니다.

```go
// 고친 코드
func slowLoop(ctx context.Context) error {
	for range 1000 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}
```

한 번에 오래 걸리는 단일 호출이라면 `QueryContext` 처럼 ctx 를 받는 버전을 쓰세요.

### 3. nil ctx 를 넘긴다

```go
// 틀린 코드
func bad() error {
	return fetch(nil, time.Second)
}
```

`go vet` 이나 린터가 `nil` Context 전달을 경고하고(staticcheck 의 SA1012), 실행하면 `ctx.Done()` 을 부르는 순간 `panic: runtime error: invalid memory address or nil pointer dereference` 가 납니다. 인터페이스 값이 nil 이라 메서드를 부를 수 없기 때문입니다. 아직 ctx 를 어디서 받아야 할지 정하지 못했다면 `context.TODO()` 를 쓰세요. 나중에 찾기 쉬운 표시 역할을 합니다.

```go
// 고친 코드
func ok() error {
	return fetch(context.TODO(), time.Millisecond)
}
```

### 4. ctx 를 구조체에 넣어 둔다

```go
// 틀린 코드
type Client struct {
	ctx context.Context
}

func (c *Client) Get(id int) error {
	return fetch(c.ctx, time.Second)
}
```

생성 시점의 ctx 가 객체 수명 내내 쓰이면, 그 ctx 가 취소된 뒤의 모든 호출이 즉시 실패하고, 호출마다 다른 마감을 줄 수도 없습니다. 어느 요청의 취소인지도 흐려집니다. ctx 는 「한 번의 호출」에 속한 것이라서 인자로 넘기는 것이 관례입니다.

```go
// 고친 코드
type Client struct{}

func (c *Client) Get(ctx context.Context, id int) error {
	return fetch(ctx, time.Second)
}
```

### 5. string 같은 기본 타입을 값의 키로 쓴다

```go
// 틀린 코드
func setUser(ctx context.Context) context.Context {
	return context.WithValue(ctx, "user", "kim")
}
```

다른 패키지가 같은 `"user"` 를 쓰면 서로 덮어씁니다. main.go 의 `mistakeKeys` 는 A 가 넣은 값을 읽었는데 B 의 값이 나옵니다. 컴파일러도 막아 주지 않고, 린터가 경고하는 정도입니다. 키는 비교할 때 타입까지 같아야 하므로, 패키지 안에서만 쓰는 비공개 타입을 키로 삼으면 충돌이 불가능합니다.

```go
// 고친 코드
type userKey struct{}

func setUser(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, userKey{}, name)
}

func getUser(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(userKey{}).(string)
	return name, ok
}
```

### 6. 함수의 필수 인자를 context 값에 숨긴다

```go
// 틀린 코드
func sendMail(ctx context.Context) error {
	to, _ := ctx.Value("to").(string) // 받는 사람이 ctx 안에 있다
	fmt.Println("보냄:", to)
	return nil
}
```

시그니처만 봐서는 무엇이 필요한지 알 수 없고, 값을 넣는 것을 잊어도 컴파일러가 못 잡으며 런타임에 빈 문자열로 동작합니다. 타입 검사도 포기한 셈입니다. 함수가 동작하는 데 필요한 입력, DB 핸들, 로거, 설정은 평범한 인자나 구조체 필드로 받으세요. context 값은 「요청 ID, 인증 정보처럼 요청을 따라 흐르되 없어도 함수가 동작하는」 것에 한정합니다.

```go
// 고친 코드
func sendMail(ctx context.Context, to string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Println("보냄:", to)
	return nil
}
```

## 주의할 점

- **`ctx.Err()` 는 취소 전에는 nil 입니다.** 「에러가 없다」가 아니라 「아직 안 끝났다」입니다. `Done()` 채널이 닫힌 뒤에만 이유가 채워지므로, `<-ctx.Done()` 로 기다린 다음에 읽으세요.
- **마감 경합.** 작업이 마감 직전에 끝났어도 `select` 에서 두 case 가 동시에 준비되면 Go 는 임의로 하나를 고릅니다. 취소 직후에도 작업 결과가 올 수 있다는 뜻이니, 취소를 확인한 뒤에는 결과를 버린다고 생각하고 짜세요.
- **고루틴을 띄울 때 ctx 를 넘기지 않으면 누수됩니다.** 요청이 끝나도 계속 도는 고루틴은 `go test` 에서는 안 보이다가 서버가 몇 주 뒤에 메모리 때문에 죽을 때 드러납니다.
- **요청 이후에도 해야 할 일에 요청 ctx 를 쓰면** 응답을 보내자마자 같이 취소됩니다. 감사 로그 기록처럼 요청과 무관하게 끝까지 해야 하는 일은 Go 1.21 에서 생긴 `context.WithoutCancel` 로 값은 유지하고 취소만 끊을 수 있습니다.
- **`time.After` 는 루프 안에서 매번 타이머를 만듭니다.** main.go 의 시연은 짧아서 괜찮지만, 아주 촘촘한 루프에서는 `time.NewTimer` 나 `Ticker` 를 재사용하세요.
- **루프 변수와 고루틴.** Go 1.22 부터 `for` 의 변수는 반복마다 새로 만들어지지만, `go.mod` 의 go 버전이 1.21 이하이면 옛 동작입니다. 고루틴에 ctx 와 루프 변수를 함께 넘길 때 헷갈리기 쉬우니 인자로 명시해 주세요.
- **Python, JavaScript 습관.** 예외나 `abort` 이벤트로 코드가 중간에 끊긴다고 기대하면 안 됩니다. Go 에서는 당신의 코드가 `Done()` 을 보고 `return` 해야 끊깁니다. 취소 원인을 자세히 남기고 싶다면 Go 1.20 의 `WithCancelCause` 와 `context.Cause` 를 쓰세요.

## 코드로 확인하기

`go run .` 의 출력은 다섯 묶음입니다.

1. 마감 시간: `slowWork` 는 `time.After` 와 `ctx.Done()` 중 먼저 준비된 쪽을 따릅니다. 20ms 마감에 2초짜리 작업은 `DeadlineExceeded` 로 끝나고, 2초 마감에 1ms 작업은 `nil` 입니다.
2. 취소 전파: 부모만 취소했는데 자식에도 `context canceled` 가 찍힙니다. 자식의 마감(1시간)이 아직 멀었어도 그렇습니다.
3. 취소를 무시하는 작업: 두 worker 는 똑같이 취소를 당합니다. 차이는 worker 가 `Done()` 을 보는지뿐입니다. 실수 버전은 150ms 를 다 자고 나서야 끝나서 30ms 대기에는 `false`, 고친 버전은 10ms 안에 반응해서 `true` 입니다. 실행 환경에 따라 시간은 달라질 수 있지만 방향은 같습니다.
4. string 키 실수: 같은 `"id"` 키라서 나중에 넣은 값이 읽힙니다.
5. 전용 키 타입: `req-42 true` 와 값 없음 `false` 를 확인합니다.

`main_test.go` 는 **고친 버전과 기본 동작만** 검증합니다. 마감 초과와 정상 완료를 테이블로, 부모 취소가 자식에 전파되는지, `respectingWorker` 가 취소 후 끝나는지, 요청 ID 가 있을 때와 없을 때를 확인합니다. 시간 차이를 크게 잡아 느린 기계에서도 결과가 뒤집히지 않게 했습니다.

## 연습 문제

1. `runWithTimeout` 을 고쳐서 `ctx.Deadline()` 으로 남은 시간을 출력하는 함수를 만드세요. (힌트: `Deadline()` 은 `(time.Time, bool)` 을 돌려주고, `time.Until` 이 편합니다.)
2. 고루틴 3개가 각자 다른 시간 동안 일하고, 하나라도 오류를 내면 나머지를 취소하는 함수를 만드세요. (힌트: `WithCancel` 과 오류 채널을 쓰고, 이미 만든 구조를 `golang.org/x/sync/errgroup` 이 어떻게 줄이는지도 읽어 보세요.)
3. `httptest.NewServer` 로 느린 핸들러를 띄우고, 클라이언트가 `NewRequestWithContext` 와 짧은 마감으로 요청해 `DeadlineExceeded` 를 받는 테스트를 쓰세요. (힌트: 핸들러에서 `r.Context().Done()` 도 기다리게 하면 서버 쪽 취소도 관찰할 수 있습니다.)

## 더 읽기

- Go Concurrency Patterns: Context (공식 블로그): https://go.dev/blog/context
- context 패키지 문서: https://pkg.go.dev/context
- Contexts and structs (공식 블로그, 구조체에 ctx 를 넣지 말라는 이유): https://go.dev/blog/context-and-structs