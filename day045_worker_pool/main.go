package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Job: 워커가 처리할 작업 단위
type Job struct {
	ID int
}

// Result: 워커가 처리한 작업의 결과
type Result struct {
	JobID int
	Msg   string
}

func main() {
	const (
		numJobs    = 10 // 처리할 총 작업 개수
		numWorkers = 3 // 워커 풀 크기 (동시성 제한)
	)

	// 버퍼 채널을 생성하여 일시적인 병목 현상을 방지합니다.
	jobs := make(chan Job, numJobs)
	results := make(chan Result, numJobs)

	// errgroup: 고루틴 그룹화 및 에러 통합 처리용
	var eg errgroup.Group
	// WaitGroup: 워커들이 모든 작업을 처리할 때까지 대기용
	var wg sync.WaitGroup

	// 1. 워커 풀 생성 (지정된 수만큼 고루틴 실행)
	for w := 1; w <= numWorkers; w++ {
		w := w // 고루틴 내부에서 변수 캡처 문제 방지를 위한 복사
		wg.Add(1)

		// errgroup.Go에 워커 로직 등록
		eg.Go(func() error {
			defer wg.Done() // 작업 완료 시 WaitGroup 카운트 감소

			// jobs 채널이 닫힐 때까지 반복하며 작업 처리
			for job := range jobs {
				// 가상의 작업 처리 시간 (0~100ms)
				time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

				// 결과 채널로 데이터 전송
				results <- Result{
					JobID: job.ID,
					Msg:   fmt.Sprintf("워커-%d | %d번 작업 완료", w, job.ID),
				}
			}
			return nil
		})
	}

	// 2. 메인 고루틴에서 작업 채널에 데이터 투입
	go func() {
		for i := 1; i <= numJobs; i++ {
			jobs <- Job{ID: i}
		}
		close(jobs) // 모든 작업 투입 후 채널 닫기
	}()

	// 3. 워커들이 jobs 채널을 모두 소비하고 종료되면 결과 채널 닫기
	go func() {
		wg.Wait()
		close(results)
	}()

	// 4. 결과 채널 읽기 (모든 결과가 들어오면 루프 종료)
	for res := range results {
		fmt.Println(res.Msg)
	}

	// 5. errgroup.Wait()로 고루틴 종료 대기 및 에러 검증
	if err := eg.Wait(); err != nil {
		fmt.Printf("에러 발생: %v\n", err)
	}
}