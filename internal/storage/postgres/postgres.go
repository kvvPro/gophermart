package postgres

import (
	"context"
	"errors"

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

	//defer pool.Close()

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
	transaction, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)

	addUserQuery := addUserQuery()
	insertRes, err := s.pool.Exec(ctx, addUserQuery, user.Login, user.Password)
	if err != nil {
		return err
	}
	if insertRes.RowsAffected() == 0 {
		return errors.New("can't add user")
	}

	transaction.Commit(ctx)
	return nil
}

func addUserQuery() string {
	return `
	INSERT INTO public.users(login, password)
		VALUES ($1, $2);
	`
}

func (s *PostgresStorage) GetUser(ctx context.Context, user *model.User) (*model.User, error) {
	var userInfo model.User
	getUserQuery := getUserQuery()
	result := s.pool.QueryRow(ctx, getUserQuery, user.Login)
	if err := result.Scan(&userInfo.Login, &userInfo.Password); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

func getUserQuery() string {
	return `
	SELECT login, password
		FROM public.users
	WHERE
		login = $1
	`
}

func getInitQuery() string {
	return `
	-- Table: public.users

	-- DROP TABLE IF EXISTS public.users;

	CREATE TABLE IF NOT EXISTS public.users
	(
		login character varying(50) NOT NULL,
		password character varying,
		CONSTRAINT users_pkey PRIMARY KEY (login)
	)

	TABLESPACE pg_default;

	ALTER TABLE IF EXISTS public.users
		OWNER to postgres;
	`
}
