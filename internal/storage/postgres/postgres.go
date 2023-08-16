package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kvvPro/gophermart/internal/model"
)

type PostgresStorage struct {
	ConnStr string
	pool    *pgxpool.Pool
}

func NewPSQLStorage(ctx context.Context, connection string) (*PostgresStorage, error) {
	// init
	init := getInitQuery()
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		return nil, err
	}

	defer pool.Close()

	_, err = pool.Exec(ctx, init)
	if err != nil {
		return nil, err
	}

	return &PostgresStorage{
		ConnStr: connection,
		pool:    pool,
	}, nil
}

func (s *PostgresStorage) Ping(ctx context.Context) error {
	err := s.pool.Ping(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *PostgresStorage) Quit(ctx context.Context) {
	s.pool.Close()
}

func (s *PostgresStorage) AddUser(ctx context.Context, user *model.User) error {
	return nil
}

func (s *PostgresStorage) GetUser(ctx context.Context, user *model.User) error {
	return nil
}

func getInitQuery() string {
	return `
	
	`
}
