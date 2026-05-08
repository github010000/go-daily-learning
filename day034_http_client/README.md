# Day 34: HTTP 클라이언트

## 개념 설명
Go 언어에서 외부 API와 통신하거나 HTTP 요청을 보내기 위해서는 `net/http` 패키지의 클라이언트 기능이 필수적입니다. `http.Get`와 `http.Post`는 가장 간단하게 GET/POST 요청을 보내는 편의 함수로, 내부에서 디폴트 `http.Client`를 사용합니다. 하지만 타임아웃, 리다이렉트 제어, 커스텀 헤더 설정, 커넥션 풀 관리 등이 필요할 때는 `http.Client` 구조체를 직접 생성하여 요청을 제어해야 합니다.

정교한 제어가 필요할 때는 `http.NewRequest`를 통해 요청 객체를 먼저 만든 후, `Client.Do(req)` 메서드를 호출합니다. 이 방식은 HTTP 메서드(GET, POST, PUT, DELETE 등)을 유연하게 지정할 수 있고, 요청 헤더(Request Header)나 바디(Body)를 세밀하게 설정할 수 있습니다. 특히 인증 토큰이나 콘텐츠 타입을 명시적으로 전달해야 하는 현대 웹 개발에서 `NewRequest` 패턴은 표준처럼 사용됩니다.

응답을 받을 때는 반드시 `resp.Body.Close()`를 호출하여 리소리를 반환해야 합니다. 일반적으로 `defer`를 사용해 함수 종료 시 자동으로 닫히도록 설정합니다. 응답 바디는 `io.ReadAll`이나 `io.Copy`를 사용해 읽고, 상태 코드(`resp.StatusCode`)를 확인하여 성공 여부나 에러를 처리해야 합니다. 실패할 경우 리드 에러가 발생할 수 있으므로 상태 코드가 200~299 범위인지 확인하는 것이 좋습니다.

## 코드 설명
1. **커스텀 Client 설정**: `&http.Client{Timeout: 10 * time.Second}`로 요청 시간 제한을 설정하여 응답이 늦거나 응답이 없을 때 고아 상태가 되는 것을 방지합니다.
2. **http.NewRequest 및 헤더**: GET 요청용 객체를 생성하고, `req.Header.Set`으로 User-Agent와 Accept 헤더를 추가합니다. 이는 서버가 요청의 형식과 출처를 인식하도록 합니다.
3. **client.Do(req)**: 커스텀 클라이언트로 요청을 실행합니다. `defer resp.Body.Close()`를 통해 응답 바디가 닫히지 않는 리소스 누수를 방지합니다.
4. **http.Get**: 디폴트 클라이언트를 사용한 단순 GET 요청 예시입니다. 헤더나 타임아웃을 세팅해야 한다면 `Do` 방식을 권장합니다.
5. **http.Post**: JSON 페이로드를 보내는 POST 요청 예시이며, `strings.NewReader`로 바디 스트림을 전달합니다. 두 번째 인자 `"application/json"`은 Content-Type을 명시합니다.
6. **응답 바디 처리**: `io.ReadAll`로 바디를 읽고, 상태 코드와 시작 부분을 출력하여 통신 결과를 확인합니다. 슬라이스 길이는 안전 범위를 위해 동적으로 조절합니다.

## 핵심 포인트
- `http.Get`/`http.Post`는 빠른 테스트용, `http.NewRequest` + `client.Do()`는 프로덕션 및 정교한 제어를 위해 사용하세요.
- 항상 `defer resp.Body.Close()`를 사용하여 파일 디스크립터와 커넥션 리소리를 누수 없이 반환하세요.
- 커스텀 헤더 설정은 `req.Header.Set("Key", "Value")` 또는 `http.Header` 맵을 통해 수행합니다.
- 응답 바디를 읽기 전에는 반드시 `resp.StatusCode`가 2xx 범위가 맞는지 확인하여 에러 처리를 강화하세요.
- `http.Client`의 `Timeout`과 `Transport`를 설정하면 리소스 낭비를 방지하고 안정적인 통신이 가능합니다.

## 참고 링크
- https://pkg.go.dev/net/http