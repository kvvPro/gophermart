package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/kvvPro/gophermart/cmd/gophermart/app"
	"github.com/kvvPro/gophermart/cmd/gophermart/config"

	"go.uber.org/zap"
)

func main() {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	logger, err := zap.NewDevelopment()
	if err != nil {
		// вызываем панику, если ошибка
		panic(err)
	}
	defer logger.Sync()

	// делаем регистратор SugaredLogger
	app.Sugar = *logger.Sugar()

	app.Sugar.Infoln("before init config")

	srvFlags := config.Initialize()

	app.Sugar.Infoln("after init config")

	srv, err := app.NewServer(context.Background(), &srvFlags)

	if err != nil {
		app.Sugar.Fatalw(err.Error(), "event", "create server")
	}

	//go srv.AsyncUpdate(context.Background())

	app.Sugar.Infoln("before starting server")

	go srv.Run(context.Background(), &srvFlags)

	sigQuit := <-shutdown
	app.Sugar.Infoln("Server shutdown by signal: ", sigQuit)
}
