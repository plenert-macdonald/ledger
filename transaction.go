package ledger

import (
	"errors"

	"github.com/shopspring/decimal"
)

var (
	ErrNeedAtLeastTwoPostings          = errors.New("need at least two postings")
	ErrNoEmptyAccountForExtraBalance   = errors.New("unable to balance transaction: no empty account to place extra balance")
	ErrMoreThanOneEmptyAccountInTx     = errors.New("unable to balance transaction: more than one account empty")
)

// IsBalanced returns nil if the transaction is balanced to 0, otherwise an error.
func (t *Transaction) IsBalanced() error {
	if len(t.AccountChanges) < 2 {
		return ErrNeedAtLeastTwoPostings
	}

	transBal := decimal.Zero
	var numEmpty int
	var emptyAccIndex int

	for i, acc := range t.AccountChanges {
		if acc.Balance.IsZero() {
			numEmpty++
			emptyAccIndex = i
		}

		if acc.Converted != nil {
			transBal = transBal.Add(acc.Converted.Neg())
		} else if acc.ConversionFactor != nil {
			transBal = transBal.Add(acc.Balance.Mul(*acc.ConversionFactor))
		} else {
			transBal = transBal.Add(acc.Balance)
		}
	}

	if !transBal.IsZero() {
		switch numEmpty {
		case 0:
			return ErrNoEmptyAccountForExtraBalance
		case 1:
			// If there is a single empty account, then it is obvious where to
			// place the remaining balance.
			t.AccountChanges[emptyAccIndex].Balance = transBal.Neg()
		default:
			return ErrMoreThanOneEmptyAccountInTx
		}
	}

	return nil
}
