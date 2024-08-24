package node

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/api/remote"
	"github.com/MikhUd/blockchain/pkg/config"
	clusterContext "github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/db/cassandra"
	"github.com/MikhUd/blockchain/pkg/domain/blockchain"
	"github.com/MikhUd/blockchain/pkg/raft"
	discovery "github.com/MikhUd/blockchain/pkg/service_discovery"
	"github.com/MikhUd/blockchain/pkg/status"
	"github.com/MikhUd/blockchain/pkg/stream"
	"github.com/MikhUd/blockchain/pkg/utils"
	"github.com/MikhUd/blockchain/pkg/waitgroup"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const AckMsg = "ACK"

type Node struct {
	id         string
	addr       string
	dbConnStr  string
	cfg        config.Config
	tlsCfg     *tls.Config
	bc         *blockchain.Blockchain
	wg         *waitgroup.WaitGroup
	mu         sync.RWMutex
	dbPool     *pgxpool.Pool
	engine     *stream.Engine
	status     atomic.Uint32
	ln         net.Listener
	stopCh     chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	raft       raft.IRaft
	privateKey *rsa.PrivateKey
	registry   *discovery.Registry
}

func New(addr string, bc *blockchain.Blockchain, cfg config.Config) (*Node, error) {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	n := &Node{
		id:     uuid.New().String(),
		addr:   addr,
		stopCh: make(chan struct{}, 1),
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		bc:     bc,
		wg:     &waitgroup.WaitGroup{},
		engine: stream.NewEngine(addr),
	}
	n.status.Store(status.Initialized)
	n.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	n.registry = discovery.NewRegistry(cassandra.NewDB(cfg.CassandraKeyspace, cfg.CassandraHosts))
	registryMembers, err := n.registry.GetMembers(ctx)
	if err != nil {
		panic(err)
		return nil, err
	}
	n.raft, err = raft.NewRaft(cfg, *raft.NewOwner(n.id, n.addr, n.stopCh), registryMembers)
	if err != nil {
		slog.Error(fmt.Sprintf("error creating raft instance: %v", err))
		return nil, err
	}
	return n, nil
}

func (n *Node) WithTimeout(timeout time.Duration) *Node {
	n.ctx, n.cancel = context.WithTimeout(n.ctx, timeout)
	return n
}

func (n *Node) Start() error {
	var err error

	// Проверка статуса
	if n.status.Load() == status.Running {
		slog.Error("node already running")
		return fmt.Errorf("node already running")
	}

	// Слушаем на порту (динамический или заданный)
	n.ln, err = net.Listen("tcp", n.addr)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("failed to listen on address %s: %w", n.addr, err))
	}

	// Установка статуса запущен
	n.status.Store(status.Running)

	// Инициализация RPC сервера
	mux := drpcmux.New()
	server := drpcserver.New(mux)
	if err = remote.DRPCRegisterRemote(mux, stream.NewReader(n)); err != nil {
		return fmt.Errorf("failed to register RPC service: %w", err)
	}

	// Запуск сервера в отдельной горутине
	n.wg.Add(1)
	errCh := make(chan error, 1)
	go func() {
		defer n.wg.Done()
		err := server.Serve(n.ctx, n.ln)
		if err != nil {
			slog.Error("server", "err", err)
		} else {
			slog.Debug("server stopped")
		}
		errCh <- err
	}()

	// Остановка по каналу и контексту
	go func() {
		select {
		case <-n.stopCh:
			n.stop()
		case <-n.ctx.Done():
			slog.Debug("context canceled")
			n.stop()
		}
	}()

	// Регистрация в реестре
	pubKey := &n.privateKey.PublicKey
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})
	if err = n.registry.RegisterMember(n.ctx, n.id, n.addr, string(publicKeyPEM)); err != nil {
		slog.Error(fmt.Sprintf("error registering node %s: %v", n.addr, err))
		n.stopCh <- struct{}{}
	}

	if err = n.raft.Start(); err != nil {
		errCh <- err
	}

	if n.cfg.HandleInterrupt == 1 {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		go func(errCh chan<- error) {
			select {
			case <-stop:
				fmt.Println("STOP TRIGG")
				if n.status.Load() != status.Stopped {
					if err = n.stop(); err != nil {
						slog.Error(fmt.Sprintf("error stopping node: %s", err.Error()))
					}
				}
				errCh <- err
			}
		}(errCh)
	}

	// Ожидание завершения сервера
	if err = <-errCh; err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	slog.Info(fmt.Sprintf("node: %s started", n.addr))

	return nil
}

func (n *Node) checkDeadline() {
	select {
	case <-n.ctx.Done():
		n.stop()
		return
	}
}

func (n *Node) stop() error {
	if n.status.Load() != status.Running {
		return fmt.Errorf(fmt.Sprintf("node %s already stopped", n.addr))
	}
	n.registry.UnRegisterMember(n.ctx, n.id)
	n.status.Store(status.Stopped)
	close(n.stopCh)
	n.ln.Close()
	n.wg.Wait()
	slog.Info(fmt.Sprintf("node: %s stopped", n.addr))
	return nil
}

func (n *Node) Blockchain() *blockchain.Blockchain {
	return n.bc
}

func (n *Node) Receive(ctx *clusterContext.Context) error {
	var err error
	switch msg := ctx.Msg().(type) {
	case *message.TransactionRequest:
		err = n.addTransaction(msg)
	case *message.JoinMessage:
		err = n.handleJoin(msg)
	}
	return err
}

func (n *Node) addTransaction(msg *message.TransactionRequest) error {
	publicKey := utils.PublicECDSAKeyFromString(msg.SenderPublicKey)
	privateKey := utils.PrivateKeyFromString(msg.SenderPrivateKey, publicKey)
	signature, err := utils.GenerateTransactionSignature(msg.SenderBlockchainAddress, privateKey, msg.RecipientBlockchainAddress, msg.Value)
	if err != nil {
		return err
	}
	_ = n.bc.CreateTransaction(msg.SenderBlockchainAddress, msg.RecipientBlockchainAddress, msg.Value, publicKey, signature)
	return nil
}

func (n *Node) handleJoin(msg *message.JoinMessage) error {
	var (
		err     error
		rawData = msg.GetData()
	)
	member, err := n.registry.GetMember(n.ctx, msg.GetRemote().GetId())
	if err != nil || member == nil {
		return fmt.Errorf(fmt.Sprintf("member not found, id: %s", msg.GetRemote().GetId()))
	}
	decodedData, err := rsa.DecryptPKCS1v15(rand.Reader, n.privateKey, rawData)
	if err != nil || string(decodedData) != AckMsg {
		return fmt.Errorf(fmt.Sprintf("illegal member join request, declined, id: %s", msg.GetRemote().GetId()))
	}
	slog.Info(fmt.Sprintf("member join approval: %s", msg.GetRemote().GetId()))

	return err
}

func (n *Node) GetAddr() string {
	return n.addr
}
