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
					Headers: []Header{
						{Type: RecordType("ACCNT"), Fields: []string{"NAME", "ACCNTTYPE", "DESC", "ACCNUM", "EXTRA"}},
					},
				},
				{
					Headers: []Header{
						{Type: RecordType("CLASS"), Fields: []string{"NAME"}},
					},
				},
				{
					Headers: []Header{
						{Type: RecordType("CUST"), Fields: []string{"NAME", "BADDR1", "BADDR2", "BADDR3", "BADDR4", "BADDR5", "SADDR1"}},
					},
				},
				{
					Headers: []Header{
						{Type: RecordType("OTHERNAME"), Fields: []string{"NAME", "BADDR1", "BADDR2", "BADDR3", "BADDR4", "BADDR5", "PHONE1", "PHONE2", "FAXNUM", "EMAIL", "NOTE", "CONT1", "CONT2", "NOTEPAD", "SALUTATION", "COMPANYNAME", "FIRSTNAME", "MIDINIT", "LASTNAME"}},
					},
				},
				{
					Headers: []Header{
						{Type: RecordType("TRNS"), Fields: []string{"TRNSID", "TRNSTYPE", "DATE", "ACCNT", "NAME", "CLASS", "AMOUNT", "DOCNUM", "MEMO", "CLEAR"}},
						{Type: RecordType("SPL"), Fields: []string{"SPLID", "TRNSTYPE", "DATE", "ACCNT", "NAME", "CLASS", "AMOUNT", "DOCNUM", "MEMO", "CLEAR"}},
						{Type: RecordType("ENDTRNS"), Fields: []string{}},
					},
					Records: [][]Record{
						{
							{
								Type: RecordType("TRNS"),
								Fields: map[string]string{
									"TRNSID":   " ",
									"TRNSTYPE": "DEPOSIT",
									"DATE":     "7/1/1998",
									"ACCNT":    "Checking",
									"NAME":     "",
									"CLASS":    "",
									"AMOUNT":   "10000",
									"DOCNUM":   "",
									"MEMO":     "",
									"CLEAR":    "N",
								},
							},
							{
								Type: RecordType("SPL"),
								Fields: map[string]string{
									"SPLID":    "",
									"TRNSTYPE": "DEPOSIT",
									"DATE":     "7/1/1998",
									"ACCNT":    "Income",
									"NAME":     "Customer",
									"CLASS":    "",
									"AMOUNT":   "-10000",
									"DOCNUM":   "",
									"MEMO":     "",
									"CLEAR":    "N",
								},
							},
							{
								Type:   RecordType("ENDTRNS"),
								Fields: map[string]string{},
							},
						},
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
				if i >= len(f.Blocks) {
					t.Errorf("expected at least %d blocks, got %d", len(tt.blocks), len(f.Blocks))
					break
				}
				if !reflect.DeepEqual(b.Headers, f.Blocks[i].Headers) {
					t.Errorf("expected headers to equal %+v != %+v", b.Headers, f.Blocks[i].Headers)
				}
				if b.Records != nil && !reflect.DeepEqual(b.Records, f.Blocks[i].Records) {
					t.Errorf("expected records to equal %+v != %+v", b.Records, f.Blocks[i].Records)
				}
			}
		})
	}
}
