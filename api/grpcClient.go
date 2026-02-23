package api

import (
	"context"
	"io"
	pb "kenmec/ha/jimmy/protoGen"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type GRPCHAClient struct {
	address        string
	conn           *grpc.ClientConn
	client         pb.HASyncServiceClient
	stream         pb.HASyncService_ExchangeStatusClient
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	reconnectDelay time.Duration
	maxRetries     int
	isConnected    bool
}

func NewGRPCClient(address string) *GRPCHAClient {

	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCHAClient{
		address:        address,
		ctx:            ctx,
		cancel:         cancel,
		reconnectDelay: 5 * time.Second,
		maxRetries:     -1,
	}
}

func (g *GRPCHAClient) Conneect() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		g.conn.Close()
	}

	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             3 * time.Second,
		PermitWithoutStream: true,
	}

	conn, err := grpc.NewClient(
		g.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp),
	)

	if err != nil {
		return err
	}

	g.conn = conn
	g.client = pb.NewHASyncServiceClient(conn)
	g.isConnected = true

	stream, err := g.client.ExchangeStatus(g.ctx)
	if err != nil {
		g.conn.Close()
		g.isConnected = false
		return err
	}
	g.stream = stream

	log.Println("✅ gRPC 連線成功")
	return nil
}

func (g *GRPCHAClient) SendMessage(msg *pb.StatusRequest) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.isConnected || g.stream == nil {
		return io.EOF
	}

	return g.stream.Send(msg)
}

func (g *GRPCHAClient) ReceiveMessage() {
	for {
		g.mu.RLock()
		stream := g.stream
		connected := g.isConnected
		g.mu.RUnlock()

		if !connected || stream == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Println("❌ 伺服器關閉連線")
			} else {
				log.Printf("❌ 接收訊息錯誤: %v", err)
			}
			g.mu.Lock()
			g.isConnected = false
			g.mu.Unlock()
			break
		}

		log.Printf("📨 收到訊息: %+v", msg)
	}
}

func (g *GRPCHAClient) StartHeartbeat(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			hbMsg := &pb.StatusRequest{

				Payload: &pb.StatusRequest_Hb{Hb: int32(time.Now().Unix())},
			}

			if err := g.SendMessage(hbMsg); err != nil {
				log.Printf("💓 心跳發送失敗: %v", err)
			}
		}
	}
}

func (g *GRPCHAClient) MaintainConnection() {
	retryCount := 0
	for {
		select {
		case <-g.ctx.Done():
			return
		default:
		}

		if !g.isConnected {
			log.Printf("🔄 嘗試重新連線... (第 %d 次)", retryCount+1)

			if err := g.Conneect(); err != nil {
				log.Printf("❌ 重連失敗: %v，%v 秒後重試...", err, g.reconnectDelay.Seconds())
				time.Sleep(g.reconnectDelay)
				retryCount++
				continue
			}
			retryCount = 0

			go g.ReceiveMessage()
		}

		time.Sleep(1 * time.Second)
	}
}

func (g *GRPCHAClient) IsConnected() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isConnected
}

func (g *GRPCHAClient) Close() {
	g.cancel()
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		g.conn.Close()
	}
	g.isConnected = false
}
