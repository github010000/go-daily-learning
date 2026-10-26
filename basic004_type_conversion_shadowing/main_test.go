package main

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExplicitConversion 은 float64 타입 변환과 strconv.Atoi 가 올바르게 동작하는지 검증합니다.
// Go 는 int 와 float64 를 자동으로 더하지 않으므로 명시적 변환이 필요합니다.
func TestExplicitConversion(t *testing.T) {
	t.Run("int-to-float64 변환", func(t *testing.T) {
		var age int = 30
		var price float64 = 19.9
		sum := price + float64(age)
		assert.Equal(t, 49.9, sum)
	})

	t.Run("문자열 to int 변환", func(t *testing.T) {
		num, err := strconv.Atoi("42")
		assert.NoError(t, err)
		assert.Equal(t, 42, num)
	})
}

// TestShadowingPattern 은 doWork() 호출 후 바깥 err 가 올바르게 갱신되는지 확인합니다.
// 고친 버전(= 사용)은 err 를 바깥으로 전파하지만, 실수 버전(:= 사용)은 그렇지 않습니다.
func TestShadowingPattern(t *testing.T) {
	t.Run("고친 버전: err 전파", func(t *testing.T) {
		var err error
		if err = doWork(); err != nil {
			t.Fatal("doWork 는 nil 을 반환해야 합니다")
		}
		assert.Nil(t, err)
	})

	t.Run("실수 버전: 섀도잉으로 err 미전파", func(t *testing.T) {
		err := fmt.Errorf("외부 에러")
		if err := doWork(); err != nil {
			t.Fatal("doWork 는 nil 을 반환해야 합니다")
		}
		// 섀도잉으로 바깥 err 는 여전히 "외부 에러"입니다.
		assert.Equal(t, "외부 에러", err.Error())
	})
}