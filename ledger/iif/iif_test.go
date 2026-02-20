package iif

import (
	"bytes"
	"reflect"
	"testing"

	_ "embed"
)

var (
	//go:embed "Full Deposit.iif"
	fullDepositIIF []byte

	//go:embed "Full Invoice.iif"
	fullInvoiceIIF []byte

	//go:embed "Full Bill payment.iif"
	fullBillPaymentIIF []byte

	//go:embed "Full Sales Tax Payment.iif"
	fullSalesTaxPaymentIIF []byte

	//go:embed "Full Transfer.iif"
	fullTransferIIF []byte
)

func TestDecodeEncode(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		blocks []Block
	}{
		{
			name: "fullDepositIIF",
			data: fullDepositIIF,
			blocks: []Block{
				{
					Header: []Header{
						{Type: RecordType("ACCNT"), Fields: []string{"NAME", "ACCNTTYPE", "DESC", "ACCNUM", "EXTRA"}},
					},
				},
				{
					Header: []Header{
						{Type: RecordType("CLASS"), Fields: []string{"NAME"}},
					},
				},
				{
					Header: []Header{
						{Type: RecordType("CUST"), Fields: []string{"NAME", "BADDR1", "BADDR2", "BADDR3", "BADDR4", "BADDR5", "SADDR1"}},
					},
				},
				{
					Header: []Header{
						{Type: RecordType("OTHERNAME"), Fields: []string{"NAME", "BADDR1", "BADDR2", "BADDR3", "BADDR4", "BADDR5", "PHONE1", "PHONE2", "FAXNUM", "EMAIL", "NOTE", "CONT1", "CONT2", "NOTEPAD", "SALUTATION", "COMPANYNAME", "FIRSTNAME", "MIDINIT", "LASTNAME"}},
					},
				},
				{
					Header: []Header{
						{Type: RecordType("TRNS"), Fields: []string{"TRNSID", "TRNSTYPE", "DATE", "ACCNT", "NAME", "CLASS", "AMOUNT", "DOCNUM", "MEMO", "CLEAR"}},
						{Type: RecordType("SPL"), Fields: []string{"SPLID", "TRNSTYPE", "DATE", "ACCNT", "NAME", "CLASS", "AMOUNT", "DOCNUM", "MEMO", "CLEAR"}},
						{Type: RecordType("ENDTRNS"), Fields: []string{}},
					},
				},
			},
		},
		{
			name: "fullInvoiceIIF",
			data: fullInvoiceIIF,
		},
		{
			name: "fullBillPaymentIIF",
			data: fullBillPaymentIIF,
		},
		{
			name: "fullSalesTaxPaymentIIF",
			data: fullSalesTaxPaymentIIF,
		},
		{
			name: "fullTransferIIF",
			data: fullTransferIIF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewDecoder(bytes.NewReader(tt.data))
			f, err := dec.Decode()
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}

			if len(f.Blocks) == 0 {
				t.Error("missing blocks from file")
			}

			for i, b := range tt.blocks {
				if !reflect.DeepEqual(b.Header, f.Blocks[i].Header) {
					t.Errorf("expected headers to equal %+v != %+v", b.Header, f.Blocks[i].Header)
				}
			}
		})
	}
}
