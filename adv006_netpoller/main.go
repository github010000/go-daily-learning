package main

import (
    "fmt"
    "net"
    "runtime"
    "strings"
    "sync/atomic"
)

// networkParkDemo는 parkNetworkReads가 만든 TCP 연결 묶음이다.
type networkParkDemo struct {
    listener net.Listener
    clients  []net.Conn
    servers  []net.Conn
}

// close는 지금까지 연 모든 연결을 닫아서 Read에 park되어 있던 goroutine이
// 정상적으로 언블록되고 종료되게 한다.
func (d *networkParkDemo) close() {
    for _, c := range d.clients {
        if c != nil {
            _ = c.Close()
        }
    }
    for _, c := range d.servers {
        if c != nil {
            _ = c.Close()
        }
    }
    if d.listener != nil {
        _ = d.listener.Close()
    }
}

// countStackNeedle은 모든 goroutine의 스택 트레이스에서 needle이 몇 번 나오는지 센다.
// netpoller에 park된 goroutine은 스택에 internal/poll.(*FD).Read가 남는다.
func countStackNeedle(needle string) int {
    buf := make([]byte, 4<<20)
    n := runtime.Stack(buf, true)
    return strings.Count(string(buf[:n]), needle)
}

func waitUntil(cond func() bool, maxIter int) bool {
    for i := 0; i < maxIter; i++ {
        if cond() {
            return true
        }
        runtime.Gosched()
    }
    return false
}

func waitForStackNeedle(needle string, minCount, maxIter int) bool {
    return waitUntil(func() bool {
        return countStackNeedle(needle) >= minCount
    }, maxIter)
}

// parkNetworkReads는 n개의 TCP 연결을 만들고 각 서버 goroutine이
// net.Conn.Read에서 park될 때까지 기다린다. 네트워크 fd는 항상 논블로킹이므로
// Read는 EAGAIN을 만나면 netpoller에게 goroutine을 맡기고 M을 반환한다.
func parkNetworkReads(n int) (*networkParkDemo, error) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return nil, err
    }
    d := &networkParkDemo{listener: ln}

    for i := 0; i < n; i++ {
        client, err := net.Dial("tcp", ln.Addr().String())
        if err != nil {
            d.close()
            return nil, err
        }
        server, err := ln.Accept()
        if err != nil {
            d.close()
            return nil, err
        }
        d.clients = append(d.clients, client)
        d.servers = append(d.servers, server)

        go func(c net.Conn) {
            buf := make([]byte, 1)
            _, _ = c.Read(buf)
        }(server)
    }

    // 스택을 직접 검사해서 실제로 park됐는지 확인한다. sleep 같은 시간 기반
    // 대기를 쓰지 않아 테스트와 시연이 결정적이다.
    if !waitForStackNeedle("internal/poll.(*FD).Read", n, 100000) {
        d.close()
        return nil, fmt.Errorf("%d goroutine이 network read에 park되지 않았음", n)
    }
    return d, nil
}

// runCPUWorkUntilTarget은 CPU만 쓰는 goroutine을 하나 띄우고 target까지 도달하기를
// 기다린다. netpoller가 G만 재우고 M을 반환했다면 GOMAXPROCS=1에서도 이 함수가
// target을 채울 수 있어야 한다.
func runCPUWorkUntilTarget(target int64) int64 {
    var counter int64
    go func() {
        for atomic.LoadInt64(&counter) < target {
            atomic.AddInt64(&counter, 1)
        }
    }()

    waitUntil(func() bool {
        return atomic.LoadInt64(&counter) >= target
    }, 10000000)

    return atomic.LoadInt64(&counter)
}

func main() {
    // 의도적으로 P를 하나만 둔다. 그래야 network read가 M을 막는다면
    // CPU worker가 전혀 진행되지 못하므로 netpoller 효과가 눈에 보인다.
    runtime.GOMAXPROCS(1)
    fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))

    demo, err := parkNetworkReads(5)
    if err != nil {
        panic(err)
    }
    defer demo.close()

    fmt.Println("parked network reads: 5")
    fmt.Println("runtime.NumGoroutine:", runtime.NumGoroutine())
    fmt.Println("runtime.NumThread before CPU work:", runtime.NumThread())

    const target = 1_000_000
    fmt.Println("CPU work target:", target)
    got := runCPUWorkUntilTarget(target)
    fmt.Println("CPU work reached:", got)
    fmt.Println("runtime.NumThread after CPU work:", runtime.NumThread())

    // 출력에서 핵심: GOMAXPROCS=1인데도 5개 Read가 park된 상태에서
    // CPU work target이 채워진다. 만약 Read가 M을 블로킹했다면 target은 0에 머문다.
}