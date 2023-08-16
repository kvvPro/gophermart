package app

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

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

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) Auth(w http.ResponseWriter, r *http.Request) {

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) PutOrders(w http.ResponseWriter, r *http.Request) {

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) GetOrders(w http.ResponseWriter, r *http.Request) {

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) GetBalance(w http.ResponseWriter, r *http.Request) {

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) Withdraw(w http.ResponseWriter, r *http.Request) {

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}

func (srv *Server) GetWithdrawals(w http.ResponseWriter, r *http.Request) {

	testbody := "OK!"
	io.WriteString(w, testbody)
	w.WriteHeader(http.StatusOK)
}
