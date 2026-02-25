package internal

import (
	"context"
	"fmt"
	"kenmec/ha/jimmy/api"
	"kenmec/ha/jimmy/config"
	gen "kenmec/ha/jimmy/protoGen"
	"log"
	"sync"
	"time"
)

type Connectivity struct {
	ECS   bool
	Fleet bool
	Ha    bool
}

type Arbiter struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	Maintenance bool

	lastFleetHb    time.Time
	hbFleetTimeout time.Duration

	lastOtherHaHb  time.Time
	hbOtherTimeout time.Duration

	Self  Connectivity // 自己機器的連線狀態
	Other Connectivity // 另外一台的連線狀態

	fleetClient   *api.GRPCFleetClient
	otherHaClient *api.GRPCHAClient
	otherHaServer *api.HAToOtherServer
}

func NewArbiter(
	fleetClient *api.GRPCFleetClient,
	otherHaClient *api.GRPCHAClient,
	otherHaServer *api.HAToOtherServer,
) *Arbiter {
	ctx, cancel := context.WithCancel(context.Background())
	return &Arbiter{
		ctx:    ctx,
		cancel: cancel,

		Maintenance: false,

		lastFleetHb:    time.Now(),
		hbFleetTimeout: time.Duration(config.Cfg.FLEET_HB_TIMEOUT) * time.Second,

		lastOtherHaHb:  time.Now(),
		hbOtherTimeout: time.Duration(config.Cfg.OTHER_HA_HB_TIMEOUT) * time.Second,

		Self: Connectivity{
			ECS:   false,
			Fleet: false,
			Ha:    true,
		},
		Other: Connectivity{
			ECS:   false,
			Fleet: false,
			Ha:    false,
		},

		fleetClient:   fleetClient,
		otherHaClient: otherHaClient,
		otherHaServer: otherHaServer,
	}
}

// 每秒傳送本機的連線資訊到另外一台HA
func (a *Arbiter) StartSyncArbiter(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_PeerArbiter{
					PeerArbiter: &gen.PeerArbiter{
						Ecs:   a.Self.ECS,
						Fleet: a.Self.Fleet,
					},
				},
			})
		}
	}

}

func (a *Arbiter) MsgHandler() {
	a.otherHaMsgHandler()
	a.fleetMsgHandler()
}

// 接收來自其他的HA的資料
func (a *Arbiter) otherHaMsgHandler() {
	a.otherHaServer.OnReceiveMsg = func(msg *gen.StatusRequest) {

		switch m := msg.Payload.(type) {
		case *gen.StatusRequest_Hb:
			a.mu.Lock()
			a.lastOtherHaHb = time.Now()
			a.mu.Unlock()
		case *gen.StatusRequest_IsHaConnected:
			a.Other.Ha = m.IsHaConnected
		case *gen.StatusRequest_IsEcsConnected:
			a.Other.Fleet = m.IsEcsConnected
		case *gen.StatusRequest_IsFleetConnected:
			a.Other.Fleet = m.IsFleetConnected

		default:
			fmt.Printf("❓ 收到未定義的訊息類型: %T", m)
		}

	}
}

// 接收來自交管資料
func (a *Arbiter) fleetMsgHandler() {
	a.fleetClient.OnReceiveMsg = func(msg *gen.ServerMessage) {
		switch m := msg.Payload.(type) {
		case *gen.ServerMessage_Hb:
			a.mu.Lock()
			a.lastFleetHb = time.Now()
			a.mu.Unlock()
		case *gen.ServerMessage_IsEcsConnected:
			a.mu.Lock()
			a.Self.ECS = m.IsEcsConnected
			a.mu.Unlock()
			log.Printf("🛐 ecs status being update %v", a.Self.ECS)
		case *gen.ServerMessage_IsFleetConnected:
			a.fleetClient.UpdateConnectStatus(m.IsFleetConnected)
			a.mu.Lock()
			defer a.mu.Unlock()
			a.Self.Fleet = m.IsFleetConnected
			log.Printf("🇦🇨 fleet status being update %v", a.Self.Fleet)
		}
	}

}

// 監測與交管心跳是否有延遲
func (a *Arbiter) StartFleetHbMonitor() {
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			last := a.lastFleetHb
			timeout := a.hbFleetTimeout
			a.mu.RUnlock()

			if time.Since(last) > timeout || !a.fleetClient.IsConnectedToFleet() {
				log.Printf("⚠️  WARN: Fleet heartbeat timeout! 超過 %v 秒未收到", timeout.Seconds())
				a.mu.Lock()
				a.Self.Fleet = false
				a.mu.Unlock()
			} else {
				a.mu.Lock()
				a.Self.Fleet = true
				a.mu.Unlock()
			}
		}
	}
}

// 跟另外一台HA心跳用
func (a *Arbiter) StartHeartbeatToOtherHA() {
	ticker := time.NewTicker(time.Duration(config.Cfg.OTHER_HA_HB_INTERVAL) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			err := a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_Hb{
					Hb: int32(time.Now().Unix()),
				},
			})

			if err != nil {
				log.Printf("💓 心跳到其他HA發送失敗: %v", err)
			}
		}
	}
}

// 監測與另外一台HA是否有延遲
func (a *Arbiter) StartOtherHaHbMonitor() {
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			last := a.lastOtherHaHb
			timeout := a.hbOtherTimeout
			a.mu.RUnlock()

			if time.Since(last) > timeout {
				log.Printf("⚠️  WARN: other ha heartbeat timeout! 超過 %v 秒未收到", timeout.Seconds())
				a.mu.Lock()
				a.Other.Ha = false
				a.mu.Unlock()
			} else {
				a.mu.Lock()
				a.Other.Ha = true
				a.mu.Unlock()
			}
		}
	}
}
