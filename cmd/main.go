package main

import (
	"flag"
	"github.com/MikhUd/blockchain/pkg/api/message"
	"github.com/MikhUd/blockchain/pkg/cluster"
	"github.com/MikhUd/blockchain/pkg/config"
	"github.com/MikhUd/blockchain/pkg/context"
	"github.com/MikhUd/blockchain/pkg/utils"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var (
	configPath = flag.String("config_path", "", "path to local config")
)

func main() {
	flag.Parse()
	cfg := config.MustLoad(*configPath)
	log := setupLogger(cfg.Env)
	log.Info("Starting application", slog.String("env", cfg.Env))

	flag.Parse()
	/*
		portBs := flag.Uint("port_bs", 5000, "TCP port for cluster server")
		appBs := blockchain_server.New(uint16(*portBs), *cfg)
		go appBs.Run()
		portWs := flag.Uint("port_ws", 8080, "TCP wallet server port")
		gateway := flag.String("gateway", net.JoinHostPort(utils.GetHost(), strconv.Itoa(int(appBs.Port()))), "Blockchain gateway")
		appWs := ws.New(uint16(*portWs), *gateway)
		go appWs.Run()

		time.Sleep(time.Second * 5)
		portBs1 := flag.Uint("port_bs1", 5001, "TCP port for cluster server")
		appBs1 := blockchain_server.New(uint16(*portBs1), *cfg)
		go appBs1.Run()
		time.Sleep(time.Second * 5)
		portBs2 := flag.Uint("port_bs2", 5002, "TCP port for cluster server")
		appBs2 := blockchain_server.New(uint16(*portBs2), *cfg)
		go appBs2.Run()
	*/

	/*
		bn := cluster.New(cfg).WithLogger(log)
		err := bn.Start()
		if err != nil {
			fmt.Printf(err.Error())
		}
	*/
	c := cluster.New(*cfg, ":8080")
	err := c.Start()
	if err != nil {
		slog.Error(err.Error())
	}
	privateKey, publicKey, err := utils.GenerateKeyPair()
	tr := &message.TransactionRequest{
		SenderPublicKey:            utils.PublicKeyStr(publicKey),
		SenderPrivateKey:           utils.PrivateKeyStr(privateKey),
		SenderBlockchainAddress:    "sender",
		RecipientBlockchainAddress: "recipient",
		Value:                      10.0,
	}
	err = c.Engine.Send(context.New(tr))

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
