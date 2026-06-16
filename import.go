package ledger

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/howeyc/ledger/internal/import/camt"
	"github.com/howeyc/ledger/internal/import/iif"
	"github.com/howeyc/ledger/internal/import/qfx"
	"github.com/howeyc/ledger/internal/import/qif"
	"github.com/jbrukh/bayesian"
	"github.com/shopspring/decimal"
)

var (
	ErrNoMatchingAccount = errors.New("Unable to find matching account.")
)

type Importer struct {
	// Public members
	ImportFilename         string
	TrainingLedgerFilePath string
	DecScale               decimal.Decimal
	AccountSubstring       string
	MatchingAccount        string
	OverrideCurrency       string
	FieldDelimiter         string
	CsvDateFormat          string
	AllowMatching          bool
	NegateAmount           bool

	// Private members
	ledger         []*Transaction
	trainingLedger []*Transaction
	reader         *os.File
	classifier     *bayesian.Classifier
}

func (imp *Importer) Import() []*Transaction {
	fileReader, err := os.Open(imp.ImportFilename)
	defer fileReader.Close()
	if err != nil {
		fmt.Println("CSV: ", err)
		return nil
	}
	imp.reader = fileReader

	// If a ledger file path is provided, load it and train the classifier.
	// Otherwise, skip loading and prediction will fall back to "unknown:unknown".
	if imp.TrainingLedgerFilePath != "" {
		var parseError error
		imp.trainingLedger, parseError = ParseLedgerFile(imp.TrainingLedgerFilePath)
		if parseError != nil {
			fmt.Printf("%s:%s\n", imp.TrainingLedgerFilePath, parseError.Error())
			return nil
		}

		matchingAccount, err := imp.findMatchingAccount(imp.AccountSubstring)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		imp.MatchingAccount = matchingAccount

		imp.classifier = imp.trainClassifier(imp.trainingLedger, imp.MatchingAccount)
	} else {
		imp.MatchingAccount = imp.AccountSubstring
	}

	lower := strings.ToLower(imp.ImportFilename)
	switch {
	case strings.HasSuffix(lower, ".xml"):
		imp.importCamt()
	case strings.HasSuffix(lower, ".qfx") || strings.HasSuffix(lower, ".ofx"):
		imp.importQFX()
	case strings.HasSuffix(lower, ".qif"):
		imp.importQIF()
	case strings.HasSuffix(lower, ".iif"):
		imp.importIIF()
	default:
		imp.importCSV()
	}

	return imp.ledger
}

func (imp *Importer) recordTransaction(trans *Transaction) {
	imp.ledger = append(imp.ledger, trans)
}

func (imp *Importer) trainClassifier(trainingLedger []*Transaction, matchingAccount string) *bayesian.Classifier {
	allAccounts := GetBalances(trainingLedger, []string{})
	uniqueAccounts := make(map[string]bool)
	for _, acc := range allAccounts {
		if ok, _ := uniqueAccounts[acc.Name]; !ok {
			uniqueAccounts[acc.Name] = true
		}
	}

	classes := []bayesian.Class{}
	for name := range uniqueAccounts {
		classes = append(classes, bayesian.Class(name))
	}

	classifier := bayesian.NewClassifier(classes...)
	for _, tran := range trainingLedger {
		payeeWords := strings.Fields(tran.Payee)
		// learn accounts names (except matchingAccount) for transactions where matchingAccount is present
		learnName := false
		for _, accChange := range tran.AccountChanges {
			if accChange.Name == matchingAccount {
				learnName = true
				break
			}
		}
		if learnName {
			for _, accChange := range tran.AccountChanges {
				if accChange.Name != matchingAccount {
					classifier.Learn(payeeWords, bayesian.Class(accChange.Name))
				}
			}
		}
	}

	return classifier
}

func (imp *Importer) predictAccount(inputPayeeWords []string) string {
	if imp.classifier == nil {
		return "unknown:unknown"
	}

	// Classify into expense account

	// Find the highest and second highest scores
	highScore1 := math.Inf(-1)
	highScore2 := math.Inf(-1)
	matchIdx := 0
	scores, _, _ := imp.classifier.LogScores(inputPayeeWords)
	for j, score := range scores {
		if score > highScore1 {
			highScore2 = highScore1
			highScore1 = score
			matchIdx = j
		}
	}
	// If the difference between the highest and second highest scores is greater than 10
	// then it indicates that highscore is a high confidence match
	if highScore1-highScore2 > 10 {
		return string(imp.classifier.Classes[matchIdx])
	} else {
		return "unknown:unknown"
	}
}

func (imp *Importer) findMatchingAccount(accountSubstring string) (string, error) {
	var matchingAccount string
	matchingAccounts := GetBalances(imp.trainingLedger, []string{accountSubstring})
	if len(matchingAccounts) < 1 {
		return "", ErrNoMatchingAccount
	}
	for _, m := range matchingAccounts {
		if strings.EqualFold(m.Name, accountSubstring) {
			matchingAccount = m.Name
			break
		}
	}
	if matchingAccount == "" {
		matchingAccount = matchingAccounts[len(matchingAccounts)-1].Name
	}

	return matchingAccount, nil
}

func (imp *Importer) importCSV() {
	csvReader := csv.NewReader(imp.reader)
	csvReader.Comma, _ = utf8.DecodeRuneInString(imp.FieldDelimiter)
	csvRecords, cerr := csvReader.ReadAll()
	if cerr != nil {
		fmt.Println("CSV parse error:", cerr.Error())
		return
	}

	// Find columns from header
	var dateColumn, payeeColumn, amountColumn, commentColumn int
	dateColumn, payeeColumn, amountColumn, commentColumn = -1, -1, -1, -1
	for fieldIndex, fieldName := range csvRecords[0] {
		fieldName = strings.ToLower(fieldName)
		if strings.Contains(fieldName, "date") {
			dateColumn = fieldIndex
		} else if strings.Contains(fieldName, "description") {
			payeeColumn = fieldIndex
		} else if strings.Contains(fieldName, "payee") {
			payeeColumn = fieldIndex
		} else if strings.Contains(fieldName, "amount") {
			amountColumn = fieldIndex
		} else if strings.Contains(fieldName, "expense") {
			amountColumn = fieldIndex
		} else if strings.Contains(fieldName, "note") {
			commentColumn = fieldIndex
		} else if strings.Contains(fieldName, "comment") {
			commentColumn = fieldIndex
		}
	}

	if dateColumn < 0 || payeeColumn < 0 || amountColumn < 0 {
		fmt.Println("Unable to find columns required from header field names.")
		return
	}

	expenseAccount := Account{Name: "unknown:unknown", Balance: decimal.Zero}
	csvAccount := Account{Name: imp.MatchingAccount, Balance: decimal.Zero}
	for _, record := range csvRecords[1:] {
		inputPayeeWords := strings.Fields(record[payeeColumn])
		csvDate, _ := time.Parse(imp.CsvDateFormat, record[dateColumn])
		if imp.AllowMatching || !imp.existingTransaction(csvDate, record[payeeColumn]) {
			expenseAccount.Name = imp.predictAccount(inputPayeeWords)

			// Parse error, set to zero
			if dec, derr := decimal.NewFromString(record[amountColumn]); derr != nil {
				expenseAccount.Balance = decimal.Zero
			} else {
				expenseAccount.Balance = dec
			}

			// Negate amount if required
			if imp.NegateAmount {
				expenseAccount.Balance = expenseAccount.Balance.Neg()
			}

			// Apply scale
			expenseAccount.Balance = expenseAccount.Balance.Mul(imp.DecScale)

			// Csv amount is the negative of the expense amount
			csvAccount.Balance = expenseAccount.Balance.Neg()

			trans := &Transaction{Date: csvDate, Payee: record[payeeColumn]}
			trans.AccountChanges = []Account{csvAccount, expenseAccount}

			if imp.OverrideCurrency != "" {
				for i := range trans.AccountChanges {
					trans.AccountChanges[i].Currency = imp.OverrideCurrency
				}
			}
			if commentColumn >= 0 && record[commentColumn] != "" {
				trans.Comments = []string{";" + record[commentColumn]}
			}
			imp.recordTransaction(trans)
		}
	}
}

func (imp *Importer) importCamt() {
	stmt, err := camt.ParseCamt(imp.reader)
	if err != nil {
		fmt.Println("CAMT parse error:", err.Error())
		return
	}

	expenseAccount := Account{Name: "unknown:unknown", Balance: decimal.Zero}
	camtAccount := Account{Name: imp.MatchingAccount, Balance: decimal.Zero}
	var lastTrans *Transaction
	for _, entry := range stmt.Ntry {
		dateTime, err := time.Parse(time.RFC3339, entry.BookgDt.DtTm)
		if err != nil {
			// Try another format if RFC3339 fails
			dateTime, err = time.Parse("2006-01-02T15:04:05.999999-07:00", entry.BookgDt.DtTm)
			if err != nil {
				fmt.Println("CAMT parse error:", err.Error())
			}
		}

		// Parse amount
		amount, err := decimal.NewFromString(entry.Amt.Value)
		if err != nil {
			fmt.Println("CAMT parse error:", err.Error())
		}

		// Get reference and payee
		reference := entry.BkTxCd.Prtry.Cd
		payee := ""

		// Extract payee from entry details if available
		if entry.NtryDtls != nil && entry.NtryDtls.TxDtls.RltdPties.Cdtr != nil {
			payee = entry.NtryDtls.TxDtls.RltdPties.Cdtr.Pty.Nm
		} else {
			// Use additional entry info as fallback
			payee = entry.AddtlNtryInf
		}
		inputPayeeWords := strings.Fields(payee)

		expenseAccount.Name = imp.predictAccount(inputPayeeWords)
		expenseAccount.Balance = amount

		// Determine if debit
		isDebit := entry.CdtDbtInd == "DBIT"
		if !isDebit {
			expenseAccount.Balance = expenseAccount.Balance.Neg()
		}

		// Apply scale
		expenseAccount.Balance = expenseAccount.Balance.Mul(imp.DecScale)

		// Csv amount is the negative of the expense amount
		camtAccount.Balance = expenseAccount.Balance.Neg()

		trans := &Transaction{Date: dateTime, Payee: payee}
		trans.AccountChanges = []Account{camtAccount, expenseAccount}
		if imp.OverrideCurrency != "" {
			for i := range trans.AccountChanges {
				trans.AccountChanges[i].Currency = imp.OverrideCurrency
			}
		} else if entry.Amt.Ccy != "" {
			for i := range trans.AccountChanges {
				trans.AccountChanges[i].Currency = entry.Amt.Ccy
			}
		}
		if reference != "" {
			trans.Comments = []string{";" + reference}
		}
		imp.recordTransaction(trans)
		lastTrans = trans
	}

	// If the statement reports both an opening and closing booked balance,
	// assert that imp.MatchingAccount's balance changes by their difference
	// over the entries above, catching entries that failed to parse or were
	// missing from the statement.
	if lastTrans != nil {
		if opening, ok := stmt.Balance("OPBD"); ok {
			if closing, ok := stmt.Balance("CLBD"); ok {
				openAmt, openErr := opening.SignedAmount()
				closeAmt, closeErr := closing.SignedAmount()
				if openErr == nil && closeErr == nil {
					assertion := closeAmt.Sub(openAmt).Mul(imp.DecScale)
					lastTrans.AccountChanges[0].BalanceAssert = &assertion
				}
			}
		}
	}
}

func (imp *Importer) importQIF() {
	entries, err := qif.ParseQIF(imp.reader)
	if err != nil {
		fmt.Println("QIF parse error:", err.Error())
		return
	}

	expenseAccount := Account{Name: "unknown:unknown", Balance: decimal.Zero}
	qifAccount := Account{Name: imp.MatchingAccount, Balance: decimal.Zero}
	for _, entry := range entries {
		// Parse date (QIF dates are often locale-specific; assume mm/dd/yyyy here)
		dateTime, err := time.Parse("01/02/2006", entry.Date)
		if err != nil {
			// Try an alternate common QIF date format (dd/mm/yyyy)
			dateTime, err = time.Parse("02/01/2006", entry.Date)
			if err != nil {
				fmt.Println("QIF date parse error:", err.Error())
				continue
			}
		}

		// Parse amount
		amount, err := decimal.NewFromString(entry.Amount)
		if err != nil {
			fmt.Println("QIF amount parse error:", err.Error())
			continue
		}

		payee := entry.Payee
		inputPayeeWords := strings.Fields(payee)

		expenseAccount.Name = imp.predictAccount(inputPayeeWords)
		expenseAccount.Balance = amount

		// Apply scale
		expenseAccount.Balance = expenseAccount.Balance.Mul(imp.DecScale)

		// Account side is the opposite of expense
		qifAccount.Balance = expenseAccount.Balance.Neg()

		trans := &Transaction{Date: dateTime, Payee: payee}
		trans.AccountChanges = []Account{qifAccount, expenseAccount}
		if imp.OverrideCurrency != "" {
			for i := range trans.AccountChanges {
				trans.AccountChanges[i].Currency = imp.OverrideCurrency
			}
		}
		if len(entry.RawLines) > 0 {
			// Join all raw lines except header/type line
			comment := strings.Join(entry.RawLines, " ")
			trans.Comments = []string{";" + comment}
		}
		imp.recordTransaction(trans)
	}
}

func (imp *Importer) importIIF() {
	f, err := iif.NewDecoder(imp.reader).Decode()
	if err != nil {
		log.Fatal(err)
		return
	}

	tx := []iif.Transaction{}
	for _, b := range f.Blocks {
		tr, err := iif.DeserializeTransactions(b)
		if err != nil {
			log.Fatal(err)
			return
		}
		tx = append(tx, tr...)
	}

	for _, itx := range tx {
		trans := &Transaction{
			Date:  itx.Tr.Date,
			Payee: itx.Tr.Class + " " + itx.Tr.Memo,
		}
		trans.AccountChanges = []Account{
			{
				Name:    itx.Tr.Account,
				Balance: itx.Tr.Amount,
			},
		}

		for _, split := range itx.Splits {
			trans.AccountChanges = append(
				trans.AccountChanges,
				Account{
					Name:    split.Account,
					Balance: split.Amount,
				},
			)
		}

		if imp.OverrideCurrency != "" {
			for i := range trans.AccountChanges {
				trans.AccountChanges[i].Currency = imp.OverrideCurrency
			}
		}
		imp.recordTransaction(trans)
	}

}

func (imp *Importer) importQFX() {
	entries, err := qfx.ParseQFX(imp.reader)
	if err != nil {
		fmt.Println("QFX parse error:", err.Error())
		return
	}

	expenseAccount := Account{Name: "unknown:unknown", Balance: decimal.Zero}
	qfxAccount := Account{Name: imp.MatchingAccount, Balance: decimal.Zero}
	for _, entry := range entries {
		// QFX DTPOSTED is typically YYYYMMDDHHMMSS.XXX; we only care about the date.
		// Take the first 8 characters as YYYYMMDD.
		dateStr := entry.DtPosted
		if len(dateStr) >= 8 {
			dateStr = dateStr[:8]
		}
		dateTime, err := time.Parse("20060102", dateStr)
		if err != nil {
			fmt.Println("QFX date parse error:", err.Error())
			continue
		}

		// Parse amount
		amount, err := decimal.NewFromString(entry.TrnAmt)
		if err != nil {
			fmt.Println("QFX amount parse error:", err.Error())
			continue
		}

		payee := entry.Memo
		inputPayeeWords := strings.Fields(payee)

		expenseAccount.Name = imp.predictAccount(inputPayeeWords)
		expenseAccount.Balance = amount

		// Apply scale
		expenseAccount.Balance = expenseAccount.Balance.Mul(imp.DecScale)

		// Account side is the opposite of expense
		qfxAccount.Balance = expenseAccount.Balance.Neg()

		trans := &Transaction{Date: dateTime, Payee: payee}
		trans.AccountChanges = []Account{qfxAccount, expenseAccount}
		if imp.OverrideCurrency != "" {
			for i := range trans.AccountChanges {
				trans.AccountChanges[i].Currency = imp.OverrideCurrency
			}
		}
		if entry.FitID != "" {
			trans.Comments = []string{";" + entry.FitID}
		}
		imp.recordTransaction(trans)
	}
}

func (imp *Importer) existingTransaction(transDate time.Time, payee string) bool {
	for _, trans := range imp.trainingLedger {
		if trans.Date.Equal(transDate) && strings.TrimSpace(trans.Payee) == strings.TrimSpace(payee) {
			return true
		}
	}
	return false
}
