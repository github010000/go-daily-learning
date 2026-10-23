package main

import (
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// main은 Go 도구체계와 초기 설정을 시연합니다.
func main() {
	showToolchainOverview()
	showGoFmtInAction()
	showGoVetInAction()
	showBuildVsRun()
	showModuleExplanation()
	showPackageMain()
}

// showToolchainOverview 은 Go 도구체계의 철학과 주요 명령어를 설명합니다.
func showToolchainOverview() {
	fmt.Println("1. Go 도구체계 (Toolchain)")
	fmt.Println("   Go는 '표준이 하나'인 언어입니다. 포맷터, 린터, 빌더, 테스터가 모두")
	fmt.Println("   표준 도구에 포함되어 있어 IDE에 구애받지 않고 동일한 결과를 얻습니다.")
	fmt.Println("   - go fmt : 코드 스타일 강제")
	fmt.Println("   - go vet : 정적 분석")
	fmt.Println("   - go build : 바이너리 생성")
	fmt.Println("   - go run : 컴파일 후 실행")
	fmt.Println("   - go test : 테스트 실행")
	fmt.Println("   - go mod : 모듈 관리")
	fmt.Println()
}

// showGoFmtInAction 은 go fmt가 코드를 어떻게 정리하는지 보여줍니다.
func showGoFmtInAction() {
	fmt.Println("2. go fmt - 자동 코드 포맷팅")
	fmt.Println("   Go는 스타일 논의 대신 도구가 강제합니다. import 순서, 탭 들여쓰기,")
	fmt.Println("   한 줄에 한 문장 등이 자동 정리됩니다.")
	fmt.Println()

	unformatted := `package main
import "fmt"
func main(){fmt.Println("안녕")}`

	formattedBytes, err := format.Source([]byte(unformatted))
	if err != nil {
		fmt.Printf("코드 파싱 실패: %v\n", err)
		return
	}

	fmt.Println("   [포맷 전 코드]")
	fmt.Println(unformatted)
	fmt.Println()

	fmt.Println("   [포맷 후 코드 - go fmt가 자동으로 정리]")
	fmt.Println(string(formattedBytes))
	fmt.Println()
}

// showGoVetInAction 은 go vet가 어떤 버그를 찾는지 보여줍니다.
func showGoVetInAction() {
	fmt.Println("3. go vet - 정적 분석")
	fmt.Println("   go vet는 컴파일은 되지만 실행 시 문제가 될 수 있는 버그를 찾습니다.")
	fmt.Println("   특히 fmt.Printf의 포맷 문자열과 인수 불일치를 감지합니다.")
	fmt.Println()

	tmpDir, err := os.MkdirTemp("", "go-vet-demo-")
	if err != nil {
		fmt.Printf("임시 디렉토리 생성 실패: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	badCode := `package main

import "fmt"

func main() {
    name := "Alice"
    // 인수가 부족하여 go vet가 에러를 감지함
    fmt.Printf("안녕, %s\n")
}
`
	filePath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(filePath, []byte(badCode), 0644); err != nil {
		fmt.Printf("임시 파일 작성 실패: %v\n", err)
		return
	}

	cmd := exec.Command("go", "vet", ".")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()

	fmt.Println("   [잠재적 버그 예시]")
	fmt.Println("    func printName(name string) {")
	fmt.Println("\t\t// 틀린 코드: 인수가 부족")
	fmt.Println("\t\tfmt.Printf(\"안녕, %s\\n\") // %s에 대응할 인수가 없음")
	fmt.Println("    }")
	fmt.Println()

	if err != nil {
		fmt.Println("   go vet 실행 결과 (에러 감지 성공):")
		fmt.Printf("   %s", output)
		fmt.Println()
		fmt.Println("   이 코드는 컴파일은 되지만, 실행 시 잘못된 값이 출력됩니다.")
		fmt.Println("   go vet는 이런 '미묘한 버그'를 컴파일 전에 잡습니다.")
	} else {
		fmt.Println("   go vet가 버그를 감지하지 못했습니다.")
	}
	fmt.Println()
}

// showBuildVsRun 은 go build와 go run의 차이를 보여줍니다.
func showBuildVsRun() {
	fmt.Println("4. go build vs go run")
	fmt.Println()
	fmt.Println("   [실수 버전 - go build 후 바이너리 확인]")
	fmt.Println("    $ go build")
	fmt.Println("    $ ls")
	fmt.Println("    hello  hello.go  // 바이너리 hello가 생성됨")
	fmt.Println()
	fmt.Println("   [고친 버전 - go run으로 바로 실행]")
	fmt.Println("    $ go run .")
	fmt.Println("    Hello, World!")
	fmt.Println("    // 임시 파일이 자동으로 정리됨")
	fmt.Println()
	fmt.Println("   go build: 배포용 바이너리 생성")
	fmt.Println("   go run: 개발 중 빠른 테스트")
	fmt.Println("   두 명령 모두 동일한 실행 결과를 가집니다.")
	fmt.Println()
}

// showModuleExplanation 은 go.mod의 중요성을 설명합니다.
func showModuleExplanation() {
	fmt.Println("5. go mod - 모듈 관리")
	fmt.Println()

	content, err := os.ReadFile("go.mod")
	if err != nil {
		fmt.Println("   go.mod 파일을 읽을 수 없습니다.")
		return
	}

	fmt.Println("   [현재 프로젝트의 go.mod]")
	fmt.Println(string(content))
	fmt.Println()

	fmt.Println("   모듈이 필요한 이유:")
	fmt.Println("   1. 의존성 버전 관리: 같은 라이브러리의 다른 버전 충돌 방지")
	fmt.Println("   2. 재현 가능한 빌드: go.sum으로 해시 검증")
	fmt.Println("   3. 네임스페이스 분리: 프로젝트 간 패키지 이름 충돌 방지")
	fmt.Println()
	fmt.Println("   Python의 requirements.txt, Node의 package.json과 유사하지만,")
	fmt.Println("   Go는 표준 도구에 통합되어 별도 패키지 매니저가 없습니다.")
	fmt.Println()
}

// showPackageMain 은 package main의 중요성을 설명합니다.
func showPackageMain() {
	fmt.Println("6. package main 과 func main")
	fmt.Println()
	fmt.Println("   [올바른 구조]")
	fmt.Println("    package main  // 실행 가능한 프로그램")
	fmt.Println("    ")
	fmt.Println("    import \"fmt\"")
	fmt.Println("    ")
	fmt.Println("    func main() {")
	fmt.Println("\t\tfmt.Println(\"Hello\")")
	fmt.Println("    }")
	fmt.Println()
	fmt.Println("   package main이어야 하는 이유:")
	fmt.Println("   - 컴파일러에게 '실행 가능한 바이너리를 만든다'고 알려줌")
	fmt.Println("   - 다른 패키지는 library로 컴파일됨")
	fmt.Println()
	fmt.Println("   다른 언어와의 차이:")
	fmt.Println("   - Python: 스크립트 파일 자체가 실행")
	fmt.Println("   - Java: public static void main(String[] args) 클래스 필요")
	fmt.Println("   - Go: 명시적 package 구조 + 명시적 main 함수")
	fmt.Println()
}

// isWindows는 현재 시스템이 Windows인지 확인합니다.
func isWindows() bool {
	if os.Getenv("GOOS") == "windows" {
		return true
	}
	cmd := exec.Command("cmd", "/c", "echo", "%OS%")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "Windows") {
		return true
	}
	return false
}