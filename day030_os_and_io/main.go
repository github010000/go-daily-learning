package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func main() {
	// 1. os.Args: 명령줄 인자 확인
	fmt.Println("=== 1. 명령줄 인자 (os.Args) ===")
	fmt.Printf("실행 파일명: %s\n", os.Args[0])
	if len(os.Args) > 1 {
		fmt.Printf("전달된 인자: %v\n", os.Args[1:])
	}

	fileName := "day30_demo.txt"

	// 2. os.Create: 파일 생성 및 io.Writer 활용
	fmt.Println("\n=== 2. os.Create로 파일 생성 ===")
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("os.Create 실패: %v\n", err)
		return
	}
	defer file.Close()

	// os.File은 io.Writer 인터페이스를 구현하므로 인터페이스로 변환 가능
	var w io.Writer = file
	_, err = w.Write([]byte("안녕하세요 Go 파일 입출력!\n"))
	if err != nil {
		fmt.Printf("io.Writer 작성 실패: %v\n", err)
		return
	}
	fmt.Println("os.Create와 io.Writer로 파일에 데이터를 썼습니다.")

	// 3. os.WriteFile: 짧은 파일 쓰기 (간편 함수)
	fmt.Println("\n=== 3. os.WriteFile로 파일 덮어쓰기 ===")
	err = os.WriteFile(fileName, []byte("첫 번째 라인\n두 번째 라인\n세 번째 라인\n"), 0644)
	if err != nil {
		fmt.Printf("os.WriteFile 실패: %v\n", err)
		return
	}
	fmt.Println("os.WriteFile로 파일 내용을 새 데이터로 변경했습니다.")

	// 4. os.Open: 파일 열기 및 io.Reader 활용
	fmt.Println("\n=== 4. os.Open과 io.Reader 활용 ===")
	file2, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("os.Open 실패: %v\n", err)
		return
	}
	defer file2.Close()

	// os.File은 io.Reader 인터페이스를 구현함
	var r io.Reader = file2
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Printf("io.Reader 읽기 실패: %v\n", err)
	} else {
		fmt.Printf("io.Reader로 읽은 데이터: %s\n", string(buf[:n]))
	}

	// 5. bufio.Scanner: 라인 단위 읽기
	fmt.Println("\n=== 5. bufio.Scanner로 라인 읽기 ===")
	scanner := bufio.NewScanner(file2)
	lineNum := 1
	for scanner.Scan() {
		fmt.Printf("라인 %d: %s\n", lineNum, scanner.Text())
		lineNum++
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("스캔 중 오류 발생: %v\n", err)
	}

	// 6. os.ReadFile: 전체 파일 읽기 (간편 함수)
	fmt.Println("\n=== 6. os.ReadFile로 전체 읽기 ===")
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Printf("os.ReadFile 실패: %v\n", err)
		return
	}
	fmt.Printf("os.ReadFile 결과:\n%s", string(content))

	// 정리: 테스트 파일 삭제
	os.Remove(fileName)
	fmt.Println("\n=== Day 30 학습 완료 ===")
}