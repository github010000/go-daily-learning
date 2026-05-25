# Day 51: 구조화 로깅 — slog

## 개념 설명
Go 1.21에서 도입된 `log/slog` 패키지는 구조화된 로깅(Structured Logging)을 위한 공식 표준 라이브러리입니다. 기존 `fmt.Printf`나 `log` 패키지는 로그를 단순 텍스트 문자열로 처리하여 검색 엔진과 분석 툴이 패턴을 추출하기 어려웠습니다. slog는 로그 메시지와 메타데이터를 키-값(Key-Value) 쌍으로 구조화하여 저장하므로, ELK 스택이나 Grafana와 같은 로그 분석 시스템에서 필터링과 집계 작업을 훨씬 효율적으로 수행할 수 있습니다.

slog의 핵심 추상화는 핸들러(Handler)와 속성(Attribute)입니다. 로그 함수(`Info`, `Warn`, `Error` 등) 호출 시 뒤에 추가하는 인수는 자동으로 `slog.Attr` 타입으로 변환되어 로그에 붙습니다. 출력 형식은 `TextHandler`(기본값, 가독성 중시)와 `JSONHandler`(기계 파싱 중시) 중 선택할 수 있습니다. 운영 환경에서는 수집 시스템의 파싱 규칙에 맞춰 `JSONHandler`를 주로 사용하며, 개발 환경이나 콘솔 출력용으로는 `TextHandler`가 적합합니다.

로그 출력량은 `slog.HandlerOptions`를 통해 세밀하게 제어할 수 있습니다. `Level` 필드를 `slog.LevelInfo`로 설정하면 `Debug` 레벨의 로그는 전처리 단계에서 차단되어 I/O 오버헤드를 줄이고 디스크 저장 공간을 절약합니다. 이렇게 레벨을 분리하면 개발 단계에서는 디버깅을 용이하게 하되, 프로덕션 환경에서는 불필요한 로그를 자동으로 필터링하는 모범 사례를 따를 수 있습니다.

## 코드 설명
- **기본 TextHandler**: `slog.Info`, `slog.Warn`, `slog.Error` 함수에 `key, value` 쌍을 전달하면 자동으로 구조화된 텍스트 로그가 생성됩니다.
- **JSONHandler 설정**: `slog.NewJSONHandler(os.Stdout, nil)`을 통해 표준 출력을 JSON 형식으로 포맷합니다. 두 번째 인자는 핸들러 옵션이며 `nil`은 기본값을 의미합니다.
- **HandlerOptions로 레벨 제한**: `&slog.HandlerOptions{Level: slog.LevelInfo}` 객체를 생성하여 텍스트 핸들러에 전달합니다. 이로 인해 `levelLogger.Debug()` 호출이 코드에는 남아있지만 실제 출력에서는 무시됩니다.

## 핵심 포인트
- slog는 Go 1.21 이상 표준 패키지로, 구조화 로깅의 공식 권장 도구입니다.
- 함수형 인자 패턴을 활용하여 키-값 메타데이터를 간결하게 로깅할 수 있습니다.
- TextHandler는 인간 독자에게, JSONHandler는 로그 수집 시스템 파싱에 최적화되어 있습니다.
- HandlerOptions.Level을 설정하여 출력할 최소 로그 레벨을 엄격히 통제해야 합니다.
- slog.Logger는 스레드 안전(Thread-safe)하므로 전역 변수로 공유해도竞态 조건이 발생하지 않습니다.

## 참고 링크
- https://pkg.go.dev/log/slog