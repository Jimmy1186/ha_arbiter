package internal

import (
	"context"
	"kenmec/ha/jimmy/config"
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

type Arbiter struct {
	mu  sync.RWMutex
	ctx context.Context

	name      string // 發送時來辨識哪個arbiter送的
	role      HARole // SLAVE or MASTER
	priority  int32
	term      int32 // 如果得知斷線之類的 自動加1 大的當master
	timestamp string
}

type OtherArbiter struct {
	Name     string
	Role     HARole
	Term     int32
	Priority int32
}

func NewArbiter() *Arbiter {
	return &Arbiter{
		name:      config.Cfg.ARBITOR_NAME,
		role:      HARole(config.Cfg.DEFAULT_ROLE),
		priority:  config.Cfg.PRIORITY,
		term:      0,
		timestamp: time.Now().Format("2006-01-02 15:04:05"),
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
