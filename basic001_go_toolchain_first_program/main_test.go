package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoFmtFormatsCode 는 go fmt가 코드를 표준 스타일로 포맷팅하는지 검증합니다.
func TestGoFmtFormatsCode(t *testing.T) {
	t.Run("gofmt는 코드를 표준 스타일로 포맷팅한다", func(t *testing.T) {
		// 현재 디렉토리에서 go fmt 실행
		cmd := exec.Command("go", "fmt", ".")
		err := cmd.Run()
		if err != nil {
			t.Fatalf("go fmt 실행 실패: %v", err)
		}

		// main.go 파일을 읽어서 포맷팅 결과 확인
		content, err := os.ReadFile("main.go")
		if err != nil {
			t.Fatalf("main.go 읽기 실패: %v", err)
		}

		// 포맷팅된 코드는 들여쓰기가 있어야 함 (탭 사용)
		code := string(content)
		if !strings.Contains(code, "\t") {
			t.Error("gofmt가 적용되지 않았습니다: 탭 들여쓰기가 없습니다")
		}

		// package main이 있어야 함
		if !strings.Contains(code, "package main") {
			t.Error("gofmt 후 package main이 제거되었습니다")
		}
	})

	t.Run("gofmt는 import 블록을 정리한다", func(t *testing.T) {
		// import 블록이 올바르게 정리되었는지 확인
		cmd := exec.Command("go", "fmt", ".")
		err := cmd.Run()
		if err != nil {
			t.Fatalf("go fmt 실행 실패: %v", err)
		}

		content, err := os.ReadFile("main.go")
		if err != nil {
			t.Fatalf("main.go 읽기 실패: %v", err)
		}

		code := string(content)

		// import 문이 있어야 함
		if !strings.Contains(code, "import") {
			t.Error("gofmt 후 import 블록이 제거되었습니다")
		}

		// import가 package 전에 있어야 함 (표준 Go 스타일)
		importIndex := strings.Index(code, "import")
		packageIndex := strings.Index(code, "package main")
		if importIndex == -1 || packageIndex == -1 || importIndex > packageIndex {
			t.Error("import가 package 전에 있어야 합니다")
		}
	})
}

// TestGoBuildCreatesBinary 는 go build가 실행 가능한 바이너리를 생성하는지 검증합니다.
func TestGoBuildCreatesBinary(t *testing.T) {
	t.Run("go build는 실행 가능한 바이너리를 생성한다", func(t *testing.T) {
		// 먼저 go fmt로 코드 정리 (빌드 전 필수)
		fmtCmd := exec.Command("go", "fmt", ".")
		if err := fmtCmd.Run(); err != nil {
			t.Fatalf("go fmt 실행 실패: %v", err)
		}

		// go build 실행
		buildCmd := exec.Command("go", "build", "-o", "test_binary", ".")
		err := buildCmd.Run()
		if err != nil {
			t.Fatalf("go build 실행 실패: %v", err)
		}

		// 바이너리 파일이 생성되었는지 확인
		binaryPath := "test_binary"
		if isWindows() {
			binaryPath = "test_binary.exe"
		}

		info, err := os.Stat(binaryPath)
		if err != nil {
			t.Fatalf("생성된 바이너리를 찾을 수 없음: %v", err)
		}

		// 파일이 실제로 존재하고 크기가 0이 아님
		if info.Size() == 0 {
			t.Error("생성된 바이너리 파일이 비어 있습니다")
		}

		// 테스트 후 정리
		defer os.Remove(binaryPath)
	})

	t.Run("go build는 에러가 있는 코드를 컴파일하지 않는다", func(t *testing.T) {
		// 에러가 있는 임시 파일 생성
		tmpDir, err := os.MkdirTemp("", "go-test-build-")
		if err != nil {
			t.Fatalf("임시 디렉토리 생성 실패: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// 에러가 있는 코드 작성
		badCode := `package main

func main() {
    x := 1
    y := "문자열"
    z := x + y // 타입 에러
}
`
		filePath := filepath.Join(tmpDir, "main.go")
		if err := os.WriteFile(filePath, []byte(badCode), 0644); err != nil {
			t.Fatalf("임시 파일 작성 실패: %v", err)
		}

		// go build 실행 (에러가 예상됨)
		buildCmd := exec.Command("go", "build", ".")
		buildCmd.Dir = tmpDir
		err = buildCmd.Run()
		if err == nil {
			t.Error("에러가 있는 코드가 컴파일되었습니다: 에러가 있어야 합니다")
		}
	})
}

// TestGoTestRunsTests 는 go test가 테스트를 실행하고 결과를 반환하는지 검증합니다.
func TestGoTestRunsTests(t *testing.T) {
	t.Run("go test는 테스트를 실행한다", func(t *testing.T) {
		// go test 실행
		cmd := exec.Command("go", "test", "./...")
		err := cmd.Run()
		if err != nil {
			t.Fatalf("go test 실행 실패: %v", err)
		}
	})

	t.Run("go test -v는 상세 출력을 생성한다", func(t *testing.T) {
		cmd := exec.Command("go", "test", "-v", "./...")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go test -v 실행 실패: %v", err)
		}

		// 상세 출력에 테스트 이름이 포함되어야 함
		outputStr := string(output)
		if !strings.Contains(outputStr, "Test") {
			t.Error("go test -v 출력에 테스트 이름이 없습니다")
		}
	})
}

// TestGoVetDetectsErrors 는 go vet가 잠재적 버그를 감지하는지 검증합니다.
func TestGoVetDetectsErrors(t *testing.T) {
	t.Run("go vet는 올바른 코드를 통과시킨다", func(t *testing.T) {
		// go vet 실행
		cmd := exec.Command("go", "vet", "./...")
		err := cmd.Run()
		if err != nil {
			t.Fatalf("go vet 실행 실패: %v", err)
		}
	})

	t.Run("go vet는 출력 포맷 에러를 감지한다", func(t *testing.T) {
		// 임시 디렉토리 생성
		tmpDir, err := os.MkdirTemp("", "go-vet-test-")
		if err != nil {
			t.Fatalf("임시 디렉토리 생성 실패: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// go vet가 감지할 수 있는 버그가 있는 코드
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
			t.Fatalf("임시 파일 작성 실패: %v", err)
		}

		// go vet 실행
		cmd := exec.Command("go", "vet", ".")
		cmd.Dir = tmpDir
		err = cmd.Run()
		if err == nil {
			t.Error("go vet가 출력 포맷 에러를 감지하지 못했습니다")
		}
	})
}

// TestGoModModuleFile 은 go.mod 파일의 필수 내용을 검증합니다.
func TestGoModModuleFile(t *testing.T) {
	t.Run("go.mod는 module 문을 포함한다", func(t *testing.T) {
		content, err := os.ReadFile("go.mod")
		if err != nil {
			t.Fatalf("go.mod 읽기 실패: %v", err)
		}

		code := string(content)
		if !strings.Contains(code, "module ") {
			t.Error("go.mod에 module문이 없습니다")
		}
	})

	t.Run("go.mod는 go 버전 문을 포함한다", func(t *testing.T) {
		content, err := os.ReadFile("go.mod")
		if err != nil {
			t.Fatalf("go.mod 읽기 실패: %v", err)
		}

		code := string(content)
		if !strings.Contains(code, "go ") {
			t.Error("go.mod에 go 버전문이 없습니다")
		}
	})
}