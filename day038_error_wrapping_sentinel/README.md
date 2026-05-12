# Day 38: 에러 래핑 심화 & 센티널 에러

## 개념 설명
Go 1.13에서 도입된 에러 래핑(Error Wrapping) 기능은 에러 처리를 훨씬 더 강력하고 유연하게 만들어 주었습니다. 과거에는 에러를 문자열로 조합하거나 구조체로 감싸야 했다면, 이제 `fmt.Errorf`의 `%w` 지시어를 통해 에러를 감쌀 수 있습니다.

에러 래핑의 핵심은 **에러 체인(Error Chain)**입니다. 에러를 래핑하면 새로운 에러가 생성되지만, 기존 에러와의 연결고리를 잃지 않습니다. 이를 통해 하위 레이어에서 발생한 구체적인 에러를 상위 레이어로 전파하면서도, 에러의 종류를 쉽게 판단할 수 있습니다.

이를 가능하게 하는 것이 `errors.Is`와 `errors.As` 함수입니다. `errors.Is`는 에러 체인을 따라가면서 특정 에러(센티널 에러)가 포함되어 있는지 확인하고, `errors.As`는 에러 체란을 따라가며 특정 타입의 에러를 추출합니다. 이를 통해 복잡한 `switch` 나 `type assertion` 없이도 유연한 에러 처리가 가능합니다.

## 코드 설명
1.  **센티널 에러 정의 (`var`)**: 애플리케이션 전역에서 사용할 수 있는 고정된 에러 변수(`ErrNotFound`, `ErrInvalidData`)를 정의합니다. 이는 특정 오류 상태를 식별하는 기준점이 됩니다.
2.  **커스텀 에러 타입 (`AppError`)**: `Code`, `Message`, `Err` 필드를 가진 구조체를 정의합니다. `Error()` 메서드로 문자열을 반환하고, `Unwrap()` 메서드를 통해 내부 에러를 반환하여 에러 체이닝을 구현합니다.
3.  **에러 감싸기 (`getUserProfile`)**: `findUser`에서 에러가 발생하면 `fmt.Errorf("...: %w", err)`를 사용하여 에러를 감쌉니다. `%w`는 에러를 감싸는 새로운 에러를 만듭니다.
4.  **에러 확인 (`handleRequest`)**:
    *   `errors.Is(err, ErrNotFound)`: 감싸진 에러 속에 `ErrNotFound`가 있는지 확인합니다.
    *   `errors.As(err, &appErr)`: 감싸진 에러 체인 속에서 `*AppError` 타입의 에러를 찾아 변수에 할당합니다.

## 핵심 포인트
*   **`%w` vs `%v`**: 에러를 감싸서 체이닝을 유지하려면 `fmt.Errorf`에서 `%w`를 사용해야 합니다. `%v`를 쓰면 에러가 단순 문자열로 결합되어 `errors.Is`로 찾을 수 없습니다.
*   **`errors.Is`**: 에러가 특정 기준(센티널 에러)과 일치하는지 확인할 때 사용 (if 문 조건으로 적합).
*   **`errors.As`**: 에러가 특정 타입인지 확인하고, 해당 타입의 필드(코드, 메시지 등)에 접근해야 할 때 사용.
*   **`Unwrap` 메서드**: 커스텀 에러 구조체를 정의할 때 `Unwrap() error` 메서드를 구현해야 `errors.Is/As`가 내부 에러를 탐색할 수 있습니다.

## 참고 링크
*   [Go Blog: Go 1.13 Errors (영어)](https://go.dev/blog/go1.13-errors)
*   [Go Standard Library: errors package](https://pkg.go.dev/errors)