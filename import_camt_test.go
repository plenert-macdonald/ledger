package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shopspring/decimal"
)

const camtWithBalances = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.10">
    <BkToCstmrStmt>
        <Stmt>
            <Acct>
                <Ccy>USD</Ccy>
            </Acct>
            <Bal>
                <Tp><CdOrPrtry><Cd>CLBD</Cd></CdOrPrtry></Tp>
                <Amt Ccy="USD">1004.29</Amt>
                <CdtDbtInd>CRDT</CdtDbtInd>
            </Bal>
            <Bal>
                <Tp><CdOrPrtry><Cd>OPBD</Cd></CdOrPrtry></Tp>
                <Amt Ccy="USD">56.29</Amt>
                <CdtDbtInd>CRDT</CdtDbtInd>
            </Bal>
            <Ntry>
                <Amt Ccy="USD">1000.00</Amt>
                <CdtDbtInd>CRDT</CdtDbtInd>
                <BookgDt><DtTm>2026-01-31T19:24:13.233587+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>TRANSFER-1948899598</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Additional paid in capital</AddtlNtryInf>
            </Ntry>
            <Ntry>
                <Amt Ccy="USD">52.00</Amt>
                <CdtDbtInd>DBIT</CdtDbtInd>
                <BookgDt><DtTm>2026-01-23T20:06:02.508842+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-3377418782</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Card transaction</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

const camtNoBalances = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.10">
    <BkToCstmrStmt>
        <Stmt>
            <Acct>
                <Ccy>USD</Ccy>
            </Acct>
            <Ntry>
                <Amt Ccy="USD">52.00</Amt>
                <CdtDbtInd>DBIT</CdtDbtInd>
                <BookgDt><DtTm>2026-01-23T20:06:02.508842+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-3377418782</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Card transaction</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

const camtOnlyClosingBalance = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.10">
    <BkToCstmrStmt>
        <Stmt>
            <Acct>
                <Ccy>USD</Ccy>
            </Acct>
            <Bal>
                <Tp><CdOrPrtry><Cd>CLBD</Cd></CdOrPrtry></Tp>
                <Amt Ccy="USD">1004.29</Amt>
                <CdtDbtInd>CRDT</CdtDbtInd>
            </Bal>
            <Ntry>
                <Amt Ccy="USD">52.00</Amt>
                <CdtDbtInd>DBIT</CdtDbtInd>
                <BookgDt><DtTm>2026-01-23T20:06:02.508842+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-3377418782</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Card transaction</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

const camtOverdrawnClosingBalance = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.10">
    <BkToCstmrStmt>
        <Stmt>
            <Acct>
                <Ccy>USD</Ccy>
            </Acct>
            <Bal>
                <Tp><CdOrPrtry><Cd>CLBD</Cd></CdOrPrtry></Tp>
                <Amt Ccy="USD">50.00</Amt>
                <CdtDbtInd>DBIT</CdtDbtInd>
            </Bal>
            <Bal>
                <Tp><CdOrPrtry><Cd>OPBD</Cd></CdOrPrtry></Tp>
                <Amt Ccy="USD">100.00</Amt>
                <CdtDbtInd>CRDT</CdtDbtInd>
            </Bal>
            <Ntry>
                <Amt Ccy="USD">150.00</Amt>
                <CdtDbtInd>DBIT</CdtDbtInd>
                <BookgDt><DtTm>2026-01-23T20:06:02.508842+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-3377418782</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Card transaction</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

func TestImportCamtBalanceAssertion(t *testing.T) {
	tests := []struct {
		name    string
		xml     string
		want    *decimal.Decimal
		wantLen int
	}{
		{
			name:    "opening and closing balance present",
			xml:     camtWithBalances,
			want:    decimalPtr("948.00"),
			wantLen: 2,
		},
		{
			name:    "no balances reported",
			xml:     camtNoBalances,
			want:    nil,
			wantLen: 1,
		},
		{
			name:    "only closing balance reported",
			xml:     camtOnlyClosingBalance,
			want:    nil,
			wantLen: 1,
		},
		{
			name:    "overdrawn closing balance",
			xml:     camtOverdrawnClosingBalance,
			want:    decimalPtr("-150.00"),
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "statement.xml")
			if err := os.WriteFile(path, []byte(tt.xml), 0644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer f.Close()

			imp := &Importer{
				MatchingAccount: "assets:wise",
				DecScale:        decimal.NewFromInt(1),
				reader:          f,
			}
			imp.importCamt()

			if len(imp.ledger) != tt.wantLen {
				t.Fatalf("len(imp.ledger) = %d, want %d", len(imp.ledger), tt.wantLen)
			}
			last := imp.ledger[len(imp.ledger)-1]
			got := last.AccountChanges[0].BalanceAssert
			if tt.want == nil {
				if got != nil {
					t.Errorf("BalanceAssert = %v, want nil", got)
				}
				return
			}
			if got == nil || !got.Equal(*tt.want) {
				t.Errorf("BalanceAssert = %v, want %v", got, tt.want)
			}
		})
	}
}

func decimalPtr(s string) *decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return &d
}
