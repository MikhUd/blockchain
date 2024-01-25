package main

import (
	"flag"
	"github.com/MikhUd/blockchain/cmd/peer_manager"
	"github.com/MikhUd/blockchain/internal/config"
	"github.com/MikhUd/blockchain/internal/utils"
	bcproto "github.com/MikhUd/blockchain/protos/blockchain"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.MustLoad()
	log := setupLogger(cfg.Env)
	log.Info("Starting application", slog.String("env", cfg.Env))

	flag.Parse()
	/*
		portBs := flag.Uint("port_bs", 5000, "TCP port for peer_manager server")
		appBs := blockchain_server.New(uint16(*portBs), *cfg)
		go appBs.Run()
		portWs := flag.Uint("port_ws", 8080, "TCP wallet server port")
		gateway := flag.String("gateway", net.JoinHostPort(utils.GetHost(), strconv.Itoa(int(appBs.Port()))), "Blockchain gateway")
		appWs := ws.New(uint16(*portWs), *gateway)
		go appWs.Run()

		time.Sleep(time.Second * 5)
		portBs1 := flag.Uint("port_bs1", 5001, "TCP port for peer_manager server")
		appBs1 := blockchain_server.New(uint16(*portBs1), *cfg)
		go appBs1.Run()
		time.Sleep(time.Second * 5)
		portBs2 := flag.Uint("port_bs2", 5002, "TCP port for peer_manager server")
		appBs2 := blockchain_server.New(uint16(*portBs2), *cfg)
		go appBs2.Run()
	*/

	/*
		bn := peer_manager.New(cfg).WithLogger(log)
		err := bn.Start()
		if err != nil {
			fmt.Printf(err.Error())
		}
	*/
	pm := peer_manager.New(cfg).WithLogger(slog.Default())
	err := pm.Start()
	if err != nil {
		slog.Error(err.Error())
	}
	s, _ := utils.GenerateRandomSignature()
	pk, _ := utils.GenerateRandomPublicKey()
	w := peer_manager.NewWriter(pm.Addr())
	tr := &bcproto.TransactionRequest{
		SenderPublicKey:            pk.String(),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Signature:                  s.String(),
		Value:                      10.0,
	}
	ctx := peer_manager.NewContext(nil, nil, nil, tr)
	err = w.Send(ctx)

	for _, n := range pm.GetNodes() {
		n.Blockchain().Print()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	sign := <-stop

	log.Info("stopping application", slog.String("signal", sign.String()))
	log.Info("application stopped")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case config.EnvLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case config.EnvDev:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case config.EnvProd:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
