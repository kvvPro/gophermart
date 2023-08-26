package app

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kvvPro/gophermart/internal/model"
	"github.com/kvvPro/gophermart/internal/retry"
)

func (srv *Server) GetBalance(ctx context.Context, userInfo *model.User) (*model.Balance, error) {
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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {

		Sugar.Errorln(err)
		return nil, err
	}

	return balance, nil
}
