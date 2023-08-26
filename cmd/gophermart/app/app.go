package app

import (
	"context"
	"errors"
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
}

func NewServer(ctx context.Context, configs *config.ServerFlags) (*Server, error) {
	st, err := postgres.NewPSQLStorage(ctx, configs.DBConnection)
	if err != nil {
		return nil, errors.New("cannot create storage for server" + err.Error())
	}

	return &Server{
		storage:              st,
		Address:              configs.Address,
		DBConnection:         configs.DBConnection,
		AccrualSystemAddress: configs.AccrualSystemAddress,
	}, nil
}

func (srv *Server) quit(ctx context.Context) {
	srv.storage.Quit(ctx)
}

func (srv *Server) Ping(ctx context.Context) error {
	return srv.storage.Ping(ctx)
}

func (srv *Server) AsyncUpdate(ctx context.Context) {

	for {
		// wait interval
		select {
		case <-time.After(time.Duration(srv.ReadingAccrualInterval) * time.Second):
		case <-ctx.Done():
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
				err = srv.UpdateOrders(ctx, ordersForUpdate)
				if err != nil {
					Sugar.Errorln(err)
				}
			}
		}
	}
}
