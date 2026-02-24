package api

import (
	"context"
	"io"
	pb "kenmec/ha/jimmy/protoGen"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type HAToOtherServer struct {
	pb.UnimplementedHASyncServiceServer

	clients     map[string]*ClientConnection
	clientsLock sync.RWMutex

	//用類似callback的方式 可以在其他地方呼叫用
	OnReceiveMsg func(msg *pb.StatusRequest)
}

// ClientConnection represents a connected client
type ClientConnection struct {
	stream     pb.HASyncService_ExchangeStatusServer
	ctx        context.Context
	cancelFunc context.CancelFunc
	lastHB     time.Time
	mu         sync.RWMutex
}

func NewHAToOtherServer() *HAToOtherServer {
	return &HAToOtherServer{
		clients: make(map[string]*ClientConnection),
	}
}

func (s *HAToOtherServer) ListenServer(port string) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("❌ 監聽失敗: %v", err)
	}

	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}

	kasp := keepalive.ServerParameters{
		Time:    10 * time.Second,
		Timeout: 3 * time.Second,
	}

	grpcServer := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	)

	pb.RegisterHASyncServiceServer(grpcServer, s)

	log.Printf("🚀 gRPC 伺服器啟動於 :%s", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ 服務啟動失敗: %v", err)
	}
}

// ✅ Implement the correct method name from proto
func (s *HAToOtherServer) ExchangeStatus(stream pb.HASyncService_ExchangeStatusServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	clientID := generateClientID()

	client := &ClientConnection{
		stream:     stream,
		ctx:        ctx,
		cancelFunc: cancel,
		lastHB:     time.Now(),
	}

	s.clientsLock.Lock()
	s.clients[clientID] = client
	s.clientsLock.Unlock()

	log.Printf("✅ 新客戶端連線: %s (總數: %d)", clientID, len(s.clients))

	defer func() {
		s.clientsLock.Lock()
		delete(s.clients, clientID)
		clientCount := len(s.clients)
		s.clientsLock.Unlock()
		log.Printf("❌ 客戶端斷線: %s (剩餘: %d)", clientID, clientCount)
	}()

	// Receive messages loop
	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("📭 客戶端 %s 正常關閉連線", clientID)
				return nil
			}
			log.Printf("❌ 客戶端 %s 接收錯誤: %v", clientID, err)
			return err
		}
		log.Printf("📨 收到訊息來自另一台 HA: %+v", msg)
		s.handleClientMessage(msg)
	}
}

// handleClientMessage processes messages from client
func (s *HAToOtherServer) handleClientMessage(msg *pb.StatusRequest) {
	s.OnReceiveMsg(msg)
}

func (s *HAToOtherServer) BroadcastMessage(msg *pb.StatusResponse) {
	s.clientsLock.RLock()
	defer s.clientsLock.RUnlock()

	for clientID, client := range s.clients {
		if err := client.stream.Send(msg); err != nil {
			log.Printf("❌ 廣播至客戶端 %s 失敗: %v", clientID, err)
		}
	}
}

func (s *HAToOtherServer) SendToClient(clientID string, msg *pb.StatusResponse) error {
	s.clientsLock.RLock()
	client, exists := s.clients[clientID]
	s.clientsLock.RUnlock()

	if !exists {
		return io.EOF
	}

	return client.stream.Send(msg)
}

func generateClientID() string {
	return time.Now().Format("20060102150405.000000")
}
