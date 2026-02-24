package internal

import (
	"context"
	"fmt"
	"kenmec/ha/jimmy/api"
	"kenmec/ha/jimmy/config"
	gen "kenmec/ha/jimmy/protoGen"
	"sync"
	"time"
)

type HARole int

const (
	RoleSlave = iota
	RoleCandidate
	RoleMaster
)

func (hr HARole) String() string {
	switch hr {
	case RoleSlave:
		return "SLAVE"
	case RoleCandidate:
		return "CANDIDATE"
	case RoleMaster:
		return "MASTER"
	default:
		return "UNKNOWN"
	}
}

type Connectivity struct {
	ECS   bool
	Fleet bool
	Ha    bool
}

type Arbiter struct {
	mu  sync.RWMutex
	ctx context.Context

	name      string // 發送時來辨識哪個arbiter送的
	role      HARole // SLAVE or MASTER
	priority  int32
	term      int32 // 如果得知斷線之類的 自動加1 大的當master
	timestamp string

	isActiveRedundancy bool
	Self               Connectivity // 自己機器的連線狀態
	Other              Connectivity // 另外一台的連線狀態

	fleetClient   *api.GRPCFleetClient
	otherHaClient *api.GRPCHAClient
	otherHaServer *api.HAToOtherServer
}

type OtherArbiter struct {
	Name     string
	Role     HARole
	Term     int32
	Priority int32

	self  Connectivity
	other Connectivity
}

func NewArbiter(
	fleetClient *api.GRPCFleetClient,
	otherHaClient *api.GRPCHAClient,
	otherHaServer *api.HAToOtherServer,
) *Arbiter {
	return &Arbiter{
		name:      config.Cfg.ARBITOR_NAME,
		role:      HARole(config.Cfg.DEFAULT_ROLE),
		priority:  config.Cfg.PRIORITY,
		term:      0,
		timestamp: time.Now().Format("2006-01-02 15:04:05"),

		isActiveRedundancy: false,
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

func (a *Arbiter) CheckRole(otherArbiter OtherArbiter) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if otherArbiter.Term > a.term {
		a.term = otherArbiter.Term
		a.role = RoleSlave
		return
	}

	if a.role == RoleMaster && otherArbiter.Role == RoleMaster {
		if otherArbiter.Term == a.term {

			if otherArbiter.Priority > a.priority {
				a.role = RoleSlave
				return
			}
		}
	}
}

func (a *Arbiter) MsgHandler() {
	a.otherHaMsgHandler()
	a.fleetMsgHandler()
}

func (a *Arbiter) otherHaMsgHandler() {
	a.otherHaServer.OnReceiveMsg = func(msg *gen.StatusRequest) {

		switch m := msg.Payload.(type) {
		case *gen.StatusRequest_Hb:
			fmt.Printf("got heartbeat for other ha")
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

func (a *Arbiter) fleetMsgHandler() {
	a.fleetClient.OnReceiveMsg = func(msg *gen.ServerMessage) {
		switch m := msg.Payload.(type) {
		case *gen.ServerMessage_Hb:
			fmt.Printf("got heartbeat for FLEET")
		case *gen.ServerMessage_IsEcsConnected:
			a.mu.Lock()
			defer a.mu.Unlock()
			a.Self.ECS = m.IsEcsConnected
		case *gen.ServerMessage_IsFleetConnected:
			a.fleetClient.UpdateConnectStatus(m.IsFleetConnected)
		}
	}

}
