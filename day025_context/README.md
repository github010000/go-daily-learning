# Day 25: context 패키지를 활용한 고루틴 제어와 취소 전파

## 개념 설명

Go 언어에서 `context` 패키지는 고루틴의 생명주기(Lifecycle)를 관리하고, 여러 고루틴에 걸쳐 취소 신호, 마감 시한(Deadline), 그리고 요청 범위의 값(Request-scoped values)을 전달하는 데 사용됩니다. 특히 네트워크 요청이나 데이터베이스 쿼리처럼 시간이 걸리는 작업을 수행할 때, 더 이상 결과가 필요 없는 상황(예: 클라이언트가 연결을 끊음)에서 연관된 모든 고루				를 안전하게 종료시키기 위해 필수적입니다.

`context`는 트리 구조로 동작합니다. 부모 컨텍스트가 취소되면 그 부모로부터 파생된 모든 자식 컨텍스트들도 연쇄적으로 취소됩니다. 이러한 **취소 전파(Cancellation Propagation)** 메커니즘 덕분에 복잡한 고루틴 시스템에서도 자원 누수(Goroutine Leak) 없이 효율적인 자원 관리가 가능합니다.

Go 언어의 강력한 관례 중 하나는 함수의 첫 번째 인자로 `ctx context.Context`를 전달하는 것입니다. 이를 통해 함수 내부에서 수행되는 작업이 상위 컨텍스트의 상태에 따라 중단될 수 있도록 설계합니다.

## 코드 설명

1.  **`context.Background()`**: 컨텍스트 계층 구조의 최상위 루트를 생성합니다. 보통 프로그램의 시작점이나 요청의 입구에서 사용됩니다.
2.  **`context.WithCancel(parent)`**: 수동으로 취소할 수 있는 컨텍스트를 생성합니다. 반환된 `cancelFunc`를 호출하면 해당 컨텍스트와 그 하위 컨텍스트에 `Done()` 신호가 전달됩니다.
3.  **`context.WithTimeout(parent, duration)`**: 지정된 시간이 지나면 자동으로 취소되는 컨텍스트를 생성합니다. 네트워크 타임아웃 구현에 매우 유용합니다.
4.  **`context.WithDeadline(parent, time)`**: 특정 시점(Absolute Time)이 되면 자동으로 취소되는 컨텍스트를 생성합니다.
5.  **`select { case <-ctx.Done(): ... }`**: 고루틴 내부에서 컨텍스트의 취소 여부를 감시하는 핵심 패턴입니다. `ctx.Done()` 채널에 신호가 들어오면 즉시 작업을 중단하고 리소스를 정리합니다.

## 핵심 포인트

*   **첫 번째 인자 관례**: 모든 동기/비동기 함수는 `ctx context.Context`를 첫 번째 인자로 받도록 설계하는 것이 좋습니다.
*   **취소 전파**: 부모의 취소는 자식에게 전파되지만, 자식의 취소가 부모에게 영향을 주지는 않습니다.
*   **리소스 정리**: `WithCancel`, `WithTimeout`, `WithDeadline`을 사용한 후에는 반드시 `cancel` 함수를 호출하거나 `defer`를 사용하여 리소스 누수를 방지해야 합니다.
*   **Error 확인**: `ctx.Err()`를 통해 취소가 어떤 이유(`Canceled` 또는 `DeadlineExceeded`)로 발생했는지 확인할 수 있습니다.

## 참고 링크

*   [Go 공식 문서: context 패키지](https://pkg.go.dev/context)
*   [Go 공식 예제: Context 사용법](https://go.dev/blog/context)