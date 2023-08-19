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
		return nil, errors.New("cannot create storage for server")
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

func (srv *Server) CheckUser(ctx context.Context, user *model.User) error {
	return srv.storage.GetUser(ctx, user)
}

func (srv *Server) Run(ctx context.Context, srvFlags *config.ServerFlags) {
	r := chi.NewMux()
	r.Use(GzipMiddleware,
		WithLogging)
	// r.Use(app.WithLogging)
	r.Get("/ping", http.HandlerFunc(srv.PingHandle))
	r.Post("/api/user/register", http.HandlerFunc(srv.Register))
	r.Post("/api/user/login", http.HandlerFunc(srv.Auth))
	r.Post("/api/user/orders", http.HandlerFunc(srv.PutOrders))
	r.Get("/api/user/orders", http.HandlerFunc(srv.GetOrders))
	r.Get("/api/user/balance", http.HandlerFunc(srv.GetBalance))
	r.Post("/api/user/balance/withdraw", http.HandlerFunc(srv.Withdraw))
	r.Get("/api/user/withdrawals", http.HandlerFunc(srv.GetWithdrawals))

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
