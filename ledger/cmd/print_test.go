package cmd_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/howeyc/ledger"
	"github.com/howeyc/ledger/ledger/cmd"
	"github.com/shopspring/decimal"
)

func p(d decimal.Decimal) *decimal.Decimal {
	return &d
}

func TestWriteTransaction(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		want    string
		trans   *ledger.Transaction
		columns int
	}{
		{
			name: "test",
			want: `; test comment
1970/01/01 Wise Charges for: BALANCE
    assets:wise                                                        EUR -8.00
    expenses:bank:fees                                                  EUR 8.00

`,
			trans: &ledger.Transaction{
				Payee:    "Wise Charges for: BALANCE",
				Date:     time.Unix(0, 0).UTC(),
				Comments: []string{"; test comment"},
				AccountChanges: []ledger.Account{
					{
						Name:     "assets:wise",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(-8.0),
					},
					{
						Name:     "expenses:bank:fees",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(8.0),
					},
				},
			},
			columns: 80,
		},
		{
			name: "with exchange rate",
			want: `1970/01/01 Converted CZK to EUR
    Assets:Wise:CZK                                          -2000.00 @@ 1000.00
    Assets:Wise:EUR                                                      1000.00

`,
			trans: &ledger.Transaction{
				Payee: "Converted CZK to EUR",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []ledger.Account{
					{
						Name:      "Assets:Wise:CZK",
						Balance:   decimal.NewFromFloat(-2000.0),
						Converted: p(decimal.NewFromFloat(1000)),
					},
					{
						Name:    "Assets:Wise:EUR",
						Balance: decimal.NewFromFloat(1000.0),
					},
				},
			},
			columns: 80,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			cmd.WriteTransaction(&buf, tt.trans, tt.columns)

			got := buf.String()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrintRegister(t *testing.T) {
	tests := []struct {
		name          string // description of this test case
		want          string
		generalLedger []*ledger.Transaction
		filterArr     []string
		columns       int
	}{

		{
			name: "with exchange rate",
			want: `1970/01/01 Converted CZK t Assets:Wise:CZK                   -2000.00   -2000.00
1970/01/01 Converted CZK t Assets:Wise:EUR                    1000.00   -1000.00
1970/01/01 Wise Charges fo assets:wise                      EUR -8.00  EUR -8.00
                                                                        -1000.00
1970/01/01 Wise Charges fo expenses:bank:fees                EUR 8.00   EUR 0.00
                                                                        -1000.00
`,
			generalLedger: []*ledger.Transaction{
				{
					Payee: "Converted CZK to EUR",
					Date:  time.Unix(0, 0).UTC(),
					AccountChanges: []ledger.Account{
						{
							Name:      "Assets:Wise:CZK",
							Balance:   decimal.NewFromFloat(-2000.0),
							Converted: p(decimal.NewFromFloat(1000)),
						},
						{
							Name:    "Assets:Wise:EUR",
							Balance: decimal.NewFromFloat(1000.0),
						},
					},
				},
				{
					Payee:    "Wise Charges for: BALANCE",
					Date:     time.Unix(0, 0).UTC(),
					Comments: []string{"; test comment"},
					AccountChanges: []ledger.Account{
						{
							Name:     "assets:wise",
							Currency: "EUR",
							Balance:  decimal.NewFromFloat(-8.0),
						},
						{
							Name:     "expenses:bank:fees",
							Currency: "EUR",
							Balance:  decimal.NewFromFloat(8.0),
						},
					},
				},
			},
			columns: 80,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			cmd.PrintRegister(&buf, tt.generalLedger, tt.filterArr, tt.columns)

			got := buf.String()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
