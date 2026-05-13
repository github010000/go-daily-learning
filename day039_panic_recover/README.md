# Day 39: panic과 recover

## 개념 설명 섹션
panic은 Go 언어에서 복구하기 어렵거나 심각한 오류가 발생했을 때 호출하는 빌트인 함수입니다. 일반적으로 nil 포인터 접근, 슬라이스 인덱스 벗어남, type assertion 실패 등의 런타임 오류가 발생하면 Go 런타임이 자동으로 panic을 호출하여 프로그램 실행을 중단합니다. 이를 통해 개발자는 문제를 빠르게 인지하고 수정할 수 있습니다.

recover는 defer와 함께 사용하여 panic으로 인한 프로그램 종료를 방지합니다. defer 함수 내에서 recover를 호출하면, 현재 실행 중인 goroutine에서 발생한 panic 값을 받아올 수 있습니다. recover를 성공적으로 호출하면 panic이 소멸되고, defer 함수가 종료된 후 원래 실행 흐름이 중단된 지점 바로 다음 줄로 넘어갑니다.

하지만 recover는 사용에 신중해야 합니다. 공식 가이드라인에 따르면 recover는 반드시 라이브러리나 패키지 경계에서만 사용해야 합니다. 애플리케이션의 비즈니스 로직 내부에서 recover를 남용하면 에러 처리 흐름이 불분명해지고, 디버깅과 테스트를 매우 어렵게 만들 수 있습니다. 따라서 라이브러리에서는 panic을 error로 변환하여 반환하는 패턴을 사용해야 합니다.

## 코드 설명 섹션
- **unsafeOperation 함수**: nil로 초기화된 슬라이스의 인덱스 접근을 통해 의도적으로 panic을 유발합니다. defer 블록 내에서 recover를 호출하여 panic 메시지를捕获하고, 프로그램이 정상적으로 종료되도록 처리합니다.
- **libraryFunction 함수**: 라이브러리 경계에서의 안전한 recover 패턴을 보여줍니다. 입력값이 음수일 경우 panic을 호출하고, defer에서 이를捕获하여 error 타입으로 변환합니다. 호출 측에서는 error를 확인하여 정상적인 에러 처리를 진행합니다.
- **main 함수**: recover를 사용하여 의도적인 panic을 처리합니다. defer로 등록된 recover 함수가 panic 값을 받아 출력하며, recover 이후의 코드는 실행되지 않음을 보여줍니다.

## 핵심 포인트 섹션
- panic은 일반적인 에러 처리가 아닌, 복구 불가능한 심각한 오류나 개발자의 실수를 알릴 때만 사용해야 합니다.
- recover는 반드시 defer 블록 내부에서 호출해야 하며, 호출되지 않으면 nil을 반환합니다.
- recover는 라이브러리 경계에서만 사용하고, 애플리케이션 코드는 error 반환으로 처리해야 합니다.
- panic이 발생하면 해당 함수의 나머지 코드는 실행되지 않지만, 등록된 defer는 모두 실행됩니다.
- recover를 사용하면 프로그램 실행이 재개되지만, panic을 일으킨 원인이 해결되지 않았으므로 주의가 필요합니다.

## 참고 링크 섹션
- [Go 공식 문서: Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover)