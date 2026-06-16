package camt_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/howeyc/ledger/internal/import/camt"
	"github.com/shopspring/decimal"
)

//go:embed sample.xml
var camtSample []byte

func TestParseCamt(t *testing.T) {
	stmt, err := camt.ParseCamt(bytes.NewBuffer(camtSample))
	if err != nil {
		t.Error(err)
	}
	if len(stmt.Ntry) != 2 {
		t.Error("Expected 2 got ", len(stmt.Ntry))
	}
}

func TestStmtBalance(t *testing.T) {
	stmt, err := camt.ParseCamt(bytes.NewBuffer(camtSample))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		code    string
		wantOK  bool
		wantAmt string
		wantInd string
	}{
		{name: "closing booked balance", code: "CLBD", wantOK: true, wantAmt: "67.71", wantInd: "CRDT"},
		{name: "opening booked balance", code: "OPBD", wantOK: true, wantAmt: "306.61", wantInd: "CRDT"},
		{name: "unknown code", code: "NOPE", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bal, ok := stmt.Balance(tt.code)
			if ok != tt.wantOK {
				t.Fatalf("Balance(%q) ok = %v, want %v", tt.code, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if bal.Amt.Value != tt.wantAmt {
				t.Errorf("Balance(%q).Amt.Value = %q, want %q", tt.code, bal.Amt.Value, tt.wantAmt)
			}
			if bal.CdtDbtInd != tt.wantInd {
				t.Errorf("Balance(%q).CdtDbtInd = %q, want %q", tt.code, bal.CdtDbtInd, tt.wantInd)
			}
		})
	}
}

func TestBalSignedAmount(t *testing.T) {
	tests := []struct {
		name    string
		bal     camt.Bal
		want    string
		wantErr bool
	}{
		{
			name: "credit balance is positive",
			bal:  camt.Bal{Amt: camt.Amount{Value: "67.71"}, CdtDbtInd: "CRDT"},
			want: "67.71",
		},
		{
			name: "debit balance is negative",
			bal:  camt.Bal{Amt: camt.Amount{Value: "67.71"}, CdtDbtInd: "DBIT"},
			want: "-67.71",
		},
		{
			name:    "invalid amount",
			bal:     camt.Bal{Amt: camt.Amount{Value: "not-a-number"}, CdtDbtInd: "CRDT"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.bal.SignedAmount()
			if tt.wantErr {
				if err == nil {
					t.Fatal("SignedAmount() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("SignedAmount() error = %v", err)
			}
			want, _ := decimal.NewFromString(tt.want)
			if !got.Equal(want) {
				t.Errorf("SignedAmount() = %v, want %v", got, want)
			}
		})
	}
}
