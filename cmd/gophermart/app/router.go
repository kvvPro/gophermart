package app

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/kvvPro/gophermart/cmd/gophermart/config"
)

func (srv *Server) StartServer(ctx context.Context, wg *sync.WaitGroup, srvFlags *config.ServerFlags) *http.Server {
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

	// записываем в лог, что сервер запускается
	Sugar.Infow(
		"Starting server",
		"srvFlags", srvFlags,
	)

	httpSrv := &http.Server{
		Addr:    srv.Address,
		Handler: r,
	}
	go func() {
		defer wg.Done()
		defer srv.quit(ctx)

		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			// записываем в лог ошибку, если сервер не запустился
			Sugar.Fatalw(err.Error(), "event", "start server")
		}
	}()

	return httpSrv
}
