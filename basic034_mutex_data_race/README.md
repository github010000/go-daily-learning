# 주제 34: Mutex·RWMutex 와 data race

## 한 줄 요약

여러 고루틴이 같은 변수를 동시에 건드리고 그중 하나라도 쓰기라면 data race 입니다. `sync.Mutex` 로 보호하고, `go test -race` 로 찾고, Mutex 를 값으로 복사하는 실수는 `go vet` 으로 잡습니다.

## 왜 필요한가

Python 은 GIL 이 있어서 `counter += 1` 같은 코드가 "그럭저럭" 돌아가는 것처럼 보이지만, 실제로는 이것도 안전하지 않습니다. Java 는 `synchronized` 와 `ReentrantLock` 이 있고, JavaScript 는 싱글 스레드 이벤트 루프라서 일반 코드에서는 이 문제를 거의 만나지 않습니다. Go 는 고루틴이 실제로 여러 코어에서 동시에 돌기 때문에, 보호 없는 공유 변수는 곧바로 문제가 됩니다. `n++` 는 CPU 에서 읽기, 더하기, 쓰기 세 단계입니다. 두 고루틴이 같은 값을 읽고 각자 1을 더해 쓰면 증가 한 번이 사라집니다.

Go 의 설계는 두 가지가 눈에 띕니다. 첫째, 언어가 "공유하지 말고 채널로 통신하라"를 권하지만 Mutex 도 표준 라이브러리에 당당히 들어 있습니다. 캐시나 카운터, 상태 보관 같은 곳에는 채널보다 Mutex 가 단순하고 빠릅니다. 둘째, Go 는 race 를 찾아 주는 도구를 툴체인에 넣었습니다. `-race` 플래그 하나로 실제 실행 중에 충돌하는 접근을 보고해 줍니다. 대신 Mutex 는 Java 와 달리 재진입이 안 되고, 락을 값으로 복사해도 컴파일러가 막지 않습니다. 이 빈틈을 `go vet` 의 copylocks 검사가 메웁니다.

## 기본 사용

Mutex 는 보호할 데이터와 같은 구조체 안에 두고, 메서드는 포인터 리시버로 만듭니다. `Lock` 직후에 `defer Unlock` 을 쓰는 것이 관용구입니다.

```go
package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func main() {
	var c Counter
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()
	fmt.Println(c.n)
}
```

읽기가 쓰기보다 훨씬 많다면 `sync.RWMutex` 를 씁니다. 읽기는 `RLock`/`RUnlock`, 쓰기는 `Lock`/`Unlock` 입니다. 읽기끼리는 동시에 들어올 수 있고, 쓰기는 모든 읽기와 쓰기를 기다렸다가 혼자 들어갑니다.

```go
package main

import "sync"

type Cache struct {
	mu sync.RWMutex
	m  map[string]int
}

func (c *Cache) Get(k string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[k]
	return v, ok
}

func main() {
	c := &Cache{m: map[string]int{"a": 1}}
	c.Get("a")
}
```

단순한 정수 카운터라면 `sync/atomic` 의 `atomic.Int64` (Go 1.19 이상)가 더 가볍습니다. 다만 두 변수를 함께 바꿔야 하는 식으로 불변식이 여러 필드에 걸치면 Mutex 를 쓰세요.

## 돌려보기

```bash
go vet ./...      # 정적 검사
go run .          # 시연 실행
go test -v ./...  # 테스트
```

- `go vet ./...`: 아무 출력이 없어야 합니다. main.go 는 Mutex 를 복사하지 않기 때문입니다. 아래 「흔한 실수」의 코드를 넣어 보면 "passes lock by value" 나 "copies lock value" 같은 경고가 나옵니다.
- `go run .`: "실수 버전(보호 없음)" 값이 기대값보다 작게 나오는지 보세요. 여러 번 돌리면 값이 매번 다릅니다. 반면 Mutex 와 atomic 버전은 항상 기대값과 같습니다. 캐시 시연에서는 실수 버전도 값이 틀리지만 race 경고는 없다는 점이 핵심입니다.
- `go test -v ./...`: 모든 `TestXxx` 와 하위 `t.Run` 이 `PASS` 인지 보세요.

```bash
go test -race ./...   # race detector 켜고 테스트
go run -race .        # 시연에서 직접 경고 보기
```

- `go test -race ./...`: `ok` 로 끝나야 합니다. 고친 코드만 테스트하기 때문입니다.
- `go run -race .`: `WARNING: DATA RACE` 와 함께 "Write at ... by goroutine N", "Previous write at ..." 같은 두 접근의 스택이 출력됩니다. 첫 번째 시연(보호 없는 카운터)에서 나옵니다. race 가 발견되면 프로세스는 종료 코드 66 으로 끝나는 것이 기본 동작이니 터미널에서 확인해 보세요.

## 실전에서는 이렇게 쓴다

**1. 서버의 메모리 캐시/레지스트리.** HTTP 핸들러는 요청마다 별도 고루틴에서 돕니다. 핸들러들이 공유하는 맵(세션, 설정, 메모이제이션)은 `RWMutex` 로 감싸고 구조체 메서드 뒤에 숨깁니다. 바깥에서는 `Get`/`Set` 만 보이고 락의 존재를 모르게 하는 것이 요점입니다.

**2. 구조체에 `mu` 를 첫 필드로 두는 관용구.** 표준 라이브러리와 많은 오픈소스가 보호 대상 필드 바로 위에 `mu sync.Mutex` 를 두고 주석으로 "mu 가 아래 필드를 보호한다"고 적습니다. 읽는 사람이 어떤 필드가 락 안쪽인지 한눈에 알 수 있습니다.

**3. `-race` 를 CI 의 기본값으로.** 많은 Go 프로젝트의 CI 는 `go test -race ./...` 를 돌립니다. race detector 는 실제로 실행된 코드 경로의 충돌만 잡기 때문에, 동시성 코드는 고루틴을 여러 개 띄우는 테스트를 만들어 두어야 의미가 있습니다. 이 자료의 `TestCacheGetSetConcurrent` 가 그런 모양입니다.

**4. 락 안에서는 "잠금 없는 내부 함수"를 부른다.** 이 자료의 `incLocked` 처럼, 공개 메서드는 잠금을 잡고, 내부 함수는 이미 잠겨 있다는 전제로 작성합니다. 이름 끝에 `Locked` 를 붙이는 관례가 널리 쓰입니다.

## 흔한 실수

### 1. Mutex 를 값으로 복사한다

```go
// 틀린 코드
package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	mu sync.Mutex
	n  int
}

// 값 리시버: 호출할 때마다 Counter 전체(Mutex 포함)가 복사됩니다.
func (c Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func main() {
	var c Counter
	c.Inc()
	fmt.Println(c.n)
}
```

컴파일은 됩니다. 하지만 `go vet` 이 "passes lock by value: ... contains sync.Mutex" 비슷한 경고를 냅니다. 실행하면 `c.n` 은 0 으로 남습니다. 복사본의 n 만 올랐기 때문입니다. 더 위험한 경우는 잠금을 쥔 상태에서 복사하는 것으로, 복사본의 Mutex 는 이미 잠긴 상태로 태어나 영원히 풀리지 않습니다. 함수 인자로 넘기거나, `b := a` 로 대입하거나, `for _, c := range counters` 처럼 range 로 꺼낼 때도 똑같이 복사됩니다. 이런 경우 vet 은 "copies lock value" 같은 말로 알려 줍니다.

고치는 법은 포인터 리시버를 쓰고, Mutex 가 든 구조체는 항상 포인터로 넘기는 것입니다.

```go
package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func main() {
	c := &Counter{}
	c.Inc()
	fmt.Println(c.n)
}
```

### 2. 잠그지 않고 읽는다 (쓰기만 보호)

```go
// 틀린 코드
package main

import "sync"

type Stat struct {
	mu sync.Mutex
	n  int
}

func (s *Stat) Inc() {
	s.mu.Lock()
	s.n++
	s.mu.Unlock()
}

func (s *Stat) Value() int {
	return s.n // 읽기에는 잠금이 없음
}

func main() {
	s := &Stat{}
	s.Inc()
	_ = s.Value()
}
```

"쓰기만 잠그면 되지 않나?" 하고 착각하기 쉽습니다. 한 고루틴이 `Inc` 로 쓰는 동안 다른 고루틴이 `Value` 로 읽으면 이것도 data race 입니다. 평소에는 멀쩡해 보이다가 `-race` 로 돌릴 때 "Read at ... by goroutine ..." 와 "Previous write at ..." 가 나옵니다. 읽기와 쓰기 모두 같은 Mutex 로 보호해야 합니다. 이 자료의 `SafeCounter.Value` 처럼 읽기에도 `Lock` 을 쓰세요.

### 3. 잠금을 쥔 채 같은 Mutex 를 또 잠근다 (재진입 교착)

```go
// 틀린 코드
package main

import "sync"

type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *Counter) IncTwice() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Inc() // 이미 쥐고 있는 락을 다시 잡으려 함
	c.Inc()
}

func main() {
	c := &Counter{}
	c.IncTwice()
}
```

Java 의 `synchronized` 와 `ReentrantLock` 은 같은 스레드가 다시 잠가도 되지만 Go 의 Mutex 는 안 됩니다. 이 프로그램은 멈추고, 다른 고루틴이 없으면 런타임이 `fatal error: all goroutines are asleep - deadlock!` 로 끝냅니다. 서버처럼 다른 고루틴이 살아 있으면 런타임이 감지하지 못하고 그냥 조용히 멈춥니다. 더 나쁜 경우입니다.

고치는 법은 잠금 없는 내부 함수를 따로 두는 것입니다. main.go 의 `incLocked` 와 `IncTwice` 를 보세요.

### 4. Unlock 을 빠뜨리거나 early return 에서 건너뛴다

```go
// 틀린 코드
package main

import (
	"errors"
	"sync"
)

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

func (s *Store) Get(k string) (int, error) {
	s.mu.Lock()
	v, ok := s.m[k]
	if !ok {
		return 0, errors.New("없음") // Unlock 없이 반환
	}
	s.mu.Unlock()
	return v, nil
}

func main() {
	s := &Store{m: map[string]int{}}
	s.Get("x")
}
```

없는 키를 한 번 조회하면 락이 영원히 잠깁니다. 이후 모든 `Get` 호출이 멈춥니다. 해결은 `Lock` 바로 다음 줄에 `defer s.mu.Unlock()` 입니다. 반환 경로가 몇 개든, panic 이 나든 풀립니다. 또 풀려 있는 Mutex 를 `Unlock` 하면 `fatal error: sync: unlock of unlocked mutex` 로 프로그램이 죽고, 이 오류는 `recover` 로 잡을 수도 없습니다.

### 5. RLock 으로 읽고, 풀고, Lock 으로 쓴다 (check-then-act)

```go
// 틀린 코드
package main

import "sync"

type Cache struct {
	mu sync.RWMutex
	m  map[string]int
}

func (c *Cache) Incr(k string) {
	c.mu.RLock()
	v := c.m[k]
	c.mu.RUnlock()

	c.mu.Lock()
	c.m[k] = v + 1 // v 는 이미 낡았을 수 있음
	c.mu.Unlock()
}

func main() {
	c := &Cache{m: map[string]int{}}
	c.Incr("a")
}
```

맵 접근은 전부 잠겨 있으니 `-race` 는 아무 경고도 내지 않습니다. 그런데도 결과가 틀립니다. 읽은 뒤 쓰기 전 사이에 다른 고루틴이 끼어들어 같은 값을 읽기 때문입니다. main.go 의 두 번째 시연이 이것을 보여 줍니다. race detector 는 "잠기지 않은 동시 접근"을 찾는 도구이지, 논리 오류를 찾는 도구가 아닙니다. 읽기와 수정과 쓰기가 한 덩어리여야 하면 한 번의 `Lock` 안에 모두 넣으세요. 또한 RLock 을 쥔 채로 Lock 을 부르면(업그레이드) 자기 자신을 기다려 교착합니다. RWMutex 는 업그레이드를 지원하지 않습니다.

### 6. 맵을 보호 없이 동시에 쓴다

```go
// 틀린 코드
package main

import "sync"

func main() {
	m := map[int]int{}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m[i] = i
		}()
	}
	wg.Wait()
}
```

Go 런타임은 맵의 동시 쓰기를 감지하면 `fatal error: concurrent map writes` 를 내며 종료합니다. 이것도 `recover` 가 안 되는 fatal error 입니다. 매번 나는 것은 아니라서 테스트를 통과하고 운영에서 터지는 전형적인 버그입니다. Mutex 로 감싸거나, 상황에 따라 `sync.Map` 을 검토하세요.

## 주의할 점

- **race detector 는 실행된 경로만 봅니다.** 테스트가 해당 코드를 동시에 돌리지 않으면 race 가 있어도 통과합니다. 또 공식 문서에 따르면 실행 속도와 메모리 사용이 눈에 띄게 늘어납니다. 운영 바이너리 상시 사용보다는 테스트와 CI, 스테이징에서 쓰는 도구입니다. 링크는 아래 「더 읽기」를 보세요.
- **race 가 없다고 해서 결과가 맞는 것은 아닙니다.** 5번 실수처럼 락을 잘게 쪼개면 경고 없이 틀립니다. 락의 범위는 "깨지면 안 되는 불변식" 단위로 잡으세요.
- **RWMutex 가 항상 빠른 것은 아닙니다.** 내부 관리가 Mutex 보다 무겁습니다. 임계 구역이 아주 짧거나 쓰기가 잦으면 일반 Mutex 가 더 빠를 수 있습니다. 가정하지 말고 벤치마크로 확인하세요. 또 쓰기를 기다리는 고루틴이 있으면 새 `RLock` 도 대기하므로, 같은 고루틴에서 `RLock` 을 중첩하면 교착할 수 있습니다.
- **Go 1.22 의 루프 변수 변경.** 1.22 부터 `for i := ...` 의 `i` 가 반복마다 새로 만들어집니다. 그래서 `go func() { use(i) }()` 가 예전처럼 같은 `i` 를 공유하는 race 를 일으키지 않습니다. 하지만 go.mod 의 go 버전이 1.21 이하면 옛 동작 그대로입니다. 오래된 프로젝트에서는 `i := i` 복사가 여전히 필요합니다.
- **락을 쥔 채 느린 일을 하지 마세요.** 네트워크 호출이나 디스크 I/O 를 임계 구역에 넣으면 다른 모든 고루틴이 그 시간만큼 줄을 섭니다. 필요한 값만 락 안에서 복사해 나온 뒤 락 밖에서 일하세요.
- **Mutex 를 가진 구조체의 복사는 vet 이 알려 주지만, 항상은 아닙니다.** 인터페이스나 `reflect` 를 거치면 놓칠 수 있습니다. 가장 안전한 습관은 이런 타입은 생성자에서 포인터로 만들어 포인터로만 돌리는 것입니다.

## 코드로 확인하기

`go run .` 의 첫 구간에서 기대값은 100 × 1000 입니다. 「실수 버전」은 이보다 작은 값이 나오기 쉽습니다. 사라진 증가분이 그 차이입니다. 코어가 하나뿐이거나 운이 좋으면 같게 나올 수도 있으니, 여러 번 돌려 보세요. 같은 코드가 Mutex 를 쓰면 항상 기대값이 나옵니다. atomic 도 마찬가지이고, 한 줄짜리 카운터에서는 atomic 이 더 단순합니다.

둘째 구간은 RWMutex 캐시입니다. 「실수 버전」은 `RLock` 으로 읽고 놓은 뒤 `Lock` 으로 쓰기 때문에 값이 모자랍니다. 그런데 `go run -race .` 로 돌려도 이 구간에서는 경고가 나오지 않습니다. 모든 접근이 잠겨 있기 때문입니다. 이 차이를 직접 눈으로 보는 것이 이 구간의 목적입니다. 「고친 버전」은 읽기와 쓰기를 한 번의 `Lock` 안에 넣어 정확히 맞습니다.

셋째 구간은 잠금을 쥔 채 `incLocked` 를 부르는 패턴이 교착 없이 끝나는 것을 보여 줍니다.

`main_test.go` 는 고친 버전과 기본 동작만 검증합니다.

- `TestCountersExact`: 테이블 주도 형식으로 여러 규모에서 Mutex 와 atomic 카운터가 정확한 합을 내는지 확인합니다.
- `TestCacheIncr`: RWMutex 캐시의 `Incr` 가 읽기-수정-쓰기를 하나로 처리하는지 확인합니다.
- `TestCacheGetSetConcurrent`: 읽기와 쓰기 고루틴을 섞어 `-race` 에서도 깨끗한지 확인합니다.
- `TestIncTwice`: 내부 함수 패턴이 교착하지 않고 값이 맞는지 확인합니다.

실수 버전의 값은 매번 달라서 일부러 테스트하지 않았습니다.

## 연습 문제

1. (쉬움) main.go 의 `RunRacy` 를 `go run -race .` 로 돌려 경고 출력을 읽고, 어느 줄이 "Write" 이고 어느 줄이 "Previous write" 인지 찾아 보세요. 힌트: 스택 맨 위의 파일명과 줄 번호를 보세요.
2. (보통) 위 「흔한 실수」 1번 코드를 파일로 저장해 `go vet` 을 돌려 보고, 경고가 사라지도록 고치세요. 힌트: 리시버 타입의 `*` 하나가 전부입니다.
3. (어려움) `Cache` 에 키가 없으면 계산 함수를 한 번만 실행해 저장하는 `GetOrCompute(k string, f func() int) int` 를 만들고, 여러 고루틴이 같은 키로 불러도 `f` 가 한 번만 실행되는지 테스트하세요. 힌트: 읽기 확인과 쓰기를 한 락 구간에 넣거나, 확인 후 다시 한 번 확인하는 double-check 패턴을 고민해 보세요.

## 더 읽기

- Data Race Detector (공식): https://go.dev/doc/articles/race_detector
- The Go Memory Model: https://go.dev/ref/mem
- sync 패키지 문서: https://pkg.go.dev/sync
- sync/atomic 패키지 문서: https://pkg.go.dev/sync/atomic