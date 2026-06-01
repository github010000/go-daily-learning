// protoc 명령어 예시 (환경 설정 후 먼저 실행):
// protoc --go_out=. --go-grpc_out=. --proto_path=. helloworld.proto
// 이 코드는 생성된 pb 패키지를 가정하여 작성된 완전한 실행 예제입니다.

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	pb "example.com/helloworld"
	"google.golang.org/grpc/credentials/insecure"
)

// GreeterServer는 proto 파일에서 정의한 Greeter 서비스 인터페이스를 구현합니다.
type GreeterServer struct {
	pb.UnimplementedGreeterServer
}

// SayHello는 proto에서 정의된 rpc 메서드의 실제 로직입니다.
func (s *GreeterServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("서버: '%s' 요청 수신 완료", in.GetName())
	return &pb.HelloReply{Message: "안녕하세요! gRPC 서버에서 응답합니다."}, nil
}

func main() {
	var wg sync.WaitGroup
	wg.Add(1)

	// 1단계: gRPC 서버 리스닝 및 서비스 등록
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("포트 리스닝 실패: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &GreeterServer{})

	// 2단계: 서버 비동기 실행
	go func() {
		defer wg.Done()
		fmt.Println(">>> gRPC 서버가 포트 50051에서 대기 중입니다.")
		if err := s.Serve(lis); err != nil {
			log.Printf("서버 실행 중 오류: %v", err)
		}
	}()

	// 서버 기동 대기
	time.Sleep(500 * time.Millisecond)

	// 3단계: gRPC 클라이언트 연결 및 호출
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("클라이언트 연결 실패: %v", err)
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Protobuf 직렬화 데이터를 사용하여 원격 호출
	resp, err := client.SayHello(ctx, &pb.HelloRequest{Name: "Go 학습자"})
	if err != nil {
		log.Fatalf("클라이언트 호출 실패: %v", err)
	}
	fmt.Printf(">>> 클라이언트 수신 응답: %s\n", resp.GetMessage())

	// 4단계: 서버 정중 종료
	s.Stop()
	fmt.Println(">>> gRPC 서버가 정상적으로 종료되었습니다.")

	wg.Wait()
	fmt.Println(">>> 모든 작업 완료.")
}