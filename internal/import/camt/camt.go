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
	NtryDtls     *NtryDtls `xml:"NtryDtls"`
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
