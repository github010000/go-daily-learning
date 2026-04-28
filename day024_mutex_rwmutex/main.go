package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeCounter는 Mutex를 사용하여 안전하게 숫자를 증가시키는 구조체입니다.
type SafeCounter struct {
	mu    sync.Mutex
	value int
}

// Inc는 Mutex를 사용하여 값을 1 증가시킵니다.
func (c *SafeCounter) Inc() {
	// Lock을 호출하여 다른 고루틴이 접근하지 못하도록 차단합니다.
	c.mu.Lock()
	// 함수가 종료될 때 반드시 Unlock을 호출하도록 defer를 사용합니다.
	defer c.mu.Unlock()
	
	c.value++
}

// Value는 현재 값을 반환합니다.
func (c *SafeCounter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// ConfigMap은 RWMutex를 사용하여 읽기/쓰기를 분리 관리하는 구조체입니다.
type ConfigMap struct {
	mu   sync.RWMutex
	data map[string]string
}

// Set은 쓰기 잠금(Lock)을 사용하여 값을 수정합니다.
func (cm *ConfigMap) Set(key, value string) {
	cm.mu.Lock() // 쓰기 작업 시에는 다른 모든 읽기/쓰기 작업이 차단됩니다.
	defer cm.mu.Unlock()
	cm.data[key] = value
}

// Get은 읽기 잠금(RLock)을 사용하여 값을 읽습니다.
func (cm *ConfigMap) Get(key string) (string, bool) {
	cm.mu.RLock()         // 읽기 잠금: 여러 고루틴이 동시에 RLock을 가질 수 있습니다.
	defer cm.mu.RUnlock() // 읽기 작업이 끝나면 RUnlock을 호출합니다.
	
	val, ok := cm.data[key]
	return val, ok
}

func main() {
	// 1. sync.Mutex 예제 실행
	counter := &SafeCounter{}
	var wg sync.WaitGroup

	fmt.Println("--- Mutex 예제 시작 ---")
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Inc()
		}()
	}
	wg.Wait()
	fmt.Printf("최종 카운트 값: %d (예상 값: 1000)\n", counter.Value())

	// 2. sync.RWMutex 예제 실행
	config := &ConfigMap{data: make(map[string]string)}
	fmt.Println("\n--- RWMutex 예제 시작 ---")

	// 쓰기 작업 고루틴
	wg.Add(1)
	go func() {
		defer wg.Done()
		config.Set("version", "1.0.0")
		config.Set("env", "production")
		fmt.Println("[Writer] 설정값 업데이트 완료")
	}()

	// 읽기 작업 고루틴들
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// 읽기 작업이 수행될 때까지 잠시 대기 (쓰기가 먼저 실행되도록 유도)
			time.Sleep(10 * time.Millisecond)
			
			val, ok := config.Get("version")
			if ok {
				fmt.Printf("[Reader %d] version: %s\n", id, val)
			} else {
				fmt.Printf("[Reader %d] 데이터를 찾을 수 없습니다.\n", id)
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("--- 모든 작업 종료 ---")
}