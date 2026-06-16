package ledger

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/expr-lang/expr"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

// ParseLedgerFile parses a ledger file and returns a list of Transactions.
func ParseLedgerFile(filename string) (generalLedger []*Transaction, err error) {
	ifile, ierr := os.Open(filename)
	if ierr != nil {
		return nil, ierr
	}
	defer ifile.Close()
	return ParseLedger(filename, ifile)
}

// ParseLedger parses a ledger file and returns a list of Transactions.
func ParseLedger(name string, ledgerReader io.Reader) (generalLedger []*Transaction, err error) {
	blocks, err := parseBlocks(name, ledgerReader)
	if err != nil {
		return nil, err
	}

	transactions, err := lo.MapErr(blocks, func(b block, _ int) (*Transaction, error) {
		trans, transErr := b.parseTransaction()
		if transErr != nil {
			return nil, fmt.Errorf("%s:%d: unable to parse transaction: %w", b.filename, b.lineNum, transErr)
		}
		return trans, nil
	})
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

// CheckBalanceAssertions validates any balance assertions embedded in
// transactions. It walks the slice in order, maintains a running balance per
// (account, currency) pair, and returns ErrBalanceAssertionFailed if any
// posting's BalanceAssert does not match the computed running balance.
//
// This is intentionally separate from ParseLedger so callers that work with
// individual statement fragments (whose absolute assertions require prior-
// period context) can defer validation to after all fragments are combined.
func CheckBalanceAssertions(transactions []*Transaction) error {
	type acctKey struct{ name, currency string }
	running := make(map[acctKey]decimal.Decimal)

	for _, trans := range transactions {
		for _, acc := range trans.AccountChanges {
			key := acctKey{acc.Name, acc.Currency}
			running[key] = running[key].Add(acc.Balance)

			if acc.BalanceAssert != nil && !running[key].Equal(*acc.BalanceAssert) {
				return ErrBalanceAssertionFailed
			}
		}
	}
	return nil
}

type parser struct {
	scanner   *bufio.Scanner
	filename  string
	lineCount int

	comments []string
}

func (lp *parser) Scan() bool {
	return lp.scanner.Scan()
}

func (lp *parser) Text() string {
	var line string
	line = lp.scanner.Text()
	lp.lineCount++
	return line
}

func (lp *parser) LineNumber() int {
	return lp.lineCount
}

func (lp *parser) Name() string {
	return lp.filename
}

func parseBlocks(filename string, ledgerReader io.Reader) ([]block, error) {
	lp := parser{
		scanner:  bufio.NewScanner(ledgerReader),
		filename: filename,
	}

	blocks := []block{}
	comments := []string{}
	for lp.Scan() {
		// remove heading and tailing space from the line
		trimmedLine := strings.TrimSpace(lp.Text())

		var currentComment string
		// handle comments
		if commentIdx := strings.Index(trimmedLine, ";"); commentIdx >= 0 {
			currentComment = trimmedLine[commentIdx:]
			trimmedLine = trimmedLine[:commentIdx]
			trimmedLine = strings.TrimSpace(trimmedLine)
		}

		// Skip empty lines
		if len(trimmedLine) == 0 {
			if len(currentComment) > 0 {
				comments = append(comments, currentComment)
			}
			continue
		}

		before, after, split := strings.Cut(trimmedLine, " ")
		if !split {
			return nil, fmt.Errorf(
				"%s:%d: unable to parse transaction: %w",
				lp.Name(),
				lp.LineNumber(),
				fmt.Errorf("unable to parse payee line: %s", trimmedLine),
			)
		}
		switch before {
		case "account":
			lp.skipAccount()
		case "include":
			paths, _ := filepath.Glob(filepath.Join(filepath.Dir(lp.Name()), after))
			if len(paths) < 1 {
				return nil, fmt.Errorf(
					"%s:%d: unable to include file(%s): %w", lp.Name(), lp.LineNumber(), after, errors.New("not found"))
			}

			b, err := lo.FlatMapErr(paths, func(path string, _ int) ([]block, error) {
				f, _ := os.Open(path)
				defer f.Close()
				return parseBlocks(path, f)
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, b...)
		default:
			transDate, derr := dateparse.ParseAny(before)
			if derr != nil {
				return nil, fmt.Errorf("%s:%d: unable to parse transaction: %w", lp.Name(), lp.LineNumber(), derr)
			}

			blocks = append(blocks, lp.parseBlock(transDate, after, currentComment, comments))
			comments = []string{}
		}
	}

	return blocks, nil
}

func (lp *parser) skipAccount() {
	for lp.Scan() {
		// Read until blank line (ignore all sub-directives)
		if len(lp.Text()) == 0 {
			return
		}
	}
}

func (a *Account) parsePosting(trimmedLine string, comment string) (err error) {
	trimmedLine = strings.TrimSpace(trimmedLine)

	// Regex groups:
	// 1: account name
	// 2: currency
	// 3: amount (number or parenthesized expression)
	// 4: @@ converted amount
	// 5: @ conversion rate
	// 6: balance assertion amount
	re := regexp.MustCompile(
		`^(?P<name>.+?)` +
			`(?:(?:\s{2,}|\t)` +
			`(?:(?P<currency>[A-Z\$]+)\s+)?` +
			`(?P<amount>[\-]?\d+(?:\.\d+)?|\([0-9+\-*\/. ]+\))` +
			`(?:\s*(?:@@\s*` +
			`(?P<converted>[\-]?\d+(?:\.\d+)?)|@\s*` +
			`(?P<factor>[\-]?\d+(?:\.\d+)?)))?` +
			`(?:\s*=\s*(?:[A-Z\$]+\s+)?(?P<assert>[\-]?\d+(?:\.\d+)?))?)?\s*$`,
	)

	m := re.FindStringSubmatch(trimmedLine)
	if m == nil {
		return fmt.Errorf("invalid posting: %q", trimmedLine)
	}

	a.Name = m[1]
	a.Currency = m[2]
	a.Comment = comment

	if m[3] != "" {
		program, err := expr.Compile(m[3])
		if err != nil {
			return err
		}
		out, err := expr.Run(program, nil)
		if err != nil {
			return err
		}

		var f float64
		switch v := out.(type) {
		case int:
			f = float64(v)
		case int64:
			f = float64(v)
		case float32:
			f = float64(v)
		case float64:
			f = v
		default:
			return fmt.Errorf("expression did not evaluate to a number: %T", out)
		}

		a.Balance = decimal.NewFromFloat(f)
	}

	// @@ explicit converted amount
	if m[4] != "" {
		conv, err := decimal.NewFromString(m[4])
		if err != nil {
			return err
		}
		a.Converted = &conv
	}

	// @ rate-based conversion
	if m[5] != "" {
		rate, err := decimal.NewFromString(m[5])
		if err != nil {
			return err
		}
		a.ConversionFactor = &rate
	}

	// = balance assertion
	if m[6] != "" {
		assert, err := decimal.NewFromString(m[6])
		if err != nil {
			return err
		}
		a.BalanceAssert = &assert
	}
	return
}

type block struct {
	transDate    time.Time
	payeeString  string
	payeeComment string
	comments     []string
	lines        []string
	filename     string
	lineNum      int
}

func (lp *parser) parseBlock(transDate time.Time, payeeString, payeeComment string, comments []string) block {
	lines := []string{}
	for lp.Scan() {
		trimmedLine := lp.Text()
		lines = append(lines, trimmedLine)
		if len(trimmedLine) == 0 {
			break
		}
	}

	return block{
		transDate:    transDate,
		payeeString:  payeeString,
		payeeComment: payeeComment,
		comments:     comments,
		lines:        lines,
		filename:     lp.Name(),
		lineNum:      lp.LineNumber(),
	}
}

func (b *block) parseTransaction() (trans *Transaction, err error) {
	trans = &Transaction{}
	for _, trimmedLine := range b.lines {
		postingComment := ""
		// handle comments
		if commentIdx := strings.Index(trimmedLine, ";"); commentIdx >= 0 {
			currentComment := trimmedLine[commentIdx:]
			trimmedLine = trimmedLine[:commentIdx]
			trimmedLine = strings.TrimSpace(trimmedLine)
			if len(trimmedLine) == 0 {
				b.comments = append(b.comments, currentComment)
				continue
			}
			postingComment = currentComment
		}

		if len(trimmedLine) == 0 {
			break
		}

		posting := Account{}
		posting.parsePosting(trimmedLine, postingComment)
		trans.AccountChanges = append(trans.AccountChanges, posting)
	}

	trans.Payee = b.payeeString
	trans.Date = b.transDate
	trans.PayeeComment = b.payeeComment
	if len(b.comments) > 0 {
		trans.Comments = b.comments
	}

	if err = trans.IsBalanced(); err != nil {
		return nil, err
	}

	return
}
