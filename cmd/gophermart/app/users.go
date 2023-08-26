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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return err
	}

	return nil
}

func (srv *Server) GetUser(ctx context.Context, user *model.User) (*model.User, error) {
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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return nil, err
	}

	return userInfo, nil
}
