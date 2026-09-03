package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	cgroupV2CPUmaxPath = "/sys/fs/cgroup/cpu.max"
	cgroupV1QuotaPath  = "/sys/fs/cgroup/cpu/cfs_quota_us"
	cgroupV1PeriodPath = "/sys/fs/cgroup/cpu/cfs_period_us"
)

// parseCPUmaxV2는 cgroup v2의 cpu.max 파일 내용을 파싱한다.
// 형식은 "$MAX $PERIOD"이며 MAX가 "max"이면 무제한을 의미한다.
// 반환값: limit(CPU 개수), unlimited 여부, error
func parseCPUmaxV2(content string) (float64, bool, error) {
	fields := strings.Fields(content)
	if len(fields) != 2 {
		return 0, false, fmt.Errorf("unexpected cpu.max format: %q", content)
	}
	if fields[0] == "max" {
		return 0, true, nil
	}
	max, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse cpu.max quota: %w", err)
	}
	period, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse cpu.max period: %w", err)
	}
	if period <= 0 {
		return 0, false, fmt.Errorf("cpu.max period must be positive, got %v", period)
	}
	return max / period, false, nil
}

// parseCPUquotaV1는 cgroup v1의 quota/period 값을 파싱한다.
// quota가 -1이면 무제한으로 처리하고 0을 반환한다.
func parseCPUquotaV1(quotaStr, periodStr string) (float64, error) {
	if quotaStr == "-1" {
		return 0, nil
	}
	quota, err := strconv.ParseFloat(quotaStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse quota: %w", err)
	}
	period, err := strconv.ParseFloat(periodStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse period: %w", err)
	}
	if period <= 0 {
		return 0, fmt.Errorf("period must be positive, got %v", period)
	}
	return quota / period, nil
}

// detectContainerCPULimit는 cgroup v2/v1에서 CPU 제한 값을 읽어 CPU 개수로 반환한다.
// 제한이 없으면 0을 반환한다.
func detectContainerCPULimit() (float64, error) {
	if data, err := os.ReadFile(cgroupV2CPUmaxPath); err == nil {
		limit, _, err := parseCPUmaxV2(strings.TrimSpace(string(data)))
		return limit, err
	}
	// v2가 없으면 v1을 시도한다.
	quotaBytes, err1 := os.ReadFile(cgroupV1QuotaPath)
	periodBytes, err2 := os.ReadFile(cgroupV1PeriodPath)
	if err1 == nil && err2 == nil {
		return parseCPUquotaV1(strings.TrimSpace(string(quotaBytes)), strings.TrimSpace(string(periodBytes)))
	}
	return 0, fmt.Errorf("no cgroup limit found: %v, %v", err1, err2)
}

// recommendedGOMAXPROCS는 컨테이너 CPU limit에 맞는 GOMAXPROCS 값을 반환한다.
// limit이 0 이하(무제한)이면 호스트 CPU 수를 그대로 사용한다.
// automaxprocs와 동일하게 소수점 이하는 올림 처리한다.
func recommendedGOMAXPROCS(limit float64) int {
	if limit <= 0 {
		return runtime.NumCPU()
	}
	return int(math.Ceil(limit))
}

// countThreads는 /proc/self/status에서 현재 프로세스의 스레드 수를 읽는다.
func countThreads() (int, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Threads:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.Atoi(fields[1])
			}
		}
	}
	return 0, fmt.Errorf("Threads not found in /proc/self/status")
}

// busyLoop는 stop이 true가 될 때까지 CPU를 소모하는 간단한 루프다.
// 컨테이너에서 GOMAXPROCS가 과다하면 이런 goroutine들이 OS 스레드를 가득 채운다.
func busyLoop(stop *atomic.Bool) {
	x := 0
	for !stop.Load() {
		x = (x + 1) & 0x7fffffff
	}
	_ = x
}

// measureMaxThreads는 GOMAXPROCS를 maxProcs로 설정하고 workers개의 busy-loop goroutine을
// duration 동안 실행하면서 관찰된 최대 스레드 수를 반환한다.
// 기본 GOMAXPROCS(호스트 CPU 수)와 권장 값의 차이를 보여주기 위한 함수다.
func measureMaxThreads(maxProcs int, workers int, duration time.Duration) int {
	old := runtime.GOMAXPROCS(maxProcs)
	defer runtime.GOMAXPROCS(old)

	var stop atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			busyLoop(&stop)
		}()
	}

	maxThreads := 0
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if n, err := countThreads(); err == nil && n > maxThreads {
			maxThreads = n
		}
		time.Sleep(50 * time.Millisecond)
	}

	stop.Store(true)
	wg.Wait()
	return maxThreads
}

// showEnvironment는 현재 런타임이 보는 환경과 권장 값을 출력한다.
func showEnvironment(limit float64, recommended int) {
	fmt.Println("=== 환경 ===")
	fmt.Printf("호스트 CPU 수: %d\n", runtime.NumCPU())
	fmt.Printf("현재 GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	if limit > 0 {
		fmt.Printf("cgroup CPU limit: %.2f\n", limit)
	} else {
		fmt.Println("cgroup CPU limit: 감지되지 않음 (시연용 limit 사용)")
	}
	fmt.Printf("권장 GOMAXPROCS: %d\n", recommended)
	fmt.Println()
}

func main() {
	limit, err := detectContainerCPULimit()
	if err != nil || limit <= 0 {
		// 실제 컨테이너가 아닌 환경에서도 GOMAXPROCS 과다 설정 문제를 시연하기 위해
		// 인위적으로 낮은 limit을 설정한다. 호스트 CPU 수보다 작게 잡아 차이를 보여준다.
		demoTarget := math.Max(1, math.Min(2, float64(runtime.NumCPU()-1)))
		limit = demoTarget
		fmt.Printf("cgroup limit이 감지되지 않아 시연용으로 %.0f CPU limit을 사용합니다.\n", limit)
	} else {
		fmt.Printf("cgroup limit 감지: %.2f CPU\n", limit)
	}
	rec := recommendedGOMAXPROCS(limit)
	showEnvironment(limit, rec)

	fmt.Println("=== 스레드 수 비교 (busy-loop goroutines 100개, 1초) ===")
	defaultThreads := measureMaxThreads(runtime.GOMAXPROCS(0), 100, 1*time.Second)
	fmt.Printf("기본 GOMAXPROCS(%d)에서 최대 스레드 수: %d\n", runtime.GOMAXPROCS(0), defaultThreads)

	recommendedThreads := measureMaxThreads(rec, 100, 1*time.Second)
	fmt.Printf("권장 GOMAXPROCS(%d)에서 최대 스레드 수: %d\n", rec, recommendedThreads)

	fmt.Println("\n해석: 기본 설정은 호스트 CPU 수를 그대로 사용하므로 컨테이너 limit보다 많은 스레드를 생성합니다.")
	fmt.Println("이는 cgroup 스로틀링과 지연 급증의 원인이 됩니다. 권장 값으로 설정하면 스레드 수가 줄어듭니다.")
}