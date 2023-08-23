package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kvvPro/gophermart/internal/model"
	"github.com/lib/pq"
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
			return status, nil
		} else {
			status = model.OrderAlreadyUploadedByAnotherUser
			return status, errors.New("номер заказа уже был загружен другим пользователем")
		}
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

func (s *PostgresStorage) GetAllOrders(ctx context.Context, user *model.User) ([]*model.Order, error) {

	orders := []*model.Order{}

	query := getAllOrdersQuery()
	result, err := s.pool.Query(ctx, query, user.Login)
	if err != nil {
		return nil, err
	}

	defer result.Close()

	for result.Next() {
		var orderInfo model.Order
		err = result.Scan(&orderInfo.ID,
			&orderInfo.Owner,
			&orderInfo.UploadDate,
			&orderInfo.Status,
			&orderInfo.Bonus)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &orderInfo)
	}

	err = result.Err()
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func getAllOrdersQuery() string {
	return `
	SELECT orders.id, 
			orders.owner, 
			orders.upload_date, 
			orders.status, 
			orders.bonus
		FROM public.orders as orders
	WHERE
		orders.owner = $1
	ORDER BY
		orders.upload_date ASC
	`
}

func (s *PostgresStorage) GetBalance(ctx context.Context, user *model.User) (*model.Balance, error) {

	var balance model.Balance

	query := getBonusBalanceQuery()
	result := s.pool.QueryRow(ctx, query, user.Login)
	switch err := result.Scan(&balance.Current, &balance.Withdrawn); err {
	case pgx.ErrNoRows:
		// нет ни одной записи в таблицах
		return &model.Balance{
			Current:   0.0,
			Withdrawn: 0.0,
		}, nil
	case nil:
		return &balance, nil
	default:
		return nil, err
	}
}

func (s *PostgresStorage) RequestWithdrawal(ctx context.Context, withdrawalInfo *model.Withdrawal) (model.EndPointStatus, error) {

	var allBonuses float64
	var allWithdrawals float64

	query := getBonusBalanceQuery()
	result := s.pool.QueryRow(ctx, query, withdrawalInfo.User)
	switch err := result.Scan(&allBonuses, &allWithdrawals); err {
	case pgx.ErrNoRows:
		// бонусов нет
		return model.WithdrawalNoBonuses, nil
	case nil:
		// бонусы есть
		// проверим хватит ли их для списания в счет заказа
		availableBonuses := allBonuses - allWithdrawals
		if availableBonuses < withdrawalInfo.Sum {
			// бонусов не хватает
			return model.WithdrawalNotEnoughBonuses, nil
		} else {
			// проверим, нет ли уже списаний по этому заказу
			queryCheck := getWithdrawalInfoQuery()
			result := s.pool.QueryRow(ctx, queryCheck, withdrawalInfo.OrderID)
			switch err := result.Scan(); err {
			case pgx.ErrNoRows:
				// списаний по этому заказу нет
				// добавляем новое списание
				insert := getAddWithdrawalQuery()
				insertRes, err := s.pool.Exec(ctx, insert, withdrawalInfo.OrderID,
					withdrawalInfo.Sum, withdrawalInfo.ProcessedDate, withdrawalInfo.User)
				if err != nil {
					return model.OtherError, err
				}
				if insertRes.RowsAffected() == 0 {
					return model.OtherError, errors.New("списание не прошло")
				}
				return model.WithdrawalAccepted, nil
			case nil:
				// списания есть - запрещаем повторное списание
				return model.WithdrawalAlreadyRequested, nil
			default:
				return model.OtherError, err
			}
		}
	default:
		return model.OtherError, err
	}
}

func getBonusBalanceQuery() string {
	return `
	SELECT
		SUM(balance.bonuses),
		SUM(balance.withdrawals)
	FROM
		(SELECT orders.bonus as bonuses, 
				0 as withdrawals,
				orders.owner as user
		FROM public.orders as orders
		WHERE 
			orders.owner = $1
		UNION ALL
		SELECT 0, 
			withdrawals.sum,
			withdrawals.user_id
		FROM public.withdrawals as withdrawals
		WHERE 
			withdrawals.user_id = $1) as balance
	GROUP BY
		balance.user
	`
}

func getWithdrawalInfoQuery() string {
	return `
	SELECT withdrawals.order_id, 
			withdrawals.sum, 
			withdrawals.processed_date,
			withdrawals.user_id
	FROM public.withdrawals as withdrawals
	WHERE 
		withdrawals.order = $1
	`
}

func getAddWithdrawalQuery() string {
	return `
	INSERT INTO public.withdrawals(
		order_id, sum, processed_date, user_id)
		VALUES ($1, $2, $3, $4);
	`
}

func (s *PostgresStorage) GetAllWithdrawals(ctx context.Context, user *model.User) ([]*model.Withdrawal, error) {

	withdrawals := []*model.Withdrawal{}

	query := getAllWithdrawalsQuery()
	result, err := s.pool.Query(ctx, query, user.Login)
	if err != nil {
		return nil, err
	}

	defer result.Close()

	for result.Next() {
		var withdrawalInfo model.Withdrawal
		err = result.Scan(&withdrawalInfo.OrderID,
			&withdrawalInfo.Sum,
			&withdrawalInfo.ProcessedDate)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, &withdrawalInfo)
	}

	err = result.Err()
	if err != nil {
		return nil, err
	}

	return withdrawals, nil
}

func getAllWithdrawalsQuery() string {
	return `
	SELECT withdrawals.order_id, 
			withdrawals.sum, 
			withdrawals.processed_date
	FROM public.withdrawals as withdrawals
		INNER JOIN public.orders as orders 
		ON orders.id = withdrawals.order_id
	WHERE 
		orders.owner = $1
	ORDER BY
		withdrawals.processed_date ASC
	`
}

func (s *PostgresStorage) GetOrdersForUpdate(ctx context.Context) ([]*model.Order, error) {

	orders := []*model.Order{}
	statusesForUpdate := StatusesForUpdate()

	query := getOrdersForUpdateQuery()
	result, err := s.pool.Query(ctx, query, pq.Array(statusesForUpdate))
	if err != nil {
		return nil, err
	}

	defer result.Close()

	for result.Next() {
		var orderInfo model.Order
		err = result.Scan(&orderInfo.ID,
			&orderInfo.Owner,
			&orderInfo.UploadDate,
			&orderInfo.Status,
			&orderInfo.Bonus)
		if err != nil {
			return nil, err
		}
		orders = append(orders, &orderInfo)
	}

	err = result.Err()
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func StatusesForUpdate() []string {
	return []string{
		model.OrderStatusNew,
		model.OrderStatusProcessing,
		model.BonusStatusNew, // под вопросом
	}
}

func getOrdersForUpdateQuery() string {
	return `
	SELECT orders.id, 
			orders.owner, 
			orders.upload_date, 
			orders.status, 
			orders.bonus
		FROM public.orders as orders
	WHERE
		orders.status=ANY($1)
	ORDER BY
		orders.upload_date ASC
	`
}

func (s *PostgresStorage) UpdateBatchOrders(ctx context.Context, orders []*model.Order) error {

	transaction, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)

	for _, el := range orders {
		err = s.updateOrder(ctx, el)
		if err != nil {
			return err
		}
	}

	transaction.Commit(ctx)

	return nil
}

func (s *PostgresStorage) updateOrder(ctx context.Context, order *model.Order) error {
	update := getUpdateOrderQuery()
	insertRes, err := s.pool.Exec(ctx, update, order.Status, order.Bonus, order.ID)
	if err != nil {
		return err
	}
	if insertRes.RowsAffected() == 0 {
		return errors.New("order not updated")
	}
	return nil
}

func getUpdateOrderQuery() string {
	return `
	UPDATE public.orders
		SET status=$1, bonus=$2
		WHERE id=$3;
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
		upload_date timestamp with time zone NOT NULL,
		status character varying NOT NULL,
		bonus double precision NOT NULL,
		CONSTRAINT orders_pkey PRIMARY KEY (id),
		CONSTRAINT fk_users FOREIGN KEY (owner)
			REFERENCES public.users (login) MATCH SIMPLE
			ON UPDATE NO ACTION
			ON DELETE CASCADE
			NOT VALID
	)

	TABLESPACE pg_default;

	ALTER TABLE IF EXISTS public.orders
		OWNER to postgres;

	-- Table: public.withdrawals

	-- DROP TABLE IF EXISTS public.withdrawals;

	CREATE TABLE IF NOT EXISTS public.withdrawals
	(
		order_id character varying NOT NULL,
		sum double precision NOT NULL,
		processed_date timestamp with time zone NOT NULL,
		user_id character varying NOT NULL,
		CONSTRAINT fk_orders FOREIGN KEY (order_id)
			REFERENCES public.orders (id) MATCH SIMPLE
			ON UPDATE NO ACTION
			ON DELETE CASCADE,
		CONSTRAINT fk_users FOREIGN KEY (user_id)
			REFERENCES public.users (login) MATCH SIMPLE
			ON UPDATE NO ACTION
			ON DELETE CASCADE
			NOT VALID
	)

	TABLESPACE pg_default;

	ALTER TABLE IF EXISTS public.withdrawals
		OWNER to postgres;
	`
}
