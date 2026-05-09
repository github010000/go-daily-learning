# Day 35: HTTP 서버

## 개념 설명
Go 표준 라이브러리인 `net/http` 패키지는 HTTP 서버와 클라이언트를 구축하는 데 필요한 모든 기능을 내장하고 있습니다. 서버를 실행하기 위해 가장 기본적으로 사용되는 함수는 `http.ListenAndServe`이며, 이 함수는 지정한 포트에서 TCP 리스닝을 시작하고 들어오는 HTTP 요청을 자동으로 파싱하여 등록된 핸들러로 전달합니다. Go의 HTTP 서버는 내부적으로 효율적인 고루틴 풀을 관리하여 동시 요청 처리에 매우 효율적입니다.

요청과 응답을 처리하는 핵심 인터페이스는 `http.Request`와 `http.ResponseWriter`입니다. `http.Request`는 클라이언트로부터 전달받은 URL 경로, 쿼리 파라미터, HTTP 메서드, 헤더, 요청 본체 등 모든 정보를 담고 있는 구조체입니다. 반대로 `http.ResponseWriter`는 서버의 응답을 클라이언트에게 전달하는 인터페이스로, 상태 코드 설정, 헤더 추가, 바디 쓰기 등의 메서드를 제공합니다. 이 두 객체는 모든 HTTP 핸들러의 필수 인자입니다.

라우팅 기능은 `http.NewServeMux()`로 생성되는 서버 머직(ServeMux) 객체를 통해 관리됩니다. `HandleFunc`를 통해 URL 패턴과 해당 로직을 연결하면, 들어오는 요청이 경로에 맞춰 자동으로 적절한 핸들러 함수로 분기됩니다. 이를 기반으로 REST API의 GET, POST, DELETE 등의 표준 HTTP 메서드와 상태 코드를 조합하면 간결하고 강력한 웹 API를 구현할 수 있습니다.

## 코드 설명
- **데이터 구조 및 상태 관리**: `Todo` 구조체를 정의하고 메모리 슬라이스 `todos`에 데이터를 저장합니다. 다중 고루틴이 동시에 접근할 수 있으므로 `sync.Mutex`를 사용하여 데이터 무결성을 보호합니다.
- **핸들러 함수 구현**: `listTodos`, `createTodo`, `deleteTodo` 함수에서 `r *http.Request`를 통해 쿼리 파라미터를 읽거나 요청 본체(JSON)를 디코딩합니다. `w http.ResponseWriter`에는 `SetHeader`, `WriteHeader`, `json.NewEncoder(w).Encode()`를 사용하여 HTTP 상태 코드와 JSON 페이로드를 적절히 반환합니다.
- **라우팅 및 서버 실행**: `http.NewServeMux()`를 통해 인스턴스를 생성하고 `mux.HandleFunc`로 경로를 매핑합니다. 마지막으로 `http.ListenAndServe(":8080", mux)`를 호출하여 서버를 구동합니다. 이 함수는 에러가 발생하기 전까지 블로킹되므로, 이후 코드는 실행되지 않습니다.

## 핵심 포인트
- `http.ListenAndServe`는 블로킹 함수이므로 서버 실행 후 추가 코드는 별도의 고루틴(`go func()`) 또는 `defer`로 처리해야 합니다.
- `http.ResponseWriter`의 헤더는 `WriteHeader` 또는 첫 바디 쓰기 전에 반드시 설정해야 하며, 이미 응답이 시작된 후에는 헤더 변경이 무시됩니다.
- `ServeMux`는 패턴 매칭을 통해 요청을 핸들러로 라우팅하며, `HandleFunc`는 `http.HandlerFunc` 타입을 인자로 받습니다.
- REST API 구현 시 HTTP 메서드(GET, POST, DELETE)와 상태 코드(200, 201, 204 등)를 적절히 구분하여 클라이언트에 명확한 피드백을 제공해야 합니다.
- 메모리 기반 데이터는 동시성 충돌을 방지하기 위해 `sync.Mutex`로 보호하는 것이 좋으며, 실제 서비스에서는 Redis나 DB를 연결해야 합니다.

## 참고 링크
- https://pkg.go.dev/net/http
- https://pkg.go.dev/net/http#ServeMux
- https://pkg.go.dev/encoding/json