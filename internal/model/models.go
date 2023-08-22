package model

import "time"

type User struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Order struct {
	ID         string    `json:"number"`
	Status     string    `json:"status"`
	Bonus      int64     `json:"accrual"`
	UploadDate time.Time `json:"uploaded_at"`
	Owner      string    // user login, who uploaded this order
}

const (
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusInvalid    = "INVALID"
	OrderStatusProcessed  = "PROCESSED"
)

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Transaction struct {
	OrderID       string    `json:"order"`
	Sum           float64   `json:"sum"`
	ProcessedDate time.Time `json:"processed_at,omitempty"`
}

type OrderBonus struct {
	ID      string `json:"order"`
	Status  string `json:"status"`
	Accrual int64  `json:"accrual"`
}

const (
	BonusStatusNew        = "REGISTERED" // заказ зарегистрирован, но вознаграждение не рассчитано
	BonusStatusInvalid    = "INVALID"    // заказ не принят к расчёту, и вознаграждение не будет начислено
	BonusStatusProcessing = "PROCESSING" // расчёт начисления в процессе
	BonusStatusProcessed  = "PROCESSED"  // расчёт начисления окончен
)

type EndPointStatus int

const (
	OrderUnknownStatus EndPointStatus = iota - 1
	OrderAlreadyUploaded
	OrderAcceptedToProcessing
	OrderAlreadyUploadedByAnotherUser
	InvalidOrderID
	ConnectionError
	OtherError
)
