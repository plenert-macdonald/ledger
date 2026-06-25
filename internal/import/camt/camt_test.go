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

const camtExchangeSample = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.10">
  <BkToCstmrStmt><Stmt>
    <Acct><Ccy>USD</Ccy></Acct>
    <Ntry>
      <Amt Ccy="USD">14283.61</Amt>
      <CdtDbtInd>CRDT</CdtDbtInd>
      <BookgDt><DtTm>2026-05-09T10:00:00+02:00</DtTm></BookgDt>
      <BkTxCd><Prtry><Cd>BALANCE-5308275565</Cd></Prtry></BkTxCd>
      <AmtDtls><TxAmt>
        <Amt Ccy="CZK">298732.17</Amt>
        <CcyXchg>
          <SrcCcy>CZK</SrcCcy>
          <TrgtCcy>USD</TrgtCcy>
          <UnitCcy>CZK</UnitCcy>
          <XchgRate>0.04781</XchgRate>
        </CcyXchg>
      </TxAmt></AmtDtls>
      <AddtlNtryInf>Converted 300,000.00 CZK to 14,283.61 USD (fee: 1,267.83 CZK)</AddtlNtryInf>
    </Ntry>
  </Stmt></BkToCstmrStmt>
</Document>`

func TestParseCamtExchange(t *testing.T) {
	stmt, err := camt.ParseCamt(bytes.NewBufferString(camtExchangeSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(stmt.Ntry) != 1 {
		t.Fatalf("got %d entries, want 1", len(stmt.Ntry))
	}
	n := stmt.Ntry[0]

	x, ok := n.Exchange()
	if !ok {
		t.Fatal("Exchange() = nil, want exchange details")
	}
	if x.SrcCcy != "CZK" || x.TrgtCcy != "USD" || x.UnitCcy != "CZK" || x.XchgRate != "0.04781" {
		t.Errorf("CcyXchg = %+v", x)
	}

	orig, ok := n.OriginalAmount()
	if !ok || orig.Ccy != "CZK" || orig.Value != "298732.17" {
		t.Errorf("OriginalAmount() = %+v, ok=%v", orig, ok)
	}
}

func TestNtryNoExchange(t *testing.T) {
	stmt, err := camt.ParseCamt(bytes.NewBuffer(camtSample))
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range stmt.Ntry {
		if _, ok := n.Exchange(); ok {
			t.Errorf("entry %d unexpectedly reports exchange details", i)
		}
	}
}

func TestCcyXchgPrice(t *testing.T) {
	const tol = 1e-9
	approx := func(got decimal.Decimal, want float64) bool {
		w := decimal.NewFromFloat(want)
		return got.Sub(w).Abs().LessThan(decimal.NewFromFloat(tol))
	}

	tests := []struct {
		name     string
		xchg     camt.CcyXchg
		from, to string
		want     float64
		wantOK   bool
	}{
		{
			name: "unit is source: src->trgt is the rate",
			xchg: camt.CcyXchg{SrcCcy: "CZK", TrgtCcy: "USD", UnitCcy: "CZK", XchgRate: "0.04781"},
			from: "CZK", to: "USD", want: 0.04781, wantOK: true,
		},
		{
			name: "unit is source: trgt->src inverts the rate",
			xchg: camt.CcyXchg{SrcCcy: "CZK", TrgtCcy: "USD", UnitCcy: "CZK", XchgRate: "0.04781"},
			from: "USD", to: "CZK", want: 1.0 / 0.04781, wantOK: true,
		},
		{
			name: "unit is target: src->trgt inverts the rate",
			xchg: camt.CcyXchg{SrcCcy: "CZK", TrgtCcy: "USD", UnitCcy: "USD", XchgRate: "20.9162"},
			from: "CZK", to: "USD", want: 1.0 / 20.9162, wantOK: true,
		},
		{
			name: "unknown currency pair",
			xchg: camt.CcyXchg{SrcCcy: "CZK", TrgtCcy: "USD", UnitCcy: "CZK", XchgRate: "0.04781"},
			from: "CZK", to: "EUR", wantOK: false,
		},
		{
			name: "unparseable rate",
			xchg: camt.CcyXchg{SrcCcy: "CZK", TrgtCcy: "USD", UnitCcy: "CZK", XchgRate: "x"},
			from: "CZK", to: "USD", wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.xchg.Price(tt.from, tt.to)
			if ok != tt.wantOK {
				t.Fatalf("Price() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !approx(got, tt.want) {
				t.Errorf("Price(%s,%s) = %s, want ~%v", tt.from, tt.to, got, tt.want)
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
