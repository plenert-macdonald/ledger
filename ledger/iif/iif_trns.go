package iif

import (
	"time"

	"github.com/shopspring/decimal"
)

type Transaction struct {
	Tr     Trns  `type:"TRNS"`
	Splits []Spl `type:"SPL"`
}

type Trns struct {
	TransactionType string          `iif:"TRNSTYPE"`
	Date            time.Time       `iif:"DATE"`
	Account         string          `iif:"ACCNT"`
	Name            string          `iif:"NAME"`
	Class           string          `iif:"CLASS"`
	Amount          decimal.Decimal `iif:"AMOUNT"`
}

type Spl struct {
	TransactionType string          `iif:"TRNSTYPE"`
	Date            time.Time       `iif:"DATE"`
	Account         string          `iif:"ACCNT"`
	Name            string          `iif:"NAME"`
	Class           string          `iif:"CLASS"`
	Amount          decimal.Decimal `iif:"AMOUNT"`
}
