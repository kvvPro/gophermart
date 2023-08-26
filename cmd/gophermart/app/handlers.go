package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/kvvPro/gophermart/cmd/gophermart/auth"
	"github.com/kvvPro/gophermart/internal/luhn"
	"github.com/kvvPro/gophermart/internal/model"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

type ctxKey string

func (srv *Server) PingHandle(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := srv.Ping(ctx)
	if err != nil {
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	body := "OK!"
	io.WriteString(w, body)
}

func (srv *Server) Register(w http.ResponseWriter, r *http.Request) {

	var user model.User

	data, err := io.ReadAll(r.Body)
	if err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	reader := io.NopCloser(bytes.NewReader(data))
	if err := json.NewDecoder(reader).Decode(&user); err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = srv.AddUser(r.Context(), &user)
	if err != nil {
		var pgErr *pgconn.PgError
		// check that user already exists (duplicated login)
		if errors.As(err, &pgErr) && pgerrcode.UniqueViolation == pgErr.Code {
			Sugar.Errorf("логин уже занят: %v", err.Error())
			http.Error(w, "логин уже занят: "+err.Error(), http.StatusConflict)
			return
		}
		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			Sugar.Errorf("внутренняя ошибка сервера: %v", err.Error())
			http.Error(w, "внутренняя ошибка сервера: "+err.Error(), http.StatusInternalServerError)
			return
		}
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// generate auth token
	token, err := auth.BuildJWTString(user.Login, user.Password)
	if err != nil {
		Sugar.Errorf("ошибка при генерации токена: %v", err.Error())
		http.Error(w, "ошибка при генерации токена: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Authorization", "Bearer "+token)

	body := "OK!"
	io.WriteString(w, body)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) Auth(w http.ResponseWriter, r *http.Request) {

	var user model.User

	data, err := io.ReadAll(r.Body)
	if err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	reader := io.NopCloser(bytes.NewReader(data))
	if err := json.NewDecoder(reader).Decode(&user); err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	userInfo, err := srv.GetUser(r.Context(), &user)
	// authentication failed, password is invalid
	// or login wasn't found
	if err != nil {
		var pgErr *pgconn.PgError

		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			Sugar.Error(err.Error())
			http.Error(w, "внутренняя ошибка сервера: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// other errros
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// проверим пароль пользователя
	if userInfo == nil || userInfo.Password != user.Password {
		http.Error(w, "неверная пара логин/пароль: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// get token
	token, err := auth.BuildJWTString(userInfo.Login, userInfo.Password)
	if err != nil {
		Sugar.Errorf("ошибка при генерации токена: %v", err.Error())
		http.Error(w, "ошибка при генерации токена: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Authorization", "Bearer "+token)

	body := "OK!"
	io.WriteString(w, body)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) PutOrder(w http.ResponseWriter, r *http.Request) {

	userInfo, _ := r.Context().Value(ctxKey("userInfo")).(*model.User)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	orderID := string(data)
	err = luhn.Validate(orderID)
	if err != nil {
		Sugar.Errorf("неверный формат номера заказа: %v", err.Error())
		http.Error(w, "неверный формат номера заказа: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	status, err := srv.UploadOrder(r.Context(), orderID, userInfo)
	if err != nil {
		// other errros
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if status == model.OrderAlreadyUploadedByAnotherUser {
		Sugar.Error("номер заказа уже был загружен другим пользователем")
		http.Error(w, "номер заказа уже был загружен другим пользователем", http.StatusConflict)
		return
	}
	if status == model.OrderAlreadyUploaded {
		w.WriteHeader(http.StatusOK)
		body := "номер заказа уже был загружен этим пользователем"
		io.WriteString(w, body)
	} else if status == model.OrderAcceptedToProcessing {
		w.WriteHeader(http.StatusAccepted)
		body := "новый номер заказа принят в обработку"
		io.WriteString(w, body)
	}
}

func (srv *Server) GetOrders(w http.ResponseWriter, r *http.Request) {

	userInfo, _ := r.Context().Value(ctxKey("userInfo")).(*model.User)

	orders, status, err := srv.OrderList(r.Context(), userInfo)
	if err != nil {
		// other errros
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if status == model.OrderListEmpty {
		w.WriteHeader(http.StatusNoContent)
		body := "отсутствуют данные по запросу"
		io.WriteString(w, body)
	} else if status == model.OrderListExists {
		bodyBuffer := new(bytes.Buffer)
		json.NewEncoder(bodyBuffer).Encode(orders)
		body := bodyBuffer.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, body)
	}
}

func (srv *Server) GetBalanceHandle(w http.ResponseWriter, r *http.Request) {

	userInfo, _ := r.Context().Value(ctxKey("userInfo")).(*model.User)

	balance, err := srv.GetBalance(r.Context(), userInfo)
	if err != nil {
		// other errros
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bodyBuffer := new(bytes.Buffer)
	json.NewEncoder(bodyBuffer).Encode(balance)
	body := bodyBuffer.String()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, body)

}

func (srv *Server) Withdraw(w http.ResponseWriter, r *http.Request) {

	var withdrawInfo *model.Withdrawal

	userInfo, _ := r.Context().Value(ctxKey("userInfo")).(*model.User)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	reader := io.NopCloser(bytes.NewReader(data))
	if err := json.NewDecoder(reader).Decode(&withdrawInfo); err != nil {
		Sugar.Errorf("неверный формат запроса: %v", err.Error())
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = luhn.Validate(withdrawInfo.OrderID)
	if err != nil {
		Sugar.Errorf("неверный формат номера заказа: %v", err.Error())
		http.Error(w, "неверный формат номера заказа: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	withdrawInfo.ProcessedDate = time.Now()
	withdrawInfo.User = userInfo.Login

	status, err := srv.RequestWithdrawal(r.Context(), withdrawInfo)
	if err != nil {
		// other errros
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if status == model.WithdrawalAccepted {
		w.WriteHeader(http.StatusOK)
		body := "списание одобрено"
		io.WriteString(w, body)
	} else if status == model.WithdrawalAlreadyRequested {
		w.WriteHeader(http.StatusUnprocessableEntity)
		body := "списание по этому заказу уже осуществлено ранее"
		io.WriteString(w, body)
	} else if status == model.WithdrawalNotEnoughBonuses || status == model.WithdrawalNoBonuses {
		w.WriteHeader(http.StatusPaymentRequired)
		body := "на счету недостаточно средств"
		io.WriteString(w, body)
	}
}

func (srv *Server) GetWithdrawals(w http.ResponseWriter, r *http.Request) {

	userInfo, _ := r.Context().Value(ctxKey("userInfo")).(*model.User)

	withdrawals, status, err := srv.AllWithdrawals(r.Context(), userInfo)
	if err != nil {
		// other errros
		Sugar.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if status == model.WithdrawalsNoData {
		w.WriteHeader(http.StatusNoContent)
		body := "нет ни одного списания"
		io.WriteString(w, body)
	} else if status == model.WithdrawalsDataExists {
		bodyBuffer := new(bytes.Buffer)
		json.NewEncoder(bodyBuffer).Encode(withdrawals)
		body := bodyBuffer.String()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, body)
	}
}
