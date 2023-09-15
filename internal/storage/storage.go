package storage

import (
	"context"

	"github.com/kvvPro/gophermart/internal/model"
)

type Storage interface {
	Ping(ctx context.Context) error
	Quit(ctx context.Context)
	AddUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, user *model.User) (*model.User, error)
	UploadOrder(ctx context.Context, orderID string, user *model.User) (model.EndPointStatus, error)
	GetAllOrders(ctx context.Context, user *model.User) ([]*model.Order, error)
	GetBalance(ctx context.Context, user *model.User) (*model.Balance, error)
	RequestWithdrawal(ctx context.Context, withdrawalInfo *model.Withdrawal) (model.EndPointStatus, error)
	GetAllWithdrawals(ctx context.Context, user *model.User) ([]*model.Withdrawal, error)
	GetOrdersForUpdate(ctx context.Context) ([]model.Order, error)
	UpdateBatchOrders(ctx context.Context, orders []model.Order) error
}
