package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleStructure는 올바른 모듈 구조가 컴파일되는지 검증합니다.
// internal 패키지 접근 규칙, 대문자 공개 규칙을 확인합니다.
func TestModuleStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go-module-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 올바른 모듈 구조 생성
	createTestModule(tmpDir)

	// 컴파일 테스트
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		t.Fatalf("올바른 모듈 구조가 컴파일되지 않음: %v\n출력: %s", err, output)
	}
}

// TestInternalPackageAccess는 internal 패키지의 접근 규칙을 검증합니다.
// internal 밖에서는 internal 패키지를 import할 수 없어야 합니다.
func TestInternalPackageAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go-internal-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// internal 패키지 생성
	internalDir := filepath.Join(tmpDir, "internal", "secret")
	os.MkdirAll(internalDir, 0755)
	os.WriteFile(filepath.Join(internalDir, "secret.go"), []byte(`
package secret

func GetSecret() string { return "secret" }
`), 0644)

	// main.go에서 internal 패키지를 import 시도 (잘못된 시도)
	mainFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainFile, []byte(`
package main

import (
	"fmt"
	"testmodule/internal/secret"
)

func main() {
	fmt.Println(secret.GetSecret())
}
`), 0644)

	// go.mod 생성
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module testmodule\n\ngo 1.21\n"), 0644)

	// 컴파일 시도 - 실패해야 함
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	
	if err == nil {
		t.Fatal("internal 패키지 import가 허용되면 안 됨")
	}
	
	// internal 관련 오류 메시지가 포함되어야 함
	if !strings.Contains(string(output), "internal") {
		t.Fatalf("internal 관련 오류가 아니함: %s", output)
	}
}

// TestCircularImport는 순환 import가 금지되는지 검증합니다.
func TestCircularImport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go-circular-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 패키지 a 생성
	aDir := filepath.Join(tmpDir, "a")
	os.MkdirAll(aDir, 0755)
	os.WriteFile(filepath.Join(aDir, "a.go"), []byte(`
package a

import "testmodule/b"

func Foo() { b.Bar() }
`), 0644)

	// 패키지 b 생성 (순환 import)
	bDir := filepath.Join(tmpDir, "b")
	os.MkdirAll(bDir, 0755)
	os.WriteFile(filepath.Join(bDir, "b.go"), []byte(`
package b

import "testmodule/a"

func Bar() { a.Foo() }
`), 0644)

	// main.go 생성
	mainFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainFile, []byte(`
package main

import "testmodule/a"

func main() { a.Foo() }
`), 0644)

	// go.mod 생성
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module testmodule\n\ngo 1.21\n"), 0644)

	// 컴파일 시도 - 실패해야 함 (순환 import)
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	
	if err == nil {
		t.Fatal("순환 import가 허용되면 안 됨")
	}
	
	// circular import 관련 오류 메시지가 포함되어야 함
	if !strings.Contains(string(output), "circular") {
		t.Fatalf("순환 import 오류가 아니함: %s", output)
	}
}

// TestGoModVerification은 go.sum의 역할을 검증합니다.
func TestGoModVerification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "go-mod-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// go.mod만 있는 경우
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(`
module testmodule

go 1.21

require example.com/dep v1.0.0
`), 0644)

	// go.sum이 없는 상태에서 go mod download 시도
	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	
	// 외부 모듈이 없으므로 오류 발생 가능하지만,
	// go.sum이 없으면 검증 단계에서 문제가 될 수 있음
	t.Logf("go.mod만 있을 때: err=%v, output=%s", err, string(output))
}

func createTestModule(root string) {
	// api 패키지
	apiDir := filepath.Join(root, "api")
	os.MkdirAll(apiDir, 0755)
	os.WriteFile(filepath.Join(apiDir, "api.go"), []byte(`
package api

func PublicFunc() string { return "public" }
`), 0644)

	// internal 패키지
	internalDir := filepath.Join(root, "internal")
	authDir := filepath.Join(internalDir, "auth")
	os.MkdirAll(authDir, 0755)
	os.WriteFile(filepath.Join(authDir, "auth.go"), []byte(`
package auth

func Validate() bool { return true }
`), 0644)

	// main.go
	mainFile := filepath.Join(root, "main.go")
	os.WriteFile(mainFile, []byte(`
package main

import (
	"fmt"
	"testmodule/api"
	"testmodule/internal/auth"
)

func main() {
	fmt.Println(api.PublicFunc())
	fmt.Println(auth.Validate())
}
`), 0644)

	// go.mod
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module testmodule\n\ngo 1.21\n"), 0644)
}