package storage

import (
	"context"

	"github.com/kvvPro/gophermart/internal/model"
)

type Storage interface {
	Ping(ctx context.Context) error
	Quit(ctx context.Context)
	AddUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, user *model.User) error
}
