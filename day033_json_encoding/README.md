# Day 33: JSON 인코딩/디코딩

## 개념 설명
Go 언어에서 JSON 데이터를 처리하려면 표준 라이브러리인 `encoding/json` 패키지를 사용합니다. 이 패키지는 데이터 직렬화(구조체 -> JSON)와 역직렬화(JSON -> 구조체)를 담당하는 `Marshal`과 `Unmarshal` 함수를 기본으로 제공합니다. 특히 구조체 필드에 백틱(`)을 사용해 `json:"필드명"` 태그를 부여하면, Go의 스네이크 케이스나 카멜 케이스 필드명을 JSON의 케빈 케이스나 원하는 이름으로 자유롭게 매핑할 수 있습니다.

`Marshal`은 Go 구조체를 JSON 바이트 슬라이스로 변환하고, `Unmarshal`은 역으로 JSON 바이트를 Go 구조체로 변환합니다. 이 과정에서 `omitempty` 옵션을 태그에 추가하면 필드 값이 공백(빈 문자열, 0, nil, false 등)일 때 해당 필드를 JSON에 포함시키지 않아 데이터 크기를 줄일 수 있습니다.

대용량 데이터나 네트워크 스트리밍 처리가 필요할 때는 `Marshal`/`Unmarshal` 대신 `json.NewEncoder`와 `json.NewDecoder`를 사용합니다. `Encoder`는 `io.Writer` 인터페이스를 구현한 객체(예: 파일, HTTP 응답, 버퍼)에 JSON을 직접 쓰며, `Decoder`는 `io.Reader`에서 읽은 데이터를 구조체에 매핑합니다. 이는 메모리 사용량을 줄이고 실시간 데이터 처리에 유리한 스트리밍 방식입니다.

## 코드 설명
- `User` 구조체에 `json:"id"`, `json:"omitempty"` 등의 태그를 설정하여 JSON 필드명과 생략 규칙을 정의했습니다.
- `Marshal`과 `Unmarshal`은 동일한 데이터를 구조체와 JSON 바이트 간에 왕복 변환하며, 에러 처리를 통해 변환 실패 시 중단합니다.
- `bytes.Buffer`는 `io.Writer`와 `io.Reader`를 모두 구현하므로 `Encoder`와 `Decoder` 테스트에 적합합니다. `Encode`와 `Decode`는 버퍼를 통해 스트림처럼 데이터를 읽고 쓰는 과정을 시연합니다.

## 핵심 포인트
- `json:"필드명"` 태그를 통해 Go 필드명과 JSON 키를 유연하게 매핑할 수 있습니다.
- `omitempty` 옵션을 활용하면 불필요한 null이나 빈 값을 JSON에서 생략할 수 있습니다.
- `Marshal`/`Unmarshal`은 전체 데이터를 메모리에 로드할 때 적합하며, 대용량 데이터에는 `Encoder`/`Decoder`를 사용해 스트리밍 처리하세요.
- 항상 에러 반환값을 체크하여 잘못된 JSON 포맷이나 타입 불일치 문제를 방어해야 합니다.
- `encoding/json`은 Go 공식 표준 라이브러리로, 별도 설치 없이 바로 사용할 수 있습니다.

## 참고 링크
- https://pkg.go.dev/encoding/json