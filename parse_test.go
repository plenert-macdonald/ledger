package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

type testCase struct {
	name         string
	data         string
	transactions []*Transaction
	err          error
}

var testCases = []testCase{
	{
		"simple",
		`1970/01/01 Payee
	Expense/test  (123 * 3)
	Assets
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(369.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-369.0),
					},
				},
			},
		},
		nil,
	},
	{
		"bad payee line",
		`1970/01/01Payee
	Expense/test  (123 * 3)
	Assets      123
`,
		nil,
		errors.New(":1: unable to parse transaction: unable to parse payee line: 1970/01/01Payee"),
	},
	{
		"bad payee date",
		`1970/02/31 Payee
	Expense/test  (123 * 3)
	Assets      123
`,
		nil,
		errors.New(`:1: unable to parse transaction: parsing time "1970/02/31": day out of range`),
	},
	{
		"unbalanced error",
		`1970/01/01 Payee
	Expense/test  (123 * 3)
	Assets      123
`,
		nil,
		errors.New(":3: unable to parse transaction: unable to balance transaction: no empty account to place extra balance"),
	},
	{
		"unbalanced error multicurrency",
		`1970/01/01 Payee
	Expense/test  EUR 100
	Assets      USD 100
	Other       USD -200
	More        EUR -100
`,
		nil,
		errors.New(":5: unable to parse transaction: unable to balance transaction: no empty account to place extra balance"),
	},
	{
		"single posting",
		`1970/01/01 Payee
	Assets:Account    5`,
		nil,
		errors.New(":2: unable to parse transaction: need at least two postings"),
	},
	{
		"no posting",
		`1970/01/01 Payee
`,
		nil,
		errors.New(":1: unable to parse transaction: need at least two postings"),
	},
	{
		"multiple empty",
		`1970/01/01 Payee
	Expense/test  (123 * 3)
	Wallet
	Assets      123
	Bank
`,
		nil,
		errors.New(":5: unable to parse transaction: unable to balance transaction: more than one account empty"),
	},
	{
		"all empty",
		`1970/01/01 Payee
	Expense/test
	Wallet
	Assets
	Bank
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name: "Expense/test",
					},
					{
						Name: "Wallet",
					},
					{
						Name: "Assets",
					},
					{
						Name: "Bank",
					},
				},
			},
		},
		nil,
	},
	{
		"multiple empty lines",
		`1970/01/01 Payee
	Expense/test  (123 * 3)
	Assets



1970/01/01 Payee
	Expense/test   123
	Assets
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(369.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-369.0),
					},
				},
			},
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(123.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-123.0),
					},
				},
			},
		},
		nil,
	},
	{
		"accounts with spaces",
		`1970/01/02 Payee
 Expense:test	369.0
 Assets

; Handle tabs between account and amount
; Also handle accounts with spaces
1970/01/01 Payee 5
	Expense:Cars R Us
	Expense:Cars  358.0
	Expense:Cranks	10
	Expense:Cranks Unlimited	10
	Expense:Cranks United  10
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC().AddDate(0, 0, 1),
				AccountChanges: []Account{
					{
						Name:    "Expense:test",
						Balance: decimal.NewFromFloat(369.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-369.0),
					},
				},
			},
			{
				Payee: "Payee 5",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense:Cars R Us",
						Balance: decimal.NewFromFloat(-388.0),
					},
					{
						Name:    "Expense:Cars",
						Balance: decimal.NewFromFloat(358.0),
					},
					{
						Name:    "Expense:Cranks",
						Balance: decimal.NewFromFloat(10.0),
					},
					{
						Name:    "Expense:Cranks Unlimited",
						Balance: decimal.NewFromFloat(10.0),
					},
					{
						Name:    "Expense:Cranks United",
						Balance: decimal.NewFromFloat(10.0),
					},
				},
				Comments: []string{
					"; Handle tabs between account and amount",
					"; Also handle accounts with spaces",
				},
			},
		},
		nil,
	},
	{
		"accounts with slashes",
		`1970-01-01 Payee
    Expense/another     5
	Expense/test
	Assets      -128
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/another",
						Balance: decimal.NewFromFloat(5.0),
					},
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(123.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-128.0),
					},
				},
			},
		},
		nil,
	},
	{
		"comment after payee",
		`; before trans
1970-01-01 Payee      ; payee comment
	Expense/test  123
	Assets
`,
		[]*Transaction{
			{
				Payee:        "Payee",
				Date:         time.Unix(0, 0).UTC(),
				PayeeComment: "; payee comment",
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(123.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-123.0),
					},
				},
				Comments: []string{
					"; before trans",
				},
			},
		},
		nil,
	},
	{
		"comment inside transaction",
		`1970-01-01 Payee
	Expense/test  123
	; Expense/test  123
	Assets
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(123.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-123.0),
					},
				},
				Comments: []string{
					"; Expense/test  123",
				},
			},
		},
		nil,
	},
	{
		"multiple comments",
		`; comment
	1970/01/01 Payee
	Expense/test   58
	Assets         -58           ; comment in trans
	Expense/unbalanced
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(58),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-58),
						Comment: "; comment in trans",
					},
					{
						Name:    "Expense/unbalanced",
						Balance: decimal.NewFromFloat(0),
					},
				},
				Comments: []string{
					"; comment",
				},
			},
		},
		nil,
	},
	{
		"empty account comment",
		`; comment
	1970/01/01 Payee
	Expense/test   58
	Assets                   ; comment in trans
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(58),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-58),
						Comment: "; comment in trans",
					},
				},
				Comments: []string{
					"; comment",
				},
			},
		},
		nil,
	},
	{
		"header comment",
		`; comment
	1970/01/01 Payee
	Expense/test   58
	Assets         -58
	Expense/test   158
	Assets         -158
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(58),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-58),
					},
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(158),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-158),
					},
				},
				Comments: []string{
					"; comment",
				},
			},
		},
		nil,
	},
	{
		"account skip",
		`1970/01/01 Payee
	Expense/test  123
	Assets

account Expense/test

account Assets
	note bambam
	payee junkjunk

1970/01/01 Payee
	Expense/test  (123 * 2)
	Assets
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(123.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-123.0),
					},
				},
			},
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(246.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-246.0),
					},
				},
			},
		},
		nil,
	},
	{
		"multiple account skip",
		`1970/01/01 Payee
	Expense/test  123
	Assets

account Banking
account Expense/test
account Assets

1970/01/01 Payee
	Expense/test  (123 * 2)
	Assets
`,
		[]*Transaction{
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(123.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-123.0),
					},
				},
			},
			{
				Payee: "Payee",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:    "Expense/test",
						Balance: decimal.NewFromFloat(246.0),
					},
					{
						Name:    "Assets",
						Balance: decimal.NewFromFloat(-246.0),
					},
				},
			},
		},
		nil,
	},
	{
		"conversion factor",
		`1970/01/01 Converted CZK to EUR
    Assets:Wise:CZK                                                   -2000.00 @ 0.5
    Assets:Wise:EUR                                                    1000.00
`,
		[]*Transaction{
			{
				Payee: "Converted CZK to EUR",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:             "Assets:Wise:CZK",
						Balance:          decimal.NewFromFloat(-2000.0),
						ConversionFactor: p(decimal.NewFromFloat(0.5)),
					},
					{
						Name:    "Assets:Wise:EUR",
						Balance: decimal.NewFromFloat(1000.0),
					},
				},
			},
		},
		nil,
	},
	{
		"conversion",
		`1970/01/01 Converted CZK to EUR
    Assets:Wise:CZK                                                   -2000.00 @@ 1000.00
    Assets:Wise:EUR                                                    1000.00
`,
		[]*Transaction{
			{
				Payee: "Converted CZK to EUR",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
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
		},
		nil,
	},
	{
		"conversion implicit rate",
		`1970/01/01 Converted CZK to EUR
    Assets:Wise:CZK                                                   CZK -2000.00
    Assets:Wise:EUR                                                   EUR  1000.00
`,
		[]*Transaction{
			{
				Payee: "Converted CZK to EUR",
				Date:  time.Unix(0, 0).UTC(),
				AccountChanges: []Account{
					{
						Name:     "Assets:Wise:CZK",
						Currency: "CZK",
						Balance:  decimal.NewFromFloat(-2000.0),
					},
					{
						Name:              "Assets:Wise:EUR",
						Currency:          "EUR",
						Balance:           decimal.NewFromFloat(1000.0),
						Converted:         p(decimal.NewFromFloat(2000.0)),
						ConvertedCurrency: "CZK",
					},
				},
			},
		},
		nil,
	},
	{
		"conversion implicit rate USD",
		`; test comment
1970/01/01 Wise Charges for: BALANCE
    assets:wise                                                EUR         -8
    expenses:bank:fees                                         EUR          8

; test comment
1970/01/01 Converted EUR to USD
    assets:wise                                                EUR      -1000
    assets:wise                                                USD       2060
`,
		[]*Transaction{
			{
				Payee:    "Wise Charges for: BALANCE",
				Date:     time.Unix(0, 0).UTC(),
				Comments: []string{"; test comment"},
				AccountChanges: []Account{
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
			{
				Payee:    "Converted EUR to USD",
				Date:     time.Unix(0, 0).UTC(),
				Comments: []string{"; test comment"},
				AccountChanges: []Account{
					{
						Name:     "assets:wise",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(-1000.0),
					},
					{
						Name:              "assets:wise",
						Currency:          "USD",
						Balance:           decimal.NewFromFloat(2060.0),
						Converted:         p(decimal.NewFromFloat(1000)),
						ConvertedCurrency: "EUR",
					},
				},
			},
		},
		nil,
	},
	{
		"balance assertions",
		`; test comment
2025-01-01 opening balances
    assets:capital_one                    USD 400.00 = USD 400.00
    assets:ibkr                           EUR 600.00 = EUR 600.00
	equity:opening/closing balances       EUR -600.00
    equity:opening/closing balances       USD -400.00
`,
		[]*Transaction{
			{
				Payee:    "opening balances",
				Date:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Comments: []string{"; test comment"},
				AccountChanges: []Account{
					{
						Name:          "assets:capital_one",
						Currency:      "USD",
						Balance:       decimal.NewFromFloat(400.0),
						BalanceAssert: p(decimal.NewFromFloat(400.0)),
					},
					{
						Name:          "assets:ibkr",
						Currency:      "EUR",
						Balance:       decimal.NewFromFloat(600.0),
						BalanceAssert: p(decimal.NewFromFloat(600.0)),
					},
					{
						Name:     "equity:opening/closing balances",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(-600.0),
					},
					{
						Name:     "equity:opening/closing balances",
						Currency: "USD",
						Balance:  decimal.NewFromFloat(-400.0),
					},
				},
			},
		},
		nil,
	},
	{
		"balance assertions",
		`; test comment
2025-01-01 opening balances
    assets:capital_one                    USD 400.00 = USD 400.00
    assets:ibkr                           EUR 600.00 = EUR 500.00
	equity:opening/closing balances       EUR -600.00
    equity:opening/closing balances       USD -400.00
`,
		[]*Transaction{
			{
				Payee:    "opening balances",
				Date:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Comments: []string{"; test comment"},
				AccountChanges: []Account{
					{
						Name:          "assets:capital_one",
						Currency:      "USD",
						Balance:       decimal.NewFromFloat(400.0),
						BalanceAssert: p(decimal.NewFromFloat(400.0)),
					},
					{
						Name:          "assets:ibkr",
						Currency:      "EUR",
						Balance:       decimal.NewFromFloat(600.0),
						BalanceAssert: p(decimal.NewFromFloat(500.0)),
					},
					{
						Name:     "equity:opening/closing balances",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(-600.0),
					},
					{
						Name:     "equity:opening/closing balances",
						Currency: "USD",
						Balance:  decimal.NewFromFloat(-400.0),
					},
				},
			},
		},
		nil,
	},
	{
		"multi-currency card transaction with @@ conversions",
		`;CARD-377
2026/05/09 Card transaction of 1,042.80 EUR issued
    assets:wise                                                       USD -32.61 @@ EUR 27.59
    assets:wise                                                 EUR -7.39
    assets:wise                                                    CZK -24612.18 @@ EUR 1007.82
    expenses:supplies                                                EUR 1042.80
`,
		[]*Transaction{
			{
				Payee:    "Card transaction of 1,042.80 EUR issued",
				Date:     time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC),
				Comments: []string{";CARD-377"},
				AccountChanges: []Account{
					{
						Name:              "assets:wise",
						Currency:          "USD",
						Balance:           decimal.NewFromFloat(-32.61),
						Converted:         p(decimal.NewFromFloat(27.59)),
						ConvertedCurrency: "EUR",
					},
					{
						Name:     "assets:wise",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(-7.39),
					},
					{
						Name:              "assets:wise",
						Currency:          "CZK",
						Balance:           decimal.NewFromFloat(-24612.18),
						Converted:         p(decimal.NewFromFloat(1007.82)),
						ConvertedCurrency: "EUR",
					},
					{
						Name:     "expenses:supplies",
						Currency: "EUR",
						Balance:  decimal.NewFromFloat(1042.80),
					},
				},
			},
		},
		nil,
	},
	{
		"transfer fee with balance assertion no currency",
		`;FEE-TRANSFER-2
2026/05/15 Wise Charges for: TRANSFER-21
    assets:wise                                           CZK -95.30 = 155.24
    expenses:bank                                                      CZK 95.30
`,
		[]*Transaction{
			{
				Payee:    "Wise Charges for: TRANSFER-21",
				Date:     time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC),
				Comments: []string{";FEE-TRANSFER-2"},
				AccountChanges: []Account{
					{
						Name:          "assets:wise",
						Currency:      "CZK",
						Balance:       decimal.NewFromFloat(-95.30),
						BalanceAssert: p(decimal.NewFromFloat(155.24)),
					},
					{
						Name:     "expenses:bank",
						Currency: "CZK",
						Balance:  decimal.NewFromFloat(95.30),
					},
				},
			},
		},
		nil,
	},
}

func p(d decimal.Decimal) *decimal.Decimal {
	return &d
}

func TestParseLedger(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := bytes.NewBufferString(tc.data)
			transactions, err := ParseLedger("", b)
			if (err != nil && tc.err == nil) || (err != nil && tc.err != nil && err.Error() != tc.err.Error()) {
				t.Errorf("Error: expected `%s`, got `%s`", tc.err, err)
			}
			exp, _ := json.Marshal(tc.transactions)
			got, _ := json.Marshal(transactions)
			if string(exp) != string(got) {
				t.Errorf("Error(%s): expected \n`%s`, \ngot \n`%s`", tc.name, exp, got)
			}
		})
	}
}

func TestCheckBalanceAssertions(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "matching assertions pass",
			data: `; test comment
2025-01-01 opening balances
    assets:capital_one                    USD 400.00 = USD 400.00
    assets:ibkr                           EUR 600.00 = EUR 600.00
    equity:opening/closing balances       EUR -600.00
    equity:opening/closing balances       USD -400.00
`,
		},
		{
			name: "wrong assertion fails",
			data: `; test comment
2025-01-01 opening balances
    assets:capital_one                    USD 400.00 = USD 400.00
    assets:ibkr                           EUR 600.00 = EUR 500.00
    equity:opening/closing balances       EUR -600.00
    equity:opening/closing balances       USD -400.00
`,
			wantErr: true,
		},
		{
			name: "no assertions always passes",
			data: `1970/01/01 Payee
    Expense  100.00
    Assets  -100.00
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs, err := ParseLedger("", bytes.NewBufferString(tt.data))
			if err != nil {
				t.Fatalf("ParseLedger() error = %v", err)
			}
			err = CheckBalanceAssertions(txs)
			if tt.wantErr {
				if !errors.Is(err, ErrBalanceAssertionFailed) {
					t.Errorf("CheckBalanceAssertions() error = %v, want ErrBalanceAssertionFailed", err)
				}
			} else if err != nil {
				t.Errorf("CheckBalanceAssertions() unexpected error = %v", err)
			}
		})
	}
}

func TestEncoderLedger(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err != nil {
				t.Skip("skipping error case")
			}

			// Parse original input
			transactions, err := ParseLedger("", bytes.NewBufferString(tc.data))
			if err != nil {
				t.Fatalf("initial parse failed: %v", err)
			}

			// Encode parsed transactions
			var buf strings.Builder
			enc := NewEncoder(&buf, 80, strings.Repeat(" ", 80))
			if err := enc.Encode(transactions); err != nil {
				t.Fatalf("encode failed: %v", err)
			}

			// Re-parse encoded output
			reparsed, err := ParseLedger("", strings.NewReader(buf.String()))
			if err != nil {
				t.Fatalf("re-parse failed: %v\nencoded:\n%s", err, buf.String())
			}

			exp, _ := json.Marshal(transactions)
			got, _ := json.Marshal(reparsed)
			if string(exp) != string(got) {
				t.Errorf("round-trip mismatch:\nencoded:\n%s\nexpected:\n%s\ngot:\n%s", buf.String(), exp, got)
			}
		})
	}
}

// TestEncoderLongPostingRoundTrip guards against the encoder collapsing the
// account/amount separator to a single space when a posting's name plus amount
// is wider than the column budget. The parser requires at least two spaces (or
// a tab) before an amount, so a single-space separator makes the amount
// unparseable: parsePosting fails, the amount is silently dropped, and the
// posting becomes "empty" — surfacing later as a confusing "more than one
// account empty" balance error rather than a parse error.
func TestEncoderLongPostingRoundTrip(t *testing.T) {
	// A long account name combined with an @@ conversion amount exceeds the
	// 80-column budget, forcing the separator down to one space.
	data := `2026/05/09 Card transaction
    assets:wise:czech:operating:long:account:name:here                CZK -24612.18 @@ EUR 1007.82
    expenses:supplies                                                 EUR 1007.82
`
	transactions, err := ParseLedger("", bytes.NewBufferString(data))
	if err != nil {
		t.Fatalf("initial parse failed: %v", err)
	}

	var buf strings.Builder
	if err := NewEncoder(&buf, 80, strings.Repeat(" ", 80)).Encode(transactions); err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	reparsed, err := ParseLedger("", strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("re-parse of encoded output failed: %v\nencoded:\n%s", err, buf.String())
	}

	exp, _ := json.Marshal(transactions)
	got, _ := json.Marshal(reparsed)
	if string(exp) != string(got) {
		t.Errorf("round-trip mismatch:\nencoded:\n%s\nexpected:\n%s\ngot:\n%s", buf.String(), exp, got)
	}
}

func BenchmarkParseLedger(b *testing.B) {
	for b.Loop() {
		_, _ = ParseLedgerFile("testdata/ledgerBench.dat")
	}
}

func TestAccount_parsePosting(t *testing.T) {
	tests := []struct {
		name        string
		trimmedLine string
		want        Account
		wantErr     bool
	}{
		{
			"simple",
			"Expense  123",
			Account{Name: "Expense", Balance: decimal.NewFromFloat(123.0)},
			false,
		},
		{
			"empty",
			"Expense",
			Account{Name: "Expense", Balance: decimal.NewFromFloat(0.0)},
			false,
		},
		{
			"spaces",
			"Expense:Cranks Unlimited	10",
			Account{Name: "Expense:Cranks Unlimited", Balance: decimal.NewFromFloat(10.0)},
			false,
		},
		{
			"multiply",
			"Expense  (123*2)",
			Account{Name: "Expense", Balance: decimal.NewFromFloat(246.0)},
			false,
		},
		{
			"slash",
			"Expense/test   158",
			Account{Name: "Expense/test", Balance: decimal.NewFromFloat(158.0)},
			false,
		},
		{
			"negative",
			"Expense/test   -158",
			Account{Name: "Expense/test", Balance: decimal.NewFromFloat(-158.0)},
			false,
		},
		{
			"math",
			"Expense:Bank of:Money  (123*2+3)",
			Account{Name: "Expense:Bank of:Money", Balance: decimal.NewFromFloat(249.0)},
			false,
		},
		{
			"math with spaces",
			"Expense/test  (123 * 3)",
			Account{Name: "Expense/test", Balance: decimal.NewFromFloat(123 * 3)},
			false,
		},
		{
			"converted",
			"Expense/test   158 @@ 200",
			Account{Name: "Expense/test", Balance: decimal.NewFromFloat(158.0), Converted: p(decimal.NewFromFloat(200.0))},
			false,
		},
		{
			"conversion",
			"Expense/test   100 @ 2",
			Account{Name: "Expense/test", Currency: "", Balance: decimal.NewFromFloat(100.0), ConversionFactor: p(decimal.NewFromFloat(2.0))},
			false,
		},
		{
			"conversion heirarchy",
			"Assets:Wise:CZK                                                   -2000.00 @ 0.5",
			Account{Name: "Assets:Wise:CZK", Balance: decimal.NewFromFloat(-2000.0), ConversionFactor: p(decimal.NewFromFloat(0.5))},
			false,
		},
		{
			"negative",
			"Expense/test   EUR -158",
			Account{Name: "Expense/test", Currency: "EUR", Balance: decimal.NewFromFloat(-158.0)},
			false,
		},
		{
			"math",
			"Expense:Bank of:Money  USD  (123*2+3)",
			Account{Name: "Expense:Bank of:Money", Currency: "USD", Balance: decimal.NewFromFloat(249.0)},
			false,
		},
		{
			"math with spaces",
			"Expense/test    CZK  (123 * 3)",
			Account{Name: "Expense/test", Currency: "CZK", Balance: decimal.NewFromFloat(123 * 3)},
			false,
		},
		{
			"converted",
			"Expense/test   USD 158 @@ 200",
			Account{Name: "Expense/test", Currency: "USD", Balance: decimal.NewFromFloat(158.0), Converted: p(decimal.NewFromFloat(200.0))},
			false,
		},
		{
			"conversion",
			"Expense/test   $ 100 @ 2",
			Account{Name: "Expense/test", Currency: "$", Balance: decimal.NewFromFloat(100.0), ConversionFactor: p(decimal.NewFromFloat(2.0))},
			false,
		},
		{
			"conversion rate with currency",
			"Assets:Wise:CZK   CZK -298732.17 @ USD 0.04781",
			Account{Name: "Assets:Wise:CZK", Currency: "CZK", Balance: decimal.NewFromFloat(-298732.17), ConversionFactor: p(decimal.RequireFromString("0.04781")), ConversionFactorCurrency: "USD"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := Account{}
			gotErr := a.parsePosting(tt.trimmedLine, "")
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("parsePosting() failed: %v", gotErr)
				}
				return
			}
			aJson, _ := json.Marshal(a)
			wantJson, _ := json.Marshal(tt.want)
			if string(aJson) != string(wantJson) {
				t.Errorf("got %+v wanted %+v", string(aJson), string(wantJson))
			}
			if tt.wantErr {
				t.Fatal("parsePosting() succeeded unexpectedly")
			}
		})
	}
}
