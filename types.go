package ledger

import (
	"time"

	"github.com/shopspring/decimal"
)

// Account holds the name and balance
type Account struct {
	Name string
	// Default "" for no currency/token displayed
	Currency string
	Balance  decimal.Decimal
	Comment  string

	// Balance converted using @@ notation. This is the total price of the
	// posting expressed in ConvertedCurrency, always stored as a positive
	// magnitude (standard ledger/hledger semantics); the cost takes the sign
	// of Balance.
	Converted *decimal.Decimal
	// Currency of the Converted amount (the "@@ <currency> <amount>" token).
	// May be "" if the source omitted it.
	ConvertedCurrency string
	// Conversion factor (per-unit price) using @ notation
	ConversionFactor *decimal.Decimal
	// Currency of the ConversionFactor (the "@ <currency> <rate>" token).
	// May be "" if the source omitted it.
	ConversionFactorCurrency string
	// Balance assertion using = notation
	BalanceAssert *decimal.Decimal
}

// Transaction is the basis of a ledger. The ledger holds a list of transactions.
// A Transaction has a Payee, Date (with no time, or to put another way, with
// hours,minutes,seconds values that probably doesn't make sense), and a list of
// Account values that hold the value of the transaction for each account.
type Transaction struct {
	Date           time.Time
	Payee          string
	PayeeComment   string
	AccountChanges []Account
	Comments       []string
}
