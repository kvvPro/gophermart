package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

	wg := &sync.WaitGroup{}

	asyncCtx, cancelUpdate := context.WithCancel(ctx)
	// размножим обновления из внешнего сервиса
	for i := 0; i < srv.UpdateThreadCount; i++ {
		wg.Add(1)
		go srv.AsyncUpdate(asyncCtx, wg)
	}

	app.Sugar.Infoln("before starting server")

	wg.Add(1)
	httpSrv := srv.StartServer(ctx, wg, srvFlags)

	sigQuit := <-shutdown

	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	app.Sugar.Infoln("Попытка мягко завершить сервер")
	if err := httpSrv.Shutdown(timeout); err != nil {
		app.Sugar.Errorf("Ошибка при попытке мягко завершить http-сервер: %v", err)
		// handle err
		if err = httpSrv.Close(); err != nil {
			app.Sugar.Errorf("Ошибка при попытке завершить http-сервер: %v", err)
		}
	}
	cancelUpdate()
	wg.Wait()
	app.Sugar.Infoln("Server shutdown by signal: ", sigQuit)
}
