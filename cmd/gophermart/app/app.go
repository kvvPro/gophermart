package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kvvPro/gophermart/cmd/gophermart/config"
	"github.com/kvvPro/gophermart/internal/model"
	"github.com/kvvPro/gophermart/internal/retry"
	"github.com/kvvPro/gophermart/internal/storage"

	"github.com/kvvPro/gophermart/internal/storage/postgres"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	Address                string
	DBConnection           string
	AccrualSystemAddress   string
	storage                storage.Storage
	ReadingAccrualInterval int
}

func NewServer(ctx context.Context, config *config.ServerFlags) (*Server, error) {
	st, err := postgres.NewPSQLStorage(ctx, config.DBConnection)
	if err != nil {
		return nil, errors.New("cannot create storage for server" + err.Error())
	}

	return &Server{
		storage:              st,
		Address:              config.Address,
		DBConnection:         config.DBConnection,
		AccrualSystemAddress: config.AccrualSystemAddress,
	}, nil
}

func (srv *Server) quit(ctx context.Context) {
	srv.storage.Quit(ctx)
}

func (srv *Server) Ping(ctx context.Context) error {
	return srv.storage.Ping(ctx)
}

func (srv *Server) AddUser(ctx context.Context, user *model.User) error {
	err := retry.Do(func() error {
		return srv.storage.AddUser(ctx, user)
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return err
	}

	return nil
}

func (srv *Server) CheckUser(ctx context.Context, user *model.User) (*model.User, error) {
	var userInfo *model.User
	var err error
	err = retry.Do(func() error {
		userInfo, err = srv.storage.GetUser(ctx, user)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return nil, err
	}

	return userInfo, nil
}

func (srv *Server) UploadOrder(ctx context.Context, orderID string, userInfo *model.User) (model.EndPointStatus, error) {
	var err error
	var result model.EndPointStatus

	err = retry.Do(func() error {
		result, err = srv.storage.UploadOrder(ctx, orderID, userInfo)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {

		Sugar.Errorln(err)
		// result = model.OtherError

		var pgErr *pgconn.PgError
		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			result = model.ConnectionError
		}

		return result, err
	}

	return result, nil
}

func (srv *Server) OrderList(ctx context.Context, userInfo *model.User) ([]*model.Order, model.EndPointStatus, error) {
	var err error
	var orders []*model.Order

	err = retry.Do(func() error {
		orders, err = srv.storage.GetAllOrders(ctx, userInfo)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {

		Sugar.Errorln(err)

		var pgErr *pgconn.PgError
		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			return nil, model.ConnectionError, err
		}

		return nil, model.OtherError, err
	}

	// check data
	if len(orders) == 0 {
		return nil, model.OrderListEmpty, nil
	}

	return orders, model.OrderListExists, nil
}

func (srv *Server) GetCurrentBalance(ctx context.Context, userInfo *model.User) (*model.Balance, error) {
	var err error
	var balance *model.Balance

	err = retry.Do(func() error {
		balance, err = srv.storage.GetBalance(ctx, userInfo)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {

		Sugar.Errorln(err)
		return nil, err
	}

	return balance, nil
}

func (srv *Server) RequestWithdraw(ctx context.Context, withdrawalInfo *model.Withdrawal) (model.EndPointStatus, error) {
	var err error
	var result model.EndPointStatus

	err = retry.Do(func() error {
		result, err = srv.storage.RequestWithdrawal(ctx, withdrawalInfo)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {

		Sugar.Errorln(err)

		var pgErr *pgconn.PgError
		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			result = model.ConnectionError
		}

		return result, err
	}

	return result, nil
}

func (srv *Server) AllWithdrawals(ctx context.Context, user *model.User) ([]*model.Withdrawal, model.EndPointStatus, error) {
	var err error
	var withdrawals []*model.Withdrawal

	err = retry.Do(func() error {
		withdrawals, err = srv.storage.GetAllWithdrawals(ctx, user)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {

		Sugar.Errorln(err)

		var pgErr *pgconn.PgError
		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			return nil, model.ConnectionError, err
		}

		return nil, model.OtherError, err
	}

	// check data
	if len(withdrawals) == 0 {
		return nil, model.WithdrawalsNoData, nil
	}

	return withdrawals, model.WithdrawalsDataExists, nil
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
			continue
		}
		// запрашиваем статусы у внещней системы
		if len(orders) > 0 {
			ordersForUpdate, _ := srv.RequestAccrual(ctx, orders)
			// обновляем информацию в нашей системе
			if len(ordersForUpdate) > 0 {
				err = srv.UpdateOrders(ctx, ordersForUpdate)
				if err != nil {
					continue
				}
			}
		}
	}
}

func (srv *Server) GetOrdersForUpdate(ctx context.Context) ([]*model.Order, error) {
	var err error
	var orders []*model.Order

	err = retry.Do(func() error {
		orders, err = srv.storage.GetOrdersForUpdate(ctx)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return nil, err
	}

	return orders, nil
}

func (srv *Server) RequestAccrual(ctx context.Context, orders []*model.Order) ([]*model.Order, error) {

	client := &http.Client{}
	url := srv.AccrualSystemAddress + "/api/orders/{number}"

	ordersForUpdate := []*model.Order{}

	for _, el := range orders {
		localURL := strings.Replace(url, "{number}", el.ID, 1)
		bodyBuffer := new(bytes.Buffer)
		request, err := http.NewRequest(http.MethodGet, localURL, bodyBuffer)
		if err != nil {
			Sugar.Infoln("Error request: ", err.Error())
			continue
		}
		request.Header.Set("Connection", "Keep-Alive")
		response, err := client.Do(request)
		if err != nil {
			Sugar.Infoln("Error response: ", err.Error())
			continue
		}

		dataResponse, err := io.ReadAll(response.Body)
		if err != nil {
			Sugar.Infoln("Error reading response body: ", err.Error())
			continue
		}

		var newInfo model.Order
		Sugar.Infoln("-----------NEW REQUEST---------------")
		Sugar.Infoln(
			"uri", request.RequestURI,
			"method", request.Method,
			"status", response.Status, // получаем код статуса ответа
		)
		Sugar.Infoln("response-from-accrual: ", string(dataResponse))

		reader := io.NopCloser(bytes.NewReader(dataResponse))
		if err := json.NewDecoder(reader).Decode(&newInfo); err != nil {
			Sugar.Infoln("Error to parse response body")
			continue
		}

		response.Body.Close()

		// анализируем ответы
		if response.StatusCode == http.StatusOK {
			// обновляем данные
			newInfo.ID = el.ID
			newInfo.Owner = el.Owner
			newInfo.UploadDate = el.UploadDate
			ordersForUpdate = append(ordersForUpdate, &newInfo)
		} else if response.StatusCode == http.StatusNoContent {
			// данных по заказу нет - можно не обновлять
			continue
		} else if response.StatusCode == http.StatusTooManyRequests {
			// надо подождать и попробовать заново через Retry-After
			continue
		} else {
			// любые другие ошибки - просто пропускаем попытку
			continue
		}
	}

	return ordersForUpdate, nil
}

func (srv *Server) UpdateOrders(ctx context.Context, orders []*model.Order) error {
	var err error

	err = retry.Do(func() error {
		err = srv.storage.UpdateBatchOrders(ctx, orders)
		return err
	},
		retry.RetryIf(func(errAttempt error) bool {
			var pgErr *pgconn.PgError
			if errors.As(errAttempt, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
				return true
			}
			return false
		}),
		retry.Attempts(3),
		retry.InitDelay(1000*time.Millisecond),
		retry.Step(2000*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return err
	}

	return nil
}

func (srv *Server) Run(ctx context.Context, srvFlags *config.ServerFlags) {
	r := chi.NewMux()
	r.Use(GzipMiddleware,
		WithLogging)
	r.Get("/ping", http.HandlerFunc(srv.PingHandle))
	r.Post("/api/user/register", http.HandlerFunc(srv.Register))
	r.Post("/api/user/login", http.HandlerFunc(srv.Auth))

	r.Group(func(r chi.Router) {
		r.Use(srv.CheckAuth)

		r.Post("/api/user/orders", http.HandlerFunc(srv.PutOrder))
		r.Get("/api/user/orders", http.HandlerFunc(srv.GetOrders))
		r.Get("/api/user/balance", http.HandlerFunc(srv.GetBalance))
		r.Post("/api/user/balance/withdraw", http.HandlerFunc(srv.Withdraw))
		r.Get("/api/user/withdrawals", http.HandlerFunc(srv.GetWithdrawals))
	})

	// close all connection after quit
	defer srv.quit(ctx)

	// записываем в лог, что сервер запускается
	Sugar.Infow(
		"Starting server",
		"srvFlags", srvFlags,
	)

	if err := http.ListenAndServe(srv.Address, r); err != nil {
		// записываем в лог ошибку, если сервер не запустился
		Sugar.Fatalw(err.Error(), "event", "start server")
	}
}
