package cmd

import (
	"reflect"
	"testing"

	"github.com/howeyc/ledger"
)

func Test_computeEquity(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		generalLedger []*ledger.Transaction
		filterArr     []string
		want          *ledger.Transaction
	}{
		{
			name: "simple",
			generalLedger: []*ledger.Transaction{
				&ledger.Transaction{
					AccountChanges: []ledger.Account{
						{Name: "Equity:Fake", Balance: 10},
						{Name: "Liability:Real", Balance: -10},
					},
				},
				&ledger.Transaction{
					AccountChanges: []ledger.Account{
						{Name: "Equity:Real", Balance: 10},
						{Name: "Liability:Real", Balance: -10},
					},
				},
			},
			filterArr: []string{"Equity"},
			want: &ledger.Transaction{
				Payee: "Opening Balances",
				AccountChanges: []ledger.Account{
					{Name: "Equity", Balance: -20},
					{Name: "Equity:Fake", Balance: 10},
					{Name: "Equity:Real", Balance: 10},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeEquity(tt.generalLedger, tt.filterArr)
			// TODO: update the condition below to compare got with tt.want.
			if got.Payee != tt.want.Payee || !reflect.DeepEqual(got.AccountChanges, tt.want.AccountChanges) {
				t.Errorf("computeEquity() = %v, want %v", got, tt.want)
			}
		})
	}
}
