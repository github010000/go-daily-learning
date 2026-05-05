package main

import (
	"fmt"
	"time"
)

func main() {
	// 1. 현재 시간 조회 및 포맷팅
	now := time.Now()
	fmt.Println("=== 1. 현재 시간 및 포맷팅 ===")
	fmt.Printf("현재 시간: %s\n", now)
	fmt.Printf("RFC3339 포맷: %s\n", now.Format(time.RFC3339))
	fmt.Printf("사용자 정의 포맷: %s\n", now.Format("2006년 01월 02일 15시 04분 05초"))
	fmt.Println()

	// 2. 시간 간격(Duration)과 지연(Sleep)
	fmt.Println("=== 2. Duration와 Sleep ===")
	delayDuration := 500 * time.Millisecond
	fmt.Printf("지정된 지연 시간: %v\n", delayDuration)

	startTime := time.Now()
	time.Sleep(delayDuration)
	elapsedTime := time.Since(startTime)

	fmt.Printf("실제 대기 시간: %v\n", elapsedTime)
	fmt.Println()

	// 3. 시간 문자열 파싱 (Parse)
	fmt.Println("=== 3. 시간 파싱 (Parse) ===")
	timeString := "2024-10-25T14:30:00+09:00"
	parsedTime, parseErr := time.Parse(time.RFC3339, timeString)
	if parseErr != nil {
		fmt.Printf("파싱 오류 발생: %v\n", parseErr)
		return
	}

	fmt.Printf("원본 문자열: %s\n", timeString)
	fmt.Printf("파싱된 시간: %s\n", parsedTime)
	fmt.Printf("년, 월, 일, 시, 분, 초: %d-%02d-%02d %02d:%02d:%02d\n",
		parsedTime.Year(), parsedTime.Month(), parsedTime.Day(),
		parsedTime.Hour(), parsedTime.Minute(), parsedTime.Second())
	fmt.Println()

	// 4. Timer (단발성 타이머)
	fmt.Println("=== 4. Timer ===")
	myTimer := time.NewTimer(2 * time.Second)

	select {
	case <-myTimer.C:
		fmt.Println("Timer: 2초가 경과하여 신호가 도착했습니다.")
	}

	// 타이머가 이미 종료되었더라도 Stop()을 호출하여 채널 리소스를 정리해야 함
	if !myTimer.Stop() {
		<-myTimer.C
	}
	fmt.Println()

	// 5. Ticker (주기적 타이머)와 time.After (지연 채널)
	fmt.Println("=== 5. Ticker와 After ===")
	myTicker := time.NewTicker(1 * time.Second)
	defer myTicker.Stop() // 함수 종료 시 자동으로 타이커 정지

	afterChannel := time.After(3 * time.Second)
	tickCount := 0

	for {
		select {
		case <-afterChannel:
			fmt.Printf("After: 3초 지연 경과. Ticker를 총 %d번 실행했습니다.\n", tickCount)
			return // 메인 함수 종료
		case <-myTicker.C:
			tickCount++
			fmt.Printf("Ticker: %d초 경과\n", tickCount)
		}
	}
}