package ledger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// Entries are in reverse-chronological order in the XML to verify that
// importCamt sorts them chronologically before processing.
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
                <BookgDt><DtTm>2026-01-31T19:24:13+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>TRANSFER-1</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Payment received</AddtlNtryInf>
            </Ntry>
            <Ntry>
                <Amt Ccy="USD">52.00</Amt>
                <CdtDbtInd>DBIT</CdtDbtInd>
                <BookgDt><DtTm>2026-01-23T20:06:02+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-1</Cd></Prtry></BkTxCd>
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
                <BookgDt><DtTm>2026-01-23T20:06:02+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-1</Cd></Prtry></BkTxCd>
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
                <BookgDt><DtTm>2026-01-23T20:06:02+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-1</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Card transaction</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

const camtOverdrawn = `<?xml version="1.0" encoding="UTF-8"?>
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
                <BookgDt><DtTm>2026-01-23T20:06:02+02:00</DtTm></BookgDt>
                <BkTxCd><Prtry><Cd>CARD-1</Cd></Prtry></BkTxCd>
                <AddtlNtryInf>Card transaction</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

func TestImportCamtBalanceAssertion(t *testing.T) {
	type entryWant struct {
		// assertion on AccountChanges[0] (the camtAccount posting), nil = no assertion
		assert *decimal.Decimal
	}

	tests := []struct {
		name    string
		xml     string
		wantLen int
		// entries in expected chronological order
		entries []entryWant
	}{
		{
			// XML has CRDT 1000 (Jan 31) before DBIT 52 (Jan 23); after sorting:
			// [0] Jan 23 DBIT 52  → OPBD(56.29) + (-52) = 4.29
			// [1] Jan 31 CRDT 1000 → 4.29 + 1000 = 1004.29 = CLBD ✓
			name:    "opening and closing balance present",
			xml:     camtWithBalances,
			wantLen: 2,
			entries: []entryWant{
				{assert: decimalPtr("4.29")},
				{assert: decimalPtr("1004.29")},
			},
		},
		{
			name:    "no balances reported — no assertions",
			xml:     camtNoBalances,
			wantLen: 1,
			entries: []entryWant{
				{assert: nil},
			},
		},
		{
			name:    "only closing balance — no assertions",
			xml:     camtOnlyClosingBalance,
			wantLen: 1,
			entries: []entryWant{
				{assert: nil},
			},
		},
		{
			// OPBD +100, DBIT 150 → running = 100-150 = -50 = CLBD (DBIT 50) ✓
			name:    "overdrawn closing balance",
			xml:     camtOverdrawn,
			wantLen: 1,
			entries: []entryWant{
				{assert: decimalPtr("-50.00")},
			},
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
			for i, want := range tt.entries {
				got := imp.ledger[i].AccountChanges[0].BalanceAssert
				if want.assert == nil {
					if got != nil {
						t.Errorf("entries[%d].BalanceAssert = %v, want nil", i, got)
					}
					continue
				}
				if got == nil || !got.Equal(*want.assert) {
					t.Errorf("entries[%d].BalanceAssert = %v, want %v", i, got, want.assert)
				}
			}
		})
	}
}

// A Wise currency-conversion entry: USD 14283.61 booked, converted from
// CZK 298732.17 at 0.04781 USD/CZK.
const camtConversion = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.10">
    <BkToCstmrStmt>
        <Stmt>
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
                <AddtlNtryInf>Converted 300,000.00 CZK to 14,283.61 USD</AddtlNtryInf>
            </Ntry>
        </Stmt>
    </BkToCstmrStmt>
</Document>`

func TestImportCamtConversion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "statement.xml")
	if err := os.WriteFile(path, []byte(camtConversion), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	imp := &Importer{
		MatchingAccount: "assets:wise",
		DecScale:        decimal.NewFromInt(1),
		reader:          f,
	}
	imp.importCamt()

	if len(imp.ledger) != 1 {
		t.Fatalf("len(imp.ledger) = %d, want 1", len(imp.ledger))
	}
	trans := imp.ledger[0]
	if len(trans.AccountChanges) != 2 {
		t.Fatalf("got %d postings, want 2", len(trans.AccountChanges))
	}

	bank := trans.AccountChanges[0]
	if bank.Currency != "USD" || bank.Balance.StringFixed(2) != "14283.61" {
		t.Errorf("bank posting = %s %s, want USD 14283.61", bank.Currency, bank.Balance.StringFixed(2))
	}

	counter := trans.AccountChanges[1]
	if counter.Currency != "CZK" || counter.Balance.StringFixed(2) != "-298732.17" {
		t.Errorf("counter posting = %s %s, want CZK -298732.17", counter.Currency, counter.Balance.StringFixed(2))
	}
	if counter.Converted == nil || counter.Converted.StringFixed(2) != "14283.61" {
		t.Errorf("counter.Converted = %v, want 14283.61", counter.Converted)
	}
	if counter.ConvertedCurrency != "USD" {
		t.Errorf("counter.ConvertedCurrency = %q, want USD", counter.ConvertedCurrency)
	}

	// The @@ price anchors the CZK leg to the booked USD amount, so the
	// transaction balances in USD: +14283.61 and -14283.61.
	var buf bytes.Buffer
	if err := NewEncoder(&buf, 80, "  ").Encode(trans); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "CZK -298732.17 @@ USD 14283.61") {
		t.Errorf("encoded entry missing CZK @@ USD price:\n%s", buf.String())
	}
}

func decimalPtr(s string) *decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return &d
}
