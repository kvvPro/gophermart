package app

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kvvPro/gophermart/cmd/gophermart/config"
)

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
		r.Get("/api/user/balance", http.HandlerFunc(srv.GetBalanceHandle))
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
