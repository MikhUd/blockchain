package raft

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/consts"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/serializer"
	discovery "github.com/MikhUd/blockchain/pkg/service_discovery"
	"github.com/MikhUd/blockchain/pkg/status"
	"github.com/MikhUd/blockchain/pkg/stream"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/MikhUd/blockchain/pkg/waitgroup"
	"github.com/google/uuid"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	activeMembersCount int32
	ser                serializer.ProtoSerializer
	defaultDialTimeout = 5 * time.Second
)

type Cluster struct {
	owner         Owner
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
	leaderId      string
}

type Member struct {
	addr                string
	publicKey           *ecdsa.PublicKey
	timestamp           int64
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

func NewOwner(id, addr string, stopCh <-chan struct{}) *Owner {
	return &Owner{id: id, addr: addr, stopCh: stopCh}
}

func NewCluster(owner Owner, electionTime time.Duration, members []discovery.Member, mainCfg config.Config) (*Cluster, error) {
	var op = "member_pool.NewMemberPool"
	c := &Cluster{
		owner:         owner,
		members:       make(map[string]*Member, len(members)),
		connPool:      make(map[string]net.Conn),
		votes:         make(map[string]struct{}),
		deadLeaderCh:  make(chan struct{}, 1),
		aliveLeaderCh: make(chan struct{}, 1),
		mainCfg:       mainCfg,
		wg:            &waitgroup.WaitGroup{},
		engine:        stream.NewEngine(owner.addr),
		electionTime:  electionTime,
	}
	if err := c.setupMembers(members); err != nil {
		return nil, err
	}
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("cluster successfully created, election time: %s ", electionTime.String()))
	return c, nil
}

func (c *Cluster) setupMembers(members []discovery.Member) error {
	for i := 0; i < len(members); i++ {
		if err := c.AddMember(uuid.New().String(), members[i].Address, utils.PublicKeyFromString(members[i].PublicKey)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) AddMember(id, addr string, publicKey *ecdsa.PublicKey) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(c.mainCfg.MemberTimeoutSec))
	m := &Member{
		addr:                addr,
		publicKey:           publicKey,
		timestamp:           time.Now().Unix(),
		heartbeatMisses:     0,
		maxHeartbeatMisses:  c.mainCfg.MaxMemberHeartbeatMisses,
		recoverMisses:       0,
		maxRecoverMisses:    c.mainCfg.MaxMemberRecoverMisses,
		heartbeatIntervalMs: time.Millisecond * time.Duration(c.mainCfg.MemberHeartbeatIntervalMs),
		recoverIntervalMs:   time.Millisecond * time.Duration(c.mainCfg.MemberRecoverIntervalMs),
		stopCh:              make(chan struct{}, 1),
		joinAckCh:           make(chan struct{}, 1),
		ctx:                 ctx,
		cancel:              cancel,
	}
	m.state.Store(Follower)
	m.status.Store(status.Initialized)
	c.mu.Lock()
	c.members[id] = m
	c.mu.Unlock()
	return nil
}

func (c *Cluster) Start() error {
	if len(c.members) > 0 {
		c.wg.Add(len(c.members))
		for id := range c.members {
			go func(id string) {
				defer c.wg.Done()
				errCh := make(chan error, 1)
				c.ackJoinWithMember(c.members[id], errCh)
				if err := <-errCh; err != nil {
					slog.Error("error join with member, member stopped", c.members[id], err)
					c.members[id].status.CompareAndSwap(status.Initialized, status.Stopped)
				}
			}(id)
		}
	}
	return nil
}

func (c *Cluster) Stop() error {
	return nil
}

func (c *Cluster) startHeartbeatWithMember(member *Member) {
	var op = "member_pool.startHeartbeat"
	ticker := time.NewTicker(time.Millisecond * member.heartbeatIntervalMs)
	defer ticker.Stop()
	errorCh := make(chan error, 1)
	go func() {
		for {
			select {
			case <-c.owner.stopCh:
				slog.With(slog.String("op", op)).Info(fmt.Sprintf("stop heartbeat, owner:%s stopped", c.owner))
				return
			case <-ticker.C:
				go func() {
					if err := c.sendHeartbeat(member); err != nil {
						slog.With(slog.String("op", op)).Warn(fmt.Sprintf("Error heartbeat with member: %s", member.addr))
						errorCh <- err
					}
				}()
			case err := <-errorCh:
				go c.handleErrorHeartbeat(member, err)
				return
			}
		}
	}()
}

func (c *Cluster) sendHeartbeat(member *Member) error {
	defer func() {
		member.heartbeatMisses++
	}()
	if member.status.Load() != status.Running {
		return fmt.Errorf(fmt.Sprintf("Can not send heartbeat to inactive member: %s", member.addr))
	}
	msg := &message.HeartbeatMessage{Remote: &message.PID{Id: c.owner.id, Addr: c.engine.Sender()}, Acknowledged: false}
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: member.addr})
	return c.engine.Send(ctx)
}

func (c *Cluster) handleHeartbeat(msg *message.HeartbeatMessage) error {
	var (
		err  error
		id   = msg.GetRemote().GetId()
		addr = msg.GetRemote().GetAddr()
	)
	fmt.Println("RECEIVE HEARTBEAT FROM:", addr)
	if msg.GetAcknowledged() == false {
		resp := &message.HeartbeatMessage{Remote: &message.PID{Id: c.owner.id, Addr: c.owner.addr}, Acknowledged: true}
		fmt.Println("send ack heartbeat to:", addr)
		ctx := clusterContext.New(resp).WithReceiver(&message.PID{Addr: addr})
		return c.engine.Send(ctx)
	}
	if member, ok := c.members[id]; ok {
		utils.Print(consts.Green, fmt.Sprintf("MEMBER ALIVE:%s HEARTBEAT MISSES SET TO ZERO", addr))
		member.heartbeatMisses = 0
	}
	return err
}

func (c *Cluster) handleErrorHeartbeat(member *Member, err error) {
	c.setMemberStatus(member, status.Pending)
	if err := c.tryToRecoverWithMember(member); err != nil {
		member.state.Store(status.Stopped)
		if member.state.Load() == Leader {
			c.deadLeaderCh <- struct{}{}
		}
		return
	}
	// start new heartbeat
	go c.startHeartbeatWithMember(member)
}

func (c *Cluster) tryToRecoverWithMember(member *Member) error {
	var op = "member_pool.tryToRecoverWithMember"
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("Start recovering with member: %s", member.addr))
	member.status.CompareAndSwap(status.Stopped, status.Recovering)
	ticker := time.NewTicker(time.Millisecond * member.recoverIntervalMs)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			member.recoverMisses++
			utils.CloseConn(c, member.addr)
			if member.recoverMisses > member.maxRecoverMisses {
				close(member.stopCh)
				return fmt.Errorf("recover attempts with member: %s, are over", member.addr)
			}
			if err := c.dialWithMember(member, nil, defaultDialTimeout); err == nil {
				return err
			}
		case <-member.ctx.Done():
		case <-member.stopCh:
			return fmt.Errorf(fmt.Sprintf("member is down, stop recovering, addr: %s", member.addr))
		case <-c.owner.stopCh:
			return fmt.Errorf(fmt.Sprintf("owner stopped: %s, abort recovering with member addr: %s", c.owner, member.addr))
		}
	}
}

func (c *Cluster) setMemberStatus(member *Member, s int) {
	switch s {
	case status.Running:
		atomic.AddInt32(&activeMembersCount, 1)
	default:
		atomic.AddInt32(&activeMembersCount, -1)
	}
	member.status.Store(uint32(s))
}

func (c *Cluster) dialWithMember(member *Member, msg any, timeout time.Duration) error {
	var (
		op      = "member_pool.dialWithMember"
		err     error
		errorCh = make(chan error, 1)
		conn    net.Conn
	)
	switch c.tlsCfg {
	case nil:
		go func() {
			if _, ok := c.connPool[member.addr]; ok != true {
				conn, err = net.Dial("tcp", member.addr)
				if conn != nil {
					utils.SaveConn(c, member.addr, conn)
				}
			}
			if err == nil {
				req := &message.JoinMessage{Remote: &message.PID{Id: c.owner.id, Addr: c.engine.Sender()}}
				if msg != nil {
					data, err := ser.Serialize(msg)
					if err != nil {
						slog.With("op", op).Error(fmt.Sprintf("error serialize message: %s", err.Error()))
						errorCh <- err
						return
					}
					req.Data = data
				}
				sendCtx := clusterContext.New(req).WithReceiver(&message.PID{Addr: member.addr})
				err = c.engine.Send(sendCtx)
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
	ticker := time.NewTicker(timeout)
	select {
	case <-ticker.C:
		return fmt.Errorf(fmt.Sprintf("dial timeout for address: %s", member.addr))
	case err = <-errorCh:
		if err != nil {
			fmt.Println("DIAL ERROR:", err, "ADDR:", member.addr)
			return fmt.Errorf(fmt.Sprintf("error dialing: %s", err.Error()))
		}
		return nil
	case <-c.owner.stopCh:
		return fmt.Errorf(fmt.Sprintf("owner stopped: %s", member.addr))
	}
}

func (c *Cluster) ackJoinWithMember(member *Member, errCh chan<- error) {
	var (
		op          = "member_pool.ackJoinWithMember"
		dialErrorCh = make(chan error, 1)
	)
	go func() {
		if err := c.dialWithMember(member, nil, defaultDialTimeout); err != nil {
			dialErrorCh <- err
		}
	}()
	ticker := time.NewTicker(time.Duration(c.mainCfg.MemberJoinTimeoutMs) * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			slog.With(slog.String("op", op)).Warn(fmt.Sprintf("member join timeout: %s", member.addr))
			errCh <- errors.New("member join timeout")
			break
		case <-member.joinAckCh:
			c.setMemberStatus(member, status.Running)
			go c.startHeartbeatWithMember(member)
			errCh <- nil
			break
		case <-member.stopCh:
			slog.With(slog.String("op", op)).Warn(fmt.Sprintf("member stopped: %s", c.owner.id))
			errCh <- errors.New("member stopped")
			break
		case <-c.owner.stopCh:
			slog.With(slog.String("op", op)).Warn(fmt.Sprintf("owner ctx is done: %s", c.owner.id))
			errCh <- errors.New("owner stopped")
			break
		case err := <-dialErrorCh:
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("ack join error: %s, owner: %s", err.Error(), c.owner.id))
			errCh <- err
			break
		}
	}
}

func (c *Cluster) election() error {
	var (
		op  = "member_pool.election"
		err error
	)
	<-c.deadLeaderCh
	c.electionTime = time.Duration(utils.RandElection(c.mainCfg)) * time.Millisecond
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("no leader node in cluster, starting election, election time: %s, members count: %d, active members: %d", c.electionTime.String(), len(c.members), int(activeMembersCount)))
	c.votes = make(map[string]struct{})
	c.mu.Lock()
	// vote for yourself
	c.votes[c.owner.id] = struct{}{}
	c.mu.Unlock()
	ticker := time.NewTicker(c.electionTime)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err = c.broadcastVote()
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("vote broadcast err: %s", err.Error()))
			}
		case <-c.aliveLeaderCh:
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("leader selected, stopping election on node: %s", c.owner.id))
			return err
		case <-c.owner.stopCh:
			slog.With(slog.String("op", op)).Error(fmt.Sprintf("owner nodr stopped, addr: %s", c.owner))
			return err
		}
	}
}

func (c *Cluster) broadcastVote() error {
	var (
		op  = "member_pool.broadcastVote"
		err error
	)
	c.wg.Add(int(activeMembersCount))
	for addr, member := range c.members {
		if member.status.Load() != status.Running {
			continue
		}
		go func(addr string) {
			defer c.wg.Done()
			err = c.requestVote(addr)
			if err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send vote error: %s", err.Error()))
			}
		}(addr)
	}
	c.wg.PeriodicCheck(time.Second)
	c.wg.Wait()
	return err
}

func (c *Cluster) requestVote(addr string) error {
	var (
		err error
	)
	fmt.Println("REQUEST VOTE FROM:", addr)
	msg := &message.ElectionMessage{Remote: &message.PID{Id: c.owner.id, Addr: c.owner.addr}}
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	err = c.engine.Send(ctx)
	return err
}

func (c *Cluster) handleVote(msg *message.ElectionMessage) error {
	var (
		op  = "member_pool.handleVote"
		id  = msg.GetRemote().GetId()
		err error
	)
	ack := true
	if msg.GetAcknowledged() == true {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.votes[id] = struct{}{}
		if len(c.votes) > len(c.members)/2 {
			slog.With(slog.String("op", op)).Info(fmt.Sprintf("this node becomes new leader: %s", c.owner.id))
			c.broadcastSetLeader()
		}
		return nil
	}
	if c.members[c.owner.id].state.Load() != Candidate {
		ack = false
	}
	resp := &message.ElectionMessage{Acknowledged: ack, Remote: &message.PID{Id: c.owner.id, Addr: c.owner.addr}}
	ctx := clusterContext.New(resp).WithReceiver(&message.PID{Addr: msg.GetRemote().GetAddr()})
	err = c.engine.Send(ctx)
	return err
}

func (c *Cluster) broadcastSetLeader() {
	fmt.Println("BROADCAST SET LEAD")
	var op = "member_pool.broadcastSetLeader"
	c.wg.Add(int(activeMembersCount))
	for addr, member := range c.members {
		if member.status.Load() != status.Running {
			continue
		}
		go func(addr string) {
			defer c.wg.Done()
			if err := c.sendSetLeader(addr); err != nil {
				slog.With(slog.String("op", op)).Error(fmt.Sprintf("send set leader error: %s", err.Error()))
			}
		}(addr)
	}
	c.wg.Wait()
	c.aliveLeaderCh <- struct{}{}
}

func (c *Cluster) manageMembers() {
	c.wg.Add(1)
	defer c.wg.Done()
	ticker := time.NewTicker(time.Millisecond * time.Duration(c.mainCfg.MemberHeartbeatIntervalMs))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.tryToRecoverInactiveMembers()
			fmt.Println("FINISH MANAGING MEMBERS")
			for addr, member := range c.members {
				fmt.Println("member:", addr, "misses:", member.heartbeatMisses)
			}
		case <-c.owner.stopCh:
			return
		}
	}
}

func (c *Cluster) tryToRecoverInactiveMembers() {
	var op = "node.tryToRecoverInactiveMembers"
	for addr, member := range c.members {
		if member.status.Load() == status.Recovering {
			continue
		}
		if member.heartbeatMisses > c.mainCfg.MaxMemberHeartbeatMisses {
			c.wg.Add(1)
			member.heartbeatMisses = 0
			member.status.CompareAndSwap(status.Stopped, status.Recovering)
			go func() {
				defer c.wg.Done()
				if err := c.tryToRecoverWithMember(member); err != nil {
					slog.With(slog.String("op", op)).Error(fmt.Sprintf("member recover error: %s, member addr: %s", err.Error(), addr))
					return
				}
				go c.startHeartbeatWithMember(member)
			}()
		} else {
			member.heartbeatMisses++
		}
	}
}

func (c *Cluster) sendSetLeader(addr string) error {
	msg := &message.SetLeaderMessage{Remote: &message.PID{Id: c.owner.id, Addr: c.owner.addr}}
	ctx := clusterContext.New(msg).WithReceiver(&message.PID{Addr: addr})
	return c.engine.Send(ctx)
}

func (c *Cluster) handleSetLeader(msg *message.SetLeaderMessage) error {
	var (
		op  = "node.handleSetLeader"
		err error
	)
	slog.With(slog.String("op", op)).Info(fmt.Sprintf("setting new leader: %s", msg.GetRemote().GetAddr()))
	c.owner.leaderAddr = msg.GetRemote().GetAddr()
	c.aliveLeaderCh <- struct{}{}
	return err
}

func (c *Cluster) GetMutex() *sync.RWMutex {
	return &c.mu
}

func (c *Cluster) GetConnPool() map[string]net.Conn {
	return c.connPool
}

func (c *Cluster) GetEngine() *stream.Engine {
	return c.engine
}

func (c *Cluster) RemoveMember(id string) error {
	delete(c.members, id)
	//TODO remove from discovery
	return nil
}

func (c *Cluster) GetMembers() []*Member {
	members := make([]*Member, 0, len(c.members))
	for _, m := range c.members {
		members = append(members, m)
	}
	return members
}

func (c *Cluster) GetLeaderID() string {
	return c.leaderId
}
