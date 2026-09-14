package main

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type snapshot struct {
	goroutines int
	threads    int
}

type connStart struct {
	c   net.Conn
	err error
}

func main() {
	// GOMAXPROCS=1 로 고정해야 차이가 극명하게 보인다.
	// P 가 하나뿐이면 netpoller 는 M 을 해방시키고,
	// raw blocking syscall 은 M 을 소모해 새 M 을 만들어야 하기 때문이다.
	old := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(old)

	const n = 100

	fmt.Println("netpoller 시연: GOMAXPROCS=1, blocking I/O 100개")
	fmt.Println("두 시나리오를 순서대로 측정합니다...")

	netBefore, netAfter := blockOnNetpoll(n, 50*time.Millisecond)
	fmt.Printf("[net.Conn.Read] %d개 블로킹\n", n)
	fmt.Printf("  goroutine: %d -> %d\n", netBefore.goroutines, netAfter.goroutines)
	fmt.Printf("  OS 스레드: %d -> %d\n", netBefore.threads, netAfter.threads)
	fmt.Println("  -> goroutine은 늘어도 M은 거의 늘지 않는다. (netpoller가 fd를 대기)")

	rawBefore, rawAfter := blockOnRawSyscallPipe(n, 50*time.Millisecond)
	fmt.Printf("[raw syscall.Read] %d개 블로킹 (일반 파일 I/O와 같은 blocking 경로)\n", n)
	fmt.Printf("  goroutine: %d -> %d\n", rawBefore.goroutines, rawAfter.goroutines)
	fmt.Printf("  OS 스레드: %d -> %d\n", rawBefore.threads, rawAfter.threads)
	fmt.Println("  -> M까지 커널에서 잠들어 런타임이 새 M을 계속 만든다.")

	fmt.Println("결론: net.Conn 계열은 그냥 blocking read를 써도 된다.")
	fmt.Println("      raw syscall이나 일반 파일 Read는 같은 느낌이지만 M을 소모할 수 있다.")
}

func blockOnNetpoll(n int, settle time.Duration) (snapshot, snapshot) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		return snapshot{runtime.NumGoroutine(), currentThreads()}, snapshot{runtime.NumGoroutine(), currentThreads()}
	}

	accepted := make(chan net.Conn, n)
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()

	started := make(chan connStart, n)
	var mu sync.Mutex
	clients := make([]net.Conn, 0, n)
	var wg sync.WaitGroup

	before := snapshot{runtime.NumGoroutine(), currentThreads()}

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := net.Dial("tcp", ln.Addr().String())
			if err != nil {
				started <- connStart{err: err}
				return
			}
			mu.Lock()
			clients = append(clients, c)
			mu.Unlock()
			started <- connStart{c: c}

			var one [1]byte
			// 여기가 핵심이다. netpoller 는 goroutine 만 park시키고
			// M 은 런타임으로 돌아가 다른 일을 할 수 있게 만든다.
			_, _ = c.Read(one[:])
		}()
	}

	// 모든 클라이언트 goroutine 이 blocking Read 에 도달할 때까지 기다린다.
	for i := 0; i < n; i++ {
		<-started
	}

	if settle > 0 {
		time.Sleep(settle)
	}
	after := snapshot{runtime.NumGoroutine(), currentThreads()}

	// 클라이언트 쪽 연결을 닫아 블로킹된 Read 를 모두 깨운다.
	mu.Lock()
	for _, c := range clients {
		_ = c.Close()
	}
	mu.Unlock()

	// accept goroutine 이 Accept 에서 빠져나오도록 listener 를 닫는다.
	_ = ln.Close()
	wg.Wait()
	<-acceptDone

	// 서버 쪽 accepted 연결도 닫아 fd 를 반환한다.
	close(accepted)
	for c := range accepted {
		_ = c.Close()
	}

	return before, after
}

func blockOnRawSyscallPipe(n int, settle time.Duration) (snapshot, snapshot) {
	pipes := make([][2]int, n)
	for i := 0; i < n; i++ {
		if err := syscall.Pipe(pipes[i][:]); err != nil {
			for j := 0; j < i; j++ {
				_ = syscall.Close(pipes[j][0])
				_ = syscall.Close(pipes[j][1])
			}
			fmt.Fprintln(os.Stderr, "pipe:", err)
			return snapshot{runtime.NumGoroutine(), currentThreads()}, snapshot{runtime.NumGoroutine(), currentThreads()}
		}
	}

	ready := make(chan struct{}, n)
	var wg sync.WaitGroup

	before := snapshot{runtime.NumGoroutine(), currentThreads()}

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(rfd int) {
			defer wg.Done()
			ready <- struct{}{}
			var one [1]byte
			// 이 raw syscall 은 M 을 커널에 묶어둔다.
			// netpoller 를 타지 않기 때문에 thread 수가 늘어난다.
			_, _ = syscall.Read(rfd, one[:])
			_ = syscall.Close(rfd)
		}(pipes[i][0])
	}

	// 모든 goroutine 이 ready 신호를 보낸 뒤 실제 syscall.Read 로
	// 들어갈 시간을 준다.
	for i := 0; i < n; i++ {
		<-ready
	}

	if settle > 0 {
		time.Sleep(settle)
	}
	after := snapshot{runtime.NumGoroutine(), currentThreads()}

	// 쓰기 쪽 fd 에 바이트를 써서 read 를 깨운다.
	for i := 0; i < n; i++ {
		_, _ = syscall.Write(pipes[i][1], []byte{0})
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		_ = syscall.Close(pipes[i][1])
	}

	return before, after
}

func currentThreads() int {
	// /proc/self/status 에서 현재 OS 스레드 수를 읽는다.
	// macOS 같은 비 리눅스 환경에서는 -1 을 반환한다.
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "Threads:") {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Threads:")))
		if err != nil {
			return -1
		}
		return v
	}
	return -1
}