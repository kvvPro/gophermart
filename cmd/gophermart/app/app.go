package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kvvPro/gophermart/cmd/gophermart/config"
	"github.com/kvvPro/gophermart/internal/storage"

	"github.com/kvvPro/gophermart/internal/storage/postgres"
)

type Server struct {
	Address                string
	DBConnection           string
	AccrualSystemAddress   string
	storage                storage.Storage
	ReadingAccrualInterval int
	UpdateThreadCount      int
}

func NewServer(ctx context.Context, configs *config.ServerFlags) (*Server, error) {
	st, err := postgres.NewPSQLStorage(ctx, configs.DBConnection)
	if err != nil {
		return nil, errors.New("cannot create storage for server" + err.Error())
	}

	return &Server{
		storage:                st,
		Address:                configs.Address,
		DBConnection:           configs.DBConnection,
		AccrualSystemAddress:   configs.AccrualSystemAddress,
		ReadingAccrualInterval: configs.ReadingAccrualInterval,
		UpdateThreadCount:      configs.UpdateThreadCount,
	}, nil
}

func (srv *Server) quit(ctx context.Context) {
	Sugar.Infoln("закрытие пула соединений")
	srv.storage.Quit(ctx)
}

func (srv *Server) Ping(ctx context.Context) error {
	return srv.storage.Ping(ctx)
}

func (srv *Server) AsyncUpdate(ctx context.Context, wg *sync.WaitGroup) {

	defer wg.Done()

	for {
		// wait interval
		select {
		case <-time.After(time.Duration(srv.ReadingAccrualInterval) * time.Second):
		case <-ctx.Done():
			Sugar.Infoln("остановка асинхронного обновления")
			return
		}

		// сначал получим все заказы для обновления
		// это заказы в статусах PROCESSING и NEW
		orders, err := srv.GetOrdersForUpdate(ctx)
		if err != nil {
			Sugar.Errorln(err)
			continue
		}
		// запрашиваем статусы у внещней системы
		if len(orders) > 0 {
			ordersForUpdate, _ := srv.RequestAccrual(ctx, orders)
			// обновляем информацию в нашей системе
			if len(ordersForUpdate) > 0 {
				// распараллелим обновление заказов
				batchSize := len(ordersForUpdate) / srv.UpdateThreadCount
				for i := 0; i < srv.UpdateThreadCount; i++ {
					end := (i + 1) * batchSize
					if i == srv.UpdateThreadCount-1 {
						if len(ordersForUpdate)%srv.UpdateThreadCount != 0 {
							end = len(ordersForUpdate) - 1
						}
					}
					wg.Add(1)
					start := i * batchSize
					go func() {
						_ = srv.UpdateOrders(ctx, wg, ordersForUpdate[start:end])
					}()
				}

			}
		}
	}
}
