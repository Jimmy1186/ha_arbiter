package internal

import (
	"context"
	"fmt"
	"kenmec/ha/jimmy/api"
	"kenmec/ha/jimmy/config"
	gen "kenmec/ha/jimmy/protoGen"
	"log"
	"net"
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

	IsMaster    bool
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

		IsMaster:    false,
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

func (a *Arbiter) UpdateMaster(master bool) {
	a.mu.Lock()
	a.IsMaster = master
	a.mu.Unlock()
	a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
		Payload: &gen.ClientMessage_IsMaster{
			IsMaster: master,
		},
	})
}

func (a *Arbiter) MsgHandler() {
	a.otherHaMsgHandler()
	a.fleetMsgHandler()
	a.whenFleetConnect()
	a.whenBackupConnect()
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

		case *gen.StatusRequest_SyncMission:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_SyncMission{
					SyncMission: m.SyncMission,
				},
			})

		case *gen.StatusRequest_AgvWorkStatus:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_AgvWorkStatus{
					AgvWorkStatus: m.AgvWorkStatus,
				},
			})

		case *gen.StatusRequest_MissionReport:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_MissionReport{
					MissionReport: m.MissionReport,
				},
			})

		case *gen.StatusRequest_UpdateCargoInfo:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_UpdateCargoInfo{
					UpdateCargoInfo: m.UpdateCargoInfo,
				},
			})

		case *gen.StatusRequest_SaveCargoInfo:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_SaveCargoInfo{
					SaveCargoInfo: m.SaveCargoInfo,
				},
			})

		case *gen.StatusRequest_UpdateAmrCargoInfo:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_UpdateAmrCargoInfo{
					UpdateAmrCargoInfo: m.UpdateAmrCargoInfo,
				},
			})

		case *gen.StatusRequest_MissionAssign:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_MissionAssign{
					MissionAssign: m.MissionAssign,
				},
			})

		case *gen.StatusRequest_BookBlock:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_BookBlock{
					BookBlock: m.BookBlock,
				},
			})

		case *gen.StatusRequest_SyncAllMission:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_SyncAllMission{
					SyncAllMission: m.SyncAllMission,
				},
			})

		case *gen.StatusRequest_SyncAllDbCargo:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_SyncAllDbCargo{
					SyncAllDbCargo: m.SyncAllDbCargo,
				},
			})

		case *gen.StatusRequest_SyncAllMemoryCargo:
			a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
				Payload: &gen.ClientMessage_SyncAllMemoryCargo{
					SyncAllMemoryCargo: m.SyncAllMemoryCargo,
				},
			})

		default:
			fmt.Printf("❓ 收到未定義的訊息類型: %T", m)
		}

	}
}

func (a *Arbiter) whenFleetConnect() {
	a.fleetClient.OnFleetConnected = func() {
		log.Println("連線到本機交管")
		a.UpdateMaster(a.IsMaster)
	}
}

func (a *Arbiter) whenBackupConnect() {
	if a.IsMaster == false {
		return
	}

	a.otherHaServer.OnClientConnected = func() {
		a.fleetClient.SendMessageToFleet(&gen.ClientMessage{
			Payload: &gen.ClientMessage_BackupConnected{
				BackupConnected: time.Now().GoString(),
			},
		})
	}
}

// 接收來自交管資料
func (a *Arbiter) fleetMsgHandler() {
	a.fleetClient.OnReceiveMsg = func(msg *gen.ServerMessage) {
		// 先檢查 payload 是否為空
		if msg.Payload == nil {
			log.Printf("⚠️ 收到空的 ServerMessage")
			return
		}

		switch m := msg.Payload.(type) {
		case *gen.ServerMessage_Hb:
			a.mu.Lock()
			a.lastFleetHb = time.Now()
			a.mu.Unlock()

		case *gen.ServerMessage_IsEcsConnected:
			a.mu.Lock()
			a.Self.ECS = m.IsEcsConnected
			a.mu.Unlock()
			log.Printf("🌐 [網路狀態] ECS 連線變更: %v", m.IsEcsConnected)

		case *gen.ServerMessage_IsFleetConnected:
			a.fleetClient.UpdateConnectStatus(m.IsFleetConnected)
			a.mu.Lock()
			a.Self.Fleet = m.IsFleetConnected
			a.mu.Unlock()
			log.Printf("🚚 [網路狀態] Fleet 連線變更: %v", m.IsFleetConnected)

		case *gen.ServerMessage_SyncMission:
			info := m.SyncMission
			log.Printf("📋 [任務同步] 收到任務")

			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_SyncMission{
					SyncMission: info,
				},
			})

		case *gen.ServerMessage_AgvWorkStatus:
			status := m.AgvWorkStatus
			log.Printf("🤖 [車輛狀態] AMR: %s (正在指派: %v, 指派中任務: %s)",
				status.AmrId, status.IsAssigning, status.CurrentMissionId)

			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_AgvWorkStatus{
					AgvWorkStatus: status,
				},
			})

		case *gen.ServerMessage_MissionReport:
			report := m.MissionReport
			log.Printf("📊 [任務報表] 類型: %s, 任務ID: %s, AMR: %s, 步驟: %d",
				report.ReportType, report.MissionId, report.AmrId, report.GetStep())

			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_MissionReport{
					MissionReport: report,
				},
			})

		case *gen.ServerMessage_UpdateCargoInfo:
			cuInfo := m.UpdateCargoInfo
			log.Printf("📦 [貨物] 編輯於地點: %s", cuInfo.LocationId)

			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_UpdateCargoInfo{
					UpdateCargoInfo: cuInfo,
				},
			})

		case *gen.ServerMessage_SaveCargoInfo:
			saveCargo := m.SaveCargoInfo
			log.Printf("📦 [貨物] 搬運: %s, 地點: %s", saveCargo.AmrId, saveCargo.LocationId)

			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_SaveCargoInfo{
					SaveCargoInfo: saveCargo,
				},
			})

		case *gen.ServerMessage_UpdateAmrCargoInfo:
			amrCargo := m.UpdateAmrCargoInfo
			log.Printf("📦 [貨物] 更新車輛貨物:%s ", amrCargo.AmrId)

			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_UpdateAmrCargoInfo{
					UpdateAmrCargoInfo: amrCargo,
				},
			})

		case *gen.ServerMessage_MissionAssign:
			log.Printf("📋 [任務指派] mission id: %s, amrId: %s", m.MissionAssign.MissionId, m.MissionAssign.AmrId)
			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_MissionAssign{
					MissionAssign: m.MissionAssign,
				},
			})

		case *gen.ServerMessage_BookBlock:
			log.Printf("📋 [儲位預定]")
			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_BookBlock{
					BookBlock: m.BookBlock,
				},
			})

		case *gen.ServerMessage_SyncAllMission:
			log.Printf("📋 [同步所有任務]")
			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_SyncAllMission{
					SyncAllMission: m.SyncAllMission,
				},
			})

		case *gen.ServerMessage_SyncAllDbCargo:
			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_SyncAllDbCargo{
					SyncAllDbCargo: m.SyncAllDbCargo,
				},
			})

		case *gen.ServerMessage_SyncAllMemoryCargo:
			a.otherHaClient.SendMessage(&gen.StatusRequest{
				Payload: &gen.StatusRequest_SyncAllMemoryCargo{
					SyncAllMemoryCargo: m.SyncAllMemoryCargo,
				},
			})

		default:
			log.Printf("❓ [未知訊息] 收到未定義的 Payload 類型: %T", m)
			//肏擬媽康寧
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

func (a *Arbiter) CheckInitRole() {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("❌ 無法取得網卡資訊: %v", err)
		return
	}

	for _, addr := range addrs {

		if ipnet, ok := addr.(*net.IPNet); ok {

			if ipnet.IP.IsLoopback() {
				continue
			}

			ip := ipnet.IP.To4()
			if ip == nil {
				continue
			}

			fmt.Printf("🔍 偵測到本機 IP: %s\n", ip.String())

			if ip.String() == config.Cfg.VIP {
				log.Printf("👑 [啟動檢查] 發現 VIP (%s)，身分確認為: MASTER", config.Cfg.VIP)
				a.UpdateMaster(true)
				return
			}
		}
	}

	log.Printf("🥈 [啟動檢查] 未發現 VIP (%s)，身分確認為: BACKUP", config.Cfg.VIP)
	a.UpdateMaster(false)
}
