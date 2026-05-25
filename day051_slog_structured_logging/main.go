package main

import (
	"fmt"
	"log/slog"
	"os"
)

func main() {
	// 1. 기본 TextHandler: 키-값 속성을 담은 구조화 로그 출력
	fmt.Println("=== 1. 기본 slog 출력 (TextHandler) ===")
	slog.Info("서버가 시작되었습니다", "port", 8080, "env", "production")
	slog.Warn("메모리 사용량 증가", "used_mb", 1024)
	slog.Error("DB 연결 실패", "error", "connection timeout")
	// 기본 로깅 레벨은 Info이므로 Debug 함수는 표시되지 않음

	// 2. JSON 핸들러: 기계가 파싱하기 쉬운 JSON 구조로 출력
	fmt.Println("\n=== 2. JSON 핸들러 사용 ===")
	jsonLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	jsonLogger.Info("API 요청 처리 완료", "method", "POST", "path", "/api/data", "latency_ms", 150)

	// 3. 핸들러 옵션: 최소 로그 레벨을 Info로 제한
	fmt.Println("\n=== 3. 로그 레벨 제한 (Info 이상만 출력) ===")
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	levelLogger := slog.New(slog.NewTextHandler(os.Stdout, opts))

	levelLogger.Debug("이 로그는 레벨이 낮아 표시되지 않습니다")
	levelLogger.Info("Info 레벨 로그는 정상 출력됩니다")
	levelLogger.Warn("경고 메시지도 함께 출력됩니다")
}