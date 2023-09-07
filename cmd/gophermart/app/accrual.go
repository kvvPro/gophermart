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

func (srv *Server) RequestAccrual(ctx context.Context, orders []model.Order) ([]model.Order, error) {

	client := &http.Client{}
	url := srv.AccrualSystemAddress + "/api/orders/%v"

	ordersForUpdate := []model.Order{}

	for _, el := range orders {
		localURL := fmt.Sprintf(url, el.ID)
		bodyBuffer := new(bytes.Buffer)
		request, err := http.NewRequest(http.MethodGet, localURL, bodyBuffer)
		if err != nil {
			Sugar.Infoln("Error request: ", err.Error())
			continue
		}
		request.Header.Set("Connection", "Keep-Alive")
		response, err := client.Do(request)
		if err != nil {
			Sugar.Infoln("Error response: ", err.Error())
			continue
		}
		defer response.Body.Close()

		dataResponse, err := io.ReadAll(response.Body)
		if err != nil {
			Sugar.Infoln("Error reading response body: ", err.Error())
			continue
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
			continue
		}

		// анализируем ответы
		if response.StatusCode == http.StatusOK {
			// обновляем данные
			newInfo.ID = el.ID
			newInfo.Owner = el.Owner
			newInfo.UploadDate = el.UploadDate
			ordersForUpdate = append(ordersForUpdate, newInfo)
		} else if response.StatusCode == http.StatusNoContent {
			// данных по заказу нет - можно не обновлять
			continue
		} else if response.StatusCode == http.StatusTooManyRequests {
			// надо подождать и попробовать заново через Retry-After
			continue
		} else {
			// любые другие ошибки - просто пропускаем попытку
			continue
		}
	}

	return ordersForUpdate, nil
}
