package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
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

func (s *PostgresStorage) UploadOrder(ctx context.Context, orderID string, user *model.User) (model.EndPointStatus, error) {

	var orderInfo model.Order
	var status model.EndPointStatus

	query := getOrderInfoQuery()
	result := s.pool.QueryRow(ctx, query, orderID)
	switch err := result.Scan(&orderInfo.ID,
		&orderInfo.Owner,
		&orderInfo.UploadDate,
		&orderInfo.Status,
		&orderInfo.Bonus); err {
	case pgx.ErrNoRows:
		// заказа нет - создаем новый
		insert := getAddOrderQuery()
		insertRes, err := s.pool.Exec(ctx, insert, orderID,
			user.Login, time.Now(), model.OrderStatusNew, 0.0)
		if err != nil {
			status = model.OtherError
			return status, err
		}
		if insertRes.RowsAffected() == 0 {
			status = model.OtherError
			return status, errors.New("order not uploaded")
		}
		status = model.OrderAcceptedToProcessing
		return status, nil
	case nil:
		// заказ есть - проверим, кем был загружен
		// автор заказа тот же
		if orderInfo.Owner == user.Login {
			status = model.OrderAlreadyUploaded
		}
		if orderInfo.Owner != user.Login {
			status = model.OrderAlreadyUploadedByAnotherUser
		}
		return status, nil
	default:
		return model.OtherError, err
	}
}

func getOrderInfoQuery() string {
	return `
	SELECT orders.id,
			orders.owner, 
			orders.upload_date, 
			orders.status, 
			orders.bonus
	FROM public.orders AS orders 
		LEFT JOIN public.users AS users 
	ON orders.owner = users.login
	WHERE
	orders.id = $1
	`
}

func getAddOrderQuery() string {
	return `
	INSERT INTO public.orders(
		id, owner, upload_date, status, bonus)
		VALUES ($1, $2, $3, $4, $5);
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

	-- Table: public.orders

	-- DROP TABLE IF EXISTS public.orders;

	CREATE TABLE IF NOT EXISTS public.orders
	(
		id character varying NOT NULL,
		owner character varying NOT NULL,
		upload_date date NOT NULL,
		status character varying NOT NULL,
		bonus double precision NOT NULL,
		CONSTRAINT orders_pkey PRIMARY KEY (id),
		CONSTRAINT fk_users FOREIGN KEY (owner)
			REFERENCES public.users (login) MATCH SIMPLE
			ON UPDATE NO ACTION
			ON DELETE NO ACTION
			NOT VALID
	)

	TABLESPACE pg_default;

	ALTER TABLE IF EXISTS public.orders
		OWNER to postgres;
	`
}
