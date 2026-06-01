# Day 58: gRPC 기초

## 개념 설명 섹션
gRPC는 HTTP/2 프로토콜을 기반으로 하는 고성능 RPC(Remote Procedure Call) 프레임워크입니다. 기존 REST API가 텍스트 형식인 JSON을 사용하는 반면, gRPC는 이진 직렬화 형식인 Protocol Buffers(Protobuf)를 사용합니다. 이는 네트워크 전송 오버헤드를 획기적으로 줄이고, 파싱 속도를 높여 대규모 마이크로서비스 아키텍처에서 선호되는 표준이 되었습니다.

Protobuf는 `syntax = "proto3";` 문법을 사용하여 서비스와 데이터 구조를 정의하는 IDL(Interface Definition Language)입니다. `.proto` 파일에 메시지 필드의 타입과 고유 번호를 선언하면, `protoc` 컴파일러가 Go, Java, Python 등 다양한 언어의 소스 코드로 자동 생성합니다. 이를 통해 서버와 클라이언트 간 데이터契约을 명확히 하고, 컴파일 타임 타입 검증을 가능하게 합니다.

단일 `.proto` 파일에서는 `message`로 데이터 구조를, `service`로 원격 호출할 수 있는 메서드를 정의합니다. `rpc MethodName(Request) returns (Response);` 구문으로 호출 패턴을 선언하며, 반환 타입을 `stream`으로 지정하면 서버 또는 클라이언트 스트리밍, 양방향 스트리밍 등 고급 통신 패턴도 쉽게 구현할 수 있습니다.

## 코드 설명 섹션
예제 코드는 생성된 pb 패키지를 사용하여 gRPC 서버와 클라이언트의 전체 라이프사이클을 시뮬레이션합니다. 먼저 `net.Listen`으로 TCP 포트를 바인드하고, `grpc.NewServer()` 인스턴스를 초기화합니다. `pb.RegisterGreeterServer`를 통해 `.proto`에서 정의한 인터페이스를 Go 구조체(`GreeterServer`)로 구현하여 서버에 등록합니다.

서버는 고루틴(`go func()`)으로 실행되어 `s.Serve(lis)`에서 요청 대기 상태가 됩니다. 메인 고루틴은 500ms 대기 후 `grpc.Dial`로 연결을 수립하고, `pb.NewGreeterClient`를 통해 클라이언트 객체를 생성합니다. `client.SayHello` 호출 시 내부적으로 HTTP/2 멀티플렉싱을 통해 Protobuf 직렬화 데이터를 전송하며, 서버는 이를 역직렬화하여 응답을 반환합니다.

마지막으로 `s.Stop()`을 호출하여 처리 중인 연결이 완료될 때까지 대기한 후 서버를 정중하게 종료합니다. `sync.WaitGroup`을 활용하여 서버 고루틴이 완전히 종료될 때까지 메인 스레드가 종료되지 않도록 동기화를 처리했습니다.

## 핵심 포인트 섹션
- **Protobuf 직렬화**: JSON 대비 10배 이상 작은 직렬화 크기와 빠른 파싱 속도를 제공하며, 필드 번호 기반의 안정적인 버전 관리가 가능합니다.
- **생성 기반 개발**: `protoc`와 언어별 플러그인을 실행해 `.proto` 파일을 컴파일하면 클라이언트/서버 스터브가 자동 생성되어 개발 효율이 높아집니다.
- **HTTP/2 기반 통신**: 헤더 압축, 멀티플렉싱, 바이너리 프레임을 사용하여 레이턴시를 줄이고 대규모 서비스 간 통신에 최적화되어 있습니다.
- **컨텍스트와 타임아웃**: 모든 gRPC 호출은 `context.Context`를 필수 인자로 받으며, 지연 방지와 리소스 누수 방지를 위해 반드시 타임아웃을 설정해야 합니다.

## 참고 링크 섹션
- [Go gRPC Quickstart](https://grpc.io/docs/languages/go/quickstart/)
- [Protocol Buffers Language Guide](https://protobuf.dev/getting-started/gotutorial/)
- [gRPC Go Repository](https://github.com/grpc/grpc-go)