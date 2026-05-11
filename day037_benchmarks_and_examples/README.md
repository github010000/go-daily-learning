# Day 37: 벤치마크와 예제 테스트

## 개념 설명
이번 시간에는 Go의 `testing` 패키지가 제공하는 **벤치마킹(Benchmarking)**과 **테스트 가능한 예제(Testable Examples)** 기능에 대해 학습합니다.

**벤치마킹**은 코드의 실행 속도와 메모리 사용량을 정량적으로 측정하는 도구입니다. 단순히 `fmt.Println`으로 시간을 재는 것보다 Go의 테스트 환경(`testing.B`)은 CPU 캐시, 가비지 컬렉션 등 실제 운영 환경과 유사한 상태에서 최적의 실행 횟수(`b.N`)를 자동으로 찾아주어 훨씬 정확한 성능 수치를 제공합니다.

**테스트 가능한 예제(Testable Examples)**는 코드 주석(Comment)에 코드 예시를 작성하고, 그 실행 결과가 주석의 출력과 일치하는지 자동으로 검증하는 기능입니다. 이는 문서를 읽는 사용자가 코드가 어떻게 동작해야 하는지 한눈에 파악할 수 있게 해주며, 동시에 예시의 정확성을 유지하는 테스트가 됩니다.

## 코드 설명
제공된 `main.go` 코드는 `testing` 패키지가 아닌 표준 라이브러리(`time`, `fmt`)만 사용하여, Go 벤치마킹의 핵심 동작 원리를 직접 구현한 시뮬레이션입니다.

1. **`benchSimulator` 함수**: `testing.B`의 동작을 흉내냅니다. `b.N` 루프(100, 1000, 10000회 반복)를 설정하고 `time.Since`로 측정된 총 시간을 연산 횟수(`n`)로 나누어 **평균 실행 시간(ns/op)**을 계산합니다.
2. **`exampleSimulator` 함수**: `Example` 테스트의 원리를 보여줍니다. 함수의 입력값("hello world")과 기대 출력값("HELLO WORLD")을 비교하여, 코드가 주석(문서)과 일치하는지 검증합니다.
3. **`targetFunction`**: 벤치마크의 대상이 되는 실제 함수입니다.

## 핵심 포인트
* **`b.N` 루프**: `testing.B`는 b.N을 동적으로 조정하여 가장 안정적인 실행 시간(ns/op)을 제공합니다.
* **`ns/op` 단위**: 벤치마크 결과의 핵심은 '연산 한 번당 걸리는 시간(나노초)'입니다.
* **`_test.go` 파일**: 벤치마크는 `xxxxx_test.go` 파일에 `func BenchmarkXxx(b *testing.B)`로 작성해야 합니다.
* **`Example` 함수**: `func ExampleXxx()` 형태로 작성하며, 주석의 출력과 코드 실행 결과가 일치해야 테스트가 통과합니다.

## 참고 링크
* [testing 패키지 (공식 문서)](https://pkg.go.dev/testing)
* [Go 블로그: Go 1.2 벤치마킹 가이드](https://go.dev/doc/articles/go_blog/)
* [Effective Go: Benchmarks](https://go.dev/doc/effective_go#benchmarks)