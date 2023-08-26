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

func (srv *Server) RequestWithdrawal(ctx context.Context, withdrawalInfo *model.Withdrawal) (model.EndPointStatus, error) {
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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
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
