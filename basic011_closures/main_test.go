package main

import "testing"

// TestLoopVariableScopeNewStyle는 Go 1.22 이후 루프 변수 스코프 규칙을 검증합니다.
// 각 클로저가 0부터 9까지 고유한 값을 반환해야 합니다.
func TestLoopVariableScopeNewStyle(t *testing.T) {
	increments := makeIncrementersNewStyle()

	tests := []struct {
		index    int
		expected int
	}{
		{0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4},
		{5, 5}, {6, 6}, {7, 7}, {8, 8}, {9, 9},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("index_%d", tt.index)
		t.Run(name, func(t *testing.T) {
			result := increments[tt.index]()
			if result != tt.expected {
				t.Errorf("인덱스 %d: 기대값 %d, 실제 %d", tt.index, tt.expected, result)
			}
		})
	}
}

// TestFixDataRace는 mutex로 보호된 클로저가 올바르게 동작하는지 검증합니다.
// 5개의 클로저가 모두 실행되면 counter는 정확히 5가 되어야 합니다.
func TestFixDataRace(t *testing.T) {
	var mu sync.Mutex
	counter := 0
	var closures []func()
	for i := 0; i < 5; i++ {
		closures = append(closures, func() {
			mu.Lock()
			counter++
			mu.Unlock()
		})
	}

	done := make(chan bool, len(closures))
	for _, fn := range closures {
		go func(f func()) {
			f()
			done <- true
		}(fn)
	}

	for i := 0; i < len(closures); i++ {
		<-done
	}

	if counter != 5 {
		t.Errorf("counter 기대값 5, 실제 %d", counter)
	}
}

// TestClosureReference는 클로저가 변수를 참조하여 수정할 수 있음을 검증합니다.
func TestClosureReference(t *testing.T) {
	values := []int{1, 2, 3}
	// 클로저가 루프 변수 i와 values 슬라이스를 함께 참조합니다.
	var closures []func()
	for i := range values {
		closures = append(closures, func() {
			values[i] = values[i] * 10
		})
	}
	
	for _, fn := range closures {
		fn()
	}
	
	expected := []int{10, 20, 30}
	for i := range values {
		if values[i] != expected[i] {
			t.Errorf("values[%d]: 기대값 %d, 실제 %d", i, expected[i], values[i])
		}
	}
}