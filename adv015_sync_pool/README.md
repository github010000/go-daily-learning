## 한 줄 요약

sync.Pool 은 P 마다 나뉜 임시 객체 캐시이며, GC 한 번은 객체를 victim cache 로 보내고 한 번 더 GC 가 돌면 객체를 완전히 버리는 2세대 구조를 갖는다. 포인터를 넣으면 할당 횟수가 급감하지만, 작은 값 타입을 그대로 넣으면 interface boxing 때문에 할당이 줄지 않을 수 있다.

## 왜 이런 설계인가

OS 스레드마다 락을 걸고 공유 큐를 만들면 매 Get/Put 에 원자 연산과 캐시 라인 경합이 생긴다. Go 초기에는 pool 을 전역 뮤텍스로 보호하는 단순한 구조를 썼지만, GOMAXPROCS 가 커지고 goroutine 이 많아지면 짧은 임시 객체 하나 꺼내는 비용이 객체 생성 비용보다 커질 수 있었다. 그래서 현재 구현은 CPU P 마다 private, shared 라는 두 단계 저장 공간을 두고, 자기 P 안에서는 락 없이 private 부터 쓰도록 바뀌었다.

GC 와의 관계도 중요한 설계 축이다. sync.Pool 이 힙 캐시처럼 동작하면 객체를 영원히 붙잡고 있어 GC 가 메모리 재활용을 할 수 없게 된다. 반대로 GC 때마다 무조건 비우면 GC 직후에 Get 이 몰리는 순간 전부 New 를 호출해 할당 스파이크가 생긴다. 그래서 Go 1.13 에서는 victim cache 라는 한 단계를 추가해, 객체를 즉시 버리지 않고 한 번 더 기회를 주는 트레이드오프를 택했다.

작은 값 타입을 pool 에 넣으면 손해인 이유는 Go 의 interface 가 값 복사와 함께 객체 헤더를 요구하기 때문이다. byte32 같은 32바이트 구조체를 any 에 넣는 순간 힙에 박스가 새로 생긴다. 매 Put 마다 박스가 만들어지면 allocator 호출이 없던 직접 할당보다 오히려 더 많은 allocation event 가 기록될 수 있다. 그래서 sync.Pool 은 포인터를 반환하는 New 를 쓰는 패턴이 사실상 기본이다.

Go 의 설계에서는 "GC 가 회수할 수 있는 캐시" 여야 한다는 전제도 자주 강조된다. sync.Pool 에 넣은 객체는 GC 주기 안에서만 재사용되지, 파일 핸들처럼 반드시 해제해야 하는 자원을 담으면 안 된다. 이는 free list 와 달리 캐시가 리소스 수명을 결정하는 주체가 아니라는 뜻이다.

## 어떻게 동작하는가

sync.Pool 은 `runtime/sync/pool.go` 가 아니라 표준 라이브러리 `go/src/sync/pool.go` 에 구현되어 있다. 핵심 필드는 `Pool.local`, `Pool.localSize`, `Pool.victim`, `Pool.victimSize` 이다. `local` 은 CPU P 개수만큼의 `poolLocal` 배열이고, 각 `poolLocal` 안에는 `private any` 와 `shared poolChain` 이 있다.

`poolLocal.private` 는 단 하나의 객체만 담을 수 있는 P 전용 슬롯이다. Put 은 private 이 비어 있으면 그 자리에 넣고, 이미 차 있으면 `shared` 큐로 밀어 넣는다. Get 은 먼저 자신의 private 을 보고, 없으면 `shared` 의 머리를 꺼낸다. 그래도 없으면 `getSlow` 가 다른 P 의 shared 를 훔치고, 마지막으로 victim cache 를 확인한다. private 은 락 없이 접근하는 대신 다른 P 가 훔치지 못하는 특징이 있다.

`shared` 는 `poolChain` 이라는 lock-free 이중 연결 리스트로 구현된다. `poolChain` 의 각 노드는 `poolDequeue` 로, pushHead 와 popHead 는 소유자 P 가 쓰고, popTail 은 도둑 P 가 쓴다. 소유자는 head 쪽에서만 push/pop 하므로 락이 필요 없고, 도둑은 tail 쪽에서 원자적으로 pop 한다. 이 구조는 Go 런타임의 work-stealing 스케줄러와 비슷한 아이디어를 재사용한 것이다.

GC 와의 연결 고리는 `runtime/mgc.go` 에서 등록된 `poolCleanup` 콜백이다. 이 함수는 GC 시작 시 실행되며, `oldPools` 리스트의 victim 을 비우고, `allPools` 리스트의 local 을 victim 으로 옮긴다. 즉 1회 GC 이후에는 local 이 victim 이 되고, 다음 GC 에서 victim 이 되어 있던 객체는 버려진다. Global 리스트는 `allPools` 와 `oldPools` 두 개로 유지되며, Put 이 처음 일어나면 `pinSlow` 를 통해 pool 이 `allPools` 에 등록된다.

위 데모에서는 GOMAXPROCS 를 잠시 1 로 고정했다. 그 이유는 private 이 P 에 귀속되기 때문이다. 만약 Put 을 P0 의 private 에 해두고 GC 후 Get 이 P1 에서 실행되면, victim cache 의 private 은 P1 에서 보이지 않는다. 구현상 `getSlow` 는 현재 P 의 victim private 만 읽고, 다른 P 의 victim private 은 훔치지 않는다. 따라서 실행 순서에 따라 재사용이 실패한 것처럼 보일 수 있다. 데모에서는 victim cache 수명이라는 핵심만 관찰하기 위해 P 를 하나로 제한했다.

## 돌려보기

이 디렉토리에서 그대로 실행할 수 있는 명령을 순서대로.

```bash
go vet ./...                 # 정적 검사
go build -o /dev/null ./...  # 컴파일 확인
go run .                     # 시연 실행
go test -v ./...             # 테스트
go test -race ./...          # 동시성 주제라면 반드시
go test -bench=. -benchmem ./...  # 할당 횟수 비교 벤치마크
```

`go run .` 에서는 victim cache 재사용 여부와 새 객체 생성 여부가 true/false 로 출력되고, 총 New 호출 수가 2 가 되는지 확인한다. 이어서 작은 객체 32바이트를 직접 할당, 포인터 pool, 값 타입 pool 로 각각 다뤘을 때 allocation event 수가 어떻게 달라지는지 본다. 포인터 pool 은 allocation 이 0 또는 1 에 가깝고, 값 타입 pool 은 직접 할당과 비슷하거나 더 많게 나온다.

`go test -v ./...` 에서는 TestVictimCacheLifecycle 이 GC 1회 후 재사용, GC 2회 후 소멸, 총 New 호출 2회를 검증한다. TestAllocationCounts 는 포인터 pool 이 직접 할당보다 allocation 이 적고, 값 타입 pool 은 직접 할당보다 allocation 이 줄지 않았는지 확인한다.

`go test -race ./...` 는 이 예제가 데이터 경합 없이 돌아가는지 확인한다. 데모 코드에 의도적으로 경합을 넣지 않았으므로 race 검사가 통과해야 한다.

`go test -bench=. -benchmem ./...` 는 직접 할당, 포인터 pool, 값 타입 pool 의 벤치마크를 b.ResetTimer 이후에 측정해 순수한 Get/Put 비용을 비교한다. benchmem 결과에서 allocs/op 가 포인터 pool 에서 0 에 가까운지, 값 타입 pool 에서는 매 루프마다 1 근처에 있는지 본다.

## 코드로 확인하기

`victimCacheResults` 는 `New` 호출 횟수를 atomic 카운터로 세면서 아래 수명을 검증한다. 처음 Get 에서 New 가 1회 호출되고, 객체를 Put 한 뒤 첫 번째 GC 를 호출하면 `pool.local` 이 `pool.victim` 으로 옮겨진다. 이어지는 Get 은 victim 에서 같은 포인터를 꺼내 재사용하므로 New 호출은 늘지 않는다. 두 번째와 세 번째 GC 를 연속으로 돌리면 이전 victim 은 비워지고, 마지막 Get 은 새 객체를 만들어야 하므로 총 New 호출 수는 2 가 된다.

`allocationCounts` 에서는 `runtime.MemStats.Mallocs` 를 사용해 allocation event 수를 센다. 직접 할당은 `new(byte32)` 를 1000번 반복하므로 거의 1000 이 된다. 포인터 pool 은 warm up 으로 내부 구조를 만들고, `runtime.GC()` 후에도 포인터가 그대로 왕복하므로 allocation 이 0 아니면 1 수준으로 떨어진다. 값 타입 pool 은 `Put(byte32)` 가 매번 `any` 로 변환되는 순간 힙에 박스가 생기므로 loop 반복 횟수와 비슷하거나 더 많은 allocation 이 나온다.

`main_test.go` 의 TestVictimCacheLifecycle 은 데모와 같은 함수를 호출하되, 개수와 불변식만 검사한다. GC 순서나 P 이동 순서에 의존하지 않도록 하기 위해 GOMAXPROCS 를 1 로 고정한 함수를 호출한다. TestAllocationCounts 는 1000번 반복 기준으로 포인터 pool 이 직접 할당보다 allocation 이 적다는 부등식, 값 타입 pool 은 직접 할당보다 allocation 이 줄지 않는다는 부등식을 확인한다. 시간 제한이나 정확한 allocation 수를 단정하지 않으므로 부하가 심한 CI 에서도 안정적이다.

## 모르면 겪는 일

값 타입을 pool 에 넣는 코드를 리뷰 없이 머지하면, allocation profile 에는 `runtime.convT32` 같은 boxing 경로가 상위권에 찍힌다. CPU 프로파일에는 보이지 않는데 p99 지연은 GC 주기마다 튀는 패턴이 생길 수 있다. 처음에는 "pool 을 썼으니 할당이 줄었겠지"라고 생각하지만 실제로는 직접 new 하는 것보다 더 많은 객체를 할당하고 있을 수 있다.

GC 1회 후 재사용과 GC 2회 소멸을 모르면, pool 에 넣은 객체가 왜 가끔 New 를 호출하는지 혼란스러워진다. 예를 들어 요청 수가 적어 GC 가 자주 돌면 pool 이 거의 항상 비어 보인다. 또 victim cache 때문에 "GC 가 돌았는데도 이전 객체가 나온다"는 버그로 오해할 수 있다. 정확히는 한 번 더 GC 가 돌아야 사라지는 의도된 동작이다.

pool 에 파일 핸들이나 네트워크 연결을 넣으면 심각한 리소스 누수가 생긴다. GC 주기가 길어지면 수천 개의 핸들이 pool 에 죽어 있는 채로 남아, OS 자원이 고갈된다. sync.Pool 은 해제 순서를 보장하지 않으므로 수명 주기를 애플리케이션이 제어해야 하는 자원에는 사용하면 안 된다.

마지막으로 private 슬롯의 P 귀속 성질을 모르고 벤치마크를 짜면 재사용이 불규칙하게 실패하는 것처럼 보인다. 특히 GOMAXPROCS 가 8, 16 인 환경에서 한 번 Put 하고 GC 후 Get 하는 실험은 P 이동에 따라 객체를 못 찾을 수 있다. 위 데모처럼 관찰 목적이면 GOMAXPROCS 를 1 로 고정하거나, 여러 개를 Put 해 shared 큐로 밀어 넣는 방식으로 통제해야 한다.

## 언제 신경 쓰고 언제 무시하나

초당 수천 건 이하의 작은 요청을 처리하는 서비스에서는 sync.Pool 이 주는 이득이 미미하다. Go 의 작은 객체 할당은 이미 mcache, mcentral 이 P 별로 최적화되어 있으므로, 직접 new 하는 것과 pool 을 쓰는 것의 차이가 수십 나노초에 불과할 수 있다. 오히려 코드가 복잡해지고 객체 수명에 대한 오해가 생길 위험이 크다.

sync.Pool 이 확실한 가치를 내는 지점은 Get/Put 빈도가 매우 높고, 객체 크기가 수백 바이트 이상이거나 초기화 비용이 큰 경우다. 예를 들어 JSON 인코더의 내부 버퍼, fmt 의 임시 버퍼, HTTP 응답 헤더 구조체처럼 매 요청마다 똑같은 모양의 객체를 반복해서 만들고 버리는 경우에 적합하다. 이럴 때는 포인터를 반환하는 New 를 쓰는 것이 거의 정답이다.

값 타입을 넣는 안티 패턴은 성능보다도 allocation profile 을 보고 판단하는 게 낫다. 작은 구조체를 any 로 넣으면 한 번의 Put 이 한 번의 heap boxing 을 부른다. 이게 병목이 되는 규모는 대개 수십만 회/초 이상이다. 그 이하에서는 코드 가독성을 위해 직접 할당을 쓰는 편이 낫다.

객체가 GC 주기 동안 살아 있는지 여부가 애플리케이션 정확성에 영향을 준다면 sync.Pool 을 쓰면 안 된다. pool 은 휘발성 캐시이므로 Get 이 항상 같은 객체를 반환한다고 보장하지 않는다. 객체 재사용 자체가 목적이라면 차라리 명시적인 free list 와 finalizer 조합 또는 sync.WaitGroup 보호 아래 수동 풀을 고려하는 것이 옳다.

## 더 파보기

- Go 소스: `go/src/sync/pool.go` — Pool, poolLocal, poolChain, poolDequeue, poolCleanup 전체 구현
- Go 소스: `go/src/runtime/mgc.go` — GC 시작 시 poolCleanup 이 호출되는 지점
- Go 소스: `go/src/runtime/mstats.go` — `runtime.MemStats.Mallocs` 필드가 allocation event 를 세는 방식
- Go 1.13 release notes: https://go.dev/doc/go1.13 — sync.Pool victim cache 도입 공식 설명
- Go issue: https://github.com/golang/go/issues/22950 — sync.Pool GC 성능 개선 관련 논의
- Go documentation: https://pkg.go.dev/sync#Pool — Pool 사용 지침과 caveat