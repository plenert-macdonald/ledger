package ledger

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	transactionDateFormat = "2006/01/02"
	newLine               = "\n"
)

type Encoder struct {
	w       io.StringWriter
	columns int
	space   string
}

func NewEncoder(w io.StringWriter, columns int, space string) *Encoder {
	return &Encoder{w, columns, space}
}

func (e *Encoder) Encode(v any) error {
	switch t := v.(type) {
	case []*Transaction:
		for _, trans := range t {
			if err := e.encodeTransaction(trans); err != nil {
				return err
			}
		}
	case *Transaction:
		return e.encodeTransaction(t)
	}

	return fmt.Errorf("unsupported type: %T", v)
}

func (e *Encoder) encodeTransaction(trans *Transaction) error {
	spaceStr := e.space
	columns := e.columns
	w := e.w

	if len(spaceStr) < columns {
		spaceStr = strings.Repeat(" ", columns)
	}

	for _, c := range trans.Comments {
		w.WriteString(c)
		w.WriteString(newLine)
	}

	// Print accounts sorted by name
	slices.SortFunc(trans.AccountChanges, func(a, b Account) int {
		return strings.Compare(a.Name, b.Name)
	})

	w.WriteString(trans.Date.Format(transactionDateFormat))
	w.WriteString(spaceStr[:1])
	w.WriteString(trans.Payee)
	if len(trans.PayeeComment) > 0 {
		spaceCount := columns - 10 - utf8.RuneCountInString(trans.Payee)
		if spaceCount < 1 {
			spaceCount = 1
		}
		w.WriteString(spaceStr[:spaceCount])
		w.WriteString(trans.PayeeComment)
	}
	w.WriteString(newLine)
	for _, accChange := range trans.AccountChanges {
		outBalanceString := accChange.Balance.StringFixedBank(2)
		if accChange.Currency != "" {
			outBalanceString = accChange.Currency + " " + outBalanceString
		}
		// Show converted amount (@@) or conversion factor (@) similar to hledger
		if accChange.Converted != nil {
			outBalanceString = outBalanceString + " @@ " + accChange.Converted.StringFixedBank(2)
		} else if accChange.ConversionFactor != nil {
			outBalanceString = outBalanceString + " @ " + accChange.ConversionFactor.String()
		}
		spaceCount := columns - 4 - utf8.RuneCountInString(accChange.Name) - utf8.RuneCountInString(outBalanceString)
		if spaceCount < 1 {
			spaceCount = 1
		}
		w.WriteString(spaceStr[:4])
		w.WriteString(accChange.Name)
		w.WriteString(spaceStr[:spaceCount])
		w.WriteString(outBalanceString)
		if len(accChange.Comment) > 0 {
			w.WriteString(spaceStr[:1])
			w.WriteString(accChange.Comment)
		}
		w.WriteString(newLine)
	}
	w.WriteString(newLine)

	return nil
}
