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

type GRPCFleetClient struct {
	address        string
	conn           *grpc.ClientConn
	client         pb.HAServiceClient
	stream         pb.HAService_HAStreamingClient
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	reconnectDelay time.Duration
	maxRetries     int
	isConnected    bool

	//用類似callback的方式 可以在其他地方呼叫用
	OnReceiveMsg func(msg *pb.ServerMessage)
}

func NewGRPCFleetClient(address string) *GRPCFleetClient {

	ctx, cancel := context.WithCancel(context.Background())

	return &GRPCFleetClient{
		address:        address,
		ctx:            ctx,
		cancel:         cancel,
		reconnectDelay: 5 * time.Second,
		maxRetries:     -1,
	}
}

func (g *GRPCFleetClient) ConneectToFleet() error {
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
	g.client = pb.NewHAServiceClient(conn)
	g.isConnected = true

	stream, err := g.client.HAStreaming(g.ctx)
	if err != nil {
		g.conn.Close()
		g.isConnected = false
		return err
	}
	g.stream = stream

	log.Println("✅ gRPC 連線成功")
	return nil
}

func (g *GRPCFleetClient) SendMessageToFleet(msg *pb.ClientMessage) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.isConnected || g.stream == nil {
		return io.EOF
	}

	return g.stream.Send(msg)
}

func (g *GRPCFleetClient) ReceiveMessageFromFleet() {
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

		g.OnReceiveMsg(msg)
		log.Printf("📨 收到訊息來自交管: %+v", msg)
	}
}

func (g *GRPCFleetClient) StartHeartbeatToFleet(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			hbMsg := &pb.ClientMessage{

				Payload: &pb.ClientMessage_Hb{Hb: int32(time.Now().Unix())},
			}

			if err := g.SendMessageToFleet(hbMsg); err != nil {
				log.Printf("💓 心跳發送失敗: %v", err)
			}
		}
	}
}

func (g *GRPCFleetClient) MaintainConnectionWithFleet() {
	retryCount := 0
	for {
		select {
		case <-g.ctx.Done():
			return
		default:
		}

		if !g.isConnected {
			log.Printf("🔄 FLEET 嘗試重新連線... (第 %d 次)", retryCount+1)

			if err := g.ConneectToFleet(); err != nil {
				log.Printf("❌ FLEET 重連失敗: %v，%v 秒後重試...", err, g.reconnectDelay.Seconds())
				time.Sleep(g.reconnectDelay)
				retryCount++
				continue
			}
			retryCount = 0

			go g.ReceiveMessageFromFleet()
		}

		time.Sleep(1 * time.Second)
	}
}

func (g *GRPCFleetClient) LoggingConnectionStatus() {
	for {
		if !g.IsConnectedToFleet() {
			log.Println("⏳ 等待 gRPC 連線到交管...")
			time.Sleep(1 * time.Second)
		}
	}
}

func (g *GRPCFleetClient) IsConnectedToFleet() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.isConnected
}

func (g *GRPCFleetClient) UpdateConnectStatus(isConnect bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.isConnected = isConnect
}

func (g *GRPCFleetClient) CloseWithFleet() {
	g.cancel()
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.conn != nil {
		g.conn.Close()
	}
	g.isConnected = false
}
