package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kvvPro/gophermart/cmd/gophermart/config"
	"github.com/kvvPro/gophermart/internal/model"
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

	// объявим канал для асинхронного запроса информации по заказам из внешней системы
	chOrders := make(chan model.Order, 5)
	chOrdersForUpdate := make(chan model.Order, 10)

	// запускаем горутины для получения инфы из внешней системы
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			srv.threadToGetInfoFromAccrual(ctx, chOrders, chOrdersForUpdate)
		}()
	}
	// поток для обновления информации в нашей системе
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.threadToUpdateOrders(ctx, chOrdersForUpdate)
	}()

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
			for _, el := range orders {
				chOrders <- el
			}
		}
	}
}

func (srv *Server) threadToGetInfoFromAccrual(ctx context.Context,
	chOrders chan model.Order,
	chOrdersForUpdate chan model.Order) {

	for {
		select {
		case order, opened := <-chOrders:
			if !opened {
				// channel is closed
				return
			}
			updatedOrder, needToUpdate := srv.RequestAccrual(ctx, order)
			if needToUpdate {
				chOrdersForUpdate <- *updatedOrder
			}
		case <-ctx.Done():
			Sugar.Infoln("остановка асинхронного обновления")
			return
		}
	}

}

func (srv *Server) threadToUpdateOrders(ctx context.Context,
	chOrdersForUpdate chan model.Order) {

	ordersForUpdate := []model.Order{}

	for {
		select {
		case <-time.After(time.Duration(srv.ReadingAccrualInterval) * time.Second):
		case order, opened := <-chOrdersForUpdate:
			if !opened {
				// channel is closed
				return
			}
			ordersForUpdate = append(ordersForUpdate, order)
			length := len(ordersForUpdate)
			if length > 0 {
				_ = srv.UpdateOrders(ctx, ordersForUpdate)
			}
		case <-ctx.Done():
			Sugar.Infoln("остановка асинхронного обновления")
			return
		}
	}
}
