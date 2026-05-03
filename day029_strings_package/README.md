# Day 29: strings & strconv 패키지

## 개념 설명
Go 언어에서 문자열은 불변 객체입니다. 기존 문자열을 수정하거나 결합할 때마다 새로운 메모리 할당이 발생하므로, 반복적인 문자열 연산은 성능 저하와 GC(가비지 컬렉터) 부하를 유발할 수 있습니다. 이를 해결하기 위해 `strings` 패키지는 문자열 탐색, 분리, 치환, 정렬 등 다양한 조작 기능을 제공하며, `strings.Builder`는 내부 버퍼를 활용해 문자열을 효율적으로 누적할 수 있게 합니다.

`strconv` 패키지는 문자열과 Go의 기본 타입(int, float, bool 등) 간의 변환을 담당합니다. 실제 개발에서는 사용자 입력 처리, 설정 파일 읽기, 외부 API 응답 파싱 시 타입 변환이 필수적으로 발생하므로, 변환 함수의 정확한 사용법과 에러 처리 패턴을 숙지하는 것이 중요합니다.

## 코드 설명
- **문자열 조작 흐름**: `TrimSpace`로 양쪽 공백을 정리한 후, `Split`로 공백 기준으로 단어를 나누고 `Join`으로 하이픈을 사이에 붙여 다시 합칩니다. `ReplaceAll`은 모든 `Go`를 `Golang`으로 치환하며, `Contains`는 최종 문자열에 `Golang`이 있는지 확인합니다.
- **strings.Builder**: `+` 연산은 매 단계마다 새 문자열을 생성하지만, `Builder`는 내부 슬라이스를 동적으로 확장해 메모리 할당을 최소화합니다. `WriteString`으로 차례로 붙인 후 `String()`으로 최종 문자열을 추출합니다.
- **타입 변환**: `strconv.Atoi`는 문자열을 `int`로, `Itoa`는 `int`를 문자열로 변환합니다. 부동소수점은 `ParseFloat`로, `bool`은 `FormatBool`로 처리하며, 변환 실패 시 에러가 반환되므로 반드시 `err` 확인이 필요합니다.

## 핵심 포인트
- 빈번한 문자열 결합이 필요할 때는 `+` 연산 대신 `strings.Builder`를 사용하여 메모리 효율과 실행 속도를 개선하세요.
- `strconv` 함수는 항상 에러 반환 값을 검증하거나, 사전에 `strings.TrimSpace` 등으로 입력을 정제해 런타임 예외를 방지하세요.
- `strings.ReplaceAll`은 모든 일치 항목을 교체하며, 특정 횟수만 교체하려면 `strings.Replace(문자열, old, new, n)`의 `n` 파라미터를 사용하세요.
- `strconv.ParseFloat`의 두 번째 인자는 소수점 자릿수(64 또는 32)를 의미하며, 부동소수점 정밀도 설정에 따라 메모리 사용량과 연산 정확도가 달라집니다.

## 참고 링크
- strings 패키지: https://pkg.go.dev/strings
- strconv 패키지: https://pkg.go.dev/strconv