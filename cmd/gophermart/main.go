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
	config.Sugar = *logger.Sugar()

	app.Sugar.Infoln("before init config")

	srvFlags, err := config.Initialize()
	if err != nil {
		app.Sugar.Fatalw(err.Error(), "event", "get config")
	}

	app.Sugar.Infoln("after init config")

	ctx := context.Background()

	srv, err := app.NewServer(ctx, srvFlags)

	if err != nil {
		app.Sugar.Fatalw(err.Error(), "event", "create server")
	}

	go srv.AsyncUpdate(ctx)

	app.Sugar.Infoln("before starting server")

	go srv.Run(ctx, srvFlags)

	sigQuit := <-shutdown
	app.Sugar.Infoln("Server shutdown by signal: ", sigQuit)
}
