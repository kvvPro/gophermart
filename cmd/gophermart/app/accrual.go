package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kvvPro/gophermart/internal/model"
)

func (srv *Server) RequestAccrual(ctx context.Context, order model.Order) (*model.Order, bool) {

	client := &http.Client{}
	url := srv.AccrualSystemAddress + "/api/orders/%v"

	localURL := fmt.Sprintf(url, order.ID)
	bodyBuffer := new(bytes.Buffer)
	request, err := http.NewRequest(http.MethodGet, localURL, bodyBuffer)
	if err != nil {
		Sugar.Infoln("Error request: ", err.Error())
		return nil, false
	}
	request.Header.Set("Connection", "Keep-Alive")
	response, err := client.Do(request)
	if err != nil {
		Sugar.Infoln("Error response: ", err.Error())
		return nil, false
	}
	defer response.Body.Close()

	dataResponse, err := io.ReadAll(response.Body)
	if err != nil {
		Sugar.Infoln("Error reading response body: ", err.Error())
		return nil, false
	}

	var newInfo model.Order
	Sugar.Infoln("-----------NEW REQUEST---------------")
	Sugar.Infoln(
		"uri", request.RequestURI,
		"method", request.Method,
		"status", response.Status, // получаем код статуса ответа
	)
	Sugar.Infoln("response-from-accrual: ", string(dataResponse))

	reader := io.NopCloser(bytes.NewReader(dataResponse))
	if err := json.NewDecoder(reader).Decode(&newInfo); err != nil {
		Sugar.Infoln("Error to parse response body")
		return nil, false
	}

	// анализируем ответы
	if response.StatusCode == http.StatusOK {
		// обновляем данные
		newInfo.ID = order.ID
		newInfo.Owner = order.Owner
		newInfo.UploadDate = order.UploadDate
		return &newInfo, true
	} else if response.StatusCode == http.StatusNoContent {
		// данных по заказу нет - можно не обновлять
		return nil, false
	} else if response.StatusCode == http.StatusTooManyRequests {
		// надо подождать и попробовать заново через Retry-After
		return nil, false
	} else {
		// любые другие ошибки - просто пропускаем попытку
		return nil, false
	}
}
