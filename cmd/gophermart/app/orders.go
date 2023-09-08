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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
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
	if len(orders) == 0 {
		return nil, model.OrderListEmpty, nil
	}

	return orders, model.OrderListExists, nil
}

func (srv *Server) GetOrdersForUpdate(ctx context.Context) ([]model.Order, error) {
	var err error
	var orders []model.Order

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
		retry.InitDelay(5*time.Millisecond),
		retry.Step(2*time.Millisecond),
		retry.Context(ctx),
	)

	if err != nil {
		Sugar.Errorln(err)
		return nil, err
	}

	return orders, nil
}

func (srv *Server) UpdateOrders(ctx context.Context, orders []model.Order) error {

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
