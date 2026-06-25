package camt

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/shopspring/decimal"
)

// XML structures for CAMT.053 format
type Document struct {
	XMLName       xml.Name      `xml:"Document"`
	BkToCstmrStmt BkToCstmrStmt `xml:"BkToCstmrStmt"`
}

type BkToCstmrStmt struct {
	Stmt Stmt `xml:"Stmt"`
}

type Stmt struct {
	Acct Acct   `xml:"Acct"`
	Bal  []Bal  `xml:"Bal"`
	Ntry []Ntry `xml:"Ntry"`
}

// Bal is a balance reported for the statement, such as the opening
// ("OPBD") or closing ("CLBD") booked balance.
type Bal struct {
	Tp        BalTp  `xml:"Tp"`
	Amt       Amount `xml:"Amt"`
	CdtDbtInd string `xml:"CdtDbtInd"`
}

type BalTp struct {
	CdOrPrtry CdOrPrtry `xml:"CdOrPrtry"`
}

type CdOrPrtry struct {
	Cd string `xml:"Cd"`
}

// SignedAmount returns b's amount, negated if b is a debit balance
// (CdtDbtInd "DBIT").
func (b Bal) SignedAmount() (decimal.Decimal, error) {
	amt, err := decimal.NewFromString(b.Amt.Value)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parsing balance amount %q: %w", b.Amt.Value, err)
	}
	if b.CdtDbtInd == "DBIT" {
		amt = amt.Neg()
	}
	return amt, nil
}

// Balance returns the Bal in s.Bal whose type code (Tp.CdOrPrtry.Cd) is
// code, e.g. "OPBD" for the opening booked balance or "CLBD" for the
// closing booked balance, and whether one was found.
func (s Stmt) Balance(code string) (Bal, bool) {
	for _, b := range s.Bal {
		if b.Tp.CdOrPrtry.Cd == code {
			return b, true
		}
	}
	return Bal{}, false
}

type Acct struct {
	Id   Id     `xml:"Id"`
	Ccy  string `xml:"Ccy"`
	Ownr Ownr   `xml:"Ownr"`
}

type Id struct {
	IBAN string `xml:"IBAN"`
}

type Ownr struct {
	Nm string `xml:"Nm"`
}

type Ntry struct {
	Amt          Amount    `xml:"Amt"`
	CdtDbtInd    string    `xml:"CdtDbtInd"`
	BookgDt      BookgDt   `xml:"BookgDt"`
	BkTxCd       BkTxCd    `xml:"BkTxCd"`
	NtryRef      string    `xml:"NtryRef"`
	AddtlNtryInf string    `xml:"AddtlNtryInf"`
	AmtDtls      *AmtDtls  `xml:"AmtDtls"`
	NtryDtls     *NtryDtls `xml:"NtryDtls"`
}

// AmtDtls holds the breakdown of an entry's amount, including the original
// (pre-conversion) transaction amount and any currency-exchange details.
type AmtDtls struct {
	TxAmt TxAmt `xml:"TxAmt"`
}

// TxAmt is the transaction amount. For a currency conversion its Amt is the
// original (source-currency) amount and CcyXchg describes the rate used to
// convert it to the entry's booked currency.
type TxAmt struct {
	Amt     Amount   `xml:"Amt"`
	CcyXchg *CcyXchg `xml:"CcyXchg"`
}

// CcyXchg is an ISO 20022 currency exchange: an amount in SrcCcy is converted
// to TrgtCcy at XchgRate, where the rate is expressed per one unit of UnitCcy
// (so "1 UnitCcy = XchgRate <other currency>").
type CcyXchg struct {
	SrcCcy   string `xml:"SrcCcy"`
	TrgtCcy  string `xml:"TrgtCcy"`
	UnitCcy  string `xml:"UnitCcy"`
	XchgRate string `xml:"XchgRate"`
}

// Exchange returns the entry's currency-exchange details, if present.
func (n Ntry) Exchange() (*CcyXchg, bool) {
	if n.AmtDtls != nil && n.AmtDtls.TxAmt.CcyXchg != nil {
		return n.AmtDtls.TxAmt.CcyXchg, true
	}
	return nil, false
}

// OriginalAmount returns the pre-conversion transaction amount
// (AmtDtls/TxAmt/Amt) — e.g. the source-currency amount of a conversion — and
// whether it is present.
func (n Ntry) OriginalAmount() (Amount, bool) {
	if n.AmtDtls != nil && n.AmtDtls.TxAmt.Amt.Value != "" {
		return n.AmtDtls.TxAmt.Amt, true
	}
	return Amount{}, false
}

// Price returns the value of one unit of currency from expressed in currency
// to, derived from the exchange rate. ok is false if the rate cannot be parsed
// or from/to are not the currency pair this exchange describes.
func (x CcyXchg) Price(from, to string) (decimal.Decimal, bool) {
	rate, err := decimal.NewFromString(x.XchgRate)
	if err != nil || rate.IsZero() {
		return decimal.Decimal{}, false
	}
	one := decimal.NewFromInt(1)

	// Normalise the rate to "1 SrcCcy = srcInTrgt TrgtCcy".
	var srcInTrgt decimal.Decimal
	switch x.UnitCcy {
	case x.SrcCcy:
		srcInTrgt = rate
	case x.TrgtCcy:
		srcInTrgt = one.Div(rate)
	default:
		return decimal.Decimal{}, false
	}

	switch {
	case from == x.SrcCcy && to == x.TrgtCcy:
		return srcInTrgt, true
	case from == x.TrgtCcy && to == x.SrcCcy:
		return one.Div(srcInTrgt), true
	default:
		return decimal.Decimal{}, false
	}
}

type Amount struct {
	Value string `xml:",chardata"`
	Ccy   string `xml:"Ccy,attr"`
}

type BookgDt struct {
	DtTm string `xml:"DtTm"`
}

type BkTxCd struct {
	Prtry Prtry `xml:"Prtry"`
}

type Prtry struct {
	Cd string `xml:"Cd"`
}

type NtryDtls struct {
	TxDtls TxDtls `xml:"TxDtls"`
}

type TxDtls struct {
	RltdPties RltdPties `xml:"RltdPties"`
}

type RltdPties struct {
	Cdtr *Cdtr `xml:"Cdtr"`
}

type Cdtr struct {
	Pty Pty `xml:"Pty"`
}

type Pty struct {
	Nm string `xml:"Nm"`
}

// ParseCamt parses a CAMT.053 statement, returning its account, balances,
// and entries.
func ParseCamt(reader io.Reader) (Stmt, error) {
	var doc Document
	if err := xml.NewDecoder(reader).Decode(&doc); err != nil {
		return Stmt{}, err
	}

	return doc.BkToCstmrStmt.Stmt, nil
}
