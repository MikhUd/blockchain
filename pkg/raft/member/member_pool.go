package member

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/raft"
	"github.com/MikhUd/blockchain/pkg/serializer"
	"github.com/MikhUd/blockchain/pkg/status"
	"github.com/MikhUd/blockchain/pkg/stream"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/MikhUd/blockchain/pkg/waitgroup"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	activeMembersCount int32
	ser                serializer.ProtoSerializer
)

const (
	Candidate = iota
	Follower
	Leader
)

type Pool struct {
	owner         Owner
	currTerm      uint32
	mainCfg       config.Config
	tlsCfg        *tls.Config
	engine        *stream.Engine
	mu            sync.RWMutex
	wg            *waitgroup.WaitGroup
	members       map[string]*Member
	connPool      map[string]net.Conn
	votes         map[string]struct{}
	deadLeaderCh  chan struct{}
	aliveLeaderCh chan struct{}
	electionTime  time.Duration
}

type Member struct {
	addr                string
	heartbeatMisses     uint8
	maxHeartbeatMisses  uint8
	recoverMisses       uint8
	maxRecoverMisses    uint8
	heartbeatIntervalMs time.Duration
	recoverIntervalMs   time.Duration
	recoverTimeoutMs    time.Duration
	state               atomic.Uint32
	status              atomic.Uint32
	stopCh              chan struct{}
	joinAckCh           chan struct{}
	ctx                 context.Context
	cancel              context.CancelFunc
}

type Owner struct {
	id,
	addr,
	leaderAddr string
	stopCh <-chan struct{}
}

func NewMemberPool(owner Owner, engine *stream.Engine, electionTime time.Duration) *Pool {
	var op = "member_pool.NewMemberPool"
	mp := &Pool{
		owner:         owner,
		currTerm:      0,
		members:       make(map[string]*Member),
		connPool:      make(map[string]net.Conn),
		votes:         make(map[string]struct{}),
		deadLeaderCh:  make(chan struct{}, 1),
		aliveLeaderCh: make(chan struct{}, 1),
		engine:        engine,
		electionTime:  electionTime,
	}
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("member pool successfully created, election time: %s ", electionTime.String()))
	return mp
}

func (mp *Pool) newMember(addr string) *Member {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(mp.mainCfg.MemberTimeoutSec))
	m := &Member{
		addr:                addr,
		heartbeatMisses:     0,
		maxHeartbeatMisses:  mp.mainCfg.MaxMemberHeartbeatMisses,
		recoverMisses:       0,
		maxRecoverMisses:    mp.mainCfg.MaxMemberRecoverMisses,
		heartbeatIntervalMs: time.Millisecond * time.Duration(mp.mainCfg.MemberHeartbeatIntervalMs),
		recoverIntervalMs:   time.Millisecond * time.Duration(mp.mainCfg.MemberRecoverIntervalMs),
		stopCh:              make(chan struct{}, 1),
		joinAckCh:           make(chan struct{}, 1),
		ctx:                 ctx,
		cancel:              cancel,
	}
	m.state.Store(Follower)
	m.status.Store(status.Initialized)
	return m
}

func (mp *Pool) startHeartbeatWithMember(member *Member) {
	var op = "member_pool.startHeartbeat"
	ticker := time.NewTicker(time.Millisecond * member.heartbeatIntervalMs)
	defer ticker.Stop()
	errorCh := make(chan error, 1)
	go func() {
		for {
			select {
			case <-mp.owner.stopCh:
				slog.With(slog.String("op", op)).Info(fmt.Sprintf("stop heartbeat, owner:%s stopped", mp.owner))
				return
			case <-ticker.C:
				go func() {
					if err := mp.sendHeartbeat(member); err != nil {
						slog.With(slog.String("op", op)).Warn(fmt.Sprintf("Error heartbeat with member: %s", member.addr))
						errorCh <- err
					}
				}()
			case err := <-errorCh:
				go mp.handleErrorHeartbeat(member, err)
				return
			}
		}
	}()
}

func (mp *Pool) sendHeartbeat(member *Member) error {
	defer func() {
		member.heartbeatMisses++
	}()
	if member.status.Load() != status.Running {
		return fmt.Errorf(fmt.Sprintf("Can not send heartbeat to inactive member: %s", member.addr))
	}
	msg := &message.HeartbeatMessage{Remote: &message.PID{Id: mp.owner.id, Addr: mp.engine.Sender()}, Acknowledged: false}
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: member.addr})
	return mp.engine.Send(ctx)
}

func (mp *Pool) handleErrorHeartbeat(member *Member, err error) {
	mp.setMemberStatus(member, status.Pending)
	if err := mp.tryToRecoverWithMember(member); err != nil {
		member.state.Store(status.Stopped)
		if member.state.Load() == Leader {
			mp.deadLeaderCh <- struct{}{}
		}
		return
	}
	// start new heartbeat
	go mp.startHeartbeatWithMember(member)
}

func (mp *Pool) tryToRecoverWithMember(member *Member) error {
	var op = "member_pool.tryToRecoverWithMember"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("Start recovering with member: %s", member.addr))
	ticker := time.NewTicker(time.Millisecond * member.recoverIntervalMs)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			member.recoverMisses++
			utils.CloseConn(mp, member.addr)
			if member.recoverMisses > member.maxRecoverMisses {
				close(member.stopCh)
				return fmt.Errorf("recover attempts with member: %s, are over", member.addr)
			}
			if err := mp.dialWithMember(member, nil); err == nil {
				return err
			}
		case <-member.ctx.Done():
		case <-member.stopCh:
			return fmt.Errorf(fmt.Sprintf("member is down, stop recovering, addr: %s", member.addr))
		case <-mp.owner.stopCh:
			return fmt.Errorf(fmt.Sprintf("owner stopped: %s, abort recovering with member addr: %s", mp.owner, member.addr))
		}
	}
}

func (mp *Pool) setMemberStatus(member *Member, s int) {
	switch s {
	case status.Running:
		atomic.AddInt32(&activeMembersCount, 1)
	default:
		atomic.AddInt32(&activeMembersCount, -1)
	}
	member.status.Store(uint32(s))
}

func (mp *Pool) dialWithMember(member *Member, msg any) error {
	var (
		op      = "member_pool.dialWithMember"
		err     error
		errorCh = make(chan error, 1)
		conn    net.Conn
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*member.recoverTimeoutMs)
	defer cancel()
	switch mp.tlsCfg {
	case nil:
		go func() {
			if _, ok := mp.connPool[member.addr]; ok != true {
				conn, err = net.Dial("tcp", member.addr)
				if conn != nil {
					utils.SaveConn(mp, member.addr, conn)
				}
			}
			if err == nil {
				req := &message.JoinMessage{Remote: &message.PID{Id: mp.owner.id, Addr: mp.engine.Sender()}}
				if msg != nil {
					data, err := ser.Serialize(msg)
					if err != nil {
						slog.With("op", op).Error(fmt.Sprintf("error serialize message: %s", err.Error()))
						errorCh <- err
						return
					}
					req.Data = data
					req.TypeName = ser.TypeName(msg)
				}
				sendCtx := clusterContext.New(req).WithReceiver(&message.PID{Addr: member.addr})
				err = mp.engine.Send(sendCtx)
				if err != nil {
					errorCh <- err
					return
				}
				close(errorCh)
				return
			}
			errorCh <- err
		}()
		//TODO: implement tls
	}
	select {
	case err = <-errorCh:
		if err != nil {
			fmt.Println("DIAL ERROR:", err, "ADDR:", member.addr)
			return fmt.Errorf(fmt.Sprintf("error dialing: %s", err.Error()))
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf(fmt.Sprintf("dial timeout for address: %s", member.addr))
	case <-mp.owner.stopCh:
		return fmt.Errorf(fmt.Sprintf("owner stopped: %s", member.addr))
	}
}

func (mp *Pool) ackJoinWithMember(member *Member) {
	var (
		op      = "member_pool.askJoinWithMember"
		errorCh = make(chan error, 1)
	)
	go func() {
		if err := mp.dialWithMember(member, nil); err != nil {
			errorCh <- err
		}
	}()
	for {
		select {
		case <-member.joinAckCh:
			mp.setMemberStatus(member, status.Running)
			go mp.startHeartbeatWithMember(member)
			return
		case <-member.stopCh:
			slog.With(slog.String("op", op)).Warn(fmt.Sprintf("member stopped: %s", mp.owner))
			return
		case <-mp.owner.stopCh:
			slog.With(slog.String("op", op)).Warn(fmt.Sprintf("owner ctx is done: %s", mp.owner))
			return
		case err := <-errorCh:
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("ack join error: %s, owner: %s", err.Error(), mp.owner))
			return
		}
	}
}

func (mp *Pool) election() error {
	var (
		op  = "member_pool.election"
		err error
	)
	<-mp.deadLeaderCh
	mp.electionTime = time.Duration(raft.RandElection(mp.mainCfg)) * time.Millisecond
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("no leader node in cluster, starting election, election time: %s, members count: %d, active members: %d", mp.electionTime.String(), len(mp.members), int(activeMembersCount)))
	mp.votes = make(map[string]struct{})
	mp.mu.Lock()
	mp.currTerm++
	// vote for yourself
	mp.votes[mp.owner.id] = struct{}{}
	mp.mu.Unlock()
	ticker := time.NewTicker(mp.electionTime)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err = mp.broadcastVote()
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("vote broadcast err: %s", err.Error()))
			}
		case <-mp.aliveLeaderCh:
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("leader selected, stopping election on node: %s", mp.owner.id))
			return err
		case <-mp.owner.stopCh:
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("owner nodr stopped, addr: %s", mp.owner))
			return err
		}
	}
}

func (mp *Pool) broadcastVote() error {
	var (
		op  = "member_pool.broadcastVote"
		err error
	)
	mp.wg.Add(int(activeMembersCount))
	for addr, member := range mp.members {
		if member.status.Load() != status.Running {
			continue
		}
		go func(addr string) {
			defer mp.wg.Done()
			err = mp.requestVote(addr)
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send vote error: %s", err.Error()))
			}
		}(addr)
	}
	mp.wg.PeriodicCheck(time.Second)
	mp.wg.Wait()
	return err
}

func (mp *Pool) requestVote(addr string) error {
	var (
		err error
	)
	fmt.Println("REQUEST VOTE FROM:", addr)
	msg := &message.ElectionMessage{Remote: &message.PID{Id: mp.owner.id, Addr: mp.owner.addr}}
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	err = mp.engine.Send(ctx)
	return err
}

func (mp *Pool) handleVote(msg *message.ElectionMessage) error {
	var (
		op  = "member_pool.handleVote"
		id  = msg.GetRemote().GetId()
		err error
	)
	ack := true
	if msg.GetAcknowledged() == true {
		mp.mu.Lock()
		defer mp.mu.Unlock()
		mp.votes[id] = struct{}{}
		if len(mp.votes) > len(mp.members)/2 {
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("this node becomes new leader: %s", mp.owner.id))
			mp.broadcastSetLeader()
		}
		return nil
	}
	if mp.members[mp.owner.id].state.Load() != Candidate {
		ack = false
	}
	resp := &message.ElectionMessage{Acknowledged: ack, Remote: &message.PID{Id: mp.owner.id, Addr: mp.owner.addr}}
	ctx := clusterContext.New(resp).WithReceiver(&message.PID{Addr: msg.GetRemote().GetAddr()})
	err = mp.engine.Send(ctx)
	return err
}

func (mp *Pool) broadcastSetLeader() {
	fmt.Println("BROADCAST SET LEAD")
	var op = "member_pool.broadcastSetLeader"
	mp.wg.Add(int(activeMembersCount))
	for addr, member := range mp.members {
		if member.status.Load() != status.Running {
			continue
		}
		go func(addr string) {
			defer mp.wg.Done()
			if err := mp.sendSetLeader(addr); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send set leader error: %s", err.Error()))
			}
		}(addr)
	}
	mp.wg.Wait()
	mp.aliveLeaderCh <- struct{}{}
}

func (mp *Pool) manageMembers() {
	mp.wg.Add(1)
	defer mp.wg.Done()
	ticker := time.NewTicker(time.Millisecond * time.Duration(mp.mainCfg.MemberHeartbeatIntervalMs))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mp.tryToRecoverInactiveMembers()
			fmt.Println("FINISH MANAGING MEMBERS")
			for addr, member := range mp.members {
				fmt.Println("member:", addr, "misses:", member.heartbeatMisses)
			}
		case <-mp.owner.stopCh:
			return
		}
	}
}

func (mp *Pool) tryToRecoverInactiveMembers() {
	var op = "node.tryToRecoverInactiveMembers"
	for addr, member := range mp.members {
		if member.status.Load() == status.Recovering {
			continue
		}
		if member.heartbeatMisses > mp.mainCfg.MaxMemberHeartbeatMisses {
			mp.wg.Add(1)
			member.heartbeatMisses = 0
			member.status.CompareAndSwap(status.Stopped, status.Recovering)
			go func() {
				defer mp.wg.Done()
				if err := mp.tryToRecoverWithMember(member); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("member recover error: %s, member addr: %s", err.Error(), addr))
					return
				}
				go mp.startHeartbeatWithMember(member)
			}()
		} else {
			member.heartbeatMisses++
		}
	}
}

func (mp *Pool) sendSetLeader(addr string) error {
	msg := &message.SetLeaderMessage{Remote: &message.PID{Id: mp.owner.id, Addr: mp.owner.addr}}
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	return mp.engine.Send(ctx)
}

func (mp *Pool) handleSetLeader(msg *message.SetLeaderMessage) error {
	var (
		op  = "node.handleSetLeader"
		err error
	)
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("setting new leader: %s", msg.GetRemote().GetAddr()))
	mp.owner.leaderAddr = msg.GetRemote().GetAddr()
	mp.aliveLeaderCh <- struct{}{}
	return err
}

func (mp *Pool) GetMutex() *sync.RWMutex {
	return &mp.mu
}

func (mp *Pool) GetConnPool() map[string]net.Conn {
	return mp.connPool
}

func (mp *Pool) GetEngine() *stream.Engine {
	return mp.engine
}
