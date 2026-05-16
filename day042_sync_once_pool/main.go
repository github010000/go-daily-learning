package main

import (
	"fmt"
	"sync"
)

// Global variable to demonstrate sync.Once
var instance *Singleton
var once sync.Once

// Singleton: 단순한 싱글턴 구조체
type Singleton struct {
	Value string
}

// NewInstance: 생성자 역할. sync.Once를 사용하여 한 번만 실행됨을 보장
func NewInstance() *Singleton {
	once.Do(func() {
		fmt.Println("[Sync.Once] 초기화 로직 수행 (싱글턴 객체 생성)")
		instance = &Singleton{Value: "Initialized"}
	})
	return instance
}

// main: 프로그램의 진입점
func main() {
	// --- 1. sync.Once 시연 ---
	fmt.Println("=== 1. sync.Once (싱글턴 초기화) ===")

	// 여러 번 호출해도 초기화는 딱 한 번만 실행됨
	NewInstance()
	NewInstance()
	fmt.Printf("최종 결과: instance.Value = %s\n\n", instance.Value)

	// --- 2. sync.Pool 시연 ---
	fmt.Println("=== 2. sync.Pool (객체 재사용) ===")

	// Pool 정의: PoolType 구조체의 인스턴스를 관리
	pool := &sync.Pool{
		New: func() interface{} {
			fmt.Println("[Pool.New] 새 객체가 필요하여 생성함")
			return &PoolType{Buffer: ""}
		},
	}

	// 객체 가져오기 (Get)
	fmt.Println(">>> Object A Get")
	objA := pool.Get().(*PoolType)
	objA.Buffer = "Data A"

	fmt.Println(">>> Object B Get")
	objB := pool.Get().(*PoolType)
	objB.Buffer = "Data B"

	// 사용 완료 후 반납 (Put)
	fmt.Println(">>> Object A Put (반납)")
	pool.Put(objA)

	// 반납된 객체는 재사용될 수 있음
	fmt.Println(">>> Object A Get (재사용)")
	objA_reuse := pool.Get().(*PoolType)
	objA_reuse.Buffer = "Data A (Reuse)"

	fmt.Printf(">>> Object B Put\n>>> Object B Get\n\n", objA_reuse.Buffer)
	pool.Put(objB)

	// --- 3. 정리 및 출력 ---
	fmt.Println("=== 3. 정리 ===")
	fmt.Printf("객체 재사용 확인 (Object A Reuse): %s\n", objA_reuse.Buffer)
}

// PoolType: Pool에서 관리할 타입
type PoolType struct {
	Buffer string
}