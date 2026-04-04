# Day 4: 상수와 iota

## 개념 설명

**상수(const)**는 프로그램 실행 동안 변경될 수 없는 고정된 값입니다. `const` 키워드를 사용해 선언하며, 초기화 없이는 정의할 수 없습니다. 상수는 컴파일 시점에 결정되며, 런타임 오버헤드 없이 빠르게 접근됩니다. 상수는 타입이 명시된(typed) 경우와 명시되지 않은(untyped) 경우로 구분됩니다. untyped 상수는 숫자 리터럴처럼 다양한 숫자 타입으로 암시적 변환(coversion)이 가능합니다.

**iota**는 Go의 `const` 블록(연속된 상수 정의 블록) 내에서만 사용 가능한 특수한 식별자입니다. const 블록의 첫 번째 줄에서 iota는 0을 반환하고, 이후 각 줄마다 1씩 증가합니다. 이를 활용해 열거형(enumeration)을 구현합니다. Go에는 `enum` 키워드가 없기 때문에, `const ( ... )`와 `iota` 조합이 표준적인 열거형 패턴입니다. iota는 비트 연산(예: `1 << iota`)과 결합해 용량 단위(KB, MB, GB 등)나 상태 비트 플래그도 간결하게 정의할 수 있습니다.

iota는 **const 블록 단위로 재설정**되며, 한 블록이 끝나면 다음 블록에서 다시 0부터 시작합니다. 또, 한 줄에서 iota를 여러 번 참조하거나, 식에 직접 `iota`를 명시적으로 넣어도 올바르게 작동합니다. untyped 상수는 타입 안정성보다 유연성을 우선하지만, 타입을 명시하면 실수를 줄이고 코드 의도를 명확히 표현할 수 있습니다.

## 코드 설명

- **Untyped Constants**: `UntypedConst = 42`, `UntypedFloatConst = 3.14`는 타입이 명시되지 않은 숫자 상수입니다. `int64`, `float64` 등 다양한 숫자형 변수에 할당 가능합니다.
- **Typed Constants**: `TypedIntConst int = 100` 등은 명시적 타입을 갖는 상수로, 해당 타입과만 호환됩니다.
- **간단 열거형**: `Red`, `Green`, `Blue`는 `iota` 덕분에 각각 0, 1, 2로 자동 할당됩니다. `const (...)` 블록의 각 줄에 iota가 암묵적으로 삽입됩니다.
- **요일 열거형**: `Monday`~`Friday`도 동일한 구조로, 0~4까지 할당됩니다.
- **비트 시프트 + iota**: `KB`, `MB`, `GB`, `TB`는 `1 << (10 * iota)`로 1, 1024, 1048576 등으로 계산됩니다. 메모리 크기, 파일 단위 등에 유용합니다.
- **사용자 정의 타입 + offset**: `Color` 타입은 `iota + 5`로 5부터 시작하는 고유한 정수열로 정의됩니다. 타입 안전성과 커스터마이징을 동시에 제공합니다.

## 핵심 포인트

- `const`는 런타임에 변경 불가능한 값을 정의하는 키워드이며, **untyped**(유연)와 **typed**(안정)으로 구분됨
- `iota`는 **const 블록 내에서만 유효**하며, 줄마다 0, 1, 2, ... 로 증가하는 자동 정수 생성자
- `iota`는 **비트 시프트 연산과 결합**해 용량 단위·상태 비트 등 복잡한 상수도 간결하게 표현 가능
- **const 블록이 닫히면 iota는 다시 0으로 초기화**되며, 별도의 블록 간 간섭 없이 사용 가능

## 참고 링크

- [Go Tour: Constants and Enumerations](https://go.dev/tour/basics/15)
- [Go Language Specification: Constants](https://go.dev/ref/spec#Constants)
- [Go Blog: iota](https://go.dev/blog/constants)