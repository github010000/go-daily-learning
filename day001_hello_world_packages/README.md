# Day 1: Hello, World! & 패키지

## 개념 설명

Go 프로그램은 **패키지**라는 단위로 구성됩니다. 가장 기본적인 실행 파일은 `main` 패키지에 속하며, `main` 함수를 포함해야 실행 가능합니다. 이 `main` 함수가 프로그램의 진입점(entry point)이 됩니다.

모든 Go 소스 파일은 `package` 선언으로 시작해야 하며, `package main`은 이 파일이 실행 가능한 프로그램의 일부임을 나타냅니다. `import` 키워드를 통해 표준 라이브러리나 타 패키지를 불러올 수 있으며, 예를 들어 `fmt` 패키지는 콘솔 입출력을 담당합니다.

Go에서는 각 파일이 하나의 패키지만 포함해야 하며, 같은 디렉터리 내 모든 소스 파일은 동일한 패키지 이름을 가져야 합니다. 실행 가능한 프로그램은 반드시 `main` 패키지여야 하며, `main()` 함수가 있어야 합니다.

## 코드 설명

- `package main`: 이 소스 파일이 실행 가능한 프로그램의 진입점임을 선언합니다.
- `import ("fmt" "os")`: 표준 라이브러리의 `fmt`(format I/O)와 `os` 패키지를 불러옵니다. `fmt.Println`은 줄바꿈이 자동으로 포함된 출력 함수입니다.
- `func main() { ... }`: 프로그램의 시작 지점입니다. 이 함수는 인자도 없고 반환값도 없습니다.
- `fmt.Println("Hello, World!")`: 기본 메시지 출력. 자동으로 개행이 추가됩니다.
- `fmt.Printf(...)`: C 스타일의 서식 문자열을 사용해 출력. `\n`으로 개행을 직접 명시해야 합니다.
- `os.Exit(0)`: 프로그램을 조기 종료하고 종료 코드 0(성공)을 반환. 실제로는 생략 가능하지만 명시적 확인용입니다.

## 핵심 포인트

- Go 프로그램은 반드시 `package main`과 `func main()`을 가져야 실행 가능합니다.
- 외부 패키지는 `import`로 불러오며, 사용하지 않으면 컴파일 오류가 발생합니다.
- `fmt.Println`은 간단하고 자주 사용되는 출력 함수이며, 자동으로 개행을 추가합니다.
- Go는 실행 가능한 프로그램과 라이브러리 패키지를 구분하며, `main` 패키지는 실행 전용입니다.

## 참고 링크

- [Go Tour - Basic Syntax](https://go.dev/tour/basics/1)
- [Go by Example - Packages](https://gobyexample.com/packages)
- [Go Wiki - Main Package](https://go.dev/doc/code#Command)