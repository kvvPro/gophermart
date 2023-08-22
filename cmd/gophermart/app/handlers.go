package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kvvPro/gophermart/cmd/gophermart/auth"
	"github.com/kvvPro/gophermart/internal/luhn"
	"github.com/kvvPro/gophermart/internal/model"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"go.uber.org/zap"
)

var Sugar zap.SugaredLogger
var ContentTypesForCompress = "application/json; text/html"

type (
	// берём структуру для хранения сведений об ответе
	responseData struct {
		status int
		size   int
	}

	// добавляем реализацию http.ResponseWriter
	loggingResponseWriter struct {
		http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
		responseData        *responseData
	}
)

type ctxKey string

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size // захватываем размер
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
}

func WithLogging(h http.Handler) http.Handler {
	logFn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
			responseData:   responseData,
		}
		h.ServeHTTP(&lw, r) // внедряем реализацию http.ResponseWriter

		duration := time.Since(start)

		Sugar.Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"status", responseData.status, // получаем перехваченный код статуса ответа
			"duration", duration,
			"size", responseData.size, // получаем перехваченный размер ответа
		)
	}
	return http.HandlerFunc(logFn)
}

func GzipMiddleware(h http.Handler) http.Handler {
	compressFunc := func(w http.ResponseWriter, r *http.Request) {
		// по умолчанию устанавливаем оригинальный http.ResponseWriter как тот,
		// который будем передавать следующей функции
		ow := w

		// проверяем, что клиент умеет получать от сервера сжатые данные в формате gzip
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		// enableCompress := strings.Contains(ContentTypesForCompress, w.Header().Get("Content-Type"))
		if supportsGzip {
			// оборачиваем оригинальный http.ResponseWriter новым с поддержкой сжатия
			cw := newCompressWriter(w)
			cw.Header().Set("Content-Encoding", "gzip")
			// меняем оригинальный http.ResponseWriter на новый
			ow = cw
			// не забываем отправить клиенту все сжатые данные после завершения middleware
			defer cw.Close()
		}

		// проверяем, что клиент отправил серверу сжатые данные в формате gzip
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.EqualFold(contentEncoding, "gzip")
		if sendsGzip {
			// оборачиваем тело запроса в io.Reader с поддержкой декомпрессии
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// меняем тело запроса на новое
			r.Body = cr
			defer cr.Close()
		}

		// передаём управление хендлеру
		h.ServeHTTP(ow, r)
	}
	return http.HandlerFunc(compressFunc)
}

func (srv *Server) CheckAuth(h http.Handler) http.Handler {
	authFn := func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.Split(r.Header.Get("Authorization"), "Bearer ")
		if len(authHeader) != 2 {
			http.Error(w, "malformed token", http.StatusUnauthorized)
			return
		}
		token := authHeader[1]
		userInfo, err := auth.GetUserInfo(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		newContext := context.WithValue(r.Context(), ctxKey("userInfo"), userInfo)

		h.ServeHTTP(w, r.WithContext(newContext))
	}
	return http.HandlerFunc(authFn)
}

func (srv *Server) PingHandle(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := srv.Ping(ctx)
	if err != nil {
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
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	reader := io.NopCloser(bytes.NewReader(data))
	if err := json.NewDecoder(reader).Decode(&user); err != nil {
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = srv.AddUser(context.Background(), &user)
	if err != nil {
		var pgErr *pgconn.PgError
		// check that user already exists (duplicated login)
		if errors.As(err, &pgErr) && pgerrcode.UniqueViolation == pgErr.Code {
			http.Error(w, "логин уже занят: "+err.Error(), http.StatusConflict)
			return
		}
		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			http.Error(w, "внутренняя ошибка сервера: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// generate auth token
	token, err := auth.BuildJWTString(user.Login, user.Password)
	if err != nil {
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
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	reader := io.NopCloser(bytes.NewReader(data))
	if err := json.NewDecoder(reader).Decode(&user); err != nil {
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	userInfo, err := srv.CheckUser(context.Background(), &user)
	// authentication failed, password is invalid
	// or login wasn't found
	if err != nil {
		var pgErr *pgconn.PgError

		// if errors.Is(err, pgx.ErrNoRows) || userInfo.Login == user.Login && userInfo.Password == user.Password {
		// 	http.Error(w, "неверная пара логин/пароль: "+err.Error(), http.StatusUnauthorized)
		// 	return
		// }

		// connection problems
		if errors.As(err, &pgErr) && pgerrcode.IsConnectionException(pgErr.Code) {
			http.Error(w, "внутренняя ошибка сервера: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// other errros
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// get token
	token, err := auth.BuildJWTString(userInfo.Login, userInfo.Password)
	if err != nil {
		http.Error(w, "ошибка при генерации токена: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Authorization", "Bearer "+token)

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) PutOrder(w http.ResponseWriter, r *http.Request) {

	userInfo, _ := r.Context().Value(ctxKey("userInfo")).(*model.User)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "неверный формат запроса: "+err.Error(), http.StatusBadRequest)
		return
	}

	orderID := string(data)
	err = luhn.Validate(orderID)
	if err != nil {
		http.Error(w, "неверный формат номера заказа: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	status, err := srv.UploadOrder(context.Background(), orderID, userInfo)
	if err != nil {
		if status == model.OrderAlreadyUploadedByAnotherUser {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		// other errros
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if status == model.OrderAlreadyUploaded {
		w.WriteHeader(http.StatusOK)
		testbody := "номер заказа уже был загружен этим пользователем"
		io.WriteString(w, testbody)
	} else if status == model.OrderAcceptedToProcessing {
		w.WriteHeader(http.StatusAccepted)
		testbody := "новый номер заказа принят в обработку"
		io.WriteString(w, testbody)
	}
}

func (srv *Server) GetOrders(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	testbody := "OK!"
	io.WriteString(w, testbody)
}

func (srv *Server) GetBalance(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)
	testbody := "OK!"
	io.WriteString(w, testbody)
}

func (srv *Server) Withdraw(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)
	testbody := "OK!"
	io.WriteString(w, testbody)
}

func (srv *Server) GetWithdrawals(w http.ResponseWriter, r *http.Request) {

	w.WriteHeader(http.StatusOK)
	testbody := "OK!"
	io.WriteString(w, testbody)
}
