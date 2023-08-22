package app

import (
	"context"
	"errors"
	"net/http"
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
	Address              string
	DBConnection         string
	AccrualSystemAddress string
	storage              storage.Storage
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
