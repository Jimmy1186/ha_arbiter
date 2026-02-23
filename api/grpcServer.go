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

	// Start heartbeat sender
	go s.sendHeartbeat(client)

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

		if err := s.handleClientMessage(clientID, client, msg); err != nil {
			log.Printf("❌ 處理訊息錯誤: %v", err)
			return err
		}
	}
}

// handleClientMessage processes messages from client
func (s *HAToOtherServer) handleClientMessage(clientID string, client *ClientConnection, msg *pb.StatusRequest) error {
	switch payload := msg.Payload.(type) {
	case *pb.StatusRequest_Hb:
		client.mu.Lock()
		client.lastHB = time.Now()
		client.mu.Unlock()
		log.Printf("💓 收到客戶端 %s 心跳: %d", clientID, payload.Hb)

	default:
		log.Printf("📨 收到客戶端 %s 訊息: %+v", clientID, msg)
	}

	return nil
}

// sendHeartbeat sends periodic heartbeats to client
func (s *HAToOtherServer) sendHeartbeat(client *ClientConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-client.ctx.Done():
			return
		case <-ticker.C:
			msg := &pb.StatusResponse{
				Payload: &pb.StatusResponse_Hb{
					Hb: int32(time.Now().Unix()),
				},
			}

			if err := client.stream.Send(msg); err != nil {
				log.Printf("❌ 發送心跳失敗: %v", err)
				client.cancelFunc()
				return
			}
		}
	}
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
