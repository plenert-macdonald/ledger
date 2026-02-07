package ledger

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alfredxing/calc/compute"
	"github.com/howeyc/ledger/decimal"
	date "github.com/joyt/godate"
)

var (
	ErrNoMoreBlocks     = errors.New("no more blocks")
	ErrEmptyBlock       = errors.New("block is empty")
	ErrEmptyLineInBlock = errors.New("line in block is empty")
	ErrTooFewPostings   = errors.New("need at least two postings")
)

type result struct {
	*Transaction
	error
}

// ParseLedgerFile parses a ledger file and returns a list of Transactions.
func ParseLedgerFile(filename string) (generalLedger []*Transaction, err error) {
	ledgerReader, ierr := os.Open(filename)
	if ierr != nil {
		return nil, ierr
	}
	defer ledgerReader.Close()

	results := make(chan result, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go parseLedger(filename, ledgerReader, results, ctx)

	for result := range results {
		if result.error != nil {
			return nil, result.error
		}
		generalLedger = append(generalLedger, result.Transaction)
	}

	return
}

// ParseLedger parses a ledger file and returns a list of Transactions.
func ParseLedger(ledgerReader io.Reader) (generalLedger []*Transaction, err error) {
	results := make(chan result, 100)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go parseLedger("", ledgerReader, results, ctx)

	for result := range results {
		if result.error != nil {
			return nil, result.error
		}
		generalLedger = append(generalLedger, result.Transaction)
	}

	return
}

// ParseLedgerAsync parses a ledger file and returns a Transaction and error channels .
func ParseLedgerAsync(ledgerReader io.Reader) (c chan *Transaction, e chan error) {
	c = make(chan *Transaction)
	e = make(chan error)

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer close(c)
		defer close(e)
		results := make(chan result, 100)

		go parseLedger("", ledgerReader, results, ctx)

		for result := range results {
			if result.error != nil {
				e <- result.error
			}
			if result.Transaction != nil {
				c <- result.Transaction
			}
		}
	}()
	return c, e
}

type parser struct {
	filename string
	scanner  *bufio.Scanner
	line     int
}

func newParser(ledgerReader io.Reader, filename string) parser {
	var lp parser
	lp.init()
	lp.scanner = bufio.NewScanner(ledgerReader)
	lp.scanner.Split(bufio.ScanLines)
	lp.filename = filename

	return lp
}

func (p *parser) init() {
	p.line = 0
}

func (lp *parser) scan() bool {
	success := lp.scanner.Scan()
	lp.line += 1
	return success
}

type block struct {
	lineNum     int
	filename    string
	body        []string
	headingLine int
}

func (lp *parser) nextBlock() (*block, error) {
	b := &block{filename: lp.filename, body: []string{}}

	for lp.scan() {
		line := strings.TrimSpace(lp.scanner.Text())

		if len(line) > 0 {
			b.body = append(b.body, line)
		} else if len(b.body) > 0 {
			b.lineNum = lp.line - len(b.body)
			return b, nil
		}
	}

	if len(b.body) > 0 {
		//
		b.lineNum = lp.line - len(b.body)
		return b, nil
	}
	return nil, ErrNoMoreBlocks
}

func (b *block) header() (string, string, string, error) {
	var comment string
	for i, line := range b.body {
		if line[0] == ';' {
			continue
		}
		if commentIdx := strings.Index(line, ";"); commentIdx >= 0 {
			comment = line[commentIdx:]
			line = strings.TrimSpace(line[:commentIdx])
		}
		before, after, split := strings.Cut(line, " ")
		if !split {
			return "", "", "", fmt.Errorf(
				"%s:%d: unable to parse transaction: %w", b.filename, b.lineNum+i,
				fmt.Errorf("unable to parse payee line: %s", line),
			)
		}
		b.headingLine = i
		return before, after, comment, nil
	}
	return "", "", "", ErrEmptyBlock
}

func (b block) transaction(dateString, payeeString, payeeComment string) (*Transaction, error) {
	trans := &Transaction{}
	transDate, _, err := date.ParseAndGetLayout(dateString)
	if err != nil {
		err = fmt.Errorf("unable to parse date(%s): %w", dateString, err)
		return nil, err
	}
	trans.Date = transDate

	transBal := decimal.Zero
	var numEmpty int

	comments := []string{}

	emptyAccIndex := 0
	for i, line := range b.body {
		if i < b.headingLine {
			if commentIdx := strings.Index(line, ";"); commentIdx >= 0 {
				comments = append(comments, line[commentIdx:])
			}
			continue
		} else if i == b.headingLine {
			continue
		}

		// handle comments
		postingComment := ""
		if commentIdx := strings.Index(line, ";"); commentIdx >= 0 {
			postingComment = line[commentIdx:]
			line = strings.TrimSpace(line[:commentIdx])
		}

		if len(line) == 0 {
			comments = append(comments, postingComment)
			continue
		}

		posting, err := parsePosting(line)
		posting.Comment = postingComment
		if err != nil {
			return nil, err
		}
		trans.AccountChanges = append(trans.AccountChanges, *posting)

		if posting.Balance.IsZero() {
			numEmpty++
			emptyAccIndex = len(trans.AccountChanges) - 1
		}

		if posting.Converted != nil {
			transBal = transBal.Add(posting.Converted.Neg())
		} else if posting.ConversionFactor != nil {
			transBal = transBal.Add(posting.Balance.Mul(
				*posting.ConversionFactor,
			))
		} else {
			transBal = transBal.Add(posting.Balance)
		}
	}

	if len(trans.AccountChanges) < 2 {
		return nil, ErrTooFewPostings
	}

	if !transBal.IsZero() {
		switch numEmpty {
		case 0:
			return nil, errors.New("unable to balance transaction: no empty account to place extra balance")
		case 1:
			// If there is a single empty account, then it is obvious where to
			// place the remaining balance.
			trans.AccountChanges[emptyAccIndex].Balance = transBal.Neg()
		default:
			return nil, errors.New("unable to balance transaction: more than one account empty")
		}
	}

	trans.Payee = payeeString
	trans.Date = transDate
	trans.PayeeComment = payeeComment
	if len(comments) > 0 {
		trans.Comments = comments
	}

	return trans, nil
}

func parseLedger(filename string, ledgerReader io.Reader, results chan result, ctx context.Context) {
	defer close(results)

	cblock := make(chan *block, 3)
	go func() {
		defer close(cblock)

		lp := newParser(ledgerReader, filename)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				b, err := lp.nextBlock()
				if err != nil {
					if err != ErrNoMoreBlocks {
						results <- result{error: err}
					}
					return
				}
				cblock <- b
			}
		}
	}()

	for b := range cblock {
		before, after, comment, err := b.header()
		if err != nil {
			results <- result{error: err}
			return
		}

		switch before {
		case "account":
			// Do nothing
		case "include":
			paths, _ := filepath.Glob(filepath.Join(filepath.Dir(filename), after))
			if len(paths) < 1 {
				results <- result{
					nil,
					fmt.Errorf("%s:%d: unable to include file(%s): %w", filename, b.lineNum, after, errors.New("not found")),
				}
				return
			}
			for _, incpath := range paths {
				ifile, _ := os.Open(incpath)
				defer ifile.Close()

				incresults := make(chan result, 1000)
				go parseLedger(incpath, ifile, incresults, ctx)
				for result := range incresults {
					results <- result
				}
			}
		default:
			trans, transErr := b.transaction(before, after, comment)
			if transErr != nil {
				results <- result{
					nil,
					fmt.Errorf("%s:%d: unable to parse transaction: %w", filename, b.lineNum, transErr),
				}
				continue
			}

			select {
			case <-ctx.Done():
				return
			default:
				results <- result{
					Transaction: trans,
				}
			}
		}
	}
}

func parsePosting(trimmedLine string) (*Account, error) {
	a := &Account{}
	trimmedLine = strings.TrimSpace(trimmedLine)

	// Regex groups:
	// 1: account name
	// 2: amount (number or parenthesized expression)
	// 3: @@ converted amount
	// 4: @ conversion rate
	re := regexp.MustCompile(
		`^(.+?)(?:(?:\s{2,}|\t)([\-]?\d+(?:\.\d+)?|\([0-9+\-*\/. ]+\))(?:\s*(?:@@\s*([\-]?\d+(?:\.\d+)?)|@\s*([\-]?\d+(?:\.\d+)?)))?)?\s*$`,
	)

	m := re.FindStringSubmatch(trimmedLine)
	if m == nil {
		return nil, fmt.Errorf("invalid posting: %q", trimmedLine)
	}

	a.Name = m[1]
	if m[2] != "" {
		bal, err := compute.Evaluate(m[2])
		if err != nil {
			return nil, err
		}
		a.Balance = decimal.NewFromFloat(bal)
	}

	// @@ explicit converted amount
	if m[3] != "" {
		conv, err := decimal.NewFromString(m[3])
		if err != nil {
			return nil, err
		}
		a.Converted = &conv
	}

	// @ rate-based conversion
	if m[4] != "" {
		rate, err := decimal.NewFromString(m[4])
		if err != nil {
			return nil, err
		}
		a.ConversionFactor = &rate
	}
	return a, nil
}
