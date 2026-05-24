package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // pprof 엔드포인트 자동 등록을 위한 사이드 이펙트 임포트
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// cpuHeavyWork: CPU 연산 부담을 시뮬레이션하는 함수
func cpuHeavyWork() {
	sum := 0
	for i := 0; i < 10000000; i++ {
		sum += i * i
	}
	fmt.Printf("🖥️ CPU 작업 완료 (결과: %d)\n", sum)
}

// memoryHeavyWork: 메모리 할당 부담을 시뮬레이션하는 함수
func memoryHeavyWork() {
	// 대용량 슬라이스 할당 및 초기화
	data := make([]byte, 10000000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	fmt.Println("💾 메모리 작업 완료 (할당 크기: 10MB)")
}

func main() {
	// 1. pprof 서버 설정 안내
	// 실제 서비스에서는 net/http.Server에 pprof 라우터 추가
	fmt.Println("🔧 net/http/pprof 서버 설정 방법:")
	fmt.Println(`   import _ "net/http/pprof"
   // 서버 시작 시 자동으로 /debug/pprof/* 엔드포인트 활성화`)

	// 2. CPU 프로파일 수집
	fmt.Println("\n📊 CPU 프로파일 생성 중...")
	cpuFile, err := os.Create("cpu.prof")
	if err != nil {
		fmt.Printf("❌ CPU 프로파일 생성 실패: %v\n", err)
		return
	}

	// CPU 프로파일링 시작
	pprof.StartCPUProfile(cpuFile)

	// CPU 집약적 코드 실행
	cpuHeavyWork()

	// 프로파일링 종료 및 파일 닫기
	pprof.StopCPUProfile()
	cpuFile.Close()
	fmt.Println("✅ cpu.prof 파일 생성 완료")

	// 3. 메모리 프로파일 수집
	fmt.Println("\n📊 메모리 프로파일 생성 중...")
	memFile, err := os.Create("mem.prof")
	if err != nil {
		fmt.Printf("❌ 메모리 프로파일 생성 실패: %v\n", err)
		return
	}

	// 정확한 메모리 분석을 위해 GC를 강제 실행하여 이전 할당 데이터 정리
	runtime.GC()
	runtime.GC()

	// 메모리 집약적 코드 실행
	memoryHeavyWork()

	// 힙 메모리 할당 상태 기록
	pprof.WriteHeapProfile(memFile)
	memFile.Close()
	fmt.Println("✅ mem.prof 파일 생성 완료")

	// 4. go tool pprof 활용 가이드 출력
	fmt.Println("\n--- 🔍 분석 명령어 (터미널에서 실행) ---")
	fmt.Println("1. CPU 프로파일 텍스트 분석:")
	fmt.Println("   $ go tool pprof -text cpu.prof")
	fmt.Println("2. 메모리 프로파일 웹 분석 (브라우저 자동 실행):")
	fmt.Println("   $ go tool pprof -web mem.prof")
	fmt.Println("3. HTTP 엔드포인트 실시간 프로파일링:")
	fmt.Println("   $ go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile")
	fmt.Println("4. 벤치마크와 연동하여 최적화 검증:")
	fmt.Println("   $ go test -bench=. -cpuprofile=bench_cpu.prof -memprofile=bench_mem.prof")
	fmt.Println("   $ go tool pprof -text bench_cpu.prof")

	// 예제 실행 대기 (서버 연동 개념 확인용)
	time.Sleep(2 * time.Second)
	fmt.Println("\n💡 프로파일링은 개발/운영 주기에 지속적으로 통합하여 리소스 누수 및 병목을 관리하세요.")
}