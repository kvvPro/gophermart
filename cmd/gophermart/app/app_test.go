package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"

	"github.com/kvvPro/gophermart/cmd/gophermart/config"
	"github.com/kvvPro/gophermart/internal/model"
	"github.com/kvvPro/gophermart/internal/storage/postgres"
)

func TestNewServer(t *testing.T) {
	// start comtainer
	commonNetwork := "gophermart_devcontainer_default"
	ctx := context.Background()
	logger, err := zap.NewDevelopment()
	if err != nil {
		// вызываем панику, если ошибка
		panic(err)
	}
	defer logger.Sync()
	Sugar = *logger.Sugar()
	dbname := "postgres"
	dbUser := "postgres"
	dbPassword := "postgres"
	dbhost := "localhost"
	port := 5432
	dbConn := fmt.Sprintf("user=postgres password=postgres host=%v port=%v dbname=postgres sslmode=disable",
		dbhost, port)
	req := testcontainers.ContainerRequest{
		Name:         "db_postgres",
		Image:        "postgres:latest",
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections"),
		SkipReaper:   true,
		Env: map[string]string{
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPassword,
			"POSTGRES_DB":       dbname,
			"POSTGRES_HOSTNAME": dbhost,
		},
		Networks: []string{
			commonNetwork,
		},
	}
	postgresC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		if err := postgresC.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err.Error())
		}
	}()

	ip, err := postgresC.Host(ctx)
	if err != nil {
		t.Error(err)
		return
	}

	mappedPort, err := postgresC.MappedPort(ctx, "5432")
	if err != nil {
		t.Error(err)
		return
	}

	t.Logf("container run on %v:%v", ip, mappedPort.Port())
	st, err := postgres.NewPSQLStorage(ctx, dbConn)
	if err != nil {
		t.Errorf(errors.New("cannot create storage for server" + err.Error()).Error())
		return
	}

	type args struct {
		ctx     context.Context
		configs *config.ServerFlags
	}
	test := struct {
		args    args
		want    *Server
		wantErr bool
	}{
		args: args{
			context.Background(),
			&config.ServerFlags{
				Address:                "localhost:8080",
				DBConnection:           dbConn,
				AccrualSystemAddress:   "-",
				ReadingAccrualInterval: 5,
				UpdateThreadCount:      2,
			},
		},
		want: &Server{
			storage:                st,
			Address:                "localhost:8080",
			DBConnection:           dbConn,
			AccrualSystemAddress:   "-",
			ReadingAccrualInterval: 5,
			UpdateThreadCount:      2,
		},
		wantErr: false,
	}
	// create environment with psql
	newSrv, err := NewServer(test.args.ctx, test.args.configs)

	if (err != nil) != test.wantErr {
		t.Errorf("NewServer() error = %v, wantErr %v", err, test.wantErr)
		return
	}
	// compare all prop exclude storage
	if newSrv.Address != test.want.Address {
		t.Errorf("Address actual: %v, expected %v", newSrv.Address, test.want.Address)
		return
	}
	if newSrv.AccrualSystemAddress != test.want.AccrualSystemAddress {
		t.Errorf("AccrualSystemAddress actual: %v, expected %v",
			newSrv.AccrualSystemAddress, test.want.AccrualSystemAddress)
		return
	}
	if newSrv.DBConnection != test.want.DBConnection {
		t.Errorf("DBConnection actual: %v, expected %v", newSrv.DBConnection, test.want.DBConnection)
		return
	}
	if newSrv.ReadingAccrualInterval != test.want.ReadingAccrualInterval {
		t.Errorf("ReadingAccrualInterval actual: %v, expected %v",
			newSrv.ReadingAccrualInterval, test.want.ReadingAccrualInterval)
		return
	}
	if newSrv.UpdateThreadCount != test.want.UpdateThreadCount {
		t.Errorf("UpdateThreadCount actual: %v, expected %v",
			newSrv.UpdateThreadCount, test.want.UpdateThreadCount)
		return
	}

	// run server
	wg := &sync.WaitGroup{}
	wg.Add(1)
	httpSrv := newSrv.StartServer(ctx, wg, test.args.configs)

	// test endpoints

	client := resty.New()

	<-time.After(time.Second * 5)

	// ping
	t.Run("ping", func(t *testing.T) {
		response, err := client.SetBaseURL("http://" + newSrv.Address).R().Get("/ping")
		if err != nil {
			t.Errorf("error from response %v %v: %v", "GET", "/ping", err.Error())
		}
		if response.StatusCode() != http.StatusOK {
			t.Errorf("another response status code actual: %v expected: %v", response.StatusCode(), http.StatusOK)
		}
	})

	// register
	users := []struct {
		name       string
		login      string
		password   string
		want       *model.User
		wantErr    bool
		wantStatus int
	}{
		{
			name:     "reg_user1",
			login:    "user1",
			password: "1",
			want: &model.User{
				Login:    "user1",
				Password: "1",
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:     "reg_user2",
			login:    "user2",
			password: "2",
			want: &model.User{
				Login:    "user2",
				Password: "2",
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "reg_user_double",
			login:      "user2",
			password:   "21",
			want:       nil,
			wantErr:    true,
			wantStatus: http.StatusConflict,
		},
	}

	for _, user := range users {
		t.Run(user.name, func(t *testing.T) {
			reqBody := []byte(`
				{
					"login": "` + user.login + `",
					"password": "` + user.password + `"
				}
			`)
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "application/json").
				SetBody(reqBody).Post("/api/user/register")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "POST", "/api/user/register", err.Error())
			}
			if response.StatusCode() != user.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), user.wantStatus, string(response.Body()))
			}
			auth := response.Header().Get("Authorization")
			if response.StatusCode() == http.StatusOK && auth == "" {
				t.Errorf("authorization token not found")
			}
		})
	}

	// auth
	usersAuth := []struct {
		name       string
		login      string
		password   string
		want       *model.User
		wantErr    bool
		wantStatus int
		token      string
	}{
		{
			name:     "auth_user1",
			login:    "user1",
			password: "1",
			want: &model.User{
				Login:    "user1",
				Password: "1",
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "auth_user2",
			login:      "user2",
			password:   "2",
			want:       nil,
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "auth_user2_fail",
			login:      "user2",
			password:   "4",
			want:       nil,
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, user := range usersAuth {
		t.Run(user.name, func(t *testing.T) {
			reqBody := []byte(`
				{
					"login": "` + user.login + `",
					"password": "` + user.password + `"
				}
			`)
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "application/json").
				SetBody(reqBody).Post("/api/user/login")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "POST", "/api/user/login", err.Error())
			}
			if response.StatusCode() != user.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), user.wantStatus, string(response.Body()))
			}
			token := response.Header().Get("Authorization")
			if response.StatusCode() == http.StatusOK && token == "" {
				t.Errorf("authorization token not found")
			}
		})
	}

	user1Token, err := getUserToken(client, newSrv, usersAuth[0].login, usersAuth[0].password)
	if err != nil {
		t.Errorf("error from response %v %v: %v", "POST", "/api/user/login", err.Error())
	}
	user2Token, err := getUserToken(client, newSrv, usersAuth[1].login, usersAuth[1].password)
	if err != nil {
		t.Errorf("error from response %v %v: %v", "POST", "/api/user/login", err.Error())
	}

	// post order
	ordersUpload := []struct {
		name       string
		token      string
		order      *model.Order
		wantErr    bool
		wantStatus int
	}{
		{
			name:  "order_user1_new",
			token: user1Token,
			order: &model.Order{
				ID: "2000000000008",
			},
			wantErr:    false,
			wantStatus: http.StatusAccepted,
		},
		{
			name:  "order_user1_uploaded_twice",
			token: user1Token,
			order: &model.Order{
				ID: "2000000000008",
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:  "order_user2_new",
			token: user2Token,
			order: &model.Order{
				ID: "1000000000009",
			},
			wantErr:    false,
			wantStatus: http.StatusAccepted,
		},
		{
			name:  "order_user2_already_uploaded",
			token: user2Token,
			order: &model.Order{
				ID: "2000000000008",
			},
			wantErr:    true,
			wantStatus: http.StatusConflict,
		},
		{
			name:  "order_user1_invalid_order",
			token: user1Token,
			order: &model.Order{
				ID: "11110000",
			},
			wantErr:    true,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:  "order_user1_unathorized",
			token: "-",
			order: &model.Order{
				ID: "11110000",
			},
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, el := range ordersUpload {
		t.Run(el.name, func(t *testing.T) {
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "text/plain").
				SetHeader("Authorization", el.token).
				SetBody(el.order.ID).Post("/api/user/orders")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "POST", "/api/user/orders", err.Error())
			}
			if response.StatusCode() != el.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), el.wantStatus, string(response.Body()))
			}
		})
	}

	// get orders
	getOrders := []struct {
		name       string
		token      string
		wantErr    bool
		wantStatus int
	}{
		{
			name:       "orders_user1",
			token:      user1Token,
			wantErr:    false,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "orders_user2",
			token:      user2Token,
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "orders_user1_unathorized",
			token:      "-",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, el := range getOrders {
		t.Run(el.name, func(t *testing.T) {
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "text/plain").
				SetHeader("Authorization", el.token).
				Get("/api/user/orders")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "GET", "/api/user/orders", err.Error())
			}
			if response.StatusCode() != el.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), el.wantStatus, string(response.Body()))
			}
		})
	}

	// get balance
	getBalance := []struct {
		name          string
		token         string
		wantBalance   float64
		wantWithdrawn float64
		wantErr       bool
		wantStatus    int
	}{
		{
			name:          "balance_user1",
			token:         user1Token,
			wantBalance:   0.0,
			wantWithdrawn: 0.0,
			wantErr:       false,
			wantStatus:    http.StatusOK,
		},
		{
			name:          "balance_user2",
			token:         user2Token,
			wantBalance:   100.0,
			wantWithdrawn: 0.0,
			wantErr:       false,
			wantStatus:    http.StatusOK,
		},
		{
			name:       "balance_user1_unathorized",
			token:      "-",
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, el := range getBalance {
		t.Run(el.name, func(t *testing.T) {
			var balance model.Balance
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "text/plain").
				SetHeader("Authorization", el.token).
				SetResult(&balance).
				Get("/api/user/balance")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "GET", "/api/user/balance", err.Error())
			}
			if response.StatusCode() != el.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), el.wantStatus, string(response.Body()))
			}
			if balance.Current != el.wantBalance {
				t.Errorf("invalid current balance - actual: %v expected: %v", balance.Current, el.wantBalance)
			}
			if balance.Withdrawn != el.wantWithdrawn {
				t.Errorf("invalid withdrawn sum - actual: %v expected: %v", balance.Withdrawn, el.wantWithdrawn)
			}
		})
	}

	// withdraw
	postWithdrawals := []struct {
		name       string
		token      string
		withdraw   *model.Withdrawal
		wantErr    bool
		wantStatus int
	}{
		{
			name:  "withdraw_user2_ok",
			token: user2Token,
			withdraw: &model.Withdrawal{
				OrderID: "1000000000009",
				Sum:     20,
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:  "withdraw_user2_no_bonuses",
			token: user2Token,
			withdraw: &model.Withdrawal{
				OrderID: "1000000000009",
				Sum:     100000,
			},
			wantErr:    true,
			wantStatus: http.StatusPaymentRequired,
		},
		{
			name:  "withdraw_user1_invalid_order",
			token: user1Token,
			withdraw: &model.Withdrawal{
				OrderID: "333333333",
				Sum:     1,
			},
			wantErr:    true,
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:  "withdraw_user1_unathorized",
			token: "-",
			withdraw: &model.Withdrawal{
				OrderID: "1000000000009",
				Sum:     100000,
			},
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, el := range postWithdrawals {
		t.Run(el.name, func(t *testing.T) {
			body := []byte(`{
				"order": "` + el.withdraw.OrderID + `",
				"sum": ` + strconv.FormatFloat(float64(el.withdraw.Sum), 'f', 2, 32) + `
			}`)
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "application/json").
				SetHeader("Authorization", el.token).
				SetBody(body).Post("/api/user/balance/withdraw")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "POST", "/api/user/balance/withdraw", err.Error())
			}
			if response.StatusCode() != el.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), el.wantStatus, string(response.Body()))
			}
		})
	}

	// get withdrawals
	getWithdrawals := []struct {
		name       string
		token      string
		withdraw   *model.Withdrawal
		wantErr    bool
		wantStatus int
	}{
		{
			name:  "withdrawal_user2_ok",
			token: user2Token,
			withdraw: &model.Withdrawal{
				OrderID:       "1000000000009",
				Sum:           20,
				ProcessedDate: time.Date(2023, time.September, 5, 20, 59, 37, 716467000, time.Local),
				//"2023-09-05T20:59:37.716467+00"
			},
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "withdrawal_user1_no_withdrawals",
			token:      user1Token,
			wantErr:    true,
			wantStatus: http.StatusNoContent,
		},
		{
			name:  "withdrawal_user1_unathorized",
			token: "-",
			withdraw: &model.Withdrawal{
				OrderID: "1000000000009",
				Sum:     100000,
			},
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, el := range getWithdrawals {
		t.Run(el.name, func(t *testing.T) {
			var withdrawals []model.Withdrawal
			response, err := client.SetBaseURL("http://"+newSrv.Address).
				R().SetHeader("Content-Type", "application/json").
				SetHeader("Authorization", el.token).
				SetResult(&withdrawals).
				Get("/api/user/withdrawals")
			if err != nil {
				t.Errorf("error from response %v %v: %v", "POST", "/api/user/withdrawals", err.Error())
			}
			if response.StatusCode() != el.wantStatus {
				t.Errorf("another response status code actual: %v expected: %v; response: %v",
					response.StatusCode(), el.wantStatus, string(response.Body()))
			}
			if !el.wantErr {
				if len(withdrawals) != 1 {
					t.Errorf("invalid count of withdrawals actual: %v expected: %v", len(withdrawals), 1)
				}
				if withdrawals[0].OrderID != el.withdraw.OrderID ||
					withdrawals[0].Sum != el.withdraw.Sum ||
					!withdrawals[0].ProcessedDate.Equal(el.withdraw.ProcessedDate) {
					t.Errorf("invalid response actual: %v expected: %v", withdrawals[0], el.withdraw)
				}
			}

		})
	}

	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(timeout); err != nil {
		t.Errorf("Ошибка при попытке мягко завершить http-сервер: %v", err)
		// handle err
		if err = httpSrv.Close(); err != nil {
			t.Errorf("Ошибка при попытке завершить http-сервер: %v", err)
		}
	}
}

func getUserToken(client *resty.Client, newSrv *Server, login string, password string) (string, error) {
	reqBody := []byte(`
				{
					"login": "` + login + `",
					"password": "` + password + `"
				}
			`)
	response, err := client.SetBaseURL("http://"+newSrv.Address).
		R().SetHeader("Content-Type", "application/json").
		SetBody(reqBody).Post("/api/user/login")
	if err != nil {
		return "", fmt.Errorf("error from response %v %v: %v", "POST", "/api/user/login", err.Error())
	}
	return response.Header().Get("Authorization"), nil
}
